package linkchecker

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BrokenLink records a detected dead/invalid external link found in a published post.
type BrokenLink struct {
	ID            uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID   uuid.UUID `gorm:"type:char(36);not null;index" json:"workspace_id"`
	PostID        uuid.UUID `gorm:"type:char(36);not null;index" json:"post_id"`
	PostTitle     string    `gorm:"type:varchar(255)" json:"post_title"`
	URL           string    `gorm:"type:text;not null" json:"url"`
	StatusCode    int       `gorm:"type:integer;default:0" json:"status_code"` // 0 = unknown/dns error
	LastCheckedAt time.Time `json:"last_checked_at"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"` // set when link becomes valid again
}

// TableName explicitly maps the entity to the broken_links table.
func (BrokenLink) TableName() string { return "broken_links" }
