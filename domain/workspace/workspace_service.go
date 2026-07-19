package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dankedev/tulis-go/config"
	"github.com/dankedev/tulis-go/utils/mail"
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
	ListAllWorkspaces(ctx context.Context) ([]Workspace, error)

	// Members
	AddMember(ctx context.Context, workspaceID, userID uuid.UUID, role string) (*WorkspaceMember, error)
	GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*WorkspaceMember, error)
	UpdateMemberRole(ctx context.Context, workspaceID, userID uuid.UUID, role string) (*WorkspaceMember, error)
	RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error
	ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceMember, error)

	// Invitations
	InviteMember(ctx context.Context, workspaceID, inviterUserID uuid.UUID, email, role string) (*WorkspaceInvitation, error)
	GetInvitationByToken(ctx context.Context, token string) (*WorkspaceInvitation, error)
	AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (*WorkspaceMember, error)
	ListInvitations(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceInvitation, error)
	RevokeInvitation(ctx context.Context, workspaceID, invitationID uuid.UUID) error
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

func (s *workspaceService) ListAllWorkspaces(ctx context.Context) ([]Workspace, error) {
	return s.repo.ListAll(ctx)
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

func (s *workspaceService) InviteMember(ctx context.Context, workspaceID, inviterUserID uuid.UUID, email, role string) (*WorkspaceInvitation, error) {
	ws, err := s.repo.FindByID(ctx, workspaceID)
	if err != nil {
		return nil, ErrWorkspaceNotFound
	}

	// Check if there is already an active pending invitation for this email
	if existing, _ := s.repo.GetPendingInvitationByEmail(ctx, workspaceID, email); existing != nil {
		return nil, errors.New("undangan aktif untuk email ini sudah ada (masih pending)")
	}

	inviterName := "Seorang anggota tim"
	if member, err := s.repo.GetMember(ctx, workspaceID, inviterUserID); err == nil && member.User != nil && member.User.Name != "" {
		inviterName = member.User.Name
	}

	token := uuid.New().String()
	invite := &WorkspaceInvitation{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Email:       email,
		Role:        role,
		Token:       token,
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour), // 7 days expiration
		Status:      "pending",
	}

	if err := s.repo.CreateInvitation(ctx, invite); err != nil {
		return nil, err
	}

	// Send branded email invitation in background
	inviteLink := fmt.Sprintf("%s/invitation/accept?token=%s", config.AppConfig.FrontURL, token)

	registerLink := fmt.Sprintf("%s/register", config.AppConfig.FrontURL)
	emailBody := mail.GetInvitationEmail(ws.Name, inviterName, inviteLink, registerLink)
	go mail.SendHTMLMail(email, fmt.Sprintf("Undangan kolaborasi di workspace %s - Tulis CMS", ws.Name), emailBody)

	return invite, nil
}

func (s *workspaceService) GetInvitationByToken(ctx context.Context, token string) (*WorkspaceInvitation, error) {
	return s.repo.GetInvitationByToken(ctx, token)
}

func (s *workspaceService) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) (*WorkspaceMember, error) {
	invite, err := s.repo.GetInvitationByToken(ctx, token)
	if err != nil {
		return nil, errors.New("undangan tidak ditemukan")
	}

	if invite.Status != "pending" {
		return nil, fmt.Errorf("undangan ini telah %s", invite.Status)
	}

	if invite.ExpiresAt.Before(time.Now()) {
		invite.Status = "expired"
		_ = s.repo.UpdateInvitation(ctx, invite)
		return nil, errors.New("undangan telah kedaluwarsa")
	}

	invite.Status = "accepted"
	if err := s.repo.UpdateInvitation(ctx, invite); err != nil {
		return nil, err
	}

	// Add user to workspace
	member, err := s.AddMember(ctx, invite.WorkspaceID, userID, invite.Role)
	if err != nil {
		return nil, err
	}

	return member, nil
}

func (s *workspaceService) ListInvitations(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceInvitation, error) {
	return s.repo.ListInvitations(ctx, workspaceID)
}

func (s *workspaceService) RevokeInvitation(ctx context.Context, workspaceID, invitationID uuid.UUID) error {
	return s.repo.DeleteInvitation(ctx, workspaceID, invitationID)
}
