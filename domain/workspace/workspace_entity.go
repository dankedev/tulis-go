package workspace

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Workspace struct {
	ID        uuid.UUID              `gorm:"type:uuid;primary_key" json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	DeletedAt gorm.DeletedAt         `gorm:"index" json:"-"`
	Name      string                 `json:"name" gorm:"type:varchar(255);not null"`
	Slug      string                 `json:"slug" gorm:"type:varchar(255);uniqueIndex;not null"`
	Plan      string                 `json:"plan" gorm:"type:varchar(50);default:'free'"`
	Settings  map[string]interface{} `json:"settings" gorm:"serializer:json"`
}
