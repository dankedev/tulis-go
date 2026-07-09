package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/dankedev/tulis-go/middleware"
	"github.com/dankedev/tulis-go/utils/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Mock WorkspaceRepository for testing RegisterWithWorkspace
type mockWorkspaceRepo struct {
	db *gorm.DB
}

func (m *mockWorkspaceRepo) Create(ctx context.Context, ws *workspace.Workspace) error {
	ws.ID = uuid.New()
	return m.db.WithContext(ctx).Create(ws).Error
}

func (m *mockWorkspaceRepo) AddMember(ctx context.Context, member *workspace.WorkspaceMember) error {
	return nil
}

func (m *mockWorkspaceRepo) GetInvitationByToken(ctx context.Context, token string) (*workspace.WorkspaceInvitation, error) {
	var invite workspace.WorkspaceInvitation
	err := m.db.WithContext(ctx).Where("token = ?", token).First(&invite).Error
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (m *mockWorkspaceRepo) UpdateInvitation(ctx context.Context, invite *workspace.WorkspaceInvitation) error {
	return m.db.WithContext(ctx).Save(invite).Error
}

func setupTestDB(t *testing.T) (*gorm.DB, UserService, *AuthHandler, jwt.JWTService) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&User{}, &workspace.Workspace{})
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewUserRepository(db)
	wsRepo := &mockWorkspaceRepo{db: db}
	jwtSvc := jwt.NewJWTService("secret", 1*time.Hour)
	svc := NewUserService(repo, wsRepo, jwtSvc)
	handler := NewAuthHandler(svc)

	return db, svc, handler, jwtSvc
}

func TestUserServiceAndHandler(t *testing.T) {
	_, _, handler, jwtSvc := setupTestDB(t)

	app := fiber.New()

	// Public Routes
	app.Post("/api/register", handler.Register)
	app.Post("/api/login", handler.Login)

	// Protected Routes (guarded by AuthGuard)
	api := app.Group("/api", middleware.AuthGuard(jwtSvc))
	api.Get("/users/me", handler.Me)
	api.Get("/users/:id", handler.GetUserByID)
	api.Put("/users/:id", handler.UpdateProfile)
	api.Post("/users/change-password", handler.ChangePassword)

	var token string
	var registeredUserID string

	t.Run("Register User with Workspace", func(t *testing.T) {
		reqBody := RegisterRequest{
			Email:    "owner@example.com",
			Password: "securepassword",
			Name:     "Owner User",
		}
		jsonBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/register", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		token = data["token"].(string)

		userMap := data["user"].(map[string]interface{})
		registeredUserID = userMap["id"].(string)
		role := userMap["role"].(string)

		if role != "superadmin" {
			t.Errorf("Expected workspace owner role to be 'superadmin', got '%s'", role)
		}
	})

	t.Run("Login User", func(t *testing.T) {
		reqBody := LoginRequest{
			Email:    "owner@example.com",
			Password: "securepassword",
		}
		jsonBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/login", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		loginToken := data["token"].(string)
		if loginToken == "" {
			t.Error("Expected token in login response")
		}
	})

	t.Run("Get Me (Authenticated Profile)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		if data["email"] != "owner@example.com" {
			t.Errorf("Expected email to be 'owner@example.com', got '%v'", data["email"])
		}
	})

	t.Run("Get User By ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users/"+registeredUserID, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		if data["name"] != "Owner User" {
			t.Errorf("Expected name to be 'Owner User', got '%v'", data["name"])
		}
	})

	t.Run("Update User Profile", func(t *testing.T) {
		reqBody := UpdateUserRequest{
			Name:      "Updated Owner User",
			AvatarURL: "https://example.com/avatar.png",
		}
		jsonBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("PUT", "/api/users/"+registeredUserID, bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		if data["name"] != "Updated Owner User" {
			t.Errorf("Expected name to be updated, got '%v'", data["name"])
		}
		if data["avatar_url"] != "https://example.com/avatar.png" {
			t.Errorf("Expected avatar_url to be updated, got '%v'", data["avatar_url"])
		}
	})

	t.Run("Change Password and Login with New Password", func(t *testing.T) {
		changeReq := map[string]string{
			"old_password": "securepassword",
			"new_password": "newsecurepassword",
		}
		jsonBytes, _ := json.Marshal(changeReq)

		req := httptest.NewRequest("POST", "/api/users/change-password", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		// Verify old password fails
		loginReq := LoginRequest{
			Email:    "owner@example.com",
			Password: "securepassword",
		}
		jsonBytes, _ = json.Marshal(loginReq)
		req = httptest.NewRequest("POST", "/api/login", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected old password to fail with 401, got %d", resp.StatusCode)
		}

		// Verify new password succeeds
		loginReq.Password = "newsecurepassword"
		jsonBytes, _ = json.Marshal(loginReq)
		req = httptest.NewRequest("POST", "/api/login", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected new password to succeed, got %d", resp.StatusCode)
		}
	})
}
