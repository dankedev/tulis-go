package user

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID              uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	Email           string         `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash    string         `json:"-" gorm:"type:varchar(255);not null"`
	Name            string         `json:"name" gorm:"type:varchar(255)"`
	AvatarURL       string         `json:"avatar_url" gorm:"type:text"`
	EmailVerifiedAt *time.Time     `json:"email_verified_at"`
	Role            string         `json:"role" gorm:"type:varchar(50);default:'subscriber'"`
}
