package apikey

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApiKey represents an API token for programmatic access (MCP, headless clients).
type ApiKey struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);not null;index" json:"workspace_id"`
	Name        string         `gorm:"type:varchar(100);not null" json:"name"`
	KeyPrefix   string         `gorm:"type:varchar(12);not null" json:"key_prefix"` // "tulis_sk_" prefix visible
	KeyHash     string         `gorm:"type:varchar(255);not null" json:"-"`         // bcrypt hash, never exposed
	Scopes      string         `gorm:"type:varchar(255);not null;default:'content:read'" json:"scopes"` // space-delimited: "content:read content:write admin"
	LastUsedAt  *time.Time     `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	IsActive    bool           `gorm:"default:true;not null" json:"is_active"`
}

func (k *ApiKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	return nil
}
