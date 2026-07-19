package routes

import (
	"github.com/dankedev/tulis-go/domain/apikey"
	"github.com/gofiber/fiber/v2"
)

func RegisterApiKeyRoutes(tenantGroup fiber.Router, handler *apikey.Handler) {
	tenantGroup.Post("/api-keys", handler.Generate)
	tenantGroup.Get("/api-keys", handler.List)
	tenantGroup.Put("/api-keys/:id", handler.Revoke)
	tenantGroup.Delete("/api-keys/:id", handler.Delete)
}
