package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrWorkspaceExists   = errors.New("workspace slug already exists")
	ErrMemberNotFound    = errors.New("member not found")
	ErrMemberExists      = errors.New("user is already a member of this workspace")
)

type WorkspaceService interface {
	CreateWorkspace(ctx context.Context, name, slug, plan string, ownerID uuid.UUID) (*Workspace, error)
	GetWorkspaceByID(ctx context.Context, id uuid.UUID) (*Workspace, error)
	GetWorkspaceBySlug(ctx context.Context, slug string) (*Workspace, error)
	UpdateWorkspace(ctx context.Context, id uuid.UUID, name, slug string, settings map[string]interface{}) (*Workspace, error)
	DeleteWorkspace(ctx context.Context, id uuid.UUID) error
	ListWorkspaces(ctx context.Context, userID uuid.UUID) ([]Workspace, error)

	// Members
	AddMember(ctx context.Context, workspaceID, userID uuid.UUID, role string) (*WorkspaceMember, error)
	GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*WorkspaceMember, error)
	UpdateMemberRole(ctx context.Context, workspaceID, userID uuid.UUID, role string) (*WorkspaceMember, error)
	RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error
	ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceMember, error)
}

type workspaceService struct {
	repo WorkspaceRepository
}

func NewWorkspaceService(repo WorkspaceRepository) WorkspaceService {
	return &workspaceService{repo: repo}
}

func (s *workspaceService) CreateWorkspace(ctx context.Context, name, slug, plan string, ownerID uuid.UUID) (*Workspace, error) {
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	}

	// Verify slug uniqueness
	existing, _ := s.repo.FindBySlug(ctx, slug)
	if existing != nil {
		return nil, ErrWorkspaceExists
	}

	ws := &Workspace{
		ID:       uuid.New(),
		Name:     name,
		Slug:     slug,
		Plan:     plan,
		Settings: map[string]interface{}{},
	}

	if err := s.repo.Create(ctx, ws); err != nil {
		return nil, err
	}

	// Add creator as superadmin member of workspace
	member := &WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: ws.ID,
		UserID:      ownerID,
		Role:        "superadmin",
	}
	if err := s.repo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	return ws, nil
}

func (s *workspaceService) GetWorkspaceByID(ctx context.Context, id uuid.UUID) (*Workspace, error) {
	ws, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrWorkspaceNotFound
	}
	return ws, nil
}

func (s *workspaceService) GetWorkspaceBySlug(ctx context.Context, slug string) (*Workspace, error) {
	ws, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, ErrWorkspaceNotFound
	}
	return ws, nil
}

func (s *workspaceService) UpdateWorkspace(ctx context.Context, id uuid.UUID, name, slug string, settings map[string]interface{}) (*Workspace, error) {
	ws, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrWorkspaceNotFound
	}

	if name != "" {
		ws.Name = name
	}

	if slug != "" && slug != ws.Slug {
		existing, _ := s.repo.FindBySlug(ctx, slug)
		if existing != nil {
			return nil, ErrWorkspaceExists
		}
		ws.Slug = slug
	}

	if settings != nil {
		ws.Settings = settings
	}

	if err := s.repo.Update(ctx, ws); err != nil {
		return nil, err
	}

	return ws, nil
}

func (s *workspaceService) DeleteWorkspace(ctx context.Context, id uuid.UUID) error {
	// Verify workspace exists
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrWorkspaceNotFound
	}
	return s.repo.Delete(ctx, id)
}

func (s *workspaceService) ListWorkspaces(ctx context.Context, userID uuid.UUID) ([]Workspace, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *workspaceService) AddMember(ctx context.Context, workspaceID, userID uuid.UUID, role string) (*WorkspaceMember, error) {
	// Check if already member
	existing, _ := s.repo.GetMember(ctx, workspaceID, userID)
	if existing != nil {
		return nil, ErrMemberExists
	}

	member := &WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
	}

	if err := s.repo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	return member, nil
}

func (s *workspaceService) GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*WorkspaceMember, error) {
	member, err := s.repo.GetMember(ctx, workspaceID, userID)
	if err != nil {
		return nil, ErrMemberNotFound
	}
	return member, nil
}

func (s *workspaceService) UpdateMemberRole(ctx context.Context, workspaceID, userID uuid.UUID, role string) (*WorkspaceMember, error) {
	member, err := s.repo.GetMember(ctx, workspaceID, userID)
	if err != nil {
		return nil, ErrMemberNotFound
	}

	member.Role = role
	if err := s.repo.UpdateMember(ctx, member); err != nil {
		return nil, err
	}

	return member, nil
}

func (s *workspaceService) RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error {
	_, err := s.repo.GetMember(ctx, workspaceID, userID)
	if err != nil {
		return ErrMemberNotFound
	}
	return s.repo.RemoveMember(ctx, workspaceID, userID)
}

func (s *workspaceService) ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceMember, error) {
	return s.repo.ListMembers(ctx, workspaceID)
}
