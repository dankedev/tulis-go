package comment

import (
	"strconv"

	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// CreatePublic godoc
// @Summary Create a comment on a post (public)
// @Description Creates a new comment. Guests provide name+email; authenticated users' ID is auto-set. Comments default to 'pending' for moderation.
// @Tags Comments
// @Accept json
// @Produce json
// @Param request body CreateCommentReq true "Comment data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/comments [post]
func (h *Handler) CreatePublic(c *fiber.Ctx) error {
	var req CreateCommentReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	// If user is authenticated, use their ID
	var authorID *uuid.UUID
	if uid := c.Locals("user_id"); uid != nil {
		if parsed, err := uuid.Parse(uid.(string)); err == nil {
			authorID = &parsed
		}
	}

	comment, err := h.svc.Create(c.Context(), req, workspaceID, authorID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Failed to create comment", nil)
	}

	return response.Success(c, comment.ToResponse(), "Comment submitted for moderation")
}

// ListByPost godoc
// @Summary List approved comments for a post (public)
// @Description Returns threaded (nested) comments for a specific post. Only approved comments are returned publicly.
// @Tags Comments
// @Produce json
// @Param post_id path string true "Post ID"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /api/posts/:post_id/comments [get]
func (h *Handler) ListByPost(c *fiber.Ctx) error {
	postID, err := uuid.Parse(c.Params("post_id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	comments, total, err := h.svc.ListByPost(c.Context(), postID, "approved", page, limit)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "No comments found", nil)
	}

	// Convert to response shape
	items := make([]CommentResponse, len(comments))
	for i, cm := range comments {
		items[i] = cm.ToResponse()
	}

	return c.JSON(fiber.Map{
		"status":  200,
		"message": "Comments retrieved",
		"data":    items,
		"meta": fiber.Map{
			"page":       page,
			"per_page":   limit,
			"total":      total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// ListByWorkspace godoc
// @Summary List all comments in a workspace (admin moderation)
// @Description Returns all comments for moderation. Requires admin+ role.
// @Tags Comments
// @Produce json
// @Security BearerAuth
// @Param status query string false "Filter by status (pending, approved, spam, trashed)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /api/comments [get]
func (h *Handler) ListByWorkspace(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	status := c.Query("status", "")

	comments, total, err := h.svc.ListByWorkspace(c.Context(), workspaceID, status, page, limit)
	if err != nil && err != gorm.ErrRecordNotFound {
		return response.Error(c, "NOT_FOUND", "No comments found", nil)
	}

	items := make([]CommentResponse, len(comments))
	for i, cm := range comments {
		items[i] = cm.ToResponse()
	}

	return c.JSON(fiber.Map{
		"status":  200,
		"message": "Comments retrieved",
		"data":    items,
		"meta": fiber.Map{
			"page":       page,
			"per_page":   limit,
			"total":      total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// UpdateStatus godoc
// @Summary Moderate a comment (approve/spam/trash)
// @Description Update comment status. Requires editor+ role.
// @Tags Comments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Comment ID"
// @Param request body UpdateCommentReq true "Status update"
// @Success 200 {object} map[string]interface{}
// @Router /api/comments/:id [put]
func (h *Handler) UpdateStatus(c *fiber.Ctx) error {
	commentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid comment ID", nil)
	}

	var req UpdateCommentReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if err := h.svc.UpdateStatus(c.Context(), commentID, req.Status); err != nil {
		return response.Error(c, "BAD_REQUEST", "Failed to update comment status", nil)
	}

	return response.Success(c, nil, "Comment status updated to "+req.Status)
}

// Delete godoc
// @Summary Delete a comment permanently
// @Description Hard-deletes a comment. Requires admin+ role.
// @Tags Comments
// @Produce json
// @Security BearerAuth
// @Param id path string true "Comment ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/comments/:id [delete]
func (h *Handler) Delete(c *fiber.Ctx) error {
	commentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid comment ID", nil)
	}

	if err := h.svc.Delete(c.Context(), commentID); err != nil {
		return response.Error(c, "NOT_FOUND", "Comment not found", nil)
	}

	return response.Success(c, nil, "Comment deleted")
}
