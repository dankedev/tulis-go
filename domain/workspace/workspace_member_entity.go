package workspace

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkspaceMember struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID uuid.UUID      `gorm:"type:uuid;index;not null" json:"workspace_id"`
	UserID      uuid.UUID      `gorm:"type:uuid;index;not null" json:"user_id"`
	Role        string         `gorm:"type:varchar(50);not null" json:"role"` // e.g. 'superadmin', 'admin', 'editor', 'author', 'subscriber'
}
