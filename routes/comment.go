package routes

import (
	"github.com/dankedev/tulis-go/domain/comment"
	"github.com/gofiber/fiber/v2"
)

func RegisterCommentRoutes(publicApi fiber.Router, tenantGroup fiber.Router, handler *comment.Handler) {
	// Public: create and list comments for a specific post
	publicApi.Post("/comments", handler.CreatePublic)
	publicApi.Get("/posts/:post_id/comments", handler.ListByPost)

	// Admin moderation (requires auth + workspace context)
	tenantGroup.Get("/comments", handler.ListByWorkspace)
	tenantGroup.Put("/comments/:id", handler.UpdateStatus)
	tenantGroup.Delete("/comments/:id", handler.Delete)
}
