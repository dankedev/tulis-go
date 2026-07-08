package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dankedev/tulis-go/config"
	"github.com/dankedev/tulis-go/domain/media"
	"github.com/dankedev/tulis-go/domain/plugin"
	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/user"
	"github.com/dankedev/tulis-go/domain/workspace"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func initTestEnvironment(t *testing.T) {
	config.LoadConfig()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to initialize memory SQLite database: %v", err)
	}

	config.DB = db
	config.AppConfig.JWTSecret = "supersecretkey"

	err = db.AutoMigrate(
		&user.User{},
		&workspace.Workspace{},
		&workspace.WorkspaceMember{},
		&post.Post{},
		&post.PostType{},
		&post.PostRevision{},
		&post.Taxonomy{},
		&post.PostTaxonomy{},
		&media.Media{},
		&plugin.WorkspacePlugin{},
	)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}
}

func TestHealthCheckEndpoint(t *testing.T) {
	app := SetupApp()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test health endpoint: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if body["status"] != float64(http.StatusOK) {
		t.Errorf("Expected body status to be %d, got %v", http.StatusOK, body["status"])
	}

	if body["message"] != "healthy" {
		t.Errorf("Expected body message to be 'healthy', got %v", body["message"])
	}
}

func TestE2EIntegrationFlow(t *testing.T) {
	initTestEnvironment(t)
	app := SetupApp()

	// 1. Register User
	registerPayload := []byte(`{
		"name": "Developer Test",
		"email": "dev@kontent.com",
		"password": "password123"
	}`)
	req := httptest.NewRequest("POST", "/api/register", bytes.NewBuffer(registerPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Register request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Register expected status 200, got %d", resp.StatusCode)
	}

	var registerRes struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&registerRes)
	if err != nil {
		t.Fatalf("Failed to decode register response: %v", err)
	}
	token := registerRes.Data.Token
	if token == "" {
		t.Fatal("Expected non-empty auth token in register response")
	}

	// 2. Create Workspace
	workspacePayload := []byte(`{
		"name": "New Test Workspace",
		"slug": "test-ws"
	}`)
	req = httptest.NewRequest("POST", "/api/workspaces", bytes.NewBuffer(workspacePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Create workspace request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Create workspace expected status 200, got %d", resp.StatusCode)
	}

	var workspaceRes struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&workspaceRes)
	if err != nil {
		t.Fatalf("Failed to decode workspace response: %v", err)
	}
	workspaceID := workspaceRes.Data.ID
	if workspaceID == "" {
		t.Fatal("Expected workspace ID in creation response")
	}

	// 3. Create Content Entry (Post)
	postPayload := []byte(`{
		"title": "Welcome to Headless CMS",
		"slug": "welcome-cpt",
		"content": "This is E2E test content body.",
		"excerpt": "Short excerpt summary.",
		"status": "published",
		"post_type": "post",
		"custom_fields": {}
	}`)
	req = httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(postPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("X-Workspace-ID", workspaceID)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Create post request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Create post expected status 200, got %d", resp.StatusCode)
	}
}
