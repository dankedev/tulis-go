package routes

import (
	"github.com/dankedev/tulis-go/domain/media"
	"github.com/gofiber/fiber/v2"
)

func RegisterMediaRoutes(publicApi fiber.Router, tenantGroup fiber.Router, mediaHandler *media.MediaHandler, publicMediaHandler *media.PublicHandler) {
	// Public Media Routes
	publicApi.Get("/media", publicMediaHandler.ListMedia)

	// Media Library Management
	tenantGroup.Post("/media/upload", mediaHandler.Upload)
	tenantGroup.Get("/media", mediaHandler.List)
	tenantGroup.Get("/media/:id", mediaHandler.GetByID)
	tenantGroup.Put("/media/:id", mediaHandler.Update)
	tenantGroup.Delete("/media/:id", mediaHandler.Delete)
}
