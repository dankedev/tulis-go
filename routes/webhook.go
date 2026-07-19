package routes

import (
	"github.com/dankedev/tulis-go/domain/webhook"
	"github.com/gofiber/fiber/v2"
)

func RegisterWebhookRoutes(tenantGroup fiber.Router, handler *webhook.Handler) {
	tenantGroup.Post("/webhooks", handler.Create)
	tenantGroup.Get("/webhooks", handler.List)
	tenantGroup.Delete("/webhooks/:id", handler.Delete)
	tenantGroup.Get("/webhooks/:id/logs", handler.GetLogs)
}
