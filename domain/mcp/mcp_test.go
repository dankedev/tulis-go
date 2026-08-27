package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dankedev/tulis-go/domain/mcp"
	"github.com/dankedev/tulis-go/domain/media"
	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/dankedev/tulis-go/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestMCP(t *testing.T) (*gorm.DB, mcp.Service, *mcp.Handler, uuid.UUID, uuid.UUID) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(
		&post.Post{},
		&post.PostType{},
		&post.PostRevision{},
		&post.Taxonomy{},
		&post.PostTaxonomy{},
		&workspace.Workspace{},
		&workspace.WorkspaceMember{},
		&media.Media{},
	)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	wsRepo := workspace.NewWorkspaceRepository(db)
	wsSvc := workspace.NewWorkspaceService(wsRepo)

	postRepo := post.NewPostRepository(db)
	postSvc := post.NewPostService(postRepo, nil)

	mediaRepo := media.NewMediaRepository(db)
	localStorage := storage.NewLocalStorage("./tmp/uploads")
	mediaSvc := media.NewMediaService(mediaRepo, localStorage)

	ownerID := uuid.New()
	ws, err := wsSvc.CreateWorkspace(context.Background(), "MCP Test Workspace", "mcp-test", "free", ownerID)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	svc := mcp.NewService(postSvc, wsSvc, mediaSvc)
	handler := mcp.NewHandler(svc)

	return db, svc, handler, ws.ID, ownerID
}

func TestMCPInitialize(t *testing.T) {
	_, svc, _, wsID, userID := setupTestMCP(t)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	resp := svc.HandleRequest(context.Background(), req, wsID, userID)
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
	if resp.ID != 1 {
		t.Errorf("expected id 1, got %v", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestMCPToolsList(t *testing.T) {
	_, svc, _, wsID, userID := setupTestMCP(t)

	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := svc.HandleRequest(context.Background(), req, wsID, userID)
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
	resMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result")
	}
	tools, ok := resMap["tools"].([]mcp.Tool)
	if !ok || len(tools) < 10 {
		t.Fatalf("expected at least 10 tools, got %d", len(tools))
	}
}

func TestMCPToolCallPostCreation(t *testing.T) {
	_, svc, _, wsID, userID := setupTestMCP(t)

	// 1. Create post
	createArgs := map[string]any{
		"title":   "Artikel dari Remote MCP",
		"content": "Ini adalah konten tulisan otomatis dari remote MCP.",
		"status":  "published",
	}
	rawArgs, _ := json.Marshal(createArgs)
	callReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_create_post","arguments":` + string(rawArgs) + `}`),
	}

	resp := svc.HandleRequest(context.Background(), callReq, wsID, userID)
	if resp.Error != nil {
		t.Fatalf("unexpected tool call error: %v", resp.Error)
	}

	toolResult, ok := resp.Result.(mcp.ToolResult)
	if !ok || toolResult.IsError || len(toolResult.Content) == 0 {
		t.Fatalf("expected success tool result, got %+v", resp.Result)
	}

	// 2. List posts
	listReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_list_posts","arguments":{}}`),
	}
	listResp := svc.HandleRequest(context.Background(), listReq, wsID, userID)
	if listResp.Error != nil {
		t.Fatalf("unexpected list error: %v", listResp.Error)
	}
}

func TestMCPHandlePostHTTP(t *testing.T) {
	_, _, handler, wsID, _ := setupTestMCP(t)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("workspace_id", wsID.String())
		return c.Next()
	})
	app.Post("/mcp", handler.HandlePost)

	payload := `{"jsonrpc":"2.0","id":10,"method":"tools/list"}`
	httpReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(payload))
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := app.Test(httpReq)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", httpResp.StatusCode)
	}

	var jsonResp mcp.JSONRPCResponse
	err = json.NewDecoder(httpResp.Body).Decode(&jsonResp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if jsonResp.Error != nil {
		t.Fatalf("unexpected error response: %v", jsonResp.Error)
	}
}

func TestMCPDirectMethodCall(t *testing.T) {
	_, svc, _, wsID, userID := setupTestMCP(t)

	// Direct call with method name "tulis_list_posts"
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tulis_list_posts",
		Params:  json.RawMessage(`{}`),
	}

	resp := svc.HandleRequest(context.Background(), req, wsID, userID)
	if resp.Error != nil {
		t.Fatalf("expected direct method call to succeed, got error: %v", resp.Error)
	}
	toolResult, ok := resp.Result.(mcp.ToolResult)
	if !ok || toolResult.IsError {
		t.Fatalf("expected successful ToolResult, got %+v", resp.Result)
	}
}
