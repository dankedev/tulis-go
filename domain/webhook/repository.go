package webhook

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, w *Webhook) error {
	return r.db.WithContext(ctx).Create(w).Error
}

func (r *Repository) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]Webhook, error) {
	var hooks []Webhook
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("created_at DESC").Find(&hooks).Error
	return hooks, err
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Webhook, error) {
	var w Webhook
	err := r.db.WithContext(ctx).First(&w, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *Repository) Update(ctx context.Context, w *Webhook) error {
	return r.db.WithContext(ctx).Save(w).Error
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Webhook{}, "id = ?", id).Error
}

func (r *Repository) ListActiveByEvent(ctx context.Context, workspaceID uuid.UUID, event string) ([]Webhook, error) {
	var hooks []Webhook
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND is_active = ? AND events LIKE ?", workspaceID, true, "%"+event+"%").
		Find(&hooks).Error
	return hooks, err
}

func (r *Repository) LogDelivery(ctx context.Context, log *DeliveryLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *Repository) ListDeliveryLogs(ctx context.Context, webhookID uuid.UUID, limit int) ([]DeliveryLog, error) {
	var logs []DeliveryLog
	err := r.db.WithContext(ctx).Where("webhook_id = ?", webhookID).Order("created_at DESC").Limit(limit).Find(&logs).Error
	return logs, err
}
