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

// Members
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
