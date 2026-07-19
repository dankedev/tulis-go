package analytics

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// RecordView godoc
// @Summary Record a page view
// @Description Records a privacy-friendly page view for analytics
// @Tags Analytics
// @Produce json
// @Param id path string true "Post ID"
// @Router /v1/analytics/view/:id [post]
func (h *Handler) RecordView(c *fiber.Ctx) error {
	postID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}

	pv := &PageView{
		PostID:    postID,
		Referrer:  c.Get("Referer", ""),
		UserAgent: c.Get("User-Agent", ""),
		IPHash:    hashIP(c.IP()),
	}
	_ = h.repo.Record(c.Context(), pv)

	return c.JSON(fiber.Map{"status": 200, "message": "ok"})
}

// GetDashboard godoc
// @Summary Get analytics dashboard data
// @Description Returns views by day, top posts, and top referrers
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Router /api/analytics [get]
func (h *Handler) GetDashboard(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	days, _ := strconv.Atoi(c.Query("days", "30"))

	viewsByDay, _ := h.repo.ViewsByDay(c.Context(), workspaceID, days)
	topPosts, _ := h.repo.TopPosts(c.Context(), workspaceID, 10)
	topRefs, _ := h.repo.TopReferrers(c.Context(), workspaceID, 5)
	totalViews, _ := h.repo.TotalViews(c.Context(), workspaceID)

	return response.Success(c, fiber.Map{
		"total_views":  totalViews,
		"views_by_day": viewsByDay,
		"top_posts":    topPosts,
		"top_referrers": topRefs,
	}, "Analytics dashboard data")
}

func hashIP(ip string) string {
	h := sha256.Sum256([]byte(ip + "tulis-salt"))
	return hex.EncodeToString(h[:])
}
