package workspace

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkspaceRepository interface {
	Create(ctx context.Context, ws *Workspace) error
	FindByID(ctx context.Context, id uuid.UUID) (*Workspace, error)
	FindBySlug(ctx context.Context, slug string) (*Workspace, error)
	Update(ctx context.Context, ws *Workspace) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Workspace, error)

	// Member operations
	AddMember(ctx context.Context, member *WorkspaceMember) error
	GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*WorkspaceMember, error)
	UpdateMember(ctx context.Context, member *WorkspaceMember) error
	RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error
	ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceMember, error)
}

type workspaceRepository struct {
	db *gorm.DB
}

func NewWorkspaceRepository(db *gorm.DB) WorkspaceRepository {
	return &workspaceRepository{db: db}
}

func (r *workspaceRepository) Create(ctx context.Context, ws *Workspace) error {
	return r.db.WithContext(ctx).Create(ws).Error
}

func (r *workspaceRepository) FindByID(ctx context.Context, id uuid.UUID) (*Workspace, error) {
	var ws Workspace
	err := r.db.WithContext(ctx).First(&ws, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &ws, nil
}

func (r *workspaceRepository) FindBySlug(ctx context.Context, slug string) (*Workspace, error) {
	var ws Workspace
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&ws).Error
	if err != nil {
		return nil, err
	}
	return &ws, nil
}

func (r *workspaceRepository) Update(ctx context.Context, ws *Workspace) error {
	return r.db.WithContext(ctx).Save(ws).Error
}

func (r *workspaceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Soft delete the workspace
		if err := tx.WithContext(ctx).Delete(&Workspace{}, "id = ?", id).Error; err != nil {
			return err
		}
		// Soft delete the workspace members associated
		if err := tx.WithContext(ctx).Delete(&WorkspaceMember{}, "workspace_id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *workspaceRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]Workspace, error) {
	var workspaces []Workspace
	err := r.db.WithContext(ctx).
		Table("workspaces").
		Joins("join workspace_members on workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ? and workspaces.deleted_at is null and workspace_members.deleted_at is null", userID).
		Find(&workspaces).Error
	return workspaces, err
}

func (r *workspaceRepository) AddMember(ctx context.Context, member *WorkspaceMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *workspaceRepository) GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*WorkspaceMember, error) {
	var member WorkspaceMember
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *workspaceRepository) UpdateMember(ctx context.Context, member *WorkspaceMember) error {
	return r.db.WithContext(ctx).Save(member).Error
}

func (r *workspaceRepository) RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&WorkspaceMember{}, "workspace_id = ? AND user_id = ?", workspaceID, userID).Error
}

func (r *workspaceRepository) ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceMember, error) {
	var members []WorkspaceMember
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&members).Error
	return members, err
}
