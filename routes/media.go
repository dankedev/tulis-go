package routes

import (
	"github.com/dankedev/tulis-go/domain/media"
	"github.com/gofiber/fiber/v2"
)

func RegisterMediaRoutes(publicApi fiber.Router, authorGroup fiber.Router, editorGroup fiber.Router, mediaHandler *media.MediaHandler, publicMediaHandler *media.PublicHandler) {
	// Public Media Routes
	publicApi.Get("/media", publicMediaHandler.ListMedia)

	// Author+ can upload and view
	authorGroup.Post("/media/upload", mediaHandler.Upload)
	authorGroup.Get("/media", mediaHandler.List)
	authorGroup.Get("/media/:id", mediaHandler.GetByID)
	authorGroup.Post("/media/upload-via-url", mediaHandler.UploadViaURL)

	// Editor+ can delete and update
	editorGroup.Put("/media/:id", mediaHandler.Update)
	editorGroup.Delete("/media/:id", mediaHandler.Delete)
}
