package notification

import (
	"context"
	"fmt"

	"github.com/dankedev/tulis-go/domain/user"
	"github.com/dankedev/tulis-go/utils/mail"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationService struct {
	repo        Repository
	db          *gorm.DB
	telegramBot *TelegramBotService
	userRepo    user.UserRepository
}

func NewNotificationService(repo Repository, db *gorm.DB, telegramBot *TelegramBotService, userRepo user.UserRepository) *NotificationService {
	return &NotificationService{
		repo:        repo,
		db:          db,
		telegramBot: telegramBot,
		userRepo:    userRepo,
	}
}

// GetUserPreferences retrieves user notification preferences for a workspace
func (s *NotificationService) GetUserPreferences(ctx context.Context, userID, workspaceID uuid.UUID) ([]NotificationPreference, error) {
	prefs, err := s.repo.GetPreferences(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}

	// Default event types if not set yet
	defaultEvents := []string{"post_published", "post_updated", "comment_created", "broken_link_alert", "workspace_invite"}
	existingEvents := make(map[string]bool)
	for _, p := range prefs {
		existingEvents[p.EventType] = true
	}

	for _, evt := range defaultEvents {
		if !existingEvents[evt] {
			p := NotificationPreference{
				ID:              uuid.New(),
				UserID:          userID,
				WorkspaceID:     workspaceID,
				EventType:       evt,
				EmailEnabled:    true,
				TelegramEnabled: true,
				InAppEnabled:    true,
			}
			_ = s.repo.UpsertPreference(ctx, &p)
			prefs = append(prefs, p)
		}
	}

	return prefs, nil
}

// UpdateUserPreferences updates or creates notification matrix preferences
func (s *NotificationService) UpdateUserPreferences(ctx context.Context, userID, workspaceID uuid.UUID, prefs []NotificationPreference) error {
	for _, p := range prefs {
		p.UserID = userID
		p.WorkspaceID = workspaceID
		if err := s.repo.UpsertPreference(ctx, &p); err != nil {
			return err
		}
	}
	return nil
}

// DispatchNotification sends notification via configured channels
func (s *NotificationService) DispatchNotification(ctx context.Context, workspaceID, userID uuid.UUID, eventType, title, message string) {
	// Find user preferences for event
	prefs, err := s.repo.GetPreferences(ctx, userID, workspaceID)
	emailEnabled := true
	telegramEnabled := true

	if err == nil {
		for _, p := range prefs {
			if p.EventType == eventType {
				emailEnabled = p.EmailEnabled
				telegramEnabled = p.TelegramEnabled
				break
			}
		}
	}

	// 1. Dispatch Email if enabled
	if emailEnabled {
		u, err := s.userRepo.FindByID(ctx, userID)
		if err == nil && u != nil && u.Email != "" {
			htmlContent := mail.GetNotificationEmailTemplate(title, message)
			_ = mail.SendHTMLMail(u.Email, title, htmlContent)
			_ = s.repo.CreateLog(ctx, &NotificationLog{
				WorkspaceID: workspaceID,
				UserID:      userID,
				Channel:     "email",
				EventType:   eventType,
				Title:       title,
				Message:     message,
				Status:      "sent",
			})
		}
	}

	// 2. Dispatch Telegram if enabled
	if telegramEnabled {
		binding, err := s.repo.GetTelegramBindingByUserID(ctx, userID)
		if err == nil && binding != nil && binding.IsVerified && binding.TelegramChatID != 0 {
			formattedMsg := fmt.Sprintf("🔔 <b>%s</b>\n\n%s", title, message)
			err := s.telegramBot.SendMessage(ctx, workspaceID, binding.TelegramChatID, formattedMsg)
			status := "sent"
			errMsg := ""
			if err != nil {
				status = "failed"
				errMsg = err.Error()
			}
			_ = s.repo.CreateLog(ctx, &NotificationLog{
				WorkspaceID: workspaceID,
				UserID:      userID,
				Channel:     "telegram",
				EventType:   eventType,
				Title:       title,
				Message:     message,
				Status:      status,
				ErrorMsg:    errMsg,
			})
		}
	}
}
