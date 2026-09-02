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

func TestMCPOrderFeature(t *testing.T) {
	_, svc, _, wsID, userID := setupTestMCP(t)

	// 1. Create Post with Order 20
	createPostReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_create_post","arguments":{"title":"Second Post","content":"Content","order":20}}`),
	}
	resp := svc.HandleRequest(context.Background(), createPostReq, wsID, userID)
	if resp.Error != nil {
		t.Fatalf("failed to create post via MCP: %v", resp.Error)
	}

	// 2. Create Post with Order 10
	createPostReq2 := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_create_post","arguments":{"title":"First Post","content":"Content","order":10}}`),
	}
	resp = svc.HandleRequest(context.Background(), createPostReq2, wsID, userID)
	if resp.Error != nil {
		t.Fatalf("failed to create post via MCP: %v", resp.Error)
	}

	// 3. List posts with sort = order asc (smallest order first)
	listReqAsc := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_list_posts","arguments":{"sort":"order asc"}}`),
	}
	resp = svc.HandleRequest(context.Background(), listReqAsc, wsID, userID)
	if resp.Error != nil {
		t.Fatalf("failed to list posts via MCP: %v", resp.Error)
	}
	tr := resp.Result.(mcp.ToolResult)
	var listData struct {
		Posts []struct {
			Title string `json:"title"`
			Order int    `json:"order"`
		} `json:"posts"`
	}
	json.Unmarshal([]byte(tr.Content[0].Text), &listData)
	if len(listData.Posts) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(listData.Posts))
	}
	if listData.Posts[0].Order != 10 || listData.Posts[1].Order != 20 {
		t.Errorf("expected order asc [10, 20], got [%d, %d]", listData.Posts[0].Order, listData.Posts[1].Order)
	}

	// 4. List posts with sort = order desc (largest order first)
	listReqDesc := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_list_posts","arguments":{"sort":"order desc"}}`),
	}
	resp = svc.HandleRequest(context.Background(), listReqDesc, wsID, userID)
	if resp.Error != nil {
		t.Fatalf("failed to list posts via MCP: %v", resp.Error)
	}
	tr = resp.Result.(mcp.ToolResult)
	json.Unmarshal([]byte(tr.Content[0].Text), &listData)
	if listData.Posts[0].Order != 20 || listData.Posts[1].Order != 10 {
		t.Errorf("expected order desc [20, 10], got [%d, %d]", listData.Posts[0].Order, listData.Posts[1].Order)
	}

	// 5. Create and sort taxonomies via MCP
	createTax1 := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_create_taxonomy","arguments":{"name":"Cat Beta","type":"category","order":50}}`),
	}
	svc.HandleRequest(context.Background(), createTax1, wsID, userID)

	createTax2 := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_create_taxonomy","arguments":{"name":"Cat Alpha","type":"category","order":5}}`),
	}
	svc.HandleRequest(context.Background(), createTax2, wsID, userID)

	listTaxAsc := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_list_taxonomies","arguments":{"type":"category","sort":"order asc"}}`),
	}
	resp = svc.HandleRequest(context.Background(), listTaxAsc, wsID, userID)
	tr = resp.Result.(mcp.ToolResult)
	var taxData []struct {
		Name  string `json:"name"`
		Order int    `json:"order"`
	}
	json.Unmarshal([]byte(tr.Content[0].Text), &taxData)
	if len(taxData) < 2 {
		t.Fatalf("expected at least 2 categories, got %d", len(taxData))
	}
	if taxData[0].Order != 5 || taxData[1].Order != 50 {
		t.Errorf("expected taxonomy order asc [5, 50], got [%d, %d]", taxData[0].Order, taxData[1].Order)
	}

	listTaxDesc := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_list_taxonomies","arguments":{"type":"category","sort":"order desc"}}`),
	}
	resp = svc.HandleRequest(context.Background(), listTaxDesc, wsID, userID)
	tr = resp.Result.(mcp.ToolResult)
	json.Unmarshal([]byte(tr.Content[0].Text), &taxData)
	if taxData[0].Order != 50 || taxData[1].Order != 5 {
		t.Errorf("expected taxonomy order desc [50, 5], got [%d, %d]", taxData[0].Order, taxData[1].Order)
	}
}

func TestMCPSlugSanitization(t *testing.T) {
	_, svc, _, wsID, userID := setupTestMCP(t)

	// 1. Create post with dirty characters in title
	createPostReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_create_post","arguments":{"title":"AI & Machine Learning @ 2026,.,!","content":"Content"}}`),
	}
	resp := svc.HandleRequest(context.Background(), createPostReq, wsID, userID)
	if resp.Error != nil {
		t.Fatalf("failed to create post via MCP: %v", resp.Error)
	}
	tr := resp.Result.(mcp.ToolResult)
	var postData struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	json.Unmarshal([]byte(tr.Content[0].Text), &postData)
	if postData.Slug != "ai-machine-learning-2026" {
		t.Errorf("expected slug 'ai-machine-learning-2026', got '%s'", postData.Slug)
	}

	// 2. Create taxonomy with dirty characters in name
	createTaxReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_create_taxonomy","arguments":{"name":"Tech,.,@News!","type":"category"}}`),
	}
	resp = svc.HandleRequest(context.Background(), createTaxReq, wsID, userID)
	if resp.Error != nil {
		t.Fatalf("failed to create taxonomy via MCP: %v", resp.Error)
	}
	tr = resp.Result.(mcp.ToolResult)
	var taxData struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	json.Unmarshal([]byte(tr.Content[0].Text), &taxData)
	if taxData.Slug != "tech-news" {
		t.Errorf("expected taxonomy slug 'tech-news', got '%s'", taxData.Slug)
	}

	// 3. Update taxonomy with dirty slug
	updateTaxReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"tulis_update_taxonomy","arguments":{"id":"` + taxData.ID + `","slug":"custom,.,@slug#v2"}}`),
	}
	resp = svc.HandleRequest(context.Background(), updateTaxReq, wsID, userID)
	if resp.Error != nil {
		t.Fatalf("failed to update taxonomy via MCP: %v", resp.Error)
	}
	tr = resp.Result.(mcp.ToolResult)
	json.Unmarshal([]byte(tr.Content[0].Text), &taxData)
	if taxData.Slug != "custom-slug-v2" {
		t.Errorf("expected taxonomy slug 'custom-slug-v2', got '%s'", taxData.Slug)
	}
}


