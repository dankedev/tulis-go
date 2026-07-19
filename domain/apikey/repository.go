package apikey

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, k *ApiKey) error {
	return r.db.WithContext(ctx).Create(k).Error
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*ApiKey, error) {
	var k ApiKey
	err := r.db.WithContext(ctx).First(&k, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *Repository) GetByHash(ctx context.Context, hash string) (*ApiKey, error) {
	var k ApiKey
	err := r.db.WithContext(ctx).Where("key_hash = ? AND is_active = ?", hash, true).First(&k).Error
	if err != nil {
		return nil, err
	}
	// Check expiration
	if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now()) {
		return nil, gorm.ErrRecordNotFound
	}
	return &k, nil
}

func (r *Repository) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]ApiKey, error) {
	var keys []ApiKey
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("created_at DESC").Find(&keys).Error
	return keys, err
}

func (r *Repository) Update(ctx context.Context, k *ApiKey) error {
	return r.db.WithContext(ctx).Save(k).Error
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&ApiKey{}, "id = ?", id).Error
}
