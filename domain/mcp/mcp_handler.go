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

type sseSession struct {
	ch     chan string
	done   chan struct{}
	keyID  string // API key that opened the stream; only it may post messages to the session
	closed sync.Once
}

func (ss *sseSession) close() { ss.closed.Do(func() { close(ss.done) }) }

const (
	sessionHeader   = "Mcp-Session-Id"
	httpSessionTTL  = 24 * time.Hour
	maxHTTPSessions = 10000
)

type httpSession struct {
	keyID   string
	created time.Time
}

type Handler struct {
	svc          Service
	sseClientsMu sync.RWMutex
	sseClients   map[string]*sseSession

	httpSessionsMu sync.Mutex
	httpSessions   map[string]httpSession // Streamable HTTP sessions (Mcp-Session-Id)
}

func NewHandler(svc Service) *Handler {
	return &Handler{
		svc:          svc,
		sseClients:   make(map[string]*sseSession),
		httpSessions: make(map[string]httpSession),
	}
}

// authenticate requires a request authenticated by an API key (set by middleware.ApiKeyAuth).
// The workspace is always the key's own workspace; headers and query parameters are ignored.
func (h *Handler) authenticate(c *fiber.Ctx) (Principal, string, bool) {
	if c.Locals("auth_method") != "api_key" {
		return Principal{}, "", false
	}
	wsID, err := uuid.Parse(fmt.Sprint(c.Locals("workspace_id")))
	if err != nil || wsID == uuid.Nil {
		return Principal{}, "", false
	}
	scopes, _ := c.Locals("api_key_scopes").(string)
	keyID := fmt.Sprint(c.Locals("api_key_id"))
	p := Principal{WorkspaceID: wsID, Scopes: scopes}
	if uid, err := uuid.Parse(fmt.Sprint(c.Locals("api_key_user_id"))); err == nil {
		p.UserID = uid
	}
	return p, keyID, true
}

func unauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &RPCError{Code: -32000, Message: "Authentication required: provide a valid API key via X-API-Key or Authorization: Bearer"},
	})
}

// HandlePost handles Streamable HTTP JSON-RPC POST requests (/mcp or /api/mcp)
func (h *Handler) HandlePost(c *fiber.Ctx) error {
	principal, keyID, ok := h.authenticate(c)
	if !ok {
		return unauthorized(c)
	}

	var req JSONRPCRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32700, Message: "Parse error: " + err.Error()},
		})
	}

	// Sessions are optional (stateless clients keep working), but a session id that is sent must
	// be live and belong to this key; per the spec an unknown one is 404 so clients re-initialize.
	if sid := c.Get(sessionHeader); sid != "" && req.Method != "initialize" && !h.validHTTPSession(sid, keyID) {
		return c.Status(fiber.StatusNotFound).JSON(JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32001, Message: "Session not found: send a new initialize request"},
		})
	}

	resp := h.svc.HandleRequest(WithPrincipal(c.Context(), principal), req, principal.WorkspaceID, uuid.Nil)

	if req.Method == "initialize" && resp.Error == nil {
		c.Set(sessionHeader, h.newHTTPSession(keyID))
	}

	// JSON-RPC 2.0: Notifications (id == nil) MUST NOT return a response object
	if req.ID == nil {
		return c.SendStatus(fiber.StatusAccepted)
	}

	return c.JSON(resp)
}

// HandleSSE handles SSE connection setup (GET /mcp or GET /mcp/sse) for clients using SSE transport
func (h *Handler) HandleSSE(c *fiber.Ctx) error {
	_, keyID, ok := h.authenticate(c)
	if !ok {
		return unauthorized(c)
	}
	sessionID := uuid.New().String()
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	session := &sseSession{ch: make(chan string, 32), done: make(chan struct{}), keyID: keyID}
	h.sseClientsMu.Lock()
	h.sseClients[sessionID] = session
	h.sseClientsMu.Unlock()

	endpointPath := "/mcp/messages"
	if len(c.Path()) >= 8 && c.Path()[:8] == "/api/mcp" {
		endpointPath = "/api/mcp/messages"
	}
	endpointURL := fmt.Sprintf("%s?sessionId=%s", endpointPath, sessionID)

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		// Clean up only when this stream writer exits
		defer func() {
			// Never close ch: senders select on done instead, so they cannot panic.
			session.close()
			h.sseClientsMu.Lock()
			delete(h.sseClients, sessionID)
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
			case <-session.done:
				return
			case msg := <-session.ch:
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
	principal, keyID, ok := h.authenticate(c)
	if !ok {
		return unauthorized(c)
	}

	sessionID := c.Query("sessionId")
	if sessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "sessionId query parameter is required",
		})
	}

	h.sseClientsMu.RLock()
	session, exists := h.sseClients[sessionID]
	h.sseClientsMu.RUnlock()
	// Report a foreign session as missing so session IDs cannot be probed.
	if !exists || session.keyID != keyID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "unknown session"})
	}

	var req JSONRPCRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32700, Message: "Parse error: " + err.Error()},
		})
	}

	resp := h.svc.HandleRequest(WithPrincipal(c.Context(), principal), req, principal.WorkspaceID, uuid.Nil)

	// JSON-RPC 2.0: Notifications (id == nil) MUST NOT receive a response
	if req.ID != nil {
		respBytes, _ := json.Marshal(resp)
		select {
		case session.ch <- string(respBytes):
		case <-session.done:
		default:
			// Buffer full: drop rather than block the request.
		}
	}

	return c.SendStatus(fiber.StatusAccepted)
}

// HandleDelete terminates a Streamable HTTP session (DELETE /mcp with Mcp-Session-Id).
func (h *Handler) HandleDelete(c *fiber.Ctx) error {
	_, keyID, ok := h.authenticate(c)
	if !ok {
		return unauthorized(c)
	}
	sid := c.Get(sessionHeader)
	if sid == "" || !h.validHTTPSession(sid, keyID) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	h.httpSessionsMu.Lock()
	delete(h.httpSessions, sid)
	h.httpSessionsMu.Unlock()
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) newHTTPSession(keyID string) string {
	h.httpSessionsMu.Lock()
	defer h.httpSessionsMu.Unlock()

	now := time.Now()
	for id, sess := range h.httpSessions { // drop expired sessions; bounded by maxHTTPSessions
		if now.Sub(sess.created) > httpSessionTTL {
			delete(h.httpSessions, id)
		}
	}
	if len(h.httpSessions) >= maxHTTPSessions {
		for id := range h.httpSessions { // still full: evict an arbitrary entry
			delete(h.httpSessions, id)
			break
		}
	}
	id := uuid.NewString()
	h.httpSessions[id] = httpSession{keyID: keyID, created: now}
	return id
}

func (h *Handler) validHTTPSession(id, keyID string) bool {
	h.httpSessionsMu.Lock()
	defer h.httpSessionsMu.Unlock()
	sess, ok := h.httpSessions[id]
	if !ok || sess.keyID != keyID {
		return false
	}
	if time.Since(sess.created) > httpSessionTTL {
		delete(h.httpSessions, id)
		return false
	}
	return true
}
