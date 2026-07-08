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
	List(ctx context.Context, workspaceID uuid.UUID, postType string, status string, search string, limit, offset int) ([]Post, int64, error)
	ListPublic(ctx context.Context, workspaceID uuid.UUID, postType string, taxonomySlug string, sortBy string, limit, offset int) ([]Post, int64, error)

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

	// Taxonomy operations
	CreateTaxonomy(ctx context.Context, taxonomy *Taxonomy) error
	FindTaxonomyByID(ctx context.Context, id uuid.UUID) (*Taxonomy, error)
	FindTaxonomyBySlug(ctx context.Context, workspaceID uuid.UUID, slug string, taxType string) (*Taxonomy, error)
	UpdateTaxonomy(ctx context.Context, taxonomy *Taxonomy) error
	DeleteTaxonomy(ctx context.Context, id uuid.UUID) error
	ListTaxonomies(ctx context.Context, workspaceID uuid.UUID, taxType string) ([]Taxonomy, error)
	AssignTaxonomies(ctx context.Context, postID uuid.UUID, taxonomyIDs []uuid.UUID) error
	GetPostTaxonomies(ctx context.Context, postID uuid.UUID) ([]Taxonomy, error)
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
	err := r.db.WithContext(ctx).Preload("Taxonomies").First(&post, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *postRepository) FindBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*Post, error) {
	var post Post
	err := r.db.WithContext(ctx).Preload("Taxonomies").Where("workspace_id = ? AND slug = ?", workspaceID, slug).First(&post).Error
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

func (r *postRepository) List(ctx context.Context, workspaceID uuid.UUID, postType string, status string, search string, limit, offset int) ([]Post, int64, error) {
	var posts []Post
	var total int64

	query := r.db.WithContext(ctx).Model(&Post{}).Preload("Taxonomies").Where("workspace_id = ?", workspaceID)

	if postType != "" {
		query = query.Where("post_type = ?", postType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		query = query.Where("title LIKE ?", "%"+search+"%")
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

// Taxonomy CRUD implementations
func (r *postRepository) CreateTaxonomy(ctx context.Context, taxonomy *Taxonomy) error {
	return r.db.WithContext(ctx).Create(taxonomy).Error
}

func (r *postRepository) FindTaxonomyByID(ctx context.Context, id uuid.UUID) (*Taxonomy, error) {
	var taxonomy Taxonomy
	err := r.db.WithContext(ctx).First(&taxonomy, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &taxonomy, nil
}

func (r *postRepository) FindTaxonomyBySlug(ctx context.Context, workspaceID uuid.UUID, slug string, taxType string) (*Taxonomy, error) {
	var taxonomy Taxonomy
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND slug = ? AND type = ?", workspaceID, slug, taxType).First(&taxonomy).Error
	if err != nil {
		return nil, err
	}
	return &taxonomy, nil
}

func (r *postRepository) UpdateTaxonomy(ctx context.Context, taxonomy *Taxonomy) error {
	return r.db.WithContext(ctx).Save(taxonomy).Error
}

func (r *postRepository) DeleteTaxonomy(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete taxonomy
		if err := tx.Delete(&Taxonomy{}, "id = ?", id).Error; err != nil {
			return err
		}
		// Clear pivot entries
		if err := tx.Delete(&PostTaxonomy{}, "taxonomy_id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *postRepository) ListTaxonomies(ctx context.Context, workspaceID uuid.UUID, taxType string) ([]Taxonomy, error) {
	var taxonomies []Taxonomy
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if taxType != "" {
		query = query.Where("type = ?", taxType)
	}
	err := query.Find(&taxonomies).Error
	return taxonomies, err
}

func (r *postRepository) AssignTaxonomies(ctx context.Context, postID uuid.UUID, taxonomyIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing entries
		if err := tx.Where("post_id = ?", postID).Delete(&PostTaxonomy{}).Error; err != nil {
			return err
		}
		// Create new entries
		for _, taxID := range taxonomyIDs {
			assoc := &PostTaxonomy{
				PostID:     postID,
				TaxonomyID: taxID,
			}
			if err := tx.Create(assoc).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *postRepository) GetPostTaxonomies(ctx context.Context, postID uuid.UUID) ([]Taxonomy, error) {
	var taxonomies []Taxonomy
	err := r.db.WithContext(ctx).
		Table("taxonomies").
		Joins("join post_taxonomies on post_taxonomies.taxonomy_id = taxonomies.id").
		Where("post_taxonomies.post_id = ? and taxonomies.deleted_at is null", postID).
		Find(&taxonomies).Error
	return taxonomies, err
}

func (r *postRepository) ListPublic(ctx context.Context, workspaceID uuid.UUID, postType string, taxonomySlug string, sortBy string, limit, offset int) ([]Post, int64, error) {
	var posts []Post
	var total int64

	query := r.db.WithContext(ctx).Model(&Post{}).Preload("Taxonomies").Where("workspace_id = ? AND status = ?", workspaceID, "published")

	if postType != "" {
		query = query.Where("post_type = ?", postType)
	}

	if taxonomySlug != "" {
		query = query.Joins("join post_taxonomies on post_taxonomies.post_id = posts.id").
			Joins("join taxonomies on taxonomies.id = post_taxonomies.taxonomy_id").
			Where("taxonomies.slug = ? AND taxonomies.deleted_at is null", taxonomySlug)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Default sort if none or invalid is provided
	sortOrder := "published_at desc"
	allowedSorts := map[string]string{
		"published_at desc": "published_at desc",
		"published_at asc":  "published_at asc",
		"title asc":         "title asc",
		"title desc":        "title desc",
		"created_at desc":   "created_at desc",
		"created_at asc":    "created_at asc",
	}

	if sortBy != "" {
		if order, exists := allowedSorts[sortBy]; exists {
			sortOrder = order
		}
	}

	err := query.Order(sortOrder).Limit(limit).Offset(offset).Find(&posts).Error
	if err != nil {
		return nil, 0, err
	}

	return posts, total, nil
}
