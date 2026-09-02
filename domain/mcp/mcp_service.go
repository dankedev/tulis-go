package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dankedev/tulis-go/domain/media"
	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/google/uuid"
)

// JSON-RPC 2.0 structures
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Service interface {
	GetTools() []Tool
	HandleRequest(ctx context.Context, req JSONRPCRequest, workspaceID uuid.UUID, userID uuid.UUID) JSONRPCResponse
}

type service struct {
	postSvc      post.PostService
	wsSvc        workspace.WorkspaceService
	mediaSvc     media.MediaService
}

func NewService(postSvc post.PostService, wsSvc workspace.WorkspaceService, mediaSvc media.MediaService) Service {
	return &service{
		postSvc:  postSvc,
		wsSvc:    wsSvc,
		mediaSvc: mediaSvc,
	}
}

func (s *service) GetTools() []Tool {
	return []Tool{
		{
			Name:        "tulis_list_posts",
			Description: "List posts in a Tulis CMS workspace. Supports filtering by status and post type.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"status":    {Type: "string", Description: "Filter by status: draft, published, scheduled, archived"},
					"post_type": {Type: "string", Description: "Filter by post type: post, page, or custom post type slug"},
					"search":    {Type: "string", Description: "Search posts by title or content"},
					"sort":      {Type: "string", Description: "Sort order: 'order asc' (default, smallest order first), 'order desc' (largest order first), 'created_at desc', 'published_at desc', 'title asc', etc."},
					"page":      {Type: "integer", Description: "Page number (default: 1)"},
					"limit":     {Type: "integer", Description: "Results per page (default: 20, max: 50)"},
				},
			},
		},
		{
			Name:        "tulis_get_post",
			Description: "Get a single post by ID or slug, including full content and metadata.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"id_or_slug": {Type: "string", Description: "Post ID (UUID) or slug"},
				},
				Required: []string{"id_or_slug"},
			},
		},
		{
			Name:        "tulis_create_post",
			Description: "Create a new post in Tulis CMS. Returns the created post with its ID.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"title":         {Type: "string", Description: "Post title (required)"},
					"content":       {Type: "string", Description: "Post content in Markdown or HTML"},
					"excerpt":       {Type: "string", Description: "Short excerpt or summary"},
					"status":        {Type: "string", Description: "Post status: draft, published, scheduled, archived (default: draft)"},
					"post_type":     {Type: "string", Description: "Post type: post, page (default: post)"},
					"feature_image": {Type: "string", Description: "URL of the featured image"},
					"seo_title":     {Type: "string", Description: "SEO title"},
					"seo_desc":      {Type: "string", Description: "SEO meta description"},
					"taxonomy_ids":  {Type: "array", Description: "Array of taxonomy (category/tag) IDs to assign"},
					"order":         {Type: "integer", Description: "Display order index (lower number appears first, default: 0)"},
				},
				Required: []string{"title"},
			},
		},
		{
			Name:        "tulis_update_post",
			Description: "Update an existing post by ID. Only provided fields will be changed.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"id":            {Type: "string", Description: "Post ID (UUID) — required"},
					"title":         {Type: "string", Description: "New title"},
					"content":       {Type: "string", Description: "New content"},
					"excerpt":       {Type: "string", Description: "New excerpt"},
					"status":        {Type: "string", Description: "New status"},
					"feature_image": {Type: "string", Description: "New featured image URL"},
					"seo_title":     {Type: "string", Description: "New SEO title"},
					"seo_desc":      {Type: "string", Description: "New SEO description"},
					"order":         {Type: "integer", Description: "New display order index"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "tulis_list_taxonomies",
			Description: "List all categories and tags in the workspace.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"type": {Type: "string", Description: "Filter by type: category or tag"},
					"sort": {Type: "string", Description: "Sort order: 'order asc' (default, smallest order first), 'order desc' (largest order first), 'name asc', 'name desc', 'created_at desc'"},
				},
			},
		},
		{
			Name:        "tulis_create_taxonomy",
			Description: "Create a new category or tag in the workspace.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":      {Type: "string", Description: "Taxonomy name (required)"},
					"slug":      {Type: "string", Description: "URL slug (auto-generated if empty)"},
					"type":      {Type: "string", Description: "category or tag (required)"},
					"parent_id": {Type: "string", Description: "Parent category ID (for hierarchical categories)"},
					"order":     {Type: "integer", Description: "Display order index (lower number appears first, default: 0)"},
				},
				Required: []string{"name", "type"},
			},
		},
		{
			Name:        "tulis_update_taxonomy",
			Description: "Update an existing category or tag by ID.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"id":        {Type: "string", Description: "Taxonomy UUID (required)"},
					"name":      {Type: "string", Description: "New taxonomy name"},
					"slug":      {Type: "string", Description: "New URL slug"},
					"parent_id": {Type: "string", Description: "New parent category ID"},
					"order":     {Type: "integer", Description: "New display order index"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "tulis_get_taxonomy",
			Description: "Get details of a single category or tag by its slug or ID, including child categories if any.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"slug_or_id": {Type: "string", Description: "Taxonomy slug or UUID (required)"},
					"type":       {Type: "string", Description: "Optional taxonomy type: category or tag"},
				},
				Required: []string{"slug_or_id"},
			},
		},
		{
			Name:        "tulis_upload_media",
			Description: "Upload an image or file to Tulis CMS from a URL. Downloads the remote file and stores it in the media library.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_url": {Type: "string", Description: "URL of the file to download and upload (required)"},
					"alt_text": {Type: "string", Description: "Alt text for accessibility"},
					"caption":  {Type: "string", Description: "Caption for the media item"},
				},
				Required: []string{"file_url"},
			},
		},
		{
			Name:        "tulis_list_workspaces",
			Description: "List all workspaces accessible to the authenticated user.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "tulis_get_current_workspace",
			Description: "Get details or ID of the currently active workspace session in MCP.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "tulis_switch_workspace",
			Description: "Switch active workspace context for subsequent MCP API requests.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"workspace_id": {Type: "string", Description: "Workspace UUID to switch to (required)"},
				},
				Required: []string{"workspace_id"},
			},
		},
		{
			Name:        "tulis_list_workspace_members",
			Description: "List members and their roles for the current active workspace or specified workspace.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"workspace_id": {Type: "string", Description: "Optional workspace UUID. If omitted, uses active workspace context."},
				},
			},
		},
	}
}

func (s *service) HandleRequest(ctx context.Context, req JSONRPCRequest, currentWorkspaceID uuid.UUID, userID uuid.UUID) JSONRPCResponse {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "tulis-remote-mcp",
				"version": "1.0.0",
			},
		}
	case "notifications/initialized", "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{
			"tools": s.GetTools(),
		}
	case "tools/call":
		var params ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
			return resp
		}
		resp.Result = s.executeTool(ctx, params.Name, params.Arguments, currentWorkspaceID, userID)
	default:
		// Fallback: Some clients/gateways call tool names directly as JSON-RPC methods (e.g. method: "tulis_list_posts")
		if strings.HasPrefix(req.Method, "tulis_") {
			resp.Result = s.executeTool(ctx, req.Method, req.Params, currentWorkspaceID, userID)
		} else {
			resp.Error = &RPCError{Code: -32601, Message: "Method not found: " + req.Method}
		}
	}

	return resp
}

func (s *service) executeTool(ctx context.Context, name string, args json.RawMessage, currentWorkspaceID uuid.UUID, userID uuid.UUID) ToolResult {
	var params map[string]any
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params == nil {
		params = make(map[string]any)
	}

	switch name {
	case "tulis_list_posts":
		postType, _ := params["post_type"].(string)
		status, _ := params["status"].(string)
		search, _ := params["search"].(string)
		page := 1
		if p, ok := params["page"].(float64); ok && p > 0 {
			page = int(p)
		}
		limit := 20
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
			if limit > 50 {
				limit = 50
			}
		}
		sortBy, _ := params["sort"].(string)
		posts, total, err := s.postSvc.ListPosts(ctx, currentWorkspaceID, post.PostFilter{
			PostType: postType,
			Status:   status,
			Search:   search,
			SortBy:   sortBy,
		}, page, limit)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(map[string]any{
			"posts": posts,
			"total": total,
			"page":  page,
			"limit": limit,
		})

	case "tulis_get_post":
		idOrSlug, _ := params["id_or_slug"].(string)
		if idOrSlug == "" {
			return formatResultError(errors.New("id_or_slug is required"))
		}
		if postUUID, err := uuid.Parse(idOrSlug); err == nil {
			p, err := s.postSvc.GetPostByID(ctx, postUUID)
			if err != nil {
				return formatResultError(err)
			}
			return formatResultJSON(p)
		}
		p, err := s.postSvc.GetPostBySlug(ctx, currentWorkspaceID, idOrSlug)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(p)

	case "tulis_create_post":
		title, _ := params["title"].(string)
		if title == "" {
			return formatResultError(errors.New("title is required"))
		}
		content, _ := params["content"].(string)
		excerpt, _ := params["excerpt"].(string)
		status, _ := params["status"].(string)
		if status == "" {
			status = "draft"
		}
		postType, _ := params["post_type"].(string)
		if postType == "" {
			postType = "post"
		}
		featureImg, _ := params["feature_image"].(string)
		seoTitle, _ := params["seo_title"].(string)
		seoDesc, _ := params["seo_desc"].(string)

		var taxStrings []string
		if rawTax, ok := params["taxonomy_ids"].([]any); ok {
			for _, t := range rawTax {
				taxStrings = append(taxStrings, fmt.Sprint(t))
			}
		}

		var order int
		if orderVal, ok := params["order"].(float64); ok {
			order = int(orderVal)
		} else if orderVal, ok := params["order"].(int); ok {
			order = orderVal
		}

		req := post.CreatePostReq{
			Title:        title,
			Content:      content,
			Excerpt:      excerpt,
			Status:       status,
			PostType:     postType,
			FeatureImage: featureImg,
			SeoTitle:     seoTitle,
			SeoDesc:      seoDesc,
			TaxonomyIDs:  taxStrings,
			Order:        order,
		}
		p, err := s.postSvc.CreatePost(ctx, req, userID, currentWorkspaceID)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(p)

	case "tulis_update_post":
		idStr, _ := params["id"].(string)
		postUUID, err := uuid.Parse(idStr)
		if err != nil {
			return formatResultError(errors.New("valid post UUID id is required"))
		}

		req := post.UpdatePostReq{}
		if title, ok := params["title"].(string); ok && title != "" {
			req.Title = &title
		}
		if content, ok := params["content"].(string); ok {
			req.Content = &content
		}
		if excerpt, ok := params["excerpt"].(string); ok {
			req.Excerpt = &excerpt
		}
		if status, ok := params["status"].(string); ok && status != "" {
			req.Status = &status
		}
		if featureImg, ok := params["feature_image"].(string); ok {
			req.FeatureImage = &featureImg
		}
		if seoTitle, ok := params["seo_title"].(string); ok {
			req.SeoTitle = &seoTitle
		}
		if seoDesc, ok := params["seo_desc"].(string); ok {
			req.SeoDesc = &seoDesc
		}
		if orderVal, ok := params["order"].(float64); ok {
			ord := int(orderVal)
			req.Order = &ord
		} else if orderVal, ok := params["order"].(int); ok {
			req.Order = &orderVal
		}

		p, err := s.postSvc.UpdatePost(ctx, postUUID, req, userID)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(p)

	case "tulis_list_taxonomies":
		taxType, _ := params["type"].(string)
		sortBy, _ := params["sort"].(string)
		taxonomies, err := s.postSvc.ListTaxonomies(ctx, currentWorkspaceID, taxType, sortBy)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(taxonomies)

	case "tulis_create_taxonomy":
		name, _ := params["name"].(string)
		taxType, _ := params["type"].(string)
		slug, _ := params["slug"].(string)
		var parentID *uuid.UUID
		if pidStr, ok := params["parent_id"].(string); ok && pidStr != "" {
			if pu, err := uuid.Parse(pidStr); err == nil {
				parentID = &pu
			}
		}
		var order int
		if orderVal, ok := params["order"].(float64); ok {
			order = int(orderVal)
		} else if orderVal, ok := params["order"].(int); ok {
			order = orderVal
		}
		t, err := s.postSvc.CreateTaxonomy(ctx, currentWorkspaceID, name, slug, taxType, parentID, order)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(t)

	case "tulis_update_taxonomy":
		idStr, _ := params["id"].(string)
		taxUUID, err := uuid.Parse(idStr)
		if err != nil {
			return formatResultError(errors.New("valid taxonomy UUID id is required"))
		}
		name, _ := params["name"].(string)
		slug, _ := params["slug"].(string)
		var parentID *uuid.UUID
		if pidStr, ok := params["parent_id"].(string); ok && pidStr != "" {
			if pu, err := uuid.Parse(pidStr); err == nil {
				parentID = &pu
			}
		}
		var reqOrder *int
		if orderVal, ok := params["order"].(float64); ok {
			ord := int(orderVal)
			reqOrder = &ord
		} else if orderVal, ok := params["order"].(int); ok {
			reqOrder = &orderVal
		}
		t, err := s.postSvc.UpdateTaxonomy(ctx, taxUUID, name, slug, parentID, reqOrder)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(t)

	case "tulis_get_taxonomy":
		slugOrID, _ := params["slug_or_id"].(string)
		if slugOrID == "" {
			return formatResultError(errors.New("slug_or_id is required"))
		}
		taxType, _ := params["type"].(string)
		var t *post.Taxonomy
		var err error
		if uid, errParse := uuid.Parse(slugOrID); errParse == nil {
			t, err = s.postSvc.GetTaxonomyByID(ctx, uid)
		} else {
			t, err = s.postSvc.GetTaxonomyBySlug(ctx, currentWorkspaceID, slugOrID, taxType)
		}
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(t)

	case "tulis_upload_media":
		fileURL, _ := params["file_url"].(string)
		if fileURL == "" {
			return formatResultError(errors.New("file_url is required"))
		}
		altText, _ := params["alt_text"].(string)
		caption, _ := params["caption"].(string)
		m, err := s.mediaSvc.UploadFromURL(ctx, currentWorkspaceID, fileURL, altText, caption)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(m)

	case "tulis_list_workspaces":
		if userID != uuid.Nil {
			workspaces, err := s.wsSvc.ListWorkspaces(ctx, userID)
			if err != nil {
				return formatResultError(err)
			}
			return formatResultJSON(workspaces)
		}
		ws, err := s.wsSvc.GetWorkspaceByID(ctx, currentWorkspaceID)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON([]any{ws})

	case "tulis_get_current_workspace":
		ws, err := s.wsSvc.GetWorkspaceByID(ctx, currentWorkspaceID)
		if err != nil {
			return formatResultJSON(map[string]any{
				"active_workspace_id": currentWorkspaceID.String(),
			})
		}
		return formatResultJSON(ws)

	case "tulis_switch_workspace":
		wsIDStr, _ := params["workspace_id"].(string)
		wsUUID, err := uuid.Parse(wsIDStr)
		if err != nil {
			return formatResultError(errors.New("valid workspace_id UUID is required"))
		}
		ws, err := s.wsSvc.GetWorkspaceByID(ctx, wsUUID)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(map[string]any{
			"status":              "success",
			"message":             fmt.Sprintf("Active workspace context: %s (%s)", ws.Name, ws.ID),
			"active_workspace_id": ws.ID.String(),
		})

	case "tulis_list_workspace_members":
		targetWS := currentWorkspaceID
		if wsIDStr, ok := params["workspace_id"].(string); ok && strings.TrimSpace(wsIDStr) != "" {
			if parsedWS, err := uuid.Parse(wsIDStr); err == nil {
				targetWS = parsedWS
			}
		}
		members, err := s.wsSvc.ListMembers(ctx, targetWS)
		if err != nil {
			return formatResultError(err)
		}
		return formatResultJSON(members)

	default:
		return formatResultError(fmt.Errorf("unknown tool: %s", name))
	}
}

func formatResultJSON(v any) ToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ToolResult{
			Content: []ToolContent{{Type: "text", Text: fmt.Sprint(v)}},
		}
	}
	return ToolResult{
		Content: []ToolContent{{Type: "text", Text: string(b)}},
	}
}

func formatResultError(err error) ToolResult {
	return ToolResult{
		IsError: true,
		Content: []ToolContent{{Type: "text", Text: "Error: " + err.Error()}},
	}
}
