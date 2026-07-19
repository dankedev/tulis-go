package importer

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ImportLog struct {
	ID           uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID  uuid.UUID      `gorm:"type:char(36);not null;index" json:"workspace_id"`
	AuthorID     uuid.UUID      `gorm:"type:char(36);not null" json:"author_id"`
	Filename     string         `gorm:"type:varchar(500)" json:"filename"`
	Status       string         `gorm:"type:varchar(50);default:'running'" json:"status"`
	PostsCount   int            `json:"posts_count"`
	PagesCount   int            `json:"pages_count"`
	MediaCount   int            `json:"media_count"`
	TaxCount     int            `json:"tax_count"`
	SkippedCount int            `json:"skipped_count"`
	Errors       string         `gorm:"type:text" json:"errors"`
	Summary      string         `gorm:"type:text" json:"summary"`
	FinishedAt   *time.Time     `json:"finished_at"`
}
