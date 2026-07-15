package workspace

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserDetail struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
}

type WorkspaceMember struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);index;not null" json:"workspace_id"`
	UserID      uuid.UUID      `gorm:"type:char(36);index;not null" json:"user_id"`
	Role        string         `gorm:"type:varchar(50);not null" json:"role"` // e.g. 'superadmin', 'admin', 'editor', 'author', 'subscriber'

	// Joined/computed fields (ignored by GORM for persistence)
	UserIDAlias uuid.UUID      `gorm:"-" json:"userID"` // to match camelCase `userID` expected by frontend settings
	User        *UserDetail    `gorm:"-" json:"user,omitempty"`

	// Relationship
	Workspace   *Workspace     `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}
