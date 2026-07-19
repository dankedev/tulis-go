package webhook

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Webhook represents an outbound webhook subscription.
type Webhook struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);not null;index" json:"workspace_id"`
	URL         string         `gorm:"type:text;not null" json:"url"`
	Events      string         `gorm:"type:varchar(255);not null" json:"events"` // space-delimited: "post.published post.updated"
	Secret      string         `gorm:"type:varchar(100)" json:"-"`               // HMAC secret, never exposed
	IsActive    bool           `gorm:"default:true;not null" json:"is_active"`
}

// DeliveryLog tracks each webhook delivery attempt.
type DeliveryLog struct {
	ID         uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	WebhookID  uuid.UUID `gorm:"type:char(36);not null;index" json:"webhook_id"`
	Event      string    `gorm:"type:varchar(50);not null" json:"event"`
	Status     string    `gorm:"type:varchar(20);not null" json:"status"` // success, failed, pending
	StatusCode int       `json:"status_code"`
	Response   string    `gorm:"type:text" json:"response,omitempty"`
	Payload    string    `gorm:"type:text" json:"-"`
}

func (w *Webhook) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

func (d *DeliveryLog) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}
