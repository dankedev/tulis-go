package workspace

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkspaceInvitation struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);index;not null" json:"workspace_id"`
	Email       string         `gorm:"type:varchar(255);not null" json:"email"`
	Role        string         `gorm:"type:varchar(50);not null" json:"role"`
	Token       string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"token"`
	ExpiresAt   time.Time      `json:"expires_at"`
	Status      string         `gorm:"type:varchar(50);default:'pending';not null" json:"status"` // pending, accepted, expired, revoked
}
