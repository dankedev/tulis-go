package post

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostRepository interface {
	Create(ctx context.Context, post *Post) error
	FindByID(ctx context.Context, id uuid.UUID) (*Post, error)
	FindBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*Post, error)
	Update(ctx context.Context, post *Post) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, workspaceID uuid.UUID, postType string, status string, limit, offset int) ([]Post, int64, error)
}

type postRepository struct {
	db *gorm.DB
}

func NewPostRepository(db *gorm.DB) PostRepository {
	return &postRepository{db: db}
}

func (r *postRepository) Create(ctx context.Context, post *Post) error {
	return r.db.WithContext(ctx).Create(post).Error
}

func (r *postRepository) FindByID(ctx context.Context, id uuid.UUID) (*Post, error) {
	var post Post
	err := r.db.WithContext(ctx).First(&post, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *postRepository) FindBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*Post, error) {
	var post Post
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND slug = ?", workspaceID, slug).First(&post).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *postRepository) Update(ctx context.Context, post *Post) error {
	return r.db.WithContext(ctx).Save(post).Error
}

func (r *postRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&Post{}, "id = ?", id).Error
}

func (r *postRepository) List(ctx context.Context, workspaceID uuid.UUID, postType string, status string, limit, offset int) ([]Post, int64, error) {
	var posts []Post
	var total int64

	query := r.db.WithContext(ctx).Model(&Post{}).Where("workspace_id = ?", workspaceID)

	if postType != "" {
		query = query.Where("post_type = ?", postType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Count total records before pagination
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch page records
	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&posts).Error
	if err != nil {
		return nil, 0, err
	}

	return posts, total, nil
}
