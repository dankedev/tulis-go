package routes

import (
	"github.com/dankedev/tulis-go/domain/linkchecker"
	"github.com/gofiber/fiber/v2"
)

// RegisterLinkCheckerRoutes wires broken-link checker endpoints under a tenant-scoped group.
// The group must already enforce auth + tenant scope (workspace_id in locals).
func RegisterLinkCheckerRoutes(tenantGroup fiber.Router, handler *linkchecker.Handler) {
	tenantGroup.Get("/workspaces/:id/broken-links", handler.ListBrokenLinks)
	tenantGroup.Post("/workspaces/:id/broken-links/check", handler.CheckNow)
	tenantGroup.Post("/workspaces/:id/broken-links/:linkId/resolve", handler.MarkResolved)
}
