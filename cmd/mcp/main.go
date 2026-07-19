// MCP Server for Tulis CMS
//
// Implements Model Context Protocol (MCP) over stdio transport.
// Allows AI agents (Claude Desktop, Cursor, etc.) to interact with Tulis CMS.
//
// Usage:
//
//	TULIS_API_URL=http://localhost:8080/api TULIS_API_KEY=tulis_sk_... ./tulis-mcp
//
// Configure in claude_desktop_config.json:
//
//	{
//	  "mcpServers": {
//	    "tulis": {
//	      "command": "./tulis-mcp",
//	      "env": {
//	        "TULIS_API_URL": "http://localhost:8080/api",
//	        "TULIS_API_KEY": "tulis_sk_your_key_here"
//	      }
//	    }
//	  }
//	}
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// JSON-RPC 2.0 types
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP Tool definition
type tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Server state
type server struct {
	name    string
	version string
	apiURL  string
	apiKey  string
}

var srv *server

func main() {
	srv = &server{
		name:    "tulis-mcp",
		version: "1.0.0",
		apiURL:  getEnv("TULIS_API_URL", "http://localhost:8080/api"),
		apiKey:  os.Getenv("TULIS_API_KEY"),
	}

	if srv.apiKey == "" {
		fmt.Fprintf(os.Stderr, "ERROR: TULIS_API_KEY environment variable is required\n")
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		handleMessage(line)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "stdin read error: %v\n", err)
		os.Exit(1)
	}
}

func handleMessage(data []byte) {
	var req jsonRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		sendError(nil, -32700, "Parse error: "+err.Error())
		return
	}

	switch req.Method {
	case "initialize":
		handleInitialize(req)
	case "notifications/initialized":
		// No response needed per MCP spec
	case "tools/list":
		handleToolsList(req)
	case "tools/call":
		handleToolsCall(req)
	default:
		sendError(req.ID, -32601, "Method not found: "+req.Method)
	}
}

func handleInitialize(req jsonRPCRequest) {
	send(req.ID, map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    srv.name,
			"version": srv.version,
		},
	})
}

func handleToolsList(req jsonRPCRequest) {
	tools := []tool{
		{
			Name:        "tulis_list_posts",
			Description: "List posts in a Tulis CMS workspace. Supports filtering by status and post type.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"status":    {Type: "string", Description: "Filter by status: draft, published, scheduled, archived"},
					"post_type": {Type: "string", Description: "Filter by post type: post, page, or custom post type slug"},
					"search":    {Type: "string", Description: "Search posts by title or content"},
					"page":      {Type: "integer", Description: "Page number (default: 1)"},
					"limit":     {Type: "integer", Description: "Results per page (default: 20, max: 50)"},
				},
			},
		},
		{
			Name:        "tulis_get_post",
			Description: "Get a single post by ID or slug, including full content and metadata.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"id_or_slug": {Type: "string", Description: "Post ID (UUID) or slug"},
				},
				Required: []string{"id_or_slug"},
			},
		},
		{
			Name:        "tulis_create_post",
			Description: "Create a new post in Tulis CMS. Returns the created post with its ID.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"title":        {Type: "string", Description: "Post title (required)"},
					"content":      {Type: "string", Description: "Post content in Markdown or HTML"},
					"excerpt":      {Type: "string", Description: "Short excerpt or summary"},
					"status":       {Type: "string", Description: "Post status: draft, published, scheduled, archived (default: draft)"},
					"post_type":    {Type: "string", Description: "Post type: post, page (default: post)"},
					"feature_image": {Type: "string", Description: "URL of the featured image"},
					"seo_title":    {Type: "string", Description: "SEO title"},
					"seo_desc":     {Type: "string", Description: "SEO meta description"},
					"taxonomy_ids": {Type: "array", Description: "Array of taxonomy (category/tag) IDs to assign"},
				},
				Required: []string{"title"},
			},
		},
		{
			Name:        "tulis_update_post",
			Description: "Update an existing post by ID. Only provided fields will be changed.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"id":           {Type: "string", Description: "Post ID (UUID) — required"},
					"title":        {Type: "string", Description: "New title"},
					"content":      {Type: "string", Description: "New content"},
					"excerpt":      {Type: "string", Description: "New excerpt"},
					"status":       {Type: "string", Description: "New status"},
					"feature_image": {Type: "string", Description: "New featured image URL"},
					"seo_title":    {Type: "string", Description: "New SEO title"},
					"seo_desc":     {Type: "string", Description: "New SEO description"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "tulis_list_taxonomies",
			Description: "List all categories and tags in the workspace.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"type": {Type: "string", Description: "Filter by type: category or tag"},
				},
			},
		},
		{
			Name:        "tulis_create_taxonomy",
			Description: "Create a new category or tag in the workspace.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"name": {Type: "string", Description: "Taxonomy name (required)"},
					"slug": {Type: "string", Description: "URL slug (auto-generated if empty)"},
					"type": {Type: "string", Description: "category or tag (required)"},
					"parent_id": {Type: "string", Description: "Parent category ID (for hierarchical categories)"},
				},
				Required: []string{"name", "type"},
			},
		},
		{
			Name:        "tulis_upload_media",
			Description: "Upload an image or file to Tulis CMS from a URL. Downloads the remote file and stores it in the media library.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"file_url": {Type: "string", Description: "URL of the file to download and upload (required)"},
					"alt_text": {Type: "string", Description: "Alt text for accessibility"},
					"caption":  {Type: "string", Description: "Caption for the media item"},
				},
				Required: []string{"file_url"},
			},
		},
	}
	send(req.ID, map[string]any{"tools": tools})
}

func handleToolsCall(req jsonRPCRequest) {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params: "+err.Error())
		return
	}

	var result toolResult

	switch params.Name {
	case "tulis_list_posts":
		result = callListPosts(params.Arguments)
	case "tulis_get_post":
		result = callGetPost(params.Arguments)
	case "tulis_create_post":
		result = callCreatePost(params.Arguments)
	case "tulis_update_post":
		result = callUpdatePost(params.Arguments)
	case "tulis_list_taxonomies":
		result = callListTaxonomies(params.Arguments)
	case "tulis_create_taxonomy":
		result = callCreateTaxonomy(params.Arguments)
	case "tulis_upload_media":
		result = callUploadMedia(params.Arguments)
	default:
		sendError(req.ID, -32602, "Unknown tool: "+params.Name)
		return
	}

	send(req.ID, result)
}

func send(id any, result any) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func sendError(id any, code int, message string) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
