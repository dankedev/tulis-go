package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

type Handler struct {
	svc               Service
	sseClientsMu      sync.RWMutex
	sseClients        map[string]chan string
	sessionWorkspaces map[string]uuid.UUID
}

func NewHandler(svc Service) *Handler {
	return &Handler{
		svc:               svc,
		sseClients:        make(map[string]chan string),
		sessionWorkspaces: make(map[string]uuid.UUID),
	}
}

// resolveContext extracts workspace_id and user_id from Fiber context
func (h *Handler) resolveContext(c *fiber.Ctx) (uuid.UUID, uuid.UUID) {
	var workspaceID uuid.UUID
	var userID uuid.UUID

	// Check if session has a switched workspace context
	sessionID := c.Query("sessionId")
	if sessionID != "" {
		h.sseClientsMu.RLock()
		if wsID, exists := h.sessionWorkspaces[sessionID]; exists && wsID != uuid.Nil {
			workspaceID = wsID
		}
		h.sseClientsMu.RUnlock()
	}

	if workspaceID == uuid.Nil {
		if wsHeader := c.Get("X-Workspace-ID"); wsHeader != "" {
			if u, err := uuid.Parse(wsHeader); err == nil {
				workspaceID = u
			}
		}
	}

	if workspaceID == uuid.Nil {
		if wsLoc := c.Locals("workspace_id"); wsLoc != nil {
			if u, err := uuid.Parse(fmt.Sprint(wsLoc)); err == nil {
				workspaceID = u
			}
		}
	}

	if userLoc := c.Locals("user_id"); userLoc != nil {
		if u, err := uuid.Parse(fmt.Sprint(userLoc)); err == nil {
			userID = u
		}
	}

	return workspaceID, userID
}

// HandlePost handles Streamable HTTP JSON-RPC POST requests (/mcp or /api/mcp)
func (h *Handler) HandlePost(c *fiber.Ctx) error {
	workspaceID, userID := h.resolveContext(c)

	var req JSONRPCRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32700, Message: "Parse error: " + err.Error()},
		})
	}

	resp := h.svc.HandleRequest(c.Context(), req, workspaceID, userID)

	// JSON-RPC 2.0: Notifications (id == nil) MUST NOT return a response object
	if req.ID == nil {
		return c.SendStatus(fiber.StatusAccepted)
	}

	return c.JSON(resp)
}

// HandleSSE handles SSE connection setup (GET /mcp or GET /mcp/sse) for clients using SSE transport
func (h *Handler) HandleSSE(c *fiber.Ctx) error {
	sessionID := uuid.New().String()
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	msgChan := make(chan string, 32)
	h.sseClientsMu.Lock()
	h.sseClients[sessionID] = msgChan
	h.sseClientsMu.Unlock()

	endpointPath := "/mcp/messages"
	if len(c.Path()) >= 8 && c.Path()[:8] == "/api/mcp" {
		endpointPath = "/api/mcp/messages"
	}
	endpointURL := fmt.Sprintf("%s?sessionId=%s", endpointPath, sessionID)

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		// Clean up only when this stream writer exits
		defer func() {
			h.sseClientsMu.Lock()
			delete(h.sseClients, sessionID)
			delete(h.sessionWorkspaces, sessionID)
			close(msgChan)
			h.sseClientsMu.Unlock()
		}()

		// Send initial endpoint event per MCP SSE spec
		fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
		if err := w.Flush(); err != nil {
			return
		}

		// Stream messages or keep alive ping every 15s
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-msgChan:
				if !ok {
					return
				}
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				fmt.Fprintf(w, ": ping\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	}))

	return nil
}

// HandleSSEMessages handles messages sent by the client during an SSE session (POST /mcp/messages)
func (h *Handler) HandleSSEMessages(c *fiber.Ctx) error {
	sessionID := c.Query("sessionId")
	if sessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "sessionId query parameter is required",
		})
	}

	workspaceID, userID := h.resolveContext(c)

	var req JSONRPCRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32700, Message: "Parse error: " + err.Error()},
		})
	}

	resp := h.svc.HandleRequest(c.Context(), req, workspaceID, userID)

	// If the request was a switch workspace tool call, track it for this session
	if req.Method == "tools/call" || req.Method == "tulis_switch_workspace" {
		var params ToolCallParams
		_ = json.Unmarshal(req.Params, &params)
		targetMethod := params.Name
		targetArgs := params.Arguments
		if req.Method == "tulis_switch_workspace" {
			targetMethod = "tulis_switch_workspace"
			targetArgs = req.Params
		}
		if targetMethod == "tulis_switch_workspace" {
			var args map[string]any
			_ = json.Unmarshal(targetArgs, &args)
			if wsStr, ok := args["workspace_id"].(string); ok {
				if parsedWS, err := uuid.Parse(wsStr); err == nil {
					h.sseClientsMu.Lock()
					h.sessionWorkspaces[sessionID] = parsedWS
					h.sseClientsMu.Unlock()
				}
			}
		}
	}

	// JSON-RPC 2.0: Notifications (id == nil) MUST NOT receive a response
	if req.ID != nil {
		respBytes, _ := json.Marshal(resp)

		h.sseClientsMu.RLock()
		clientChan, exists := h.sseClients[sessionID]
		h.sseClientsMu.RUnlock()

		if exists {
			select {
			case clientChan <- string(respBytes):
			default:
				// If channel buffer is full, drop or log without deadlock
			}
		}
	}

	return c.SendStatus(fiber.StatusAccepted)
}
