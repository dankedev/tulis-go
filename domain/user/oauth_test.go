package user

import (
	"context"
	"testing"
	"time"

	"github.com/dankedev/tulis-go/config"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/dankedev/tulis-go/utils/jwt"
	"github.com/google/uuid"
)

type mockUserRepo struct {
	users []User
}

func (m *mockUserRepo) Create(ctx context.Context, user *User) error {
	m.users = append(m.users, *user)
	return nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return &u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	for i := range m.users {
		if m.users[i].Email == email {
			return &m.users[i], nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) FindByVerificationToken(ctx context.Context, token string) (*User, error) {
	return nil, nil
}

func (m *mockUserRepo) FindByResetToken(ctx context.Context, token string) (*User, error) {
	return nil, nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *User) error {
	for i, u := range m.users {
		if u.ID == user.ID {
			m.users[i] = *user
			return nil
		}
	}
	return nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockUserRepo) ListAll(ctx context.Context) ([]User, error) {
	return m.users, nil
}

type mockOAuthWorkspaceRepo struct {
	workspaces []workspace.Workspace
	members    []workspace.WorkspaceMember
}

func (m *mockOAuthWorkspaceRepo) Create(ctx context.Context, ws *workspace.Workspace) error {
	m.workspaces = append(m.workspaces, *ws)
	return nil
}

func (m *mockOAuthWorkspaceRepo) AddMember(ctx context.Context, member *workspace.WorkspaceMember) error {
	m.members = append(m.members, *member)
	return nil
}

func (m *mockOAuthWorkspaceRepo) GetInvitationByToken(ctx context.Context, token string) (*workspace.WorkspaceInvitation, error) {
	return nil, nil
}

func (m *mockOAuthWorkspaceRepo) UpdateInvitation(ctx context.Context, invite *workspace.WorkspaceInvitation) error {
	return nil
}

func init() {
	config.AppConfig = &config.Config{
		JWTSecret:           "test-secret",
		JWTExpiryHours:      24,
		WorkspaceRestricted: false,
		AllowRegistration:   true,
		GoogleClientID:      "test-google-client-id",
		GoogleClientSecret:  "test-google-client-secret",
		GoogleRedirectURL:   "http://localhost:8080/api/auth/google/callback",
		GitHubClientID:      "test-github-client-id",
		GitHubClientSecret:  "test-github-client-secret",
		GitHubRedirectURL:   "http://localhost:8080/api/auth/github/callback",
		GitLabClientID:      "test-gitlab-client-id",
		GitLabClientSecret:  "test-gitlab-client-secret",
		GitLabRedirectURL:   "http://localhost:8080/api/auth/gitlab/callback",
	}
}

func TestGetAuthURL(t *testing.T) {
	jwtSvc := jwt.NewJWTService("test-secret", 24*time.Hour)
	repo := &mockUserRepo{}
	wsRepo := &mockOAuthWorkspaceRepo{}
	svc := NewOAuthService(repo, wsRepo, jwtSvc)

	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{
			name:     "Google provider",
			provider: "google",
			wantErr:  false,
		},
		{
			name:     "GitHub provider",
			provider: "github",
			wantErr:  false,
		},
		{
			name:     "GitLab provider",
			provider: "gitlab",
			wantErr:  false,
		},
		{
			name:     "Unsupported provider",
			provider: "unsupported",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := svc.GetAuthURL(tt.provider, "test-state")
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if url == "" {
				t.Error("expected non-empty URL")
			}
		})
	}
}

func TestGetAuthURLGoogleContainsAuthCode(t *testing.T) {
	jwtSvc := jwt.NewJWTService("test-secret", 24*time.Hour)
	repo := &mockUserRepo{}
	wsRepo := &mockOAuthWorkspaceRepo{}
	svc := NewOAuthService(repo, wsRepo, jwtSvc)

	url, err := svc.GetAuthURL("google", "my-state")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url == "" {
		t.Fatal("expected non-empty URL")
	}

	if !contains(url, "accounts.google.com") {
		t.Error("expected Google auth URL to contain accounts.google.com")
	}
	if !contains(url, "my-state") {
		t.Error("expected URL to contain state parameter")
	}
}

func TestGetAuthURLGitHubContainsParams(t *testing.T) {
	jwtSvc := jwt.NewJWTService("test-secret", 24*time.Hour)
	repo := &mockUserRepo{}
	wsRepo := &mockOAuthWorkspaceRepo{}
	svc := NewOAuthService(repo, wsRepo, jwtSvc)

	url, err := svc.GetAuthURL("github", "github-state")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url == "" {
		t.Fatal("expected non-empty URL")
	}

	if !contains(url, "github.com") {
		t.Error("expected GitHub auth URL to contain github.com")
	}
	if !contains(url, "test-github-client-id") {
		t.Error("expected URL to contain client ID")
	}
	if !contains(url, "github-state") {
		t.Error("expected URL to contain state parameter")
	}
}

func TestGetAuthURLGitLabContainsParams(t *testing.T) {
	jwtSvc := jwt.NewJWTService("test-secret", 24*time.Hour)
	repo := &mockUserRepo{}
	wsRepo := &mockOAuthWorkspaceRepo{}
	svc := NewOAuthService(repo, wsRepo, jwtSvc)

	url, err := svc.GetAuthURL("gitlab", "gitlab-state")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url == "" {
		t.Fatal("expected non-empty URL")
	}

	if !contains(url, "gitlab.com") {
		t.Error("expected GitLab auth URL to contain gitlab.com")
	}
	if !contains(url, "test-gitlab-client-id") {
		t.Error("expected URL to contain client ID")
	}
}

func TestHandleCallbackUnsupportedProvider(t *testing.T) {
	jwtSvc := jwt.NewJWTService("test-secret", 24*time.Hour)
	repo := &mockUserRepo{}
	wsRepo := &mockOAuthWorkspaceRepo{}
	svc := NewOAuthService(repo, wsRepo, jwtSvc)

	_, _, _, err := svc.HandleCallback(context.Background(), "unsupported", "code", "state")
	if err != ErrUnsupportedProvider {
		t.Errorf("expected ErrUnsupportedProvider, got %v", err)
	}
}

func TestFindOrCreateUserNewUser(t *testing.T) {
	jwtSvc := jwt.NewJWTService("test-secret", 24*time.Hour)
	repo := &mockUserRepo{}
	wsRepo := &mockOAuthWorkspaceRepo{}
	svc := NewOAuthService(repo, wsRepo, jwtSvc)

	ctx := context.Background()
	user, token, ws, err := svc.(*oauthService).findOrCreateUser(ctx, ProviderGoogle, "123", "test@example.com", "Test User", "https://avatar.url", true)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user == nil {
		t.Fatal("expected user to be non-nil")
	}

	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}

	if user.Name != "Test User" {
		t.Errorf("expected name Test User, got %s", user.Name)
	}

	if user.AuthProvider != "google" {
		t.Errorf("expected auth provider google, got %s", user.AuthProvider)
	}

	if user.AuthProviderID != "123" {
		t.Errorf("expected auth provider ID 123, got %s", user.AuthProviderID)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}

	if ws == nil {
		t.Error("expected workspace to be created")
	}
}

func TestFindOrCreateUserExistingUser(t *testing.T) {
	jwtSvc := jwt.NewJWTService("test-secret", 24*time.Hour)
	repo := &mockUserRepo{}
	wsRepo := &mockOAuthWorkspaceRepo{}
	svc := NewOAuthService(repo, wsRepo, jwtSvc)

	ctx := context.Background()

	existingUser := &User{
		ID:       uuid.New(),
		Email:    "existing@example.com",
		Name:     "",
		PasswordHash: "hash",
		Role:     "subscriber",
	}
	repo.users = append(repo.users, *existingUser)

	user, token, ws, err := svc.(*oauthService).findOrCreateUser(ctx, ProviderGitHub, "456", "existing@example.com", "Updated Name", "https://avatar.url/updated", true)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user == nil {
		t.Fatal("expected user to be non-nil")
	}

	if user.Email != "existing@example.com" {
		t.Errorf("expected email existing@example.com, got %s", user.Email)
	}

	if user.Name != "Updated Name" {
		t.Errorf("expected name Updated Name, got %s", user.Name)
	}

	if user.AuthProvider != "github" {
		t.Errorf("expected auth provider github, got %s", user.AuthProvider)
	}

	if user.AuthProviderID != "456" {
		t.Errorf("expected auth provider ID 456, got %s", user.AuthProviderID)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}

	if ws != nil {
		t.Error("expected workspace to be nil for existing user")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
