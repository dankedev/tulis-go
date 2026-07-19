package routes

import (
	"github.com/dankedev/tulis-go/domain/analytics"
	"github.com/gofiber/fiber/v2"
)

func RegisterAnalyticsRoutes(publicApi fiber.Router, tenantGroup fiber.Router, handler *analytics.Handler) {
	publicApi.Post("/analytics/view/:id", handler.RecordView)
	tenantGroup.Get("/analytics", handler.GetDashboard)
}
