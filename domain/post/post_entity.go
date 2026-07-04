package post

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Post struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Title       string         `gorm:"type:varchar(255);not null" json:"title"`
	Slug        string         `gorm:"type:varchar(255);index:idx_slug_ws;not null" json:"slug"`
	Content     string         `gorm:"type:text" json:"content"`
	Excerpt     string         `gorm:"type:text" json:"excerpt"`
	Status      string         `gorm:"type:varchar(50);default:'draft';not null" json:"status"` // draft, published, scheduled, archived
	AuthorID    uuid.UUID      `gorm:"type:char(36);not null;index" json:"author_id"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);not null;index:idx_slug_ws" json:"workspace_id"`
	PostType    string         `gorm:"type:varchar(50);default:'post';not null;index" json:"post_type"` // 'post', 'page', CPTs
	PublishedAt *time.Time     `json:"published_at"`
	CustomFields map[string]interface{} `gorm:"serializer:json" json:"custom_fields"`
	Taxonomies   []Taxonomy             `gorm:"many2many:post_taxonomies;" json:"taxonomies"`
}
