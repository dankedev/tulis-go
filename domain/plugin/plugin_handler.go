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
		return response.Error(c, fiber.StatusBadRequest, "X-Workspace-ID header is required", nil)
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid Workspace ID format", nil)
	}

	plugins, err := h.service.ListPlugins(c.UserContext(), workspaceID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Plugins loaded successfully", plugins)
}

func (h *Handler) Toggle(c *fiber.Ctx) error {
	pluginID := c.Params("id")
	if pluginID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Plugin ID is required", nil)
	}

	workspaceIDStr := c.Get("X-Workspace-ID")
	if workspaceIDStr == "" {
		return response.Error(c, fiber.StatusBadRequest, "X-Workspace-ID header is required", nil)
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid Workspace ID format", nil)
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body", nil)
	}

	err = h.service.TogglePlugin(c.UserContext(), workspaceID, pluginID, body.Enabled)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	statusMsg := "disabled"
	if body.Enabled {
		statusMsg = "enabled"
	}

	return response.Success(c, fiber.StatusOK, "Plugin "+statusMsg+" successfully", nil)
}

func (h *Handler) SaveSettings(c *fiber.Ctx) error {
	pluginID := c.Params("id")
	if pluginID == "" {
		return response.Error(c, fiber.StatusBadRequest, "Plugin ID is required", nil)
	}

	workspaceIDStr := c.Get("X-Workspace-ID")
	if workspaceIDStr == "" {
		return response.Error(c, fiber.StatusBadRequest, "X-Workspace-ID header is required", nil)
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid Workspace ID format", nil)
	}

	var body struct {
		Settings map[string]interface{} `json:"settings"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body", nil)
	}

	err = h.service.SaveSettings(c.UserContext(), workspaceID, pluginID, body.Settings)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Plugin settings saved successfully", nil)
}
