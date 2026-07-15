package routes

import (
	"github.com/dankedev/tulis-go/domain/setup"
	"github.com/gofiber/fiber/v2"
)

func RegisterSetupRoutes(router fiber.Router, handler *setup.SetupHandler) {
	setupGroup := router.Group("/setup")
	setupGroup.Get("/status", handler.GetStatus)
	setupGroup.Post("/", handler.RunSetup)
}
