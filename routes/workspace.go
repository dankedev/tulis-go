package routes

import (
	"github.com/dankedev/kontent/domain/workspace"
	"github.com/gofiber/fiber/v2"
)

func RegisterWorkspaceRoutes(authGroup fiber.Router, wsHandler *workspace.WorkspaceHandler) {
	// Workspace management (only requires authentication)
	authGroup.Post("/workspaces", wsHandler.Create)
	authGroup.Get("/workspaces", wsHandler.List)
	authGroup.Get("/workspaces/:id", wsHandler.GetByID)
	authGroup.Put("/workspaces/:id", wsHandler.Update)
	authGroup.Delete("/workspaces/:id", wsHandler.Delete)
}

func RegisterWorkspaceMemberRoutes(tenantGroup fiber.Router, wsHandler *workspace.WorkspaceHandler) {
	// Workspace members (requires workspace context)
	tenantGroup.Post("/workspaces/:id/members", wsHandler.AddMember)
	tenantGroup.Get("/workspaces/:id/members", wsHandler.ListMembers)
	tenantGroup.Put("/workspaces/:id/members/:userId", wsHandler.UpdateMemberRole)
	tenantGroup.Delete("/workspaces/:id/members/:userId", wsHandler.RemoveMember)
}
