package notification

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type NotificationHandler struct {
	svc         *NotificationService
	telegramBot *TelegramBotService
	repo        Repository
}

func NewNotificationHandler(svc *NotificationService, telegramBot *TelegramBotService, repo Repository) *NotificationHandler {
	return &NotificationHandler{
		svc:         svc,
		telegramBot: telegramBot,
		repo:        repo,
	}
}

// GetPreferences GET /api/notifications/preferences
func (h *NotificationHandler) GetPreferences(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return response.Error(c, "UNAUTHORIZED", "Unauthorized", nil)
	}
	userID, _ := uuid.Parse(userIDStr)

	wsIDStr, ok := c.Locals("workspace_id").(string)
	if !ok {
		return response.Error(c, "BAD_REQUEST", "Workspace ID required", nil)
	}
	wsID, _ := uuid.Parse(wsIDStr)

	prefs, err := h.svc.GetUserPreferences(c.Context(), userID, wsID)
	if err != nil {
		return response.Error(c, "INTERNAL_SERVER_ERROR", err.Error(), nil)
	}

	telegramBinding, _ := h.repo.GetTelegramBindingByUserID(c.Context(), userID)
	botConfig, _ := h.repo.GetTelegramBotConfig(c.Context(), wsID)

	botUsername := ""
	if botConfig != nil {
		botUsername = botConfig.BotUsername
	}

	return response.Success(c, fiber.Map{
		"preferences":       prefs,
		"telegram_linked":   telegramBinding != nil && telegramBinding.IsVerified,
		"telegram_chat_id":  func() int64 { if telegramBinding != nil { return telegramBinding.TelegramChatID }; return 0 }(),
		"telegram_username": func() string { if telegramBinding != nil { return telegramBinding.TelegramUsername }; return "" }(),
		"telegram_bot_name": botUsername,
		"has_bot_token":     botConfig != nil && botConfig.BotToken != "",
	}, "Notification preferences retrieved")
}

// UpdatePreferences PUT /api/notifications/preferences
func (h *NotificationHandler) UpdatePreferences(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return response.Error(c, "UNAUTHORIZED", "Unauthorized", nil)
	}
	userID, _ := uuid.Parse(userIDStr)

	wsIDStr, ok := c.Locals("workspace_id").(string)
	if !ok {
		return response.Error(c, "BAD_REQUEST", "Workspace ID required", nil)
	}
	wsID, _ := uuid.Parse(wsIDStr)

	var req struct {
		Preferences []NotificationPreference `json:"preferences"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if err := h.svc.UpdateUserPreferences(c.Context(), userID, wsID, req.Preferences); err != nil {
		return response.Error(c, "INTERNAL_SERVER_ERROR", err.Error(), nil)
	}

	return response.Success(c, nil, "Preferences updated successfully")
}

// SaveTelegramBotToken POST /api/notifications/telegram/token
func (h *NotificationHandler) SaveTelegramBotToken(c *fiber.Ctx) error {
	wsIDStr, ok := c.Locals("workspace_id").(string)
	if !ok {
		return response.Error(c, "BAD_REQUEST", "Workspace ID required", nil)
	}
	wsID, _ := uuid.Parse(wsIDStr)

	var req struct {
		BotToken    string `json:"bot_token"`
		BotUsername string `json:"bot_username"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid payload", nil)
	}

	cfg := &TelegramBotConfig{
		WorkspaceID: wsID,
		BotToken:    req.BotToken,
		BotUsername: req.BotUsername,
		IsActive:    true,
	}

	if err := h.repo.SaveTelegramBotConfig(c.Context(), cfg); err != nil {
		return response.Error(c, "INTERNAL_SERVER_ERROR", err.Error(), nil)
	}

	return response.Success(c, nil, "Telegram Bot token saved successfully")
}

// GenerateTelegramLinkCode POST /api/notifications/telegram/link-code
func (h *NotificationHandler) GenerateTelegramLinkCode(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return response.Error(c, "UNAUTHORIZED", "Unauthorized", nil)
	}
	userID, _ := uuid.Parse(userIDStr)

	wsIDStr, _ := c.Locals("workspace_id").(string)
	wsID, _ := uuid.Parse(wsIDStr)

	bytes := make([]byte, 4)
	rand.Read(bytes)
	code := hex.EncodeToString(bytes)
	exp := time.Now().Add(15 * time.Minute)

	binding, err := h.repo.GetTelegramBindingByUserID(c.Context(), userID)
	if err != nil || binding == nil {
		binding = &TelegramUserBinding{
			UserID:            userID,
			VerificationCode:  code,
			VerificationExpAt: &exp,
			IsVerified:        false,
		}
	} else {
		binding.VerificationCode = code
		binding.VerificationExpAt = &exp
	}

	if wsID != uuid.Nil {
		binding.ActiveWorkspaceID = &wsID
	}

	if err := h.repo.SaveTelegramBinding(c.Context(), binding); err != nil {
		return response.Error(c, "INTERNAL_SERVER_ERROR", err.Error(), nil)
	}

	botConfig, _ := h.repo.GetTelegramBotConfig(c.Context(), wsID)
	botUsername := ""
	if botConfig != nil {
		botUsername = botConfig.BotUsername
	}

	return response.Success(c, fiber.Map{
		"code":          code,
		"expires_at":    exp,
		"bot_username":  botUsername,
		"deep_link_url": fmt.Sprintf("https://t.me/%s?start=%s", botUsername, code),
	}, "Pairing code generated")
}

// TelegramWebhook POST /api/notifications/telegram/webhook (or /webhook/:workspace_id)
func (h *NotificationHandler) TelegramWebhook(c *fiber.Ctx) error {
	wsIDStr := c.Params("workspace_id")
	var wsID uuid.UUID
	if wsIDStr != "" {
		wsID, _ = uuid.Parse(wsIDStr)
	}

	var update TelegramUpdate
	if err := c.BodyParser(&update); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid webhook body", nil)
	}

	if err := h.telegramBot.ProcessUpdate(c.Context(), wsID, update); err != nil {
		// Log error but respond HTTP 200 to Telegram so it doesn't retry endlessly
		fmt.Printf("[TelegramWebhook] Error processing update: %v\n", err)
	}

	return c.SendStatus(fiber.StatusOK)
}
