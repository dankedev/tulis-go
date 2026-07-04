package user

import (
	"context"
	"errors"
	"strings"

	"github.com/dankedev/kontent/domain/workspace"
	"github.com/dankedev/kontent/utils/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrEmailExists     = errors.New("email already registered")
	ErrInvalidPassword = errors.New("invalid credentials")
)

type WorkspaceRepository interface {
	Create(ctx context.Context, ws *workspace.Workspace) error
}

type UserService interface {
	Register(ctx context.Context, user *User, password string) (*User, error)
	RegisterWithWorkspace(ctx context.Context, user *User, password string) (*User, string, *workspace.Workspace, error)
	Login(ctx context.Context, email, password string) (*User, string, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error
}

type userService struct {
	repo          UserRepository
	workspaceRepo WorkspaceRepository
	jwtSvc        jwt.JWTService
}

func NewUserService(repo UserRepository, workspaceRepo WorkspaceRepository, jwtSvc jwt.JWTService) UserService {
	return &userService{repo: repo, workspaceRepo: workspaceRepo, jwtSvc: jwtSvc}
}

func (s *userService) Register(ctx context.Context, user *User, password string) (*User, error) {
	existing, _ := s.repo.FindByEmail(ctx, user.Email)
	if existing != nil {
		return nil, ErrEmailExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user.ID = uuid.New()
	user.PasswordHash = string(hashedPassword)
	if user.Role == "" {
		user.Role = "subscriber"
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) RegisterWithWorkspace(ctx context.Context, user *User, password string) (*User, string, *workspace.Workspace, error) {
	existing, _ := s.repo.FindByEmail(ctx, user.Email)
	if existing != nil {
		return nil, "", nil, ErrEmailExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", nil, err
	}

	user.ID = uuid.New()
	user.PasswordHash = string(hashedPassword)
	if user.Role == "" {
		user.Role = "superadmin"
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, "", nil, err
	}

	token, err := s.jwtSvc.GenerateToken(user.ID.String())
	if err != nil {
		return nil, "", nil, err
	}

	ws := &workspace.Workspace{
		Name:     user.Name + "'s Workspace",
		Slug:     strings.ToLower(strings.ReplaceAll(user.Name, " ", "-") + "-" + uuid.New().String()[:8]),
		Plan:     "free",
		Settings: map[string]interface{}{},
	}
	if err := s.workspaceRepo.Create(ctx, ws); err != nil {
		return user, token, nil, nil
	}

	return user, token, ws, nil
}

func (s *userService) Login(ctx context.Context, email, password string) (*User, string, error) {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, "", ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidPassword
	}

	token, err := s.jwtSvc.GenerateToken(user.ID.String())
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (s *userService) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *userService) GetByEmail(ctx context.Context, email string) (*User, error) {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *userService) Update(ctx context.Context, user *User) error {
	return s.repo.Update(ctx, user)
}

func (s *userService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidPassword
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	return s.repo.Update(ctx, user)
}
