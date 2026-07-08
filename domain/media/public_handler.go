// Package media Tulis CMS Public Media API
//
//	Public-facing read-only media endpoints
//
//	Schemes: http
//	BasePath: /api/v1/public
//	Version: 1.0.0
package media

import (
	"strconv"

	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type PublicHandler struct {
	mediaSvc MediaService
}

func NewPublicHandler(mediaSvc MediaService) *PublicHandler {
	return &PublicHandler{mediaSvc: mediaSvc}
}

// ListMedia godoc
// @Summary List media
// @Description Returns a paginated list of media items (public, rate-limited)
// @Tags Public Media
// @Accept json
// @Produce json
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/public/media [get]
func (h *PublicHandler) ListMedia(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "10"))
	search := c.Query("search", "")

	mediaList, total, err := h.mediaSvc.ListMedia(c.Context(), workspaceID, page, perPage, search)
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
		"message": "Public media library retrieved successfully",
		"data":    mediaList,
		"meta":    meta,
	})
}
