package routes

import (
	"github.com/dankedev/tulis-go/domain/user"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/gofiber/fiber/v2"
)

func RegisterAdminRoutes(adminGroup fiber.Router, authHandler *user.AuthHandler, wsHandler *workspace.WorkspaceHandler) {
	// Users management (Superadmin only)
	adminGroup.Get("/users", authHandler.AdminListUsers)
	adminGroup.Put("/users/:id", authHandler.AdminUpdateUser)
	adminGroup.Delete("/users/:id", authHandler.AdminDeleteUser)

	// Workspaces management (Superadmin only)
	adminGroup.Get("/workspaces", wsHandler.AdminListWorkspaces)
	adminGroup.Put("/workspaces/:id", wsHandler.AdminUpdateWorkspace)
	adminGroup.Delete("/workspaces/:id", wsHandler.AdminDeleteWorkspace)
}
