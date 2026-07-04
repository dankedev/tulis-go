package post

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dankedev/kontent/utils/helpers"
	"github.com/google/uuid"
)

var (
	ErrPostNotFound      = errors.New("post not found")
	ErrPostTypeNotFound  = errors.New("custom post type not found")
	ErrPostTypeExists    = errors.New("custom post type slug already exists in this workspace")
	ErrInvalidStatus     = errors.New("invalid status value")
)

type PostService interface {
	CreatePost(ctx context.Context, req CreatePostReq, authorID, workspaceID uuid.UUID) (*Post, error)
	GetPostByID(ctx context.Context, id uuid.UUID) (*Post, error)
	GetPostBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*Post, error)
	UpdatePost(ctx context.Context, id uuid.UUID, req UpdatePostReq) (*Post, error)
	DeletePost(ctx context.Context, id uuid.UUID) error
	ListPosts(ctx context.Context, workspaceID uuid.UUID, postType string, status string, page, perPage int) ([]Post, int64, error)

	// Custom Post Type (CPT) registrations
	RegisterPostType(ctx context.Context, workspaceID uuid.UUID, name, slug, description string, fields []CustomFieldSchema) (*PostType, error)
	GetPostTypeByID(ctx context.Context, id uuid.UUID) (*PostType, error)
	GetPostTypeBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*PostType, error)
	ListPostTypes(ctx context.Context, workspaceID uuid.UUID) ([]PostType, error)
	DeletePostType(ctx context.Context, id uuid.UUID) error
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

	// Validate CPT fields if applicable
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

	// Resolve duplicates within workspace
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
	}

	if err := s.repo.Create(ctx, post); err != nil {
		return nil, err
	}

	return post, nil
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

func (s *postService) UpdatePost(ctx context.Context, id uuid.UUID, req UpdatePostReq) (*Post, error) {
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
		// If postType is a CPT, we validate custom fields as well
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

	if err := s.repo.Update(ctx, post); err != nil {
		return nil, err
	}

	return post, nil
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
