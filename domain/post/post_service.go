package post

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dankedev/tulis-go/utils/helpers"
	"github.com/google/uuid"
)

var (
	ErrPostNotFound     = errors.New("post not found")
	ErrPostTypeNotFound = errors.New("custom post type not found")
	ErrPostTypeExists   = errors.New("custom post type slug already exists in this workspace")
	ErrInvalidStatus    = errors.New("invalid status value")
	ErrRevisionNotFound = errors.New("revision not found")
	ErrTaxonomyNotFound = errors.New("taxonomy not found")
	ErrTaxonomyExists   = errors.New("taxonomy slug already exists in this workspace for this type")
)

type PostService interface {
	CreatePost(ctx context.Context, req CreatePostReq, authorID, workspaceID uuid.UUID) (*Post, error)
	GetPostByID(ctx context.Context, id uuid.UUID) (*Post, error)
	GetPostBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*Post, error)
	UpdatePost(ctx context.Context, id uuid.UUID, req UpdatePostReq, authorID uuid.UUID) (*Post, error)
	DeletePost(ctx context.Context, id uuid.UUID) error
	ListPosts(ctx context.Context, workspaceID uuid.UUID, postType string, status string, page, perPage int) ([]Post, int64, error)

	// Custom Post Type (CPT) registrations
	RegisterPostType(ctx context.Context, workspaceID uuid.UUID, name, slug, description string, fields []CustomFieldSchema) (*PostType, error)
	GetPostTypeByID(ctx context.Context, id uuid.UUID) (*PostType, error)
	GetPostTypeBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*PostType, error)
	ListPostTypes(ctx context.Context, workspaceID uuid.UUID) ([]PostType, error)
	DeletePostType(ctx context.Context, id uuid.UUID) error

	// Revisions
	ListRevisions(ctx context.Context, postID uuid.UUID) ([]PostRevision, error)
	RestoreRevision(ctx context.Context, revisionID uuid.UUID, authorID uuid.UUID) (*Post, error)

	// Taxonomy
	CreateTaxonomy(ctx context.Context, workspaceID uuid.UUID, name, slug, taxType string, parentID *uuid.UUID) (*Taxonomy, error)
	GetTaxonomyByID(ctx context.Context, id uuid.UUID) (*Taxonomy, error)
	UpdateTaxonomy(ctx context.Context, id uuid.UUID, name, slug string, parentID *uuid.UUID) (*Taxonomy, error)
	DeleteTaxonomy(ctx context.Context, id uuid.UUID) error
	ListTaxonomies(ctx context.Context, workspaceID uuid.UUID, taxType string) ([]Taxonomy, error)
	AssignTaxonomiesToPost(ctx context.Context, postID uuid.UUID, taxonomyIDs []uuid.UUID) error

	// Public consumption methods
	ListPublicPosts(ctx context.Context, workspaceID uuid.UUID, postType string, taxonomySlug string, sortBy string, page, perPage int) ([]Post, int64, error)
	GetPublicPostBySlugOrID(ctx context.Context, workspaceID uuid.UUID, slugOrID string) (*Post, error)
}

type postService struct {
	repo PostRepository
}

func NewPostService(repo PostRepository) PostService {
	return &postService{repo: repo}
}

func (s *postService) CreatePost(ctx context.Context, req CreatePostReq, authorID, workspaceID uuid.UUID) (*Post, error) {
	if req.Title == "" {
		return nil, errors.New("title is required")
	}

	postType := req.PostType
	if postType == "" {
		postType = "post"
	}

	if postType != "post" && postType != "page" {
		cpt, err := s.repo.FindPostTypeBySlug(ctx, workspaceID, postType)
		if err != nil {
			return nil, fmt.Errorf("custom post type '%s' is not registered in this workspace", postType)
		}
		if req.CustomFields == nil {
			req.CustomFields = make(map[string]interface{})
		}
		for _, schema := range cpt.FieldsConfig {
			val, present := req.CustomFields[schema.Name]
			if schema.Required && (!present || val == nil || val == "") {
				return nil, fmt.Errorf("custom field '%s' is required for post type '%s'", schema.Name, postType)
			}
			if !present && schema.DefaultVal != "" {
				req.CustomFields[schema.Name] = schema.DefaultVal
			}
		}
	}

	slug := req.Slug
	if slug == "" {
		slug = helpers.Slugify(req.Title)
	}

	originalSlug := slug
	counter := 1
	for {
		existing, _ := s.repo.FindBySlug(ctx, workspaceID, slug)
		if existing == nil {
			break
		}
		slug = fmt.Sprintf("%s-%d", originalSlug, counter)
		counter++
	}

	status := req.Status
	if status == "" {
		status = "draft"
	}
	if status != "draft" && status != "published" && status != "scheduled" && status != "archived" {
		return nil, ErrInvalidStatus
	}

	var publishedAt *time.Time
	if status == "published" {
		if req.PublishedAt != nil {
			publishedAt = req.PublishedAt
		} else {
			now := time.Now()
			publishedAt = &now
		}
	} else if status == "scheduled" {
		if req.PublishedAt == nil {
			return nil, errors.New("published_at is required for scheduled status")
		}
		publishedAt = req.PublishedAt
	}

	post := &Post{
		ID:           uuid.New(),
		Title:        req.Title,
		Slug:         slug,
		Content:      req.Content,
		Excerpt:      req.Excerpt,
		Status:       status,
		AuthorID:     authorID,
		WorkspaceID:  workspaceID,
		PostType:     postType,
		PublishedAt:  publishedAt,
		CustomFields: req.CustomFields,
		FeatureImage: req.FeatureImage,
		EditedAt:     time.Now(),
	}

	if err := s.repo.Create(ctx, post); err != nil {
		return nil, err
	}

	// Assign taxonomy IDs if present
	if len(req.TaxonomyIDs) > 0 {
		var taxUUIDs []uuid.UUID
		for _, idStr := range req.TaxonomyIDs {
			if uid, err := uuid.Parse(idStr); err == nil {
				taxUUIDs = append(taxUUIDs, uid)
			}
		}
		_ = s.repo.AssignTaxonomies(ctx, post.ID, taxUUIDs)
	}

	// Create initial revision
	revision := &PostRevision{
		ID:           uuid.New(),
		PostID:       post.ID,
		Title:        post.Title,
		Content:      post.Content,
		Excerpt:      post.Excerpt,
		CustomFields: post.CustomFields,
		AuthorID:     authorID,
		FeatureImage: post.FeatureImage,
	}
	_ = s.repo.CreateRevision(ctx, revision)

	// Fetch fresh post to return preloaded taxonomies
	return s.repo.FindByID(ctx, post.ID)
}

func (s *postService) GetPostByID(ctx context.Context, id uuid.UUID) (*Post, error) {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}
	return post, nil
}

func (s *postService) GetPostBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*Post, error) {
	post, err := s.repo.FindBySlug(ctx, workspaceID, slug)
	if err != nil {
		return nil, ErrPostNotFound
	}
	return post, nil
}

func (s *postService) UpdatePost(ctx context.Context, id uuid.UUID, req UpdatePostReq, authorID uuid.UUID) (*Post, error) {
	post, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrPostNotFound
	}

	if req.Title != nil {
		post.Title = *req.Title
	}
	if req.Content != nil {
		post.Content = *req.Content
	}
	if req.Excerpt != nil {
		post.Excerpt = *req.Excerpt
	}

	postType := post.PostType
	if req.PostType != nil {
		postType = *req.PostType
		post.PostType = postType
	}

	if req.Status != nil {
		status := *req.Status
		if status != "draft" && status != "published" && status != "scheduled" && status != "archived" {
			return nil, ErrInvalidStatus
		}
		post.Status = status
	}

	if req.PublishedAt != nil {
		post.PublishedAt = req.PublishedAt
	} else if post.Status == "published" && post.PublishedAt == nil {
		now := time.Now()
		post.PublishedAt = &now
	}

	if req.Slug != nil && *req.Slug != "" && *req.Slug != post.Slug {
		slug := *req.Slug
		originalSlug := slug
		counter := 1
		for {
			existing, _ := s.repo.FindBySlug(ctx, post.WorkspaceID, slug)
			if existing == nil || existing.ID == post.ID {
				break
			}
			slug = fmt.Sprintf("%s-%d", originalSlug, counter)
			counter++
		}
		post.Slug = slug
	}

	if req.CustomFields != nil {
		if postType != "post" && postType != "page" {
			cpt, err := s.repo.FindPostTypeBySlug(ctx, post.WorkspaceID, postType)
			if err == nil {
				for _, schema := range cpt.FieldsConfig {
					val, present := req.CustomFields[schema.Name]
					if schema.Required && (!present || val == nil || val == "") {
						return nil, fmt.Errorf("custom field '%s' is required for post type '%s'", schema.Name, postType)
					}
					if !present && schema.DefaultVal != "" {
						req.CustomFields[schema.Name] = schema.DefaultVal
					}
				}
			}
		}
		post.CustomFields = req.CustomFields
	}

	if req.FeatureImage != nil {
		post.FeatureImage = *req.FeatureImage
	}

	if req.AuthorID != nil && *req.AuthorID != "" {
		if uid, err := uuid.Parse(*req.AuthorID); err == nil {
			post.AuthorID = uid
		}
	}

	post.EditedAt = time.Now()

	if err := s.repo.Update(ctx, post); err != nil {
		return nil, err
	}

	// Update taxonomy IDs if present
	if req.TaxonomyIDs != nil {
		var taxUUIDs []uuid.UUID
		for _, idStr := range *req.TaxonomyIDs {
			if uid, err := uuid.Parse(idStr); err == nil {
				taxUUIDs = append(taxUUIDs, uid)
			}
		}
		_ = s.repo.AssignTaxonomies(ctx, post.ID, taxUUIDs)
	}

	// Auto-save revision
	revision := &PostRevision{
		ID:           uuid.New(),
		PostID:       post.ID,
		Title:        post.Title,
		Content:      post.Content,
		Excerpt:      post.Excerpt,
		CustomFields: post.CustomFields,
		AuthorID:     authorID,
		FeatureImage: post.FeatureImage,
	}
	_ = s.repo.CreateRevision(ctx, revision)

	return s.repo.FindByID(ctx, post.ID)
}

func (s *postService) DeletePost(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrPostNotFound
	}
	return s.repo.Delete(ctx, id)
}

func (s *postService) ListPosts(ctx context.Context, workspaceID uuid.UUID, postType string, status string, page, perPage int) ([]Post, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	offset := (page - 1) * perPage
	return s.repo.List(ctx, workspaceID, postType, status, perPage, offset)
}

// CPT operations
func (s *postService) RegisterPostType(ctx context.Context, workspaceID uuid.UUID, name, slug, description string, fields []CustomFieldSchema) (*PostType, error) {
	if name == "" {
		return nil, errors.New("post type name is required")
	}
	if slug == "" {
		slug = helpers.Slugify(name)
	}

	existing, _ := s.repo.FindPostTypeBySlug(ctx, workspaceID, slug)
	if existing != nil {
		return nil, ErrPostTypeExists
	}

	cpt := &PostType{
		ID:           uuid.New(),
		WorkspaceID:  workspaceID,
		Name:         name,
		Slug:         slug,
		Description:  description,
		FieldsConfig: fields,
	}

	if err := s.repo.CreatePostType(ctx, cpt); err != nil {
		return nil, err
	}

	return cpt, nil
}

func (s *postService) GetPostTypeByID(ctx context.Context, id uuid.UUID) (*PostType, error) {
	cpt, err := s.repo.FindPostTypeByID(ctx, id)
	if err != nil {
		return nil, ErrPostTypeNotFound
	}
	return cpt, nil
}

func (s *postService) GetPostTypeBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*PostType, error) {
	cpt, err := s.repo.FindPostTypeBySlug(ctx, workspaceID, slug)
	if err != nil {
		return nil, ErrPostTypeNotFound
	}
	return cpt, nil
}

func (s *postService) ListPostTypes(ctx context.Context, workspaceID uuid.UUID) ([]PostType, error) {
	return s.repo.ListPostTypes(ctx, workspaceID)
}

func (s *postService) DeletePostType(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.FindPostTypeByID(ctx, id)
	if err != nil {
		return ErrPostTypeNotFound
	}
	return s.repo.DeletePostType(ctx, id)
}

// Revisions implementations
func (s *postService) ListRevisions(ctx context.Context, postID uuid.UUID) ([]PostRevision, error) {
	_, err := s.repo.FindByID(ctx, postID)
	if err != nil {
		return nil, ErrPostNotFound
	}
	return s.repo.ListRevisions(ctx, postID)
}

func (s *postService) RestoreRevision(ctx context.Context, revisionID uuid.UUID, authorID uuid.UUID) (*Post, error) {
	revision, err := s.repo.FindRevisionByID(ctx, revisionID)
	if err != nil {
		return nil, ErrRevisionNotFound
	}

	post, err := s.repo.FindByID(ctx, revision.PostID)
	if err != nil {
		return nil, ErrPostNotFound
	}

	post.Title = revision.Title
	post.Content = revision.Content
	post.Excerpt = revision.Excerpt
	post.CustomFields = revision.CustomFields
	post.FeatureImage = revision.FeatureImage

	if err := s.repo.Update(ctx, post); err != nil {
		return nil, err
	}

	newRevision := &PostRevision{
		ID:           uuid.New(),
		PostID:       post.ID,
		Title:        post.Title,
		Content:      post.Content,
		Excerpt:      post.Excerpt,
		CustomFields: post.CustomFields,
		AuthorID:     authorID,
		FeatureImage: post.FeatureImage,
	}
	_ = s.repo.CreateRevision(ctx, newRevision)

	return s.repo.FindByID(ctx, post.ID)
}

// Taxonomy implementations
func (s *postService) CreateTaxonomy(ctx context.Context, workspaceID uuid.UUID, name, slug, taxType string, parentID *uuid.UUID) (*Taxonomy, error) {
	if name == "" {
		return nil, errors.New("taxonomy name is required")
	}
	if taxType != "category" && taxType != "tag" {
		return nil, errors.New("taxonomy type must be category or tag")
	}

	if slug == "" {
		slug = helpers.Slugify(name)
	}

	existing, _ := s.repo.FindTaxonomyBySlug(ctx, workspaceID, slug, taxType)
	if existing != nil {
		return nil, ErrTaxonomyExists
	}

	tax := &Taxonomy{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Name:        name,
		Slug:        slug,
		Type:        taxType,
		ParentID:    parentID,
	}

	if err := s.repo.CreateTaxonomy(ctx, tax); err != nil {
		return nil, err
	}

	return tax, nil
}

func (s *postService) GetTaxonomyByID(ctx context.Context, id uuid.UUID) (*Taxonomy, error) {
	tax, err := s.repo.FindTaxonomyByID(ctx, id)
	if err != nil {
		return nil, ErrTaxonomyNotFound
	}
	return tax, nil
}

func (s *postService) UpdateTaxonomy(ctx context.Context, id uuid.UUID, name, slug string, parentID *uuid.UUID) (*Taxonomy, error) {
	tax, err := s.repo.FindTaxonomyByID(ctx, id)
	if err != nil {
		return nil, ErrTaxonomyNotFound
	}

	if name != "" {
		tax.Name = name
	}

	if slug != "" && slug != tax.Slug {
		existing, _ := s.repo.FindTaxonomyBySlug(ctx, tax.WorkspaceID, slug, tax.Type)
		if existing != nil {
			return nil, ErrTaxonomyExists
		}
		tax.Slug = slug
	}

	if parentID != nil {
		tax.ParentID = parentID
	}

	if err := s.repo.UpdateTaxonomy(ctx, tax); err != nil {
		return nil, err
	}

	return tax, nil
}

func (s *postService) DeleteTaxonomy(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.FindTaxonomyByID(ctx, id)
	if err != nil {
		return ErrTaxonomyNotFound
	}
	return s.repo.DeleteTaxonomy(ctx, id)
}

func (s *postService) ListTaxonomies(ctx context.Context, workspaceID uuid.UUID, taxType string) ([]Taxonomy, error) {
	return s.repo.ListTaxonomies(ctx, workspaceID, taxType)
}

func (s *postService) AssignTaxonomiesToPost(ctx context.Context, postID uuid.UUID, taxonomyIDs []uuid.UUID) error {
	_, err := s.repo.FindByID(ctx, postID)
	if err != nil {
		return ErrPostNotFound
	}
	return s.repo.AssignTaxonomies(ctx, postID, taxonomyIDs)
}

func (s *postService) ListPublicPosts(ctx context.Context, workspaceID uuid.UUID, postType string, taxonomySlug string, sortBy string, page, perPage int) ([]Post, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	offset := (page - 1) * perPage
	return s.repo.ListPublic(ctx, workspaceID, postType, taxonomySlug, sortBy, perPage, offset)
}

func (s *postService) GetPublicPostBySlugOrID(ctx context.Context, workspaceID uuid.UUID, slugOrID string) (*Post, error) {
	var post *Post
	var err error

	id, errParse := uuid.Parse(slugOrID)
	if errParse == nil {
		post, err = s.repo.FindByID(ctx, id)
	} else {
		post, err = s.repo.FindBySlug(ctx, workspaceID, slugOrID)
	}

	if err != nil || post == nil {
		return nil, ErrPostNotFound
	}

	// Double check that post belongs to the tenant and is published
	if post.WorkspaceID != workspaceID || post.Status != "published" {
		return nil, ErrPostNotFound
	}

	return post, nil
}
