// Package post Kontent CMS Public API
//
//	Public-facing read-only endpoints for posts and taxonomies
//
//	Schemes: http
//	BasePath: /api/v1/public
//	Version: 1.0.0
package post

import (
	"strconv"

	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type PublicHandler struct {
	postSvc PostService
}

func NewPublicHandler(postSvc PostService) *PublicHandler {
	return &PublicHandler{postSvc: postSvc}
}

// ListPosts godoc
// @Summary List published posts
// @Description Returns a paginated list of published posts for the workspace (public, rate-limited)
// @Tags Public Posts
// @Accept json
// @Produce json
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param type query string false "Filter by post type"
// @Param taxonomy query string false "Filter by taxonomy slug"
// @Param sort query string false "Sort order" default(published_at desc)
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/public/posts [get]
func (h *PublicHandler) ListPosts(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	postType := c.Query("type", "")
	taxonomy := c.Query("taxonomy", "")
	sortBy := c.Query("sort", "published_at desc")

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "10"))

	posts, total, err := h.postSvc.ListPublicPosts(c.Context(), workspaceID, postType, taxonomy, sortBy, page, perPage)
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
		"message": "Public posts retrieved successfully",
		"data":    posts,
		"meta":    meta,
	})
}

// GetPost godoc
// @Summary Get a published post
// @Description Returns a single published post by slug or ID (public, rate-limited)
// @Tags Public Posts
// @Accept json
// @Produce json
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param slugOrId path string true "Post slug or UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/public/posts/{slugOrId} [get]
func (h *PublicHandler) GetPost(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	slugOrID := c.Params("slugOrId")
	post, err := h.postSvc.GetPublicPostBySlugOrID(c.Context(), workspaceID, slugOrID)
	if err != nil {
		return response.Error(c, "NOT_FOUND", err.Error(), nil)
	}

	return response.Success(c, post, "Public post retrieved successfully")
}

// ListTaxonomies godoc
// @Summary List taxonomies
// @Description Returns all taxonomies for the workspace (public, rate-limited)
// @Tags Public Taxonomies
// @Accept json
// @Produce json
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param type query string false "Filter by taxonomy type"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/public/taxonomies [get]
func (h *PublicHandler) ListTaxonomies(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	taxType := c.Query("type", "")

	taxonomies, err := h.postSvc.ListTaxonomies(c.Context(), workspaceID, taxType)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, taxonomies, "Public taxonomies retrieved successfully")
}
