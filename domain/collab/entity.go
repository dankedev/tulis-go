package collab

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ContentLock prevents concurrent edits on the same post.
type ContentLock struct {
	ID        uuid.UUID  `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	PostID    uuid.UUID  `gorm:"type:char(36);uniqueIndex;not null" json:"post_id"`
	UserID    uuid.UUID  `gorm:"type:char(36);not null" json:"user_id"`
	UserName  string     `gorm:"type:varchar(100)" json:"user_name"`
	ExpiresAt time.Time  `json:"expires_at"`
}

func (l *ContentLock) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}
