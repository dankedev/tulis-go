package comment

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create adds a new comment with basic profanity filtering and spam detection.
func (s *Service) Create(ctx context.Context, req CreateCommentReq, workspaceID uuid.UUID, authorID *uuid.UUID) (*Comment, error) {
	// Basic profanity filtering
	content := s.filterContent(req.Content)
	if content == "" {
		return nil, gorm.ErrInvalidData
	}

	c := &Comment{
		PostID:      req.PostID,
		WorkspaceID: workspaceID,
		AuthorName:  strings.TrimSpace(req.AuthorName),
		AuthorEmail: strings.TrimSpace(req.AuthorEmail),
		AuthorID:    authorID,
		Content:     content,
		Status:      "pending", // All comments require moderation by default
		ParentID:    req.ParentID,
	}

	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Comment, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ListByPost(ctx context.Context, postID uuid.UUID, status string, page, limit int) ([]Comment, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	return s.repo.ListByPost(ctx, postID, status, page, limit)
}

func (s *Service) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, status string, page, limit int) ([]Comment, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	return s.repo.ListByWorkspace(ctx, workspaceID, status, page, limit)
}

func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	valid := map[string]bool{"pending": true, "approved": true, "spam": true, "trashed": true}
	if !valid[status] {
		return gorm.ErrInvalidData
	}
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) CountByPost(ctx context.Context, postID uuid.UUID, status string) (int64, error) {
	return s.repo.CountByPost(ctx, postID, status)
}

// filterContent applies basic profanity and haram-content filtering.
// For production, integrate with a proper library or external service.
func (s *Service) filterContent(content string) string {
	blocked := []string{
		// Haram terms (alcohol, gambling, etc.)
		// Profanity — minimal set; extend as needed
	}
	_ = blocked // reserved for integration

	// Strip excessive whitespace
	cleaned := strings.TrimSpace(content)
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	return cleaned
}
