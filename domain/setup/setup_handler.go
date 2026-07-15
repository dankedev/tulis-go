package setup

import (
	"context"
	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
)

type SetupHandler struct {
	setupSvc SetupService
}

func NewSetupHandler(setupSvc SetupService) *SetupHandler {
	return &SetupHandler{setupSvc: setupSvc}
}

// GetStatus godoc
// @Summary Check if setup is completed
// @Description Returns whether the initial setup has been completed
// @Tags Setup
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/setup/status [get]
func (h *SetupHandler) GetStatus(c *fiber.Ctx) error {
	completed, err := h.setupSvc.IsSetupCompleted(c.Context())
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to check setup status", nil)
	}

	return response.Success(c, SetupStatusResponse{
		IsSetupCompleted: completed,
	}, "Setup status retrieved successfully")
}

// RunSetup godoc
// @Summary Run initial setup
// @Description Creates the first superadmin user and default workspace
// @Tags Setup
// @Accept json
// @Produce json
// @Param request body SetupRequest true "Setup details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/setup [post]
func (h *SetupHandler) RunSetup(c *fiber.Ctx) error {
	var req SetupRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.WorkspaceName == "" || req.AdminName == "" || req.AdminEmail == "" || req.AdminPassword == "" {
		return response.Error(c, "VALIDATION_ERROR", "All fields are required", nil)
	}

	if len(req.AdminPassword) < 6 {
		return response.Error(c, "VALIDATION_ERROR", "Password must be at least 6 characters", nil)
	}

	createdUser, token, ws, err := h.setupSvc.RunSetup(c.Context(), req)
	if err != nil {
		if err == context.Canceled {
			return response.Error(c, "FORBIDDEN", "Setup has already been completed", nil)
		}
		return response.Error(c, "INTERNAL_ERROR", err.Error(), nil)
	}

	return response.Success(c, fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":    createdUser.ID,
			"name":  createdUser.Name,
			"email": createdUser.Email,
			"role":  createdUser.Role,
		},
		"workspace": fiber.Map{
			"id":   ws.ID,
			"name": ws.Name,
			"slug": ws.Slug,
		},
	}, "Setup completed successfully")
}
