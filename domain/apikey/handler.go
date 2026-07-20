package apikey

import (
	"strconv"

	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Generate godoc
// @Summary Generate a new API key
// @Description Creates a new API key for programmatic access. Returns the raw key once — store it securely.
// @Tags API Keys
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateApiKeyReq true "API key config"
// @Success 200 {object} map[string]interface{}
// @Router /api/api-keys [post]
func (h *Handler) Generate(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	var req CreateApiKeyReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}
	if req.Name == "" {
		return response.Error(c, "BAD_REQUEST", "Name is required", nil)
	}

	result, err := h.svc.Generate(c.Context(), workspaceID, req)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, result, "API key generated — copy it now, it won't be shown again")
}

// List godoc
// @Summary List all API keys for a workspace
// @Description Returns all API keys (without raw key values). Requires admin+ role.
// @Tags API Keys
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/api-keys [get]
func (h *Handler) List(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	keys, err := h.svc.ListByWorkspace(c.Context(), workspaceID)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "No API keys found", nil)
	}

	items := make([]ApiKeyListResponse, len(keys))
	for i, k := range keys {
		items[i] = k.ToListResponse()
	}

	return response.Success(c, items, "API keys retrieved")
}

// Revoke godoc
// @Summary Revoke an API key
// @Description Deactivates an API key (soft-delete). Requires admin+ role.
// @Tags API Keys
// @Produce json
// @Security BearerAuth
// @Param id path string true "API Key ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/api-keys/:id [put]
func (h *Handler) Revoke(c *fiber.Ctx) error {
	keyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid API key ID", nil)
	}

	if err := h.svc.Revoke(c.Context(), keyID); err != nil {
		return response.Error(c, "NOT_FOUND", "API key not found", nil)
	}

	return response.Success(c, nil, "API key revoked")
}

// Delete godoc
// @Summary Delete an API key permanently
// @Description Hard-deletes an API key. Requires admin+ role.
// @Tags API Keys
// @Produce json
// @Security BearerAuth
// @Param id path string true "API Key ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/api-keys/:id [delete]
func (h *Handler) Delete(c *fiber.Ctx) error {
	keyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid API key ID", nil)
	}

	if err := h.svc.Delete(c.Context(), keyID); err != nil {
		return response.Error(c, "NOT_FOUND", "API key not found", nil)
	}

	return response.Success(c, nil, "API key deleted")
}

// parsePageLimit is a convenience helper.
func parsePageLimit(c *fiber.Ctx) (int, int) {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}
