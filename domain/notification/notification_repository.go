package notification

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	GetPreferences(ctx context.Context, userID, workspaceID uuid.UUID) ([]NotificationPreference, error)
	UpsertPreference(ctx context.Context, pref *NotificationPreference) error
	GetTelegramBindingByUserID(ctx context.Context, userID uuid.UUID) (*TelegramUserBinding, error)
	GetTelegramBindingByChatID(ctx context.Context, chatID int64) (*TelegramUserBinding, error)
	GetTelegramBindingByCode(ctx context.Context, code string) (*TelegramUserBinding, error)
	SaveTelegramBinding(ctx context.Context, binding *TelegramUserBinding) error
	GetTelegramBotConfig(ctx context.Context, workspaceID uuid.UUID) (*TelegramBotConfig, error)
	SaveTelegramBotConfig(ctx context.Context, cfg *TelegramBotConfig) error
	CreateLog(ctx context.Context, log *NotificationLog) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetPreferences(ctx context.Context, userID, workspaceID uuid.UUID) ([]NotificationPreference, error) {
	var prefs []NotificationPreference
	err := r.db.WithContext(ctx).Where("user_id = ? AND workspace_id = ?", userID, workspaceID).Find(&prefs).Error
	return prefs, err
}

func (r *repository) UpsertPreference(ctx context.Context, pref *NotificationPreference) error {
	var existing NotificationPreference
	err := r.db.WithContext(ctx).Where("user_id = ? AND workspace_id = ? AND event_type = ?", pref.UserID, pref.WorkspaceID, pref.EventType).First(&existing).Error
	if err == nil {
		pref.ID = existing.ID
		return r.db.WithContext(ctx).Save(pref).Error
	}
	if pref.ID == uuid.Nil {
		pref.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(pref).Error
}

func (r *repository) GetTelegramBindingByUserID(ctx context.Context, userID uuid.UUID) (*TelegramUserBinding, error) {
	var b TelegramUserBinding
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *repository) GetTelegramBindingByChatID(ctx context.Context, chatID int64) (*TelegramUserBinding, error) {
	var b TelegramUserBinding
	err := r.db.WithContext(ctx).Where("telegram_chat_id = ? AND is_verified = ?", chatID, true).First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *repository) GetTelegramBindingByCode(ctx context.Context, code string) (*TelegramUserBinding, error) {
	var b TelegramUserBinding
	err := r.db.WithContext(ctx).Where("verification_code = ?", code).First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *repository) SaveTelegramBinding(ctx context.Context, binding *TelegramUserBinding) error {
	if binding.ID == uuid.Nil {
		binding.ID = uuid.New()
		return r.db.WithContext(ctx).Create(binding).Error
	}
	return r.db.WithContext(ctx).Save(binding).Error
}

func (r *repository) GetTelegramBotConfig(ctx context.Context, workspaceID uuid.UUID) (*TelegramBotConfig, error) {
	var cfg TelegramBotConfig
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *repository) SaveTelegramBotConfig(ctx context.Context, cfg *TelegramBotConfig) error {
	var existing TelegramBotConfig
	err := r.db.WithContext(ctx).Where("workspace_id = ?", cfg.WorkspaceID).First(&existing).Error
	if err == nil {
		cfg.ID = existing.ID
		return r.db.WithContext(ctx).Save(cfg).Error
	}
	if cfg.ID == uuid.Nil {
		cfg.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(cfg).Error
}

func (r *repository) CreateLog(ctx context.Context, log *NotificationLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(log).Error
}
