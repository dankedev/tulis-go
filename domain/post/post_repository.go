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

	// PostType operations
	CreatePostType(ctx context.Context, cpt *PostType) error
	FindPostTypeByID(ctx context.Context, id uuid.UUID) (*PostType, error)
	FindPostTypeBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*PostType, error)
	ListPostTypes(ctx context.Context, workspaceID uuid.UUID) ([]PostType, error)
	DeletePostType(ctx context.Context, id uuid.UUID) error

	// PostRevision operations
	CreateRevision(ctx context.Context, revision *PostRevision) error
	ListRevisions(ctx context.Context, postID uuid.UUID) ([]PostRevision, error)
	FindRevisionByID(ctx context.Context, id uuid.UUID) (*PostRevision, error)
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

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&posts).Error
	if err != nil {
		return nil, 0, err
	}

	return posts, total, nil
}

// PostType CRUD implementations
func (r *postRepository) CreatePostType(ctx context.Context, cpt *PostType) error {
	return r.db.WithContext(ctx).Create(cpt).Error
}

func (r *postRepository) FindPostTypeByID(ctx context.Context, id uuid.UUID) (*PostType, error) {
	var cpt PostType
	err := r.db.WithContext(ctx).First(&cpt, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &cpt, nil
}

func (r *postRepository) FindPostTypeBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*PostType, error) {
	var cpt PostType
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND slug = ?", workspaceID, slug).First(&cpt).Error
	if err != nil {
		return nil, err
	}
	return &cpt, nil
}

func (r *postRepository) ListPostTypes(ctx context.Context, workspaceID uuid.UUID) ([]PostType, error) {
	var cpts []PostType
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&cpts).Error
	return cpts, err
}

func (r *postRepository) DeletePostType(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&PostType{}, "id = ?", id).Error
}

// PostRevision CRUD implementations
func (r *postRepository) CreateRevision(ctx context.Context, revision *PostRevision) error {
	return r.db.WithContext(ctx).Create(revision).Error
}

func (r *postRepository) ListRevisions(ctx context.Context, postID uuid.UUID) ([]PostRevision, error) {
	var revisions []PostRevision
	err := r.db.WithContext(ctx).Where("post_id = ?", postID).Order("created_at desc").Find(&revisions).Error
	return revisions, err
}

func (r *postRepository) FindRevisionByID(ctx context.Context, id uuid.UUID) (*PostRevision, error) {
	var rev PostRevision
	err := r.db.WithContext(ctx).First(&rev, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rev, nil
}
