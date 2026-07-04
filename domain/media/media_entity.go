package media

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Media struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);not null;index" json:"workspace_id"`
	Filename    string         `gorm:"type:varchar(255);not null" json:"filename"`
	Path        string         `gorm:"type:text;not null" json:"path"` // URL path to access the file
	MimeType    string         `gorm:"type:varchar(100);not null" json:"mime_type"`
	Size        int64          `gorm:"type:bigint;not null" json:"size"`
	AltText     string         `gorm:"type:varchar(255)" json:"alt_text"`
	Caption     string         `gorm:"type:text" json:"caption"`
}
