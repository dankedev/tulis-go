// Package workspaces Kontent CMS Workspace API
//
//	Workspace and member management endpoints
//
//	Schemes: http
//	BasePath: /api
//	Version: 1.0.0
//
//	SecurityDefinitions:
//	Bearer:
//	     type: apiKey
//	     name: Authorization
//	     in: header
//
package workspace

import (
	"github.com/dankedev/kontent/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type WorkspaceHandler struct {
	svc WorkspaceService
}

func NewWorkspaceHandler(svc WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{svc: svc}
}

// Create godoc
// @Summary Create a new workspace
// @Description Creates a new workspace and sets the authenticated user as superadmin
// @Tags Workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param request body CreateWorkspaceReq true "Workspace details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/workspaces [post]
func (h *WorkspaceHandler) Create(c *fiber.Ctx) error {
	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}

	userID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	var req CreateWorkspaceReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.Name == "" {
		return response.Error(c, "VALIDATION_ERROR", "Workspace name is required", nil)
	}

	ws, err := h.svc.CreateWorkspace(c.Context(), req.Name, req.Slug, req.Plan, userID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, ws, "Workspace created successfully")
}

// GetByID godoc
// @Summary Get workspace by ID
// @Description Returns a single workspace by its UUID
// @Tags Workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Workspace UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/workspaces/{id} [get]
func (h *WorkspaceHandler) GetByID(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	ws, err := h.svc.GetWorkspaceByID(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", err.Error(), nil)
	}

	return response.Success(c, ws, "Workspace retrieved successfully")
}

// Update godoc
// @Summary Update workspace
// @Description Updates the name, slug, and settings of a workspace
// @Tags Workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Workspace UUID"
// @Param request body UpdateWorkspaceReq true "Update fields"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/workspaces/{id} [put]
func (h *WorkspaceHandler) Update(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	var req UpdateWorkspaceReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	ws, err := h.svc.UpdateWorkspace(c.Context(), id, req.Name, req.Slug, req.Settings)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, ws, "Workspace updated successfully")
}

// Delete godoc
// @Summary Delete workspace
// @Description Permanently deletes a workspace and all its content
// @Tags Workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Workspace UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/workspaces/{id} [delete]
func (h *WorkspaceHandler) Delete(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	if err := h.svc.DeleteWorkspace(c.Context(), id); err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, nil, "Workspace deleted successfully")
}

// List godoc
// @Summary List workspaces for current user
// @Description Returns all workspaces the authenticated user is a member of
// @Tags Workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/workspaces [get]
func (h *WorkspaceHandler) List(c *fiber.Ctx) error {
	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}

	userID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	workspaces, err := h.svc.ListWorkspaces(c.Context(), userID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, workspaces, "Workspaces retrieved successfully")
}

// AddMember godoc
// @Summary Add workspace member
// @Description Adds a user to the workspace with a specified role
// @Tags Workspace Members
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Workspace UUID"
// @Param request body AddMemberReq true "Member user ID and role"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/workspaces/{id}/members [post]
func (h *WorkspaceHandler) AddMember(c *fiber.Ctx) error {
	wsIDStr := c.Params("id")
	wsID, err := uuid.Parse(wsIDStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	var req AddMemberReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	memberUserID, err := uuid.Parse(req.UserID)
	if err != nil {
		return response.Error(c, "VALIDATION_ERROR", "Invalid user ID", nil)
	}

	if req.Role == "" {
		req.Role = "subscriber"
	}

	member, err := h.svc.AddMember(c.Context(), wsID, memberUserID, req.Role)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, member, "Member added successfully")
}

// ListMembers godoc
// @Summary List workspace members
// @Description Returns all members of a workspace with their roles
// @Tags Workspace Members
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Workspace UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/workspaces/{id}/members [get]
func (h *WorkspaceHandler) ListMembers(c *fiber.Ctx) error {
	wsIDStr := c.Params("id")
	wsID, err := uuid.Parse(wsIDStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	members, err := h.svc.ListMembers(c.Context(), wsID)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, members, "Workspace members retrieved successfully")
}

// UpdateMemberRole godoc
// @Summary Update member role
// @Description Changes the role of a workspace member
// @Tags Workspace Members
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Workspace UUID"
// @Param userId path string true "Member User UUID"
// @Param request body UpdateMemberRoleReq true "New role"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/workspaces/{id}/members/{userId} [put]
func (h *WorkspaceHandler) UpdateMemberRole(c *fiber.Ctx) error {
	wsIDStr := c.Params("id")
	wsID, err := uuid.Parse(wsIDStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	memberUserIDStr := c.Params("userId")
	memberUserID, err := uuid.Parse(memberUserIDStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid member user ID", nil)
	}

	var req UpdateMemberRoleReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.Role == "" {
		return response.Error(c, "VALIDATION_ERROR", "Role is required", nil)
	}

	member, err := h.svc.UpdateMemberRole(c.Context(), wsID, memberUserID, req.Role)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, member, "Member role updated successfully")
}

// RemoveMember godoc
// @Summary Remove workspace member
// @Description Removes a user from the workspace
// @Tags Workspace Members
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Workspace UUID"
// @Param userId path string true "Member User UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/workspaces/{id}/members/{userId} [delete]
func (h *WorkspaceHandler) RemoveMember(c *fiber.Ctx) error {
	wsIDStr := c.Params("id")
	wsID, err := uuid.Parse(wsIDStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	memberUserIDStr := c.Params("userId")
	memberUserID, err := uuid.Parse(memberUserIDStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid member user ID", nil)
	}

	if err := h.svc.RemoveMember(c.Context(), wsID, memberUserID); err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, nil, "Member removed successfully")
}
