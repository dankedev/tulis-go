package routes

import (
	"github.com/dankedev/tulis-go/domain/ai"
	"github.com/gofiber/fiber/v2"
)

func RegisterAIRoutes(tenantGroup fiber.Router, handler *ai.Handler) {
	tenantGroup.Post("/ai/generate-titles", handler.GenerateTitles)
	tenantGroup.Post("/ai/generate-meta", handler.GenerateMetaDescription)
	tenantGroup.Post("/ai/suggest-taxonomies", handler.SuggestTaxonomies)
	tenantGroup.Post("/ai/generate-snippets", handler.GenerateSocialSnippets)
}
