package comment

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Comment represents a reader or author comment on a post.
// Supports threaded (nested) comments via ParentID.
type Comment struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	PostID      uuid.UUID      `gorm:"type:char(36);not null;index" json:"post_id"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);not null;index" json:"workspace_id"`
	AuthorName  string         `gorm:"type:varchar(100);not null" json:"author_name"`
	AuthorEmail string         `gorm:"type:varchar(255);not null" json:"author_email"`
	AuthorID    *uuid.UUID     `gorm:"type:char(36);index" json:"author_id,omitempty"` // nil for anonymous/guest
	Content     string         `gorm:"type:text;not null" json:"content"`
	Status      string         `gorm:"type:varchar(20);default:'pending';not null;index" json:"status"` // pending, approved, spam, trashed
	ParentID    *uuid.UUID     `gorm:"type:char(36);index" json:"parent_id,omitempty"` // nil = top-level comment
	Children    []Comment      `gorm:"foreignkey:ParentID" json:"children,omitempty"`
}

func (c *Comment) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
