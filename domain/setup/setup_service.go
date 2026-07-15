package setup

import (
	"context"

	"github.com/dankedev/tulis-go/domain/user"
	"github.com/dankedev/tulis-go/domain/workspace"
)

type SetupService interface {
	IsSetupCompleted(ctx context.Context) (bool, error)
	RunSetup(ctx context.Context, req SetupRequest) (*user.User, string, *workspace.Workspace, error)
}

type setupService struct {
	userRepo user.UserRepository
	userSvc  user.UserService
	wsSvc    workspace.WorkspaceService
}

func NewSetupService(userRepo user.UserRepository, userSvc user.UserService, wsSvc workspace.WorkspaceService) SetupService {
	return &setupService{userRepo: userRepo, userSvc: userSvc, wsSvc: wsSvc}
}

func (s *setupService) IsSetupCompleted(ctx context.Context) (bool, error) {
	users, err := s.userRepo.ListAll(ctx)
	if err != nil {
		return false, err
	}
	return len(users) > 0, nil
}

func (s *setupService) RunSetup(ctx context.Context, req SetupRequest) (*user.User, string, *workspace.Workspace, error) {
	completed, err := s.IsSetupCompleted(ctx)
	if err != nil {
		return nil, "", nil, err
	}
	if completed {
		return nil, "", nil, context.Canceled 
	}

	u := &user.User{
		Name:  req.AdminName,
		Email: req.AdminEmail,
		Role:  "superadmin",
	}

	createdUser, token, ws, err := s.userSvc.RegisterWithWorkspace(ctx, u, req.AdminPassword)
	if err != nil {
		return nil, "", nil, err
	}

	if ws != nil {
		ws, err = s.wsSvc.UpdateWorkspace(ctx, ws.ID, req.WorkspaceName, "", nil)
		if err != nil {
			return nil, "", nil, err
		}
	}

	return createdUser, token, ws, nil
}
