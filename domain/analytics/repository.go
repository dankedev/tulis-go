package analytics

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

func (r *Repository) Record(ctx context.Context, pv *PageView) error {
	return r.db.WithContext(ctx).Create(pv).Error
}

func (r *Repository) ViewsByDay(ctx context.Context, workspaceID uuid.UUID, days int) ([]DailyStat, error) {
	var stats []DailyStat
	cutoff := time.Now().AddDate(0, 0, -days)
	err := r.db.WithContext(ctx).
		Table("page_views").
		Select("DATE(created_at) as date, COUNT(*) as views").
		Joins("JOIN posts ON posts.id = page_views.post_id").
		Where("posts.workspace_id = ? AND page_views.created_at >= ?", workspaceID, cutoff).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&stats).Error
	return stats, err
}

func (r *Repository) TopPosts(ctx context.Context, workspaceID uuid.UUID, limit int) ([]TopPost, error) {
	var tops []TopPost
	err := r.db.WithContext(ctx).
		Table("page_views").
		Select("page_views.post_id, posts.title, posts.slug, COUNT(*) as views").
		Joins("JOIN posts ON posts.id = page_views.post_id").
		Where("posts.workspace_id = ?", workspaceID).
		Group("page_views.post_id").
		Order("views DESC").
		Limit(limit).
		Scan(&tops).Error
	return tops, err
}

func (r *Repository) TopReferrers(ctx context.Context, workspaceID uuid.UUID, limit int) ([]ReferrerStat, error) {
	var refs []ReferrerStat
	err := r.db.WithContext(ctx).
		Table("page_views").
		Select("referrer, COUNT(*) as views").
		Joins("JOIN posts ON posts.id = page_views.post_id").
		Where("posts.workspace_id = ? AND referrer != ''", workspaceID).
		Group("referrer").
		Order("views DESC").
		Limit(limit).
		Scan(&refs).Error
	return refs, err
}

func (r *Repository) TotalViews(ctx context.Context, workspaceID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("page_views").
		Joins("JOIN posts ON posts.id = page_views.post_id").
		Where("posts.workspace_id = ?", workspaceID).
		Count(&count).Error
	return count, err
}
