package routes

import (
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/gofiber/fiber/v2"
)

func RegisterWorkspaceRoutes(authGroup fiber.Router, wsHandler *workspace.WorkspaceHandler) {
	// Workspace management (only requires authentication)
	authGroup.Post("/workspaces", wsHandler.Create)
	authGroup.Get("/workspaces", wsHandler.List)
	authGroup.Get("/workspaces/:id", wsHandler.GetByID)
	authGroup.Put("/workspaces/:id", wsHandler.Update)
	authGroup.Delete("/workspaces/:id", wsHandler.Delete)

	// Accept invitation (requires auth)
	authGroup.Post("/invitations/:token/accept", wsHandler.AcceptInvitation)
}

func RegisterWorkspacePublicRoutes(api fiber.Router, wsHandler *workspace.WorkspaceHandler) {
	// Publicly view invitation details
	api.Get("/invitations/:token", wsHandler.GetInvitation)
}

func RegisterWorkspaceMemberRoutes(tenantGroup fiber.Router, wsHandler *workspace.WorkspaceHandler) {
	// Workspace members (requires workspace context)
	tenantGroup.Post("/workspaces/:id/members", wsHandler.AddMember)
	tenantGroup.Get("/workspaces/:id/members", wsHandler.ListMembers)
	tenantGroup.Put("/workspaces/:id/members/:userId", wsHandler.UpdateMemberRole)
	tenantGroup.Delete("/workspaces/:id/members/:userId", wsHandler.RemoveMember)

	// Invite member (requires workspace context)
	tenantGroup.Post("/workspaces/:id/invitations", wsHandler.InviteMember)
	tenantGroup.Get("/workspaces/:id/invitations", wsHandler.ListInvitations)
	tenantGroup.Delete("/workspaces/:id/invitations/:inviteId", wsHandler.RevokeInvitation)
}
