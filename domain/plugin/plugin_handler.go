package plugin

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/dankedev/kontent/utils/response"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

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

	var body struct {
		Enabled bool `json:"enabled"`
	}
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

	var body struct {
		Settings map[string]interface{} `json:"settings"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	err = h.service.SaveSettings(c.UserContext(), workspaceID, pluginID, body.Settings)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", err.Error(), nil)
	}

	return response.Success(c, nil, "Plugin settings saved successfully")
}
