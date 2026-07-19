package comment

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, c *Comment) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Comment, error) {
	var c Comment
	err := r.db.WithContext(ctx).Preload("Children", func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ?", "approved").Order("created_at ASC")
	}).First(&c, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) ListByPost(ctx context.Context, postID uuid.UUID, status string, page, limit int) ([]Comment, int64, error) {
	var comments []Comment
	var total int64

	query := r.db.WithContext(ctx).Model(&Comment{}).Where("post_id = ?", postID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Only fetch top-level comments; children loaded via Preload on demand
	err := query.Where("parent_id IS NULL").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			q := db.Order("created_at ASC")
			if status != "" {
				q = q.Where("status = ?", status)
			}
			return q
		}).
		Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&comments).Error

	return comments, total, err
}

func (r *Repository) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, status string, page, limit int) ([]Comment, int64, error) {
	var comments []Comment
	var total int64

	query := r.db.WithContext(ctx).Model(&Comment{}).Where("workspace_id = ?", workspaceID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("Children").
		Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&comments).Error

	return comments, total, err
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.WithContext(ctx).Model(&Comment{}).Where("id = ?", id).Update("status", status).Error
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Comment{}, "id = ?", id).Error
}

func (r *Repository) CountByPost(ctx context.Context, postID uuid.UUID, status string) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&Comment{}).Where("post_id = ?", postID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Count(&count).Error
	return count, err
}
