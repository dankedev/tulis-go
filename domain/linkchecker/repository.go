package linkchecker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository handles persistence for BrokenLink records.
type Repository interface {
	Create(ctx context.Context, link *BrokenLink) error
	Upsert(ctx context.Context, link *BrokenLink) error
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, onlyUnresolved bool) ([]BrokenLink, error)
	CountUnresolvedByWorkspace(ctx context.Context, workspaceID uuid.UUID) (int64, error)
	MarkResolved(ctx context.Context, id uuid.UUID) error
	DeleteByPost(ctx context.Context, postID uuid.UUID) error
	DeleteByURL(ctx context.Context, workspaceID uuid.UUID, url string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, link *BrokenLink) error {
	return r.db.WithContext(ctx).Create(link).Error
}

// Upsert inserts a new broken link, or updates the existing unresolved record
// for the same workspace+post+url (refreshes status code & check time).
func (r *repository) Upsert(ctx context.Context, link *BrokenLink) error {
	var existing BrokenLink
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND post_id = ? AND url = ? AND resolved_at IS NULL",
			link.WorkspaceID, link.PostID, link.URL).
		First(&existing).Error
	if err == nil {
		existing.StatusCode = link.StatusCode
		existing.LastCheckedAt = link.LastCheckedAt
		existing.PostTitle = link.PostTitle
		return r.db.WithContext(ctx).Save(&existing).Error
	}
	if link.ID == uuid.Nil {
		link.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(link).Error
}

func (r *repository) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, onlyUnresolved bool) ([]BrokenLink, error) {
	var links []BrokenLink
	q := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if onlyUnresolved {
		q = q.Where("resolved_at IS NULL")
	}
	err := q.Order("last_checked_at DESC").Find(&links).Error
	return links, err
}

func (r *repository) CountUnresolvedByWorkspace(ctx context.Context, workspaceID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&BrokenLink{}).
		Where("workspace_id = ? AND resolved_at IS NULL", workspaceID).
		Count(&count).Error
	return count, err
}

func (r *repository) MarkResolved(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&BrokenLink{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"resolved_at": &now}).Error
}

func (r *repository) DeleteByPost(ctx context.Context, postID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("post_id = ?", postID).Delete(&BrokenLink{}).Error
}

func (r *repository) DeleteByURL(ctx context.Context, workspaceID uuid.UUID, url string) error {
	return r.db.WithContext(ctx).
		Where("workspace_id = ? AND url = ?", workspaceID, url).
		Delete(&BrokenLink{}).Error
}
