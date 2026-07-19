package routes

import (
	"github.com/dankedev/tulis-go/domain/membership"
	"github.com/gofiber/fiber/v2"
)

func RegisterMembershipRoutes(tenantGroup fiber.Router, handler *membership.Handler) {
	tenantGroup.Post("/membership/tiers", handler.CreateTier)
	tenantGroup.Get("/membership/tiers", handler.ListTiers)
	tenantGroup.Post("/membership/subscribe", handler.Subscribe)
}
