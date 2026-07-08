package media

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MediaRepository interface {
	Create(ctx context.Context, media *Media) error
	Update(ctx context.Context, media *Media) error
	FindByID(ctx context.Context, id uuid.UUID) (*Media, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, workspaceID uuid.UUID, limit, offset int, search string) ([]Media, int64, error)
}

type mediaRepository struct {
	db *gorm.DB
}

func NewMediaRepository(db *gorm.DB) MediaRepository {
	return &mediaRepository{db: db}
}

func (r *mediaRepository) Create(ctx context.Context, m *Media) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *mediaRepository) FindByID(ctx context.Context, id uuid.UUID) (*Media, error) {
	var m Media
	err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *mediaRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Media{}, "id = ?", id).Error
}

func (r *mediaRepository) List(ctx context.Context, workspaceID uuid.UUID, limit, offset int, search string) ([]Media, int64, error) {
	var mediaList []Media
	var total int64

	query := r.db.WithContext(ctx).Model(&Media{}).Where("workspace_id = ?", workspaceID)

	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where("filename LIKE ? OR alt_text LIKE ? OR caption LIKE ?", searchTerm, searchTerm, searchTerm)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&mediaList).Error
	if err != nil {
		return nil, 0, err
	}

	return mediaList, total, nil
}

func (r *mediaRepository) Update(ctx context.Context, m *Media) error {
	return r.db.WithContext(ctx).Save(m).Error
}
