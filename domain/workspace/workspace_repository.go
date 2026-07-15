package workspace

import (
	"context"
	"time"

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
	ListAll(ctx context.Context) ([]Workspace, error)

	// Member operations
	AddMember(ctx context.Context, member *WorkspaceMember) error
	GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*WorkspaceMember, error)
	UpdateMember(ctx context.Context, member *WorkspaceMember) error
	RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error
	ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceMember, error)

	// Invitation operations
	CreateInvitation(ctx context.Context, invite *WorkspaceInvitation) error
	GetInvitationByToken(ctx context.Context, token string) (*WorkspaceInvitation, error)
	UpdateInvitation(ctx context.Context, invite *WorkspaceInvitation) error
	ListInvitations(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceInvitation, error)
	GetPendingInvitationByEmail(ctx context.Context, workspaceID uuid.UUID, email string) (*WorkspaceInvitation, error)
	DeleteInvitation(ctx context.Context, workspaceID, invitationID uuid.UUID) error
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
		// Soft delete workspace invitations
		if err := tx.WithContext(ctx).Delete(&WorkspaceInvitation{}, "workspace_id = ?", id).Error; err != nil {
			return err
		}
		// Soft delete other related entities via raw queries since their structs are in other packages to avoid cyclic dependencies
		now := time.Now()
		tables := []string{"posts", "post_revisions", "taxonomies", "media"}
		for _, table := range tables {
			if err := tx.WithContext(ctx).Table(table).Where("workspace_id = ? AND deleted_at IS NULL", id).Update("deleted_at", now).Error; err != nil {
				return err
			}
		}

		// post_taxonomies is a join table. Usually it doesn't have deleted_at, so we can delete the relations,
		// or let them be since the parent posts/taxonomies are soft deleted.
		// Actually, we can just delete from post_taxonomies for the soft-deleted posts, but it's okay to skip for soft delete.
		
		return nil
	})
}

func (r *workspaceRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]Workspace, error) {
	var workspaces []Workspace
	err := r.db.WithContext(ctx).
		Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ? AND workspace_members.deleted_at IS NULL", userID).
		Find(&workspaces).Error
	return workspaces, err
}

func (r *workspaceRepository) ListAll(ctx context.Context) ([]Workspace, error) {
	var workspaces []Workspace
	err := r.db.WithContext(ctx).Find(&workspaces).Error
	return workspaces, err
}

func (r *workspaceRepository) AddMember(ctx context.Context, member *WorkspaceMember) error {
	err := r.db.WithContext(ctx).Create(member).Error
	if err == nil {
		member.UserIDAlias = member.UserID
	}
	return err
}

func (r *workspaceRepository) GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*WorkspaceMember, error) {
	var member WorkspaceMember
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	member.UserIDAlias = member.UserID

	type DbUser struct {
		ID    uuid.UUID
		Name  string
		Email string
	}
	var u DbUser
	if err := r.db.WithContext(ctx).Table("users").Where("id = ?", member.UserID).First(&u).Error; err == nil {
		member.User = &UserDetail{
			ID:    u.ID,
			Name:  u.Name,
			Email: u.Email,
		}
	}
	return &member, nil
}

func (r *workspaceRepository) UpdateMember(ctx context.Context, member *WorkspaceMember) error {
	err := r.db.WithContext(ctx).Save(member).Error
	if err == nil {
		member.UserIDAlias = member.UserID
	}
	return err
}

func (r *workspaceRepository) RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&WorkspaceMember{}, "workspace_id = ? AND user_id = ?", workspaceID, userID).Error
}

func (r *workspaceRepository) ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceMember, error) {
	var members []WorkspaceMember
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&members).Error
	if err != nil {
		return nil, err
	}

	if len(members) > 0 {
		var userIDs []uuid.UUID
		for _, m := range members {
			userIDs = append(userIDs, m.UserID)
		}

		type DbUser struct {
			ID    uuid.UUID
			Name  string
			Email string
		}
		var dbUsers []DbUser
		if err := r.db.WithContext(ctx).Table("users").Where("id IN ?", userIDs).Find(&dbUsers).Error; err == nil {
			userMap := make(map[uuid.UUID]DbUser)
			for _, u := range dbUsers {
				userMap[u.ID] = u
			}

			for i := range members {
				members[i].UserIDAlias = members[i].UserID
				if u, ok := userMap[members[i].UserID]; ok {
					members[i].User = &UserDetail{
						ID:    u.ID,
						Name:  u.Name,
						Email: u.Email,
					}
				}
			}
		}
	}

	return members, nil
}

func (r *workspaceRepository) CreateInvitation(ctx context.Context, invite *WorkspaceInvitation) error {
	return r.db.WithContext(ctx).Create(invite).Error
}

func (r *workspaceRepository) GetInvitationByToken(ctx context.Context, token string) (*WorkspaceInvitation, error) {
	var invite WorkspaceInvitation
	err := r.db.WithContext(ctx).Where("token = ? AND deleted_at IS NULL", token).First(&invite).Error
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *workspaceRepository) UpdateInvitation(ctx context.Context, invite *WorkspaceInvitation) error {
	return r.db.WithContext(ctx).Save(invite).Error
}

func (r *workspaceRepository) ListInvitations(ctx context.Context, workspaceID uuid.UUID) ([]WorkspaceInvitation, error) {
	var invites []WorkspaceInvitation
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND deleted_at IS NULL", workspaceID).Find(&invites).Error
	return invites, err
}

func (r *workspaceRepository) GetPendingInvitationByEmail(ctx context.Context, workspaceID uuid.UUID, email string) (*WorkspaceInvitation, error) {
	var invite WorkspaceInvitation
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND email = ? AND status = ? AND expires_at > ? AND deleted_at IS NULL", workspaceID, email, "pending", time.Now()).
		First(&invite).Error
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *workspaceRepository) DeleteInvitation(ctx context.Context, workspaceID, invitationID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ?", invitationID, workspaceID).
		Delete(&WorkspaceInvitation{}).Error
}

