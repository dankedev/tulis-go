package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dankedev/tulis-go/config"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/dankedev/tulis-go/utils/jwt"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var (
	ErrUnsupportedProvider = errors.New("unsupported OAuth provider")
	ErrInvalidCode        = errors.New("invalid authorization code")
	ErrEmailNotRetrieved  = errors.New("could not retrieve email from provider")
)

type OAuthProvider string

const (
	ProviderGoogle OAuthProvider = "google"
	ProviderGitHub OAuthProvider = "github"
	ProviderGitLab OAuthProvider = "gitlab"
)

type OAuthUserInfo struct {
	Email     string
	Name      string
	AvatarURL string
	ProviderID string
}

type OAuthService interface {
	GetAuthURL(provider, state string) (string, error)
	HandleCallback(ctx context.Context, provider, code, state string) (*User, string, *workspace.Workspace, error)
}

type oauthService struct {
	userRepo       UserRepository
	workspaceRepo  WorkspaceRepository
	jwtSvc        jwt.JWTService
	oauth2Config  map[OAuthProvider]*oauth2.Config
}

func NewOAuthService(userRepo UserRepository, workspaceRepo WorkspaceRepository, jwtSvc jwt.JWTService) OAuthService {
	oauth2Config := map[OAuthProvider]*oauth2.Config{
		ProviderGoogle: {
			ClientID:     config.AppConfig.GoogleClientID,
			ClientSecret: config.AppConfig.GoogleClientSecret,
			RedirectURL:  config.AppConfig.GoogleRedirectURL,
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
			Endpoint:     oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/auth", TokenURL: "https://oauth2.googleapis.com/token"},
		},
		ProviderGitHub: {
			ClientID:     config.AppConfig.GitHubClientID,
			ClientSecret: config.AppConfig.GitHubClientSecret,
			RedirectURL:  config.AppConfig.GitHubRedirectURL,
			Scopes:       []string{"user:email"},
			Endpoint:     oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token"},
		},
		ProviderGitLab: {
			ClientID:     config.AppConfig.GitLabClientID,
			ClientSecret: config.AppConfig.GitLabClientSecret,
			RedirectURL:  config.AppConfig.GitLabRedirectURL,
			Scopes:       []string{"read_user"},
			Endpoint:     oauth2.Endpoint{AuthURL: "https://gitlab.com/oauth/authorize", TokenURL: "https://gitlab.com/oauth/token"},
		},
	}

	return &oauthService{
		userRepo:      userRepo,
		workspaceRepo: workspaceRepo,
		jwtSvc:       jwtSvc,
		oauth2Config: oauth2Config,
	}
}

func (s *oauthService) GetAuthURL(provider, state string) (string, error) {
	p := OAuthProvider(provider)
	cfg, ok := s.oauth2Config[p]
	if !ok {
		return "", ErrUnsupportedProvider
	}

	var authURL string
	switch p {
	case ProviderGoogle:
		authURL = cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	case ProviderGitHub:
		authURL = fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user:email&state=%s",
			cfg.ClientID, url.QueryEscape(cfg.RedirectURL), state)
		return authURL, nil
	case ProviderGitLab:
		authURL = fmt.Sprintf("https://gitlab.com/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=read_user&state=%s",
			cfg.ClientID, url.QueryEscape(cfg.RedirectURL), state)
		return authURL, nil
	}

	return authURL, nil
}

func (s *oauthService) HandleCallback(ctx context.Context, provider, code, state string) (*User, string, *workspace.Workspace, error) {
	p := OAuthProvider(provider)

	switch p {
	case ProviderGoogle:
		return s.handleGoogleCallback(ctx, code)
	case ProviderGitHub:
		return s.handleGitHubCallback(ctx, code)
	case ProviderGitLab:
		return s.handleGitLabCallback(ctx, code)
	default:
		return nil, "", nil, ErrUnsupportedProvider
	}
}

func (s *oauthService) handleGoogleCallback(ctx context.Context, code string) (*User, string, *workspace.Workspace, error) {
	cfg := s.oauth2Config[ProviderGoogle]

	// Exchange code for token
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, "", nil, ErrInvalidCode
	}

	// Get user info
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", nil, err
	}

	var userInfo struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		EmailVerified bool   `json:"verified_email"`
	}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, "", nil, err
	}

	if userInfo.Email == "" {
		return nil, "", nil, ErrEmailNotRetrieved
	}

	return s.findOrCreateUser(ctx, ProviderGoogle, userInfo.ID, userInfo.Email, userInfo.Name, userInfo.Picture, userInfo.EmailVerified)
}

func (s *oauthService) handleGitHubCallback(ctx context.Context, code string) (*User, string, *workspace.Workspace, error) {
	cfg := s.oauth2Config[ProviderGitHub]

	// Exchange code for token
	tokenURL := "https://github.com/login/oauth/access_token"
	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	data.Set("client_secret", cfg.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", cfg.RedirectURL)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, "", nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", nil, err
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, "", nil, err
	}

	if tokenResp.Error != "" || tokenResp.AccessToken == "" {
		return nil, "", nil, ErrInvalidCode
	}

	// Get user info
	userReq, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, "", nil, err
	}
	userReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	userResp, err := client.Do(userReq)
	if err != nil {
		return nil, "", nil, err
	}
	defer userResp.Body.Close()

	userBody, err := io.ReadAll(userResp.Body)
	if err != nil {
		return nil, "", nil, err
	}

	var githubUser struct {
		ID    float64 `json:"id"`
		Login string  `json:"login"`
		Name  string  `json:"name"`
		Email string  `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(userBody, &githubUser); err != nil {
		return nil, "", nil, err
	}

	// GitHub may not return email in /user endpoint, need to fetch emails
	email := githubUser.Email
	if email == "" {
		email, err = s.getGitHubEmail(tokenResp.AccessToken)
		if err != nil {
			return nil, "", nil, ErrEmailNotRetrieved
		}
	}

	name := githubUser.Name
	if name == "" {
		name = githubUser.Login
	}

	return s.findOrCreateUser(ctx, ProviderGitHub, fmt.Sprintf("%f", githubUser.ID), email, name, githubUser.AvatarURL, true)
}

func (s *oauthService) getGitHubEmail(accessToken string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	return "", ErrEmailNotRetrieved
}

func (s *oauthService) handleGitLabCallback(ctx context.Context, code string) (*User, string, *workspace.Workspace, error) {
	cfg := s.oauth2Config[ProviderGitLab]

	// Exchange code for token
	tokenURL := "https://gitlab.com/oauth/token"
	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	data.Set("client_secret", cfg.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", cfg.RedirectURL)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, "", nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", nil, err
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, "", nil, err
	}

	if tokenResp.Error != "" || tokenResp.AccessToken == "" {
		return nil, "", nil, ErrInvalidCode
	}

	// Get user info
	userReq, err := http.NewRequest("GET", "https://gitlab.com/api/v4/user", nil)
	if err != nil {
		return nil, "", nil, err
	}
	userReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	userResp, err := client.Do(userReq)
	if err != nil {
		return nil, "", nil, err
	}
	defer userResp.Body.Close()

	userBody, err := io.ReadAll(userResp.Body)
	if err != nil {
		return nil, "", nil, err
	}

	var gitlabUser struct {
		ID        float64 `json:"id"`
		Username  string  `json:"username"`
		Name      string  `json:"name"`
		Email     string  `json:"email"`
		AvatarURL string  `json:"avatar_url"`
	}
	if err := json.Unmarshal(userBody, &gitlabUser); err != nil {
		return nil, "", nil, err
	}

	if gitlabUser.Email == "" {
		return nil, "", nil, ErrEmailNotRetrieved
	}

	return s.findOrCreateUser(ctx, ProviderGitLab, fmt.Sprintf("%f", gitlabUser.ID), gitlabUser.Email, gitlabUser.Name, gitlabUser.AvatarURL, true)
}

func (s *oauthService) findOrCreateUser(ctx context.Context, provider OAuthProvider, providerID, email, name, avatarURL string, emailVerified bool) (*User, string, *workspace.Workspace, error) {
	// Try to find existing user by email
	existingUser, err := s.userRepo.FindByEmail(ctx, email)
	if err == nil && existingUser != nil {
		// User exists - update OAuth info and login
		existingUser.AuthProvider = string(provider)
		existingUser.AuthProviderID = providerID
		if existingUser.AvatarURL == "" && avatarURL != "" {
			existingUser.AvatarURL = avatarURL
		}
		if existingUser.Name == "" && name != "" {
			existingUser.Name = name
		}
		
		now := time.Now()
		existingUser.LastLoginAt = &now
		_ = s.userRepo.Update(ctx, existingUser)

		token, err := s.jwtSvc.GenerateToken(existingUser.ID.String())
		if err != nil {
			return nil, "", nil, err
		}

		return existingUser, token, nil, nil
	}

	// User doesn't exist - create new user
	newUser := &User{
		ID:              uuid.New(),
		Email:           email,
		Name:            name,
		AvatarURL:       avatarURL,
		AuthProvider:    string(provider),
		AuthProviderID:  providerID,
		Role:            "subscriber",
	}

	if emailVerified {
		now := time.Now()
		newUser.EmailVerifiedAt = &now
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, "", nil, err
	}

	// Generate JWT token
	token, err := s.jwtSvc.GenerateToken(newUser.ID.String())
	if err != nil {
		return nil, "", nil, err
	}

	// Create workspace if not restricted
	var ws *workspace.Workspace
	if config.AppConfig != nil && !config.AppConfig.WorkspaceRestricted {
		ws = &workspace.Workspace{
			ID:       uuid.New(),
			Name:     newUser.Name + "'s Workspace",
			Slug:     strings.ToLower(strings.ReplaceAll(newUser.Name, " ", "-") + "-" + uuid.New().String()[:8]),
			Plan:     "free",
			Settings: map[string]interface{}{},
		}
		if err := s.workspaceRepo.Create(ctx, ws); err != nil {
			// Log but don't fail
		} else {
			// Add user as superadmin
			member := &workspace.WorkspaceMember{
				ID:          uuid.New(),
				WorkspaceID: ws.ID,
				UserID:      newUser.ID,
				Role:        "superadmin",
			}
			_ = s.workspaceRepo.AddMember(ctx, member)
		}
	}

	return newUser, token, ws, nil
}
