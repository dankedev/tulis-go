package post

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Taxonomy struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID uuid.UUID      `gorm:"type:uuid;not null;index:idx_tax_ws" json:"workspace_id"`
	Name        string         `gorm:"type:varchar(255);not null" json:"name"`
	Slug        string         `gorm:"type:varchar(255);not null;index:idx_tax_ws" json:"slug"`
	Type        string         `gorm:"type:varchar(50);not null;index" json:"type"` // 'category' or 'tag'
	ParentID    *uuid.UUID     `gorm:"type:uuid;index" json:"parent_id"`            // For category hierarchy
}

type PostTaxonomy struct {
	PostID     uuid.UUID `gorm:"type:uuid;primary_key" json:"post_id"`
	TaxonomyID uuid.UUID `gorm:"type:uuid;primary_key" json:"taxonomy_id"`
}
