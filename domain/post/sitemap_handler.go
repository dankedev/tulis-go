package post

import (
	"encoding/xml"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// SitemapURL represents a single URL entry in the sitemap.
type SitemapURL struct {
	XMLName    xml.Name `xml:"url"`
	Loc        string   `xml:"loc"`
	LastMod    string   `xml:"lastmod,omitempty"`
	ChangeFreq string   `xml:"changefreq,omitempty"`
	Priority   string   `xml:"priority,omitempty"`
}

// SitemapURLSet is the root sitemap element.
type SitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []SitemapURL `xml:"url"`
}

// SitemapHandler handles dynamic XML sitemap generation.
type SitemapHandler struct {
	svc PostService
}

func NewSitemapHandler(svc PostService) *SitemapHandler {
	return &SitemapHandler{svc: svc}
}

// GetSitemap generates an XML sitemap for all published content in a workspace.
// GET /api/workspaces/:id/sitemap
func (h *SitemapHandler) GetSitemap(c *fiber.Ctx) error {
	wsIDStr := c.Params("id")
	if wsIDStr == "" {
		wsIDStr = c.Locals("workspace_id").(string)
	}

	workspaceID, err := uuid.Parse(wsIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid workspace ID")
	}

	baseURL := c.Query("base_url", c.Hostname())
	scheme := "https"
	if c.Protocol() == "HTTP/1.1" || c.Protocol() == "HTTP/1.0" {
		if c.Hostname() == "localhost" || c.Hostname() == "127.0.0.1" {
			scheme = "http"
		}
	}

	// Get all published posts and pages
	posts, _, err := h.svc.ListPublicPosts(c.Context(), workspaceID, "", "", "updated_at", 1, 5000)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate sitemap")
	}

	urls := make([]SitemapURL, 0, len(posts)+1)

	// Homepage
	urls = append(urls, SitemapURL{
		Loc:        scheme + "://" + baseURL,
		ChangeFreq: "daily",
		Priority:   "1.0",
		LastMod:    time.Now().UTC().Format("2006-01-02"),
	})

	for _, post := range posts {
		lastMod := post.UpdatedAt.UTC().Format("2006-01-02")
		if post.PublishedAt != nil {
			lastMod = post.PublishedAt.UTC().Format("2006-01-02")
		}

		changeFreq := "monthly"
		priority := "0.6"
		if post.PostType == "page" {
			priority = "0.8"
			changeFreq = "weekly"
		}

		urls = append(urls, SitemapURL{
			Loc:        scheme + "://" + baseURL + "/" + post.Slug,
			LastMod:    lastMod,
			ChangeFreq: changeFreq,
			Priority:   priority,
		})
	}

	sitemap := SitemapURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	xmlBytes, err := xml.MarshalIndent(sitemap, "", "  ")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate sitemap")
	}

	c.Set("Content-Type", "application/xml; charset=utf-8")
	// Prepend XML declaration
	output := xml.Header + string(xmlBytes)
	return c.SendString(output)
}
