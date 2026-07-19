package ai

import (
	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	provider Provider
}

func NewHandler(provider Provider) *Handler {
	return &Handler{provider: provider}
}

// GenerateTitles godoc
// @Summary Generate AI-powered title suggestions
// @Description Uses AI to generate SEO-optimized post titles
// @Tags AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body map[string]string true "JSON with topic and keywords"
// @Router /api/ai/generate-titles [post]
func (h *Handler) GenerateTitles(c *fiber.Ctx) error {
	if h.provider == nil {
		return response.Error(c, "SERVICE_UNAVAILABLE", "AI is not configured. Set AI_API_KEY and AI_PROVIDER env vars.", nil)
	}

	var req struct {
		Topic    string `json:"topic"`
		Keywords string `json:"keywords"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}
	if req.Topic == "" {
		return response.Error(c, "BAD_REQUEST", "topic is required", nil)
	}

	titles, err := GenerateTitles(c.Context(), h.provider, req.Topic, req.Keywords)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, fiber.Map{"titles": titles}, "Titles generated")
}

// GenerateMetaDescription godoc
// @Summary Generate AI-powered meta description
// @Description Uses AI to generate an SEO meta description
// @Tags AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body map[string]string true "JSON with title and content"
// @Router /api/ai/generate-meta [post]
func (h *Handler) GenerateMetaDescription(c *fiber.Ctx) error {
	if h.provider == nil {
		return response.Error(c, "SERVICE_UNAVAILABLE", "AI is not configured", nil)
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	desc, err := GenerateMetaDescription(c.Context(), h.provider, req.Title, req.Content)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, fiber.Map{"description": desc}, "Meta description generated")
}

// SuggestTaxonomies godoc
// @Summary Generate AI-powered taxonomy suggestions
// @Description Uses AI to suggest categories and tags for post content
// @Tags AI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body map[string]string true "JSON with title and content"
// @Router /api/ai/suggest-taxonomies [post]
func (h *Handler) SuggestTaxonomies(c *fiber.Ctx) error {
	if h.provider == nil {
		return response.Error(c, "SERVICE_UNAVAILABLE", "AI is not configured", nil)
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	cats, tags, err := SuggestTaxonomies(c.Context(), h.provider, req.Title, req.Content)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, fiber.Map{
		"categories": cats,
		"tags":       tags,
	}, "Taxonomies suggested")
}
