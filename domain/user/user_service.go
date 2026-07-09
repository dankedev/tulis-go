package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/dankedev/tulis-go/utils/jwt"
	"github.com/dankedev/tulis-go/utils/mail"
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
	AddMember(ctx context.Context, member *workspace.WorkspaceMember) error
	GetInvitationByToken(ctx context.Context, token string) (*workspace.WorkspaceInvitation, error)
	UpdateInvitation(ctx context.Context, invite *workspace.WorkspaceInvitation) error
}

type UserService interface {
	Register(ctx context.Context, user *User, password string) (*User, error)
	RegisterWithWorkspace(ctx context.Context, user *User, password string) (*User, string, *workspace.Workspace, error)
	Login(ctx context.Context, email, password string) (*User, string, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error
	VerifyEmail(ctx context.Context, token string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	RegisterWithInvitation(ctx context.Context, token, name, password string) (*User, string, *workspace.WorkspaceMember, error)
	ListUsers(ctx context.Context) ([]User, error)
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
	user.VerificationToken = uuid.New().String()

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Send branded verification email in background
	verifyLink := fmt.Sprintf("https://app.tulis.org/verify-email?token=%s", user.VerificationToken)
	emailBody := mail.GetVerificationEmail(user.Name, verifyLink)
	go mail.SendHTMLMail(user.Email, "Konfirmasi Email Anda - Tulis CMS", emailBody)

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
	user.VerificationToken = uuid.New().String()

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, "", nil, err
	}

	token, err := s.jwtSvc.GenerateToken(user.ID.String())
	if err != nil {
		return nil, "", nil, err
	}

	ws := &workspace.Workspace{
		ID:       uuid.New(),
		Name:     user.Name + "'s Workspace",
		Slug:     strings.ToLower(strings.ReplaceAll(user.Name, " ", "-") + "-" + uuid.New().String()[:8]),
		Plan:     "free",
		Settings: map[string]interface{}{},
	}
	if err := s.workspaceRepo.Create(ctx, ws); err != nil {
		return user, token, nil, nil
	}

	// Add creator as superadmin member of workspace
	member := &workspace.WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        "superadmin",
	}
	if err := s.workspaceRepo.AddMember(ctx, member); err != nil {
		// Log error or handle it
	}

	// Send branded verification email in background
	verifyLink := fmt.Sprintf("https://app.tulis.org/verify-email?token=%s", user.VerificationToken)
	emailBody := mail.GetVerificationEmail(user.Name, verifyLink)
	go mail.SendHTMLMail(user.Email, "Konfirmasi Email Anda - Tulis CMS", emailBody)

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

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	_ = s.repo.Update(ctx, user)

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
	
	if err := s.repo.Update(ctx, user); err != nil {
		return err
	}

	// Send notification email that password was changed
	emailBody := mail.GetGeneralNotificationEmail(user.Name, "Password Berhasil Diubah", "Password akun Tulis CMS Anda baru saja diubah. Jika ini bukan tindakan Anda, silakan hubungi tim keamanan kami segera.")
	go mail.SendHTMLMail(user.Email, "Keamanan: Password Akun Diubah", emailBody)

	return nil
}

func (s *userService) VerifyEmail(ctx context.Context, token string) error {
	user, err := s.repo.FindByVerificationToken(ctx, token)
	if err != nil {
		return errors.New("token verifikasi tidak valid")
	}
	now := time.Now()
	user.EmailVerifiedAt = &now
	user.VerificationToken = ""
	return s.repo.Update(ctx, user)
}

func (s *userService) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return ErrUserNotFound
	}
	
	token := uuid.New().String()
	expiry := time.Now().Add(1 * time.Hour)
	user.ResetPasswordToken = token
	user.ResetPasswordExpiresAt = &expiry

	if err := s.repo.Update(ctx, user); err != nil {
		return err
	}

	resetLink := fmt.Sprintf("https://app.tulis.org/reset-password?token=%s", token)
	emailBody := mail.GetPasswordResetEmail(user.Name, resetLink)
	go mail.SendHTMLMail(user.Email, "Reset Password Akun Anda - Tulis CMS", emailBody)

	return nil
}

func (s *userService) ResetPassword(ctx context.Context, token, newPassword string) error {
	user, err := s.repo.FindByResetToken(ctx, token)
	if err != nil {
		return errors.New("token reset tidak valid")
	}

	if user.ResetPasswordExpiresAt == nil || user.ResetPasswordExpiresAt.Before(time.Now()) {
		return errors.New("token reset password telah kedaluwarsa")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	user.ResetPasswordToken = ""
	user.ResetPasswordExpiresAt = nil

	if err := s.repo.Update(ctx, user); err != nil {
		return err
	}

	emailBody := mail.GetGeneralNotificationEmail(user.Name, "Password Berhasil Diubah", "Password akun Tulis CMS Anda telah berhasil diatur ulang. Anda sekarang dapat masuk kembali menggunakan password baru Anda.")
	go mail.SendHTMLMail(user.Email, "Keamanan: Password Akun Diubah", emailBody)

	return nil
}

func (s *userService) ListUsers(ctx context.Context) ([]User, error) {
	return s.repo.ListAll(ctx)
}

func (s *userService) RegisterWithInvitation(ctx context.Context, token, name, password string) (*User, string, *workspace.WorkspaceMember, error) {
	invite, err := s.workspaceRepo.GetInvitationByToken(ctx, token)
	if err != nil {
		return nil, "", nil, errors.New("undangan tidak ditemukan")
	}

	if invite.Status != "pending" {
		return nil, "", nil, fmt.Errorf("undangan ini telah %s", invite.Status)
	}

	if invite.ExpiresAt.Before(time.Now()) {
		invite.Status = "expired"
		_ = s.workspaceRepo.UpdateInvitation(ctx, invite)
		return nil, "", nil, errors.New("undangan telah kedaluwarsa")
	}

	existing, _ := s.repo.FindByEmail(ctx, invite.Email)
	if existing != nil {
		return nil, "", nil, ErrEmailExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", nil, err
	}

	now := time.Now()
	u := &User{
		ID:              uuid.New(),
		Name:            name,
		Email:           invite.Email,
		PasswordHash:    string(hashedPassword),
		Role:            "subscriber",
		EmailVerifiedAt: &now,
		LastLoginAt:     &now,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, "", nil, err
	}

	invite.Status = "accepted"
	if err := s.workspaceRepo.UpdateInvitation(ctx, invite); err != nil {
		return nil, "", nil, err
	}

	member := &workspace.WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: invite.WorkspaceID,
		UserID:      u.ID,
		Role:        invite.Role,
	}
	if err := s.workspaceRepo.AddMember(ctx, member); err != nil {
		return nil, "", nil, err
	}

	jwtToken, err := s.jwtSvc.GenerateToken(u.ID.String())
	if err != nil {
		return nil, "", nil, err
	}

	return u, jwtToken, member, nil
}


