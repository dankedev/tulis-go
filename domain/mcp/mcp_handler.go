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
	svc          Service
	sseClientsMu sync.RWMutex
	sseClients   map[string]chan string
}

func NewHandler(svc Service) *Handler {
	return &Handler{
		svc:        svc,
		sseClients: make(map[string]chan string),
	}
}

// resolveContext extracts workspace_id and user_id from Fiber context
func (h *Handler) resolveContext(c *fiber.Ctx) (uuid.UUID, uuid.UUID) {
	var workspaceID uuid.UUID
	var userID uuid.UUID

	if wsHeader := c.Get("X-Workspace-ID"); wsHeader != "" {
		if u, err := uuid.Parse(wsHeader); err == nil {
			workspaceID = u
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
	return c.JSON(resp)
}

// HandleSSE handles SSE connection setup (GET /mcp or GET /mcp/sse) for clients using SSE transport
func (h *Handler) HandleSSE(c *fiber.Ctx) error {
	sessionID := uuid.New().String()
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	msgChan := make(chan string, 16)
	h.sseClientsMu.Lock()
	h.sseClients[sessionID] = msgChan
	h.sseClientsMu.Unlock()

	defer func() {
		h.sseClientsMu.Lock()
		delete(h.sseClients, sessionID)
		close(msgChan)
		h.sseClientsMu.Unlock()
	}()

	endpointURL := fmt.Sprintf("/mcp/messages?sessionId=%s", sessionID)
	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		// Send initial endpoint event per MCP SSE spec
		fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
		w.Flush()

		// Stream messages or keep alive
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
	respBytes, _ := json.Marshal(resp)

	h.sseClientsMu.RLock()
	clientChan, exists := h.sseClients[sessionID]
	h.sseClientsMu.RUnlock()

	if exists {
		clientChan <- string(respBytes)
	}

	return c.SendStatus(fiber.StatusAccepted)
}
