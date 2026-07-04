package post

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CustomFieldSchema struct {
	Name       string `json:"name"`        // e.g. "price", "author_bio"
	Label      string `json:"label"`       // e.g. "Price", "Author Bio"
	Type       string `json:"type"`        // e.g. "text", "number", "boolean", "textarea"
	Required   bool   `json:"required"`
	DefaultVal string `json:"default_val"`
}

type PostType struct {
	ID           uuid.UUID           `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	DeletedAt    gorm.DeletedAt      `gorm:"index" json:"-"`
	WorkspaceID  uuid.UUID           `gorm:"type:char(36);not null;index:idx_cpt_ws" json:"workspace_id"`
	Name         string              `gorm:"type:varchar(255);not null" json:"name"` // e.g. "Portfolio"
	Slug         string              `gorm:"type:varchar(255);not null;index:idx_cpt_ws" json:"slug"` // e.g. "portfolio"
	Description  string              `gorm:"type:text" json:"description"`
	FieldsConfig []CustomFieldSchema `gorm:"serializer:json" json:"fields_config"`
}
