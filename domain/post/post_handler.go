// Package post Tulis CMS Post & Taxonomy API
//
//	Post, custom post type, revision, and taxonomy management
//
//	Schemes: http
//	BasePath: /api
//	Version: 1.0.0
//
//	SecurityDefinitions:
//	Bearer:
//	     type: apiKey
//	     name: Authorization
//	     in: header
package post

import (
	"strconv"

	"github.com/dankedev/tulis-go/domain/webhook"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type PostHandler struct {
	svc        PostService
	wsSvc      workspace.WorkspaceService
	webhookSvc *webhook.Service
}

func NewPostHandler(svc PostService, wsSvc workspace.WorkspaceService, webhookSvc *webhook.Service) *PostHandler {
	return &PostHandler{svc: svc, wsSvc: wsSvc, webhookSvc: webhookSvc}
}

// Create godoc
// @Summary Create a new post
// @Description Creates a new post within the authenticated user's workspace
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param request body CreatePostReq true "Post data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/posts [post]
func (h *PostHandler) Create(c *fiber.Ctx) error {
	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	// CHECK WORKSPACE ROLE
	member, err := h.wsSvc.GetMember(c.Context(), workspaceID, authorID)
	if err != nil {
		return response.Error(c, "FORBIDDEN", "Access denied: you are not a member of this workspace", nil)
	}
	if member.Role == "subscriber" {
		return response.Error(c, "FORBIDDEN", "Access denied: subscriber cannot create posts", nil)
	}

	var req CreatePostReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	post, err := h.svc.CreatePost(c.Context(), req, authorID, workspaceID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	if h.webhookSvc != nil {
		h.webhookSvc.Dispatch(c.Context(), workspaceID, "post.created", post)
		if post.Status == "published" {
			h.webhookSvc.Dispatch(c.Context(), workspaceID, "post.published", post)
		}
	}

	return response.Success(c, post, "Post created successfully")
}

// GetByID godoc
// @Summary Get post by ID
// @Description Returns a single post by its UUID
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Post UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/posts/{id} [get]
func (h *PostHandler) GetByID(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}

	post, err := h.svc.GetPostByID(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", err.Error(), nil)
	}

	return response.Success(c, post, "Post retrieved successfully")
}

// Update godoc
// @Summary Update a post
// @Description Updates an existing post by its UUID
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Post UUID"
// @Param request body UpdatePostReq true "Update data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/posts/{id} [put]
func (h *PostHandler) Update(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}

	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	// Fetch post first to check ownership / workspace alignment
	existingPost, err := h.svc.GetPostByID(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "Post not found", nil)
	}

	if existingPost.WorkspaceID != workspaceID {
		return response.Error(c, "FORBIDDEN", "Post does not belong to this workspace", nil)
	}

	// CHECK WORKSPACE ROLE
	member, err := h.wsSvc.GetMember(c.Context(), workspaceID, authorID)
	if err != nil {
		return response.Error(c, "FORBIDDEN", "Access denied: you are not a member of this workspace", nil)
	}

	if member.Role == "subscriber" {
		return response.Error(c, "FORBIDDEN", "Access denied: subscriber cannot edit posts", nil)
	}

	if member.Role == "author" && existingPost.AuthorID != authorID {
		return response.Error(c, "FORBIDDEN", "Access denied: author can only edit their own posts", nil)
	}

	var req UpdatePostReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	post, err := h.svc.UpdatePost(c.Context(), id, req, authorID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	if h.webhookSvc != nil {
		h.webhookSvc.Dispatch(c.Context(), post.WorkspaceID, "post.updated", post)
		if post.Status == "published" {
			h.webhookSvc.Dispatch(c.Context(), post.WorkspaceID, "post.published", post)
		}
	}

	return response.Success(c, post, "Post updated successfully")
}

// Delete godoc
// @Summary Delete a post
// @Description Permanently deletes a post by its UUID
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Post UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/posts/{id} [delete]
func (h *PostHandler) Delete(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}

	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	// Fetch post first to check ownership / workspace alignment
	existingPost, err := h.svc.GetPostByID(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "Post not found", nil)
	}

	if existingPost.WorkspaceID != workspaceID {
		return response.Error(c, "FORBIDDEN", "Post does not belong to this workspace", nil)
	}

	// CHECK WORKSPACE ROLE
	member, err := h.wsSvc.GetMember(c.Context(), workspaceID, authorID)
	if err != nil {
		return response.Error(c, "FORBIDDEN", "Access denied: you are not a member of this workspace", nil)
	}

	if member.Role == "subscriber" {
		return response.Error(c, "FORBIDDEN", "Access denied: subscriber cannot delete posts", nil)
	}

	if member.Role == "author" && existingPost.AuthorID != authorID {
		return response.Error(c, "FORBIDDEN", "Access denied: author can only delete their own posts", nil)
	}

	if err := h.svc.DeletePost(c.Context(), id); err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	if h.webhookSvc != nil {
		h.webhookSvc.Dispatch(c.Context(), workspaceID, "post.deleted", existingPost)
	}

	return response.Success(c, nil, "Post deleted successfully")
}

// List godoc
// @Summary List posts
// @Description Returns a paginated list of posts in the workspace
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param type query string false "Filter by post type"
// @Param status query string false "Filter by status"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/posts [get]
func (h *PostHandler) List(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	postType := c.Query("type", "")
	status := c.Query("status", "")
	search := c.Query("search", "")

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "10"))

	posts, total, err := h.svc.ListPosts(c.Context(), workspaceID, postType, status, search, page, perPage)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	if totalPages == 0 {
		totalPages = 1
	}

	meta := &response.Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  fiber.StatusOK,
		"message": "Posts retrieved successfully",
		"data":    posts,
		"meta":    meta,
	})
}

// RegisterPostType godoc
// @Summary Register a custom post type
// @Description Creates a new custom post type within the workspace
// @Tags Post Types
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param request body CreatePostTypeReq true "Post type data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/post-types [post]
func (h *PostHandler) RegisterPostType(c *fiber.Ctx) error {
	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	// CHECK WORKSPACE ROLE (Only owner/superadmin or admin can register custom post types)
	member, err := h.wsSvc.GetMember(c.Context(), workspaceID, authorID)
	if err != nil {
		return response.Error(c, "FORBIDDEN", "Access denied: you are not a member of this workspace", nil)
	}
	if member.Role != "superadmin" && member.Role != "admin" {
		return response.Error(c, "FORBIDDEN", "Access denied: only workspace owner/admin can register custom post types", nil)
	}

	var req CreatePostTypeReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.Name == "" {
		return response.Error(c, "VALIDATION_ERROR", "Post type name is required", nil)
	}

	cpt, err := h.svc.RegisterPostType(c.Context(), workspaceID, req.Name, req.Slug, req.Description, req.Fields)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, cpt, "Custom post type registered successfully")
}

// GetPostTypeByID godoc
// @Summary Get post type by ID
// @Description Returns a single post type by its UUID
// @Tags Post Types
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Post type UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/post-types/{id} [get]
func (h *PostHandler) GetPostTypeByID(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post type ID", nil)
	}

	cpt, err := h.svc.GetPostTypeByID(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", err.Error(), nil)
	}

	return response.Success(c, cpt, "Custom post type retrieved successfully")
}

// ListPostTypes godoc
// @Summary List post types
// @Description Returns all registered post types in the workspace
// @Tags Post Types
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/post-types [get]
func (h *PostHandler) ListPostTypes(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	cpts, err := h.svc.ListPostTypes(c.Context(), workspaceID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, cpts, "Custom post types retrieved successfully")
}

// DeletePostType godoc
// @Summary Delete a post type
// @Description Permanently deletes a post type by its UUID
// @Tags Post Types
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Post type UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/post-types/{id} [delete]
func (h *PostHandler) DeletePostType(c *fiber.Ctx) error {
	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	// CHECK WORKSPACE ROLE (Only owner/superadmin or admin can delete custom post types)
	member, err := h.wsSvc.GetMember(c.Context(), workspaceID, authorID)
	if err != nil {
		return response.Error(c, "FORBIDDEN", "Access denied: you are not a member of this workspace", nil)
	}
	if member.Role != "superadmin" && member.Role != "admin" {
		return response.Error(c, "FORBIDDEN", "Access denied: only workspace owner/admin can delete custom post types", nil)
	}

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post type ID", nil)
	}

	if err := h.svc.DeletePostType(c.Context(), id); err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, nil, "Custom post type deleted successfully")
}

// ListRevisions godoc
// @Summary List post revisions
// @Description Returns all revisions (history snapshots) of a post
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Post UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/posts/{id}/revisions [get]
func (h *PostHandler) ListRevisions(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}

	revisions, err := h.svc.ListRevisions(c.Context(), id)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, revisions, "Post revisions retrieved successfully")
}

// RestoreRevision godoc
// @Summary Restore a post revision
// @Description Restores a post to a specific revision state
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Post UUID"
// @Param revisionId path string true "Revision UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/posts/{id}/revisions/{revisionId}/restore [post]
func (h *PostHandler) RestoreRevision(c *fiber.Ctx) error {
	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}

	// Fetch post first to check ownership / workspace alignment
	existingPost, err := h.svc.GetPostByID(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "Post not found", nil)
	}

	if existingPost.WorkspaceID != workspaceID {
		return response.Error(c, "FORBIDDEN", "Post does not belong to this workspace", nil)
	}

	// CHECK WORKSPACE ROLE
	member, err := h.wsSvc.GetMember(c.Context(), workspaceID, authorID)
	if err != nil {
		return response.Error(c, "FORBIDDEN", "Access denied: you are not a member of this workspace", nil)
	}

	if member.Role == "subscriber" {
		return response.Error(c, "FORBIDDEN", "Access denied: subscriber cannot restore revisions", nil)
	}

	if member.Role == "author" && existingPost.AuthorID != authorID {
		return response.Error(c, "FORBIDDEN", "Access denied: author can only restore revisions for their own posts", nil)
	}

	revisionIDStr := c.Params("revisionId")
	revisionID, err := uuid.Parse(revisionIDStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid revision ID", nil)
	}

	post, err := h.svc.RestoreRevision(c.Context(), revisionID, authorID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, post, "Post restored to revision successfully")
}

// CreateTaxonomy godoc
// @Summary Create a taxonomy
// @Description Creates a new taxonomy (category or tag) within the workspace
// @Tags Taxonomies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param request body CreateTaxonomyReq true "Taxonomy data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/taxonomies [post]
func (h *PostHandler) CreateTaxonomy(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	var req CreateTaxonomyReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.Name == "" || req.Type == "" {
		return response.Error(c, "VALIDATION_ERROR", "Name and type are required", nil)
	}

	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		parsedParent, err := uuid.Parse(*req.ParentID)
		if err == nil {
			parentID = &parsedParent
		}
	}

	tax, err := h.svc.CreateTaxonomy(c.Context(), workspaceID, req.Name, req.Slug, req.Type, parentID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, tax, "Taxonomy created successfully")
}

// GetTaxonomyByID godoc
// @Summary Get taxonomy by ID
// @Description Returns a single taxonomy by its UUID
// @Tags Taxonomies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Taxonomy UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/taxonomies/{id} [get]
func (h *PostHandler) GetTaxonomyByID(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid taxonomy ID", nil)
	}

	tax, err := h.svc.GetTaxonomyByID(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", err.Error(), nil)
	}

	return response.Success(c, tax, "Taxonomy retrieved successfully")
}

// UpdateTaxonomy godoc
// @Summary Update a taxonomy
// @Description Updates an existing taxonomy by its UUID
// @Tags Taxonomies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Taxonomy UUID"
// @Param request body UpdateTaxonomyReq true "Update data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/taxonomies/{id} [put]
func (h *PostHandler) UpdateTaxonomy(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid taxonomy ID", nil)
	}

	var req UpdateTaxonomyReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		parsedParent, err := uuid.Parse(*req.ParentID)
		if err == nil {
			parentID = &parsedParent
		}
	}

	tax, err := h.svc.UpdateTaxonomy(c.Context(), id, req.Name, req.Slug, parentID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, tax, "Taxonomy updated successfully")
}

// DeleteTaxonomy godoc
// @Summary Delete a taxonomy
// @Description Permanently deletes a taxonomy by its UUID
// @Tags Taxonomies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Taxonomy UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/taxonomies/{id} [delete]
func (h *PostHandler) DeleteTaxonomy(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid taxonomy ID", nil)
	}

	if err := h.svc.DeleteTaxonomy(c.Context(), id); err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, nil, "Taxonomy deleted successfully")
}

// ListTaxonomies godoc
// @Summary List taxonomies
// @Description Returns all taxonomies in the workspace, optionally filtered by type
// @Tags Taxonomies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param type query string false "Filter by taxonomy type (category, tag)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/taxonomies [get]
func (h *PostHandler) ListTaxonomies(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	taxType := c.Query("type", "")

	taxonomies, err := h.svc.ListTaxonomies(c.Context(), workspaceID, taxType)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, taxonomies, "Taxonomies retrieved successfully")
}
