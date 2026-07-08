package routes

import (
	"github.com/dankedev/tulis-go/domain/post"
	"github.com/gofiber/fiber/v2"
)

func RegisterPostRoutes(publicApi fiber.Router, tenantGroup fiber.Router, postHandler *post.PostHandler, publicPostHandler *post.PublicHandler) {
	// Public Post Routes
	publicApi.Get("/posts", publicPostHandler.ListPosts)
	publicApi.Get("/posts/:slugOrId", publicPostHandler.GetPost)

	// Content CRUD & Custom Post Types (CPT)
	tenantGroup.Post("/posts", postHandler.Create)
	tenantGroup.Get("/posts", postHandler.List)
	tenantGroup.Get("/posts/:id", postHandler.GetByID)
	tenantGroup.Put("/posts/:id", postHandler.Update)
	tenantGroup.Delete("/posts/:id", postHandler.Delete)

	tenantGroup.Post("/post-types", postHandler.RegisterPostType)
	tenantGroup.Get("/post-types", postHandler.ListPostTypes)
	tenantGroup.Get("/post-types/:id", postHandler.GetPostTypeByID)
	tenantGroup.Delete("/post-types/:id", postHandler.DeletePostType)

	// Post Revisions
	tenantGroup.Get("/posts/:id/revisions", postHandler.ListRevisions)
	tenantGroup.Post("/posts/:id/revisions/:revisionId/restore", postHandler.RestoreRevision)
}
