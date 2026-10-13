package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dankedev/tulis-go/domain/mcp"
	"github.com/dankedev/tulis-go/domain/post"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// useAPIKey simulates middleware.ApiKeyAuth having accepted a key bound to wsID.
func useAPIKey(app *fiber.App, wsID uuid.UUID, scopes string) {
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("workspace_id", wsID.String())
		c.Locals("api_key_id", "key-"+wsID.String())
		c.Locals("api_key_scopes", scopes)
		c.Locals("auth_method", "api_key")
		return c.Next()
	})
}

func callTool(t *testing.T, svc mcp.Service, ctx context.Context, wsID, userID uuid.UUID, name string, args map[string]any) mcp.ToolResult {
	t.Helper()
	raw, _ := json.Marshal(args)
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": json.RawMessage(raw)})
	resp := svc.HandleRequest(ctx, mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: params}, wsID, userID)
	if resp.Error != nil {
		t.Fatalf("rpc error: %v", resp.Error)
	}
	res, ok := resp.Result.(mcp.ToolResult)
	if !ok {
		t.Fatalf("unexpected result type %T", resp.Result)
	}
	return res
}

func firstText(r mcp.ToolResult) string {
	if len(r.Content) == 0 {
		return ""
	}
	return r.Content[0].Text
}

func TestMCPHTTPRejectsUnauthenticated(t *testing.T) {
	_, _, handler, _, _ := setupTestMCP(t)

	app := fiber.New()
	app.Post("/mcp", handler.HandlePost)
	app.Get("/mcp/sse", handler.HandleSSE)
	app.Post("/mcp/messages", handler.HandleSSEMessages)

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/mcp"},
		{http.MethodGet, "/mcp/sse"},
		{http.MethodPost, "/mcp/messages?sessionId=x"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", tc.method, tc.path, resp.StatusCode)
		}
	}
}

func TestMCPHTTPIgnoresWorkspaceHeader(t *testing.T) {
	_, _, handler, wsID, _ := setupTestMCP(t)

	app := fiber.New()
	useAPIKey(app, wsID, "content:read")
	app.Post("/mcp", handler.HandlePost)

	other := uuid.New()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tulis_get_current_workspace","arguments":{}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-ID", other.String())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Result mcp.ToolResult `json:"result"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.Result.IsError || !strings.Contains(firstText(out.Result), wsID.String()) {
		t.Fatalf("expected key's own workspace %s, got %q", wsID, firstText(out.Result))
	}
}

func TestMCPSSEMessagesRejectOtherKeysSession(t *testing.T) {
	_, _, handler, wsID, _ := setupTestMCP(t)

	app := fiber.New()
	useAPIKey(app, wsID, "content:read")
	app.Post("/mcp/messages", handler.HandleSSEMessages)

	req := httptest.NewRequest(http.MethodPost, "/mcp/messages?sessionId="+uuid.NewString(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown session, got %d", resp.StatusCode)
	}
}

func TestMCPScopesEnforced(t *testing.T) {
	_, svc, _, wsID, _ := setupTestMCP(t)

	readOnly := mcp.WithPrincipal(context.Background(), mcp.Principal{WorkspaceID: wsID, Scopes: "content:read"})
	res := callTool(t, svc, readOnly, wsID, uuid.Nil, "tulis_create_post", map[string]any{"title": "nope"})
	if !res.IsError || !strings.Contains(firstText(res), "content:write") {
		t.Fatalf("read-only key must not create posts, got %q", firstText(res))
	}
	if res := callTool(t, svc, readOnly, wsID, uuid.Nil, "tulis_list_posts", nil); res.IsError {
		t.Fatalf("read-only key must list posts: %s", firstText(res))
	}
	if res := callTool(t, svc, readOnly, wsID, uuid.Nil, "tulis_list_workspace_members", nil); !res.IsError {
		t.Fatal("members listing requires admin scope")
	}

	writer := mcp.WithPrincipal(context.Background(), mcp.Principal{WorkspaceID: wsID, Scopes: "content:write"})
	if res := callTool(t, svc, writer, wsID, uuid.Nil, "tulis_create_post", map[string]any{"title": "ok"}); res.IsError {
		t.Fatalf("write key must create posts: %s", firstText(res))
	}
	if res := callTool(t, svc, writer, wsID, uuid.Nil, "tulis_list_workspace_members", nil); !res.IsError {
		t.Fatal("content:write must not grant members listing")
	}

	admin := mcp.WithPrincipal(context.Background(), mcp.Principal{WorkspaceID: wsID, Scopes: "admin"})
	if res := callTool(t, svc, admin, wsID, uuid.Nil, "tulis_list_workspace_members", nil); res.IsError {
		t.Fatalf("admin key must list members: %s", firstText(res))
	}
}

func TestMCPNoWorkspaceNoFallback(t *testing.T) {
	_, svc, _, _, _ := setupTestMCP(t)
	res := callTool(t, svc, context.Background(), uuid.Nil, uuid.Nil, "tulis_create_post", map[string]any{"title": "x"})
	if !res.IsError {
		t.Fatal("must not fall back to an arbitrary workspace")
	}
}

func TestMCPCrossWorkspaceIsolation(t *testing.T) {
	db, svc, _, wsA, _ := setupTestMCP(t)

	wsB := uuid.New()
	postB := &post.Post{ID: uuid.New(), WorkspaceID: wsB, Title: "secret", Slug: "secret", Status: "published", PostType: "post"}
	taxB := &post.Taxonomy{ID: uuid.New(), WorkspaceID: wsB, Name: "tag-b", Slug: "tag-b", Type: "tag"}
	if err := db.Create(postB).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(taxB).Error; err != nil {
		t.Fatal(err)
	}

	ctx := mcp.WithPrincipal(context.Background(), mcp.Principal{WorkspaceID: wsA, Scopes: "admin"})
	deny := func(name string, args map[string]any) {
		t.Helper()
		res := callTool(t, svc, ctx, wsA, uuid.Nil, name, args)
		if !res.IsError {
			t.Errorf("%s must not reach workspace B, got %q", name, firstText(res))
		}
	}
	deny("tulis_get_post", map[string]any{"id_or_slug": postB.ID.String()})
	deny("tulis_update_post", map[string]any{"id": postB.ID.String(), "title": "pwned"})
	deny("tulis_get_taxonomy", map[string]any{"slug_or_id": taxB.ID.String()})
	deny("tulis_update_taxonomy", map[string]any{"id": taxB.ID.String(), "name": "pwned"})
	deny("tulis_create_post", map[string]any{"title": "x", "taxonomy_ids": []any{taxB.ID.String()}})
	deny("tulis_create_taxonomy", map[string]any{"name": "child", "type": "category", "parent_id": taxB.ID.String()})
	deny("tulis_switch_workspace", map[string]any{"workspace_id": wsB.String()})
	deny("tulis_list_workspace_members", map[string]any{"workspace_id": wsB.String()})

	var got post.Post
	if err := db.First(&got, "id = ?", postB.ID).Error; err != nil || got.Title != "secret" {
		t.Fatalf("workspace B post was modified: %+v err=%v", got, err)
	}

	// Switching to the key's own workspace is allowed, and listing shows only that workspace.
	if res := callTool(t, svc, ctx, wsA, uuid.Nil, "tulis_switch_workspace", map[string]any{"workspace_id": wsA.String()}); res.IsError {
		t.Fatalf("switch to own workspace failed: %s", firstText(res))
	}
	res := callTool(t, svc, ctx, wsA, uuid.Nil, "tulis_list_workspaces", nil)
	if res.IsError || strings.Contains(firstText(res), wsB.String()) {
		t.Fatalf("list_workspaces leaked or failed: %q", firstText(res))
	}
}

func TestMCPDeletePostAndTaxonomyUpdate(t *testing.T) {
	db, svc, _, wsA, _ := setupTestMCP(t)

	wsB := uuid.New()
	postB := &post.Post{ID: uuid.New(), WorkspaceID: wsB, Title: "other", Slug: "other", Status: "published", PostType: "post"}
	if err := db.Create(postB).Error; err != nil {
		t.Fatal(err)
	}

	writer := mcp.WithPrincipal(context.Background(), mcp.Principal{WorkspaceID: wsA, Scopes: "content:write"})
	reader := mcp.WithPrincipal(context.Background(), mcp.Principal{WorkspaceID: wsA, Scopes: "content:read"})

	created := callTool(t, svc, writer, wsA, uuid.Nil, "tulis_create_post", map[string]any{"title": "to delete"})
	var p post.Post
	if err := json.Unmarshal([]byte(firstText(created)), &p); err != nil || p.ID == uuid.Nil {
		t.Fatalf("create failed: %q", firstText(created))
	}

	if res := callTool(t, svc, reader, wsA, uuid.Nil, "tulis_delete_post", map[string]any{"id": p.ID.String()}); !res.IsError {
		t.Fatal("read-only key must not delete")
	}
	if res := callTool(t, svc, writer, wsA, uuid.Nil, "tulis_delete_post", map[string]any{"id": postB.ID.String()}); !res.IsError {
		t.Fatal("must not delete a post from another workspace")
	}
	if err := db.First(&post.Post{}, "id = ?", postB.ID).Error; err != nil {
		t.Fatalf("workspace B post must survive: %v", err)
	}
	if res := callTool(t, svc, writer, wsA, uuid.Nil, "tulis_delete_post", map[string]any{"id": p.ID.String()}); res.IsError {
		t.Fatalf("delete failed: %s", firstText(res))
	}
	if res := callTool(t, svc, writer, wsA, uuid.Nil, "tulis_get_post", map[string]any{"id_or_slug": p.ID.String()}); !res.IsError {
		t.Fatal("deleted post must be gone")
	}
}

func TestMCPUpdatePostTaxonomyIDsAndEmptyFields(t *testing.T) {
	_, svc, _, wsA, _ := setupTestMCP(t)
	ctx := mcp.WithPrincipal(context.Background(), mcp.Principal{WorkspaceID: wsA, Scopes: "content:write"})

	tax := callTool(t, svc, ctx, wsA, uuid.Nil, "tulis_create_taxonomy", map[string]any{"name": "Go", "type": "tag"})
	var tx post.Taxonomy
	if err := json.Unmarshal([]byte(firstText(tax)), &tx); err != nil {
		t.Fatal(err)
	}
	created := callTool(t, svc, ctx, wsA, uuid.Nil, "tulis_create_post", map[string]any{"title": "p", "taxonomy_ids": []any{tx.ID.String()}})
	var p post.Post
	_ = json.Unmarshal([]byte(firstText(created)), &p)

	if res := callTool(t, svc, ctx, wsA, uuid.Nil, "tulis_update_post", map[string]any{"id": p.ID.String(), "title": "  "}); !res.IsError {
		t.Fatal("empty title must be rejected, not ignored")
	}
	if res := callTool(t, svc, ctx, wsA, uuid.Nil, "tulis_update_post", map[string]any{"id": p.ID.String(), "taxonomy_ids": []any{}}); res.IsError {
		t.Fatalf("clearing taxonomies failed: %s", firstText(res))
	}
	got := callTool(t, svc, ctx, wsA, uuid.Nil, "tulis_get_post", map[string]any{"id_or_slug": p.ID.String()})
	var after post.Post
	_ = json.Unmarshal([]byte(firstText(got)), &after)
	if len(after.Taxonomies) != 0 {
		t.Fatalf("expected taxonomies cleared, got %d", len(after.Taxonomies))
	}
}

func TestMCPPostAuthoredByKeyIssuer(t *testing.T) {
	_, svc, _, wsA, owner := setupTestMCP(t)

	issuer := uuid.New() // never added to the workspace
	ctx := mcp.WithPrincipal(context.Background(), mcp.Principal{WorkspaceID: wsA, Scopes: "content:write", UserID: owner})
	res := callTool(t, svc, ctx, wsA, uuid.Nil, "tulis_create_post", map[string]any{"title": "authored"})
	var p post.Post
	if err := json.Unmarshal([]byte(firstText(res)), &p); err != nil {
		t.Fatalf("create failed: %q", firstText(res))
	}
	if p.AuthorID != owner {
		t.Fatalf("expected author %s, got %s", owner, p.AuthorID)
	}

	// An issuer who is not (or no longer) a member must not become the author.
	ctx = mcp.WithPrincipal(context.Background(), mcp.Principal{WorkspaceID: wsA, Scopes: "content:write", UserID: issuer})
	res = callTool(t, svc, ctx, wsA, uuid.Nil, "tulis_create_post", map[string]any{"title": "fallback"})
	var q post.Post
	_ = json.Unmarshal([]byte(firstText(res)), &q)
	if q.AuthorID == issuer || q.AuthorID == uuid.Nil {
		t.Fatalf("non-member issuer must not be author, got %s", q.AuthorID)
	}
}

func TestMCPStreamableHTTPSessions(t *testing.T) {
	_, _, handler, wsID, _ := setupTestMCP(t)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("workspace_id", wsID.String())
		c.Locals("api_key_id", c.Get("X-Test-Key", "key-1"))
		c.Locals("api_key_scopes", "content:read")
		c.Locals("auth_method", "api_key")
		return c.Next()
	})
	app.Post("/mcp", handler.HandlePost)
	app.Delete("/mcp", handler.HandleDelete)

	do := func(method, body, sid, key string) *http.Response {
		req := httptest.NewRequest(method, "/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if sid != "" {
			req.Header.Set("Mcp-Session-Id", sid)
		}
		if key != "" {
			req.Header.Set("X-Test-Key", key)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	const list = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

	init := do(http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, "", "")
	sid := init.Header.Get("Mcp-Session-Id")
	if init.StatusCode != http.StatusOK || sid == "" {
		t.Fatalf("initialize must return a session id, status=%d sid=%q", init.StatusCode, sid)
	}
	if r := do(http.MethodPost, list, sid, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("valid session rejected: %d", r.StatusCode)
	}
	if r := do(http.MethodPost, list, "", ""); r.StatusCode != http.StatusOK {
		t.Fatalf("stateless request must still work: %d", r.StatusCode)
	}
	if r := do(http.MethodPost, list, sid, "other-key"); r.StatusCode != http.StatusNotFound {
		t.Fatalf("another key must not reuse the session: %d", r.StatusCode)
	}
	if r := do(http.MethodPost, list, uuid.NewString(), ""); r.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown session must be 404: %d", r.StatusCode)
	}
	if r := do(http.MethodDelete, "", sid, "other-key"); r.StatusCode != http.StatusNotFound {
		t.Fatalf("another key must not delete the session: %d", r.StatusCode)
	}
	if r := do(http.MethodDelete, "", sid, ""); r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete failed: %d", r.StatusCode)
	}
	if r := do(http.MethodPost, list, sid, ""); r.StatusCode != http.StatusNotFound {
		t.Fatalf("terminated session must be 404: %d", r.StatusCode)
	}
}
