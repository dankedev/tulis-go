package routes

import (
	"github.com/dankedev/tulis-go/domain/plugin"
	"github.com/gofiber/fiber/v2"
)

func RegisterPluginRoutes(contentGroup fiber.Router, editorGroup fiber.Router, pluginHandler *plugin.Handler) {
	// Authors can view plugins
	contentGroup.Get("/plugins", pluginHandler.List)
	// Only editors can manage plugins
	editorGroup.Post("/plugins/:id/toggle", pluginHandler.Toggle)
	editorGroup.Put("/plugins/:id/settings", pluginHandler.SaveSettings)
}
