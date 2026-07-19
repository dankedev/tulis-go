package post

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// OGImageHandler generates dynamic Open Graph images as SVG.
type OGImageHandler struct {
	svc PostService
}

func NewOGImageHandler(svc PostService) *OGImageHandler {
	return &OGImageHandler{svc: svc}
}

// GetOGImage generates an SVG OG image for a post.
// GET /api/og-image/:id
func (h *OGImageHandler) GetOGImage(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		// Try by slug
		wsIDStr := c.Locals("workspace_id")
		if wsIDStr == nil {
			return c.Status(400).SendString("Invalid ID")
		}
		wsID, _ := uuid.Parse(wsIDStr.(string))
		post, err := h.svc.GetPublicPostBySlugOrID(c.Context(), wsID, c.Params("id"))
		if err != nil {
			return c.Status(404).SendString("Post not found")
		}
		return h.renderSVG(c, post)
	}

	post, err := h.svc.GetPostByID(c.Context(), id)
	if err != nil {
		return c.Status(404).SendString("Post not found")
	}
	return h.renderSVG(c, post)
}

func (h *OGImageHandler) renderSVG(c *fiber.Ctx, post *Post) error {
	title := truncateStr(post.Title, 120)
	excerpt := truncateStr(post.Excerpt, 200)
	readingTime := fmt.Sprintf("%d min read", post.ReadingTime)
	if post.ReadingTime == 0 {
		readingTime = ""
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <defs>
    <linearGradient id="bg" x1="0%%" y1="0%%" x2="100%%" y2="100%%">
      <stop offset="0%%" stop-color="#0F172A"/>
      <stop offset="100%%" stop-color="#1E293B"/>
    </linearGradient>
  </defs>
  <rect width="1200" height="630" fill="url(#bg)"/>
  <rect x="80" y="80" width="1040" height="6" rx="3" fill="#2563EB" opacity="0.8"/>
  <text x="80" y="180" font-family="sans-serif" font-size="48" font-weight="bold" fill="#F8FAFC" letter-spacing="-0.5">%s</text>
  <text x="80" y="280" font-family="sans-serif" font-size="24" fill="#94A3B8" letter-spacing="0">%s</text>
  <rect x="80" y="420" width="1040" height="1" fill="#334155"/>
  <text x="80" y="480" font-family="sans-serif" font-size="20" fill="#64748B">Tulis CMS</text>
  <text x="1120" y="480" font-family="sans-serif" font-size="20" fill="#64748B" text-anchor="end">%s</text>
</svg>`, escapeXML(title), escapeXML(excerpt), readingTime)

	c.Set("Content-Type", "image/svg+xml")
	c.Set("Cache-Control", "public, max-age=86400")
	return c.SendString(svg)
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
