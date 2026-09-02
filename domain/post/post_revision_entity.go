package post

import (
	"time"

	"github.com/google/uuid"
)

type PostRevision struct {
	ID           uuid.UUID              `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt    time.Time              `json:"created_at"`
	PostID       uuid.UUID              `gorm:"type:char(36);not null;index" json:"post_id"`
	Title        string                 `gorm:"type:varchar(255);not null" json:"title"`
	Content      string                 `gorm:"type:text" json:"content"`
	Excerpt      string                 `gorm:"type:text" json:"excerpt"`
	CustomFields map[string]interface{} `gorm:"serializer:json" json:"custom_fields"`
	AuthorID     uuid.UUID              `gorm:"type:char(36);not null" json:"author_id"`
	FeatureImage string                 `gorm:"type:varchar(255)" json:"feature_image"`
	Order        int                    `gorm:"type:integer;default:0" json:"order"`
}

