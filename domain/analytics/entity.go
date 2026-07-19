package analytics

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PageView struct {
	ID         uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	PostID     uuid.UUID `gorm:"type:char(36);not null;index" json:"post_id"`
	Referrer   string    `gorm:"type:varchar(500)" json:"referrer,omitempty"`
	UserAgent  string    `gorm:"type:varchar(500)" json:"-"`
	IPHash     string    `gorm:"type:varchar(64)" json:"-"` // privacy-friendly
}

func (pv *PageView) BeforeCreate(tx *gorm.DB) error {
	if pv.ID == uuid.Nil {
		pv.ID = uuid.New()
	}
	return nil
}

type DailyStat struct {
	Date  string `json:"date"`
	Views int64  `json:"views"`
}

type TopPost struct {
	PostID uuid.UUID `json:"post_id"`
	Title  string    `json:"title"`
	Slug   string    `json:"slug"`
	Views  int64     `json:"views"`
}

type ReferrerStat struct {
	Referrer string `json:"referrer"`
	Views    int64  `json:"views"`
}
