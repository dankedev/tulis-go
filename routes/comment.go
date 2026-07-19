package routes

import (
	"github.com/dankedev/tulis-go/domain/comment"
	"github.com/gofiber/fiber/v2"
)

func RegisterCommentRoutes(publicApi fiber.Router, authorGroup fiber.Router, editorGroup fiber.Router, handler *comment.Handler) {
	// Public: create and list comments for a specific post
	publicApi.Post("/comments", handler.CreatePublic)
	publicApi.Get("/posts/:post_id/comments", handler.ListByPost)

	// Moderation (editor+ only)
	editorGroup.Get("/comments", handler.ListByWorkspace)
	editorGroup.Put("/comments/:id", handler.UpdateStatus)
	editorGroup.Delete("/comments/:id", handler.Delete)
}
