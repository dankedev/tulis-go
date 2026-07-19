package webhook

import (
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

// Create godoc
// @Summary Create a webhook subscription
// @Tags Webhooks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateWebhookReq true "Webhook config"
// @Router /api/webhooks [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	var req CreateWebhookReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	w, err := h.svc.Create(c.Context(), workspaceID, req)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, w.ToResponse(), "Webhook created")
}

// List godoc
// @Summary List webhook subscriptions
// @Tags Webhooks
// @Produce json
// @Security BearerAuth
// @Router /api/webhooks [get]
func (h *Handler) List(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	hooks, err := h.svc.List(c.Context(), workspaceID)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "No webhooks found", nil)
	}

	items := make([]WebhookResponse, len(hooks))
	for i, hk := range hooks {
		items[i] = hk.ToResponse()
	}

	return response.Success(c, items, "Webhooks retrieved")
}

// Delete godoc
// @Summary Delete a webhook
// @Tags Webhooks
// @Produce json
// @Security BearerAuth
// @Param id path string true "Webhook ID"
// @Router /api/webhooks/:id [delete]
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid webhook ID", nil)
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return response.Error(c, "NOT_FOUND", "Webhook not found", nil)
	}

	return response.Success(c, nil, "Webhook deleted")
}

// GetLogs godoc
// @Summary Get webhook delivery logs
// @Tags Webhooks
// @Produce json
// @Security BearerAuth
// @Param id path string true "Webhook ID"
// @Router /api/webhooks/:id/logs [get]
func (h *Handler) GetLogs(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid webhook ID", nil)
	}

	logs, err := h.svc.GetDeliveryLogs(c.Context(), id, 20)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "No logs found", nil)
	}

	return response.Success(c, logs, "Delivery logs retrieved")
}
