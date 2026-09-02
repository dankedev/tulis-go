package routes

import (
	"github.com/dankedev/tulis-go/domain/post"
	"github.com/gofiber/fiber/v2"
)

func RegisterTaxonomyRoutes(publicApi fiber.Router, tenantGroup fiber.Router, postHandler *post.PostHandler, publicPostHandler *post.PublicHandler) {
	// Public Taxonomy Routes
	publicApi.Get("/taxonomies", publicPostHandler.ListTaxonomies)
	publicApi.Get("/taxonomies/slug/:slug", publicPostHandler.GetTaxonomyBySlug)
	publicApi.Get("/taxonomies/:slug", publicPostHandler.GetTaxonomyBySlug)

	// Post Taxonomies
	tenantGroup.Post("/taxonomies", postHandler.CreateTaxonomy)
	tenantGroup.Get("/taxonomies", postHandler.ListTaxonomies)
	tenantGroup.Get("/taxonomies/slug/:slug", postHandler.GetTaxonomyBySlug)
	tenantGroup.Get("/taxonomies/:id", postHandler.GetTaxonomyByID)
	tenantGroup.Put("/taxonomies/:id", postHandler.UpdateTaxonomy)
	tenantGroup.Delete("/taxonomies/:id", postHandler.DeleteTaxonomy)
}
