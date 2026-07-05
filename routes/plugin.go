package routes

import (
	"github.com/dankedev/kontent/domain/plugin"
	"github.com/gofiber/fiber/v2"
)

func RegisterPluginRoutes(tenantGroup fiber.Router, pluginHandler *plugin.Handler) {
	// Plugin Management
	tenantGroup.Get("/plugins", pluginHandler.List)
	tenantGroup.Post("/plugins/:id/toggle", pluginHandler.Toggle)
	tenantGroup.Put("/plugins/:id/settings", pluginHandler.SaveSettings)
}
