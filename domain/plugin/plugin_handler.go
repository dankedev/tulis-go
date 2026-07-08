// Package plugin Kontent CMS Plugin API
//
//	Plugin listing, toggle, and settings management
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
package plugin

import (
	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// List godoc
// @Summary List workspace plugins
// @Description Returns all plugins available in the workspace with enabled status and settings
// @Tags Plugins
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/plugins [get]
func (h *Handler) List(c *fiber.Ctx) error {
	workspaceIDStr := c.Get("X-Workspace-ID")
	if workspaceIDStr == "" {
		return response.Error(c, "BAD_REQUEST", "X-Workspace-ID header is required", nil)
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid Workspace ID format", nil)
	}

	plugins, err := h.service.ListPlugins(c.UserContext(), workspaceID)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", err.Error(), nil)
	}

	return response.Success(c, plugins, "Plugins loaded successfully")
}

// Toggle godoc
// @Summary Toggle plugin enabled state
// @Description Enables or disables a plugin for the workspace
// @Tags Plugins
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Plugin ID"
// @Param request body TogglePluginReq true "Enabled state"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/plugins/{id}/toggle [post]
func (h *Handler) Toggle(c *fiber.Ctx) error {
	pluginID := c.Params("id")
	if pluginID == "" {
		return response.Error(c, "BAD_REQUEST", "Plugin ID is required", nil)
	}

	workspaceIDStr := c.Get("X-Workspace-ID")
	if workspaceIDStr == "" {
		return response.Error(c, "BAD_REQUEST", "X-Workspace-ID header is required", nil)
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid Workspace ID format", nil)
	}

	var body TogglePluginReq
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	err = h.service.TogglePlugin(c.UserContext(), workspaceID, pluginID, body.Enabled)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", err.Error(), nil)
	}

	statusMsg := "disabled"
	if body.Enabled {
		statusMsg = "enabled"
	}

	return response.Success(c, nil, "Plugin "+statusMsg+" successfully")
}

// SaveSettings godoc
// @Summary Save plugin settings
// @Description Saves configuration settings for a plugin
// @Tags Plugins
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Plugin ID"
// @Param request body SaveSettingsReq true "Plugin settings"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/plugins/{id}/settings [put]
func (h *Handler) SaveSettings(c *fiber.Ctx) error {
	pluginID := c.Params("id")
	if pluginID == "" {
		return response.Error(c, "BAD_REQUEST", "Plugin ID is required", nil)
	}

	workspaceIDStr := c.Get("X-Workspace-ID")
	if workspaceIDStr == "" {
		return response.Error(c, "BAD_REQUEST", "X-Workspace-ID header is required", nil)
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid Workspace ID format", nil)
	}

	var body SaveSettingsReq
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	err = h.service.SaveSettings(c.UserContext(), workspaceID, pluginID, body.Settings)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", err.Error(), nil)
	}

	return response.Success(c, nil, "Plugin settings saved successfully")
}
