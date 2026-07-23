package routes

import (
	"github.com/dankedev/tulis-go/domain/notification"
	"github.com/gofiber/fiber/v2"
)

func RegisterNotificationRoutes(publicGroup fiber.Router, tenantGroup fiber.Router, editorGroup fiber.Router, handler *notification.NotificationHandler) {
	// Public Webhook for Telegram updates
	publicGroup.Post("/notifications/telegram/webhook/:workspace_id?", handler.TelegramWebhook)

	// Tenant-scoped routes (Requires user auth + workspace context)
	tenantGroup.Get("/notifications/preferences", handler.GetPreferences)
	tenantGroup.Put("/notifications/preferences", handler.UpdatePreferences)
	tenantGroup.Post("/notifications/telegram/link-code", handler.GenerateTelegramLinkCode)

	// Editor+ routes (Only Editor/Admin can configure bot token)
	editorGroup.Post("/notifications/telegram/token", handler.SaveTelegramBotToken)
}
