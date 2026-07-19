package linkchecker

import (
	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Handler exposes broken-link checker endpoints (workspace-scoped).
type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// ListBrokenLinks godoc
// @Summary List broken links in a workspace
// @Tags Link Checker
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param unresolved query bool false "Only unresolved links"
// @Success 200 {object} map[string]interface{}
// @Router /api/workspaces/{id}/broken-links [get]
func (h *Handler) ListBrokenLinks(c *fiber.Ctx) error {
	wsID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	unresolved := c.Query("unresolved", "true") == "true"
	links, err := h.svc.ListBrokenLinks(c.Context(), wsID, unresolved)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to list broken links", nil)
	}

	count, err := h.svc.CountBrokenLinks(c.Context(), wsID)
	if err != nil {
		count = int64(len(links))
	}

	return response.Success(c, fiber.Map{
		"links":        links,
		"total":        count,
		"unresolved":   unresolved,
	}, "Broken links retrieved")
}

// CheckNow godoc
// @Summary Trigger an immediate broken-link scan for a workspace
// @Tags Link Checker
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/workspaces/{id}/broken-links/check [post]
func (h *Handler) CheckNow(c *fiber.Ctx) error {
	wsID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	checked, broken, err := h.svc.CheckWorkspace(c.Context(), wsID)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to run link check", nil)
	}

	return response.Success(c, fiber.Map{
		"checked_posts": checked,
		"broken_links":  broken,
	}, "Link check completed")
}

// MarkResolved godoc
// @Summary Mark a broken link as resolved
// @Tags Link Checker
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param linkId path string true "Broken link ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/workspaces/{id}/broken-links/{linkId}/resolve [post]
func (h *Handler) MarkResolved(c *fiber.Ctx) error {
	linkID, err := uuid.Parse(c.Params("linkId"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid link ID", nil)
	}

	if err := h.svc.MarkResolved(c.Context(), linkID); err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to resolve link", nil)
	}

	return response.Success(c, nil, "Link marked as resolved")
}
