package membership

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SubscriptionTier defines a membership level.
type SubscriptionTier struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);not null;index" json:"workspace_id"`
	Name        string         `gorm:"type:varchar(100);not null" json:"name"`        // "Free", "Pro", "Premium"
	Slug        string         `gorm:"type:varchar(50);not null" json:"slug"`          // "free", "pro", "premium"
	Price       int            `gorm:"type:int;default:0" json:"price"`                // in IDR, 0 = free
	Features    string         `gorm:"type:text" json:"features"`                       // JSON array of feature names
	IsDefault   bool           `gorm:"default:false" json:"is_default"`
}

// UserSubscription links a user to a tier.
type UserSubscription struct {
	ID        uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserID    uuid.UUID `gorm:"type:char(36);not null;index" json:"user_id"`
	TierID    uuid.UUID `gorm:"type:char(36);not null" json:"tier_id"`
	Status    string    `gorm:"type:varchar(20);default:'active'" json:"status"` // active, cancelled, expired
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (s *SubscriptionTier) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (u *UserSubscription) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
