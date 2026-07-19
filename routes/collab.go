package routes

import (
	"github.com/dankedev/tulis-go/domain/collab"
	"github.com/gofiber/fiber/v2"
)

func RegisterCollabRoutes(tenantGroup fiber.Router, handler *collab.Handler) {
	tenantGroup.Post("/collab/lock/:id", handler.AcquireLock)
	tenantGroup.Post("/collab/unlock/:id", handler.ReleaseLock)
	tenantGroup.Get("/collab/lock/:id", handler.GetLock)
}
