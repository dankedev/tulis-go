package notification

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationPreference stores event & channel notification options per user per workspace
type NotificationPreference struct {
	ID              uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	UserID          uuid.UUID      `gorm:"type:char(36);not null;index:idx_usr_ws_evt,unique" json:"user_id"`
	WorkspaceID     uuid.UUID      `gorm:"type:char(36);not null;index:idx_usr_ws_evt,unique" json:"workspace_id"`
	EventType       string         `gorm:"type:varchar(50);not null;index:idx_usr_ws_evt,unique" json:"event_type"` // post_published, post_updated, comment_created, broken_link_alert, workspace_invite
	EmailEnabled    bool           `gorm:"default:true" json:"email_enabled"`
	TelegramEnabled bool           `gorm:"default:true" json:"telegram_enabled"`
	InAppEnabled    bool           `gorm:"default:true" json:"inapp_enabled"`
}

// TelegramUserBinding maps Telegram Chat ID to Tulis User
type TelegramUserBinding struct {
	ID                uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
	UserID            uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex" json:"user_id"`
	TelegramChatID    int64          `gorm:"index" json:"telegram_chat_id"`
	TelegramUsername  string         `gorm:"type:varchar(100)" json:"telegram_username"`
	VerificationCode  string         `gorm:"type:varchar(64);index" json:"-"`
	VerificationExpAt *time.Time     `json:"-"`
	IsVerified        bool           `gorm:"default:false" json:"is_verified"`
	ActiveWorkspaceID *uuid.UUID     `gorm:"type:char(36)" json:"active_workspace_id"`
}

// TelegramBotConfig stores bot tokens per workspace (or fallback global)
type TelegramBotConfig struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex" json:"workspace_id"`
	BotToken    string         `gorm:"type:varchar(255);not null" json:"bot_token"`
	BotUsername string         `gorm:"type:varchar(100)" json:"bot_username"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
}

// NotificationLog tracks sent notification history
type NotificationLog struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);not null;index" json:"workspace_id"`
	UserID      uuid.UUID      `gorm:"type:char(36);not null;index" json:"user_id"`
	Channel     string         `gorm:"type:varchar(20);not null" json:"channel"` // email, telegram, inapp
	EventType   string         `gorm:"type:varchar(50);not null" json:"event_type"`
	Title       string         `gorm:"type:varchar(255)" json:"title"`
	Message     string         `gorm:"type:text" json:"message"`
	Status      string         `gorm:"type:varchar(20);default:'sent'" json:"status"` // sent, failed
	ErrorMsg    string         `gorm:"type:text" json:"error_msg,omitempty"`
}
