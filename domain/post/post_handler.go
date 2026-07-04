package post

import (
	"strconv"

	"github.com/dankedev/kontent/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type PostHandler struct {
	svc PostService
}

func NewPostHandler(svc PostService) *PostHandler {
	return &PostHandler{svc: svc}
}

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

	var req CreatePostReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	post, err := h.svc.CreatePost(c.Context(), req, authorID, workspaceID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, post, "Post created successfully")
}

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

	var req UpdatePostReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	post, err := h.svc.UpdatePost(c.Context(), id, req, authorID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, post, "Post updated successfully")
}

func (h *PostHandler) Delete(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}

	if err := h.svc.DeletePost(c.Context(), id); err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, nil, "Post deleted successfully")
}

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

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "10"))

	posts, total, err := h.svc.ListPosts(c.Context(), workspaceID, postType, status, page, perPage)
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

// Custom Post Type mappings
func (h *PostHandler) RegisterPostType(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
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

func (h *PostHandler) DeletePostType(c *fiber.Ctx) error {
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

// Revisions mappings
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

func (h *PostHandler) RestoreRevision(c *fiber.Ctx) error {
	// Extract author
	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
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
