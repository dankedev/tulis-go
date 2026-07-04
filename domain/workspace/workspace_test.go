package workspace_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dankedev/kontent/domain/workspace"
	"github.com/dankedev/kontent/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestWorkspaceDB(t *testing.T) (*gorm.DB, workspace.WorkspaceService, *workspace.WorkspaceHandler) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&workspace.Workspace{}, &workspace.WorkspaceMember{})
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := workspace.NewWorkspaceRepository(db)
	svc := workspace.NewWorkspaceService(repo)
	handler := workspace.NewWorkspaceHandler(svc)

	return db, svc, handler
}

func TestWorkspaceServiceAndHandler(t *testing.T) {
	_, svc, handler := setupTestWorkspaceDB(t)

	app := fiber.New()
	userID := uuid.New()

	// Inject auth session mock middleware
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID.String())
		return c.Next()
	})

	// Register routes
	app.Post("/api/workspaces", handler.Create)
	app.Get("/api/workspaces", handler.List)
	app.Get("/api/workspaces/:id", handler.GetByID)
	app.Put("/api/workspaces/:id", handler.Update)
	app.Delete("/api/workspaces/:id", handler.Delete)
	app.Post("/api/workspaces/:id/members", handler.AddMember)
	app.Get("/api/workspaces/:id/members", handler.ListMembers)
	app.Put("/api/workspaces/:id/members/:userId", handler.UpdateMemberRole)
	app.Delete("/api/workspaces/:id/members/:userId", handler.RemoveMember)

	var wsID string

	t.Run("Create Workspace", func(t *testing.T) {
		reqBody := workspace.CreateWorkspaceReq{
			Name: "Enterprise Workspace",
			Slug: "enterprise-ws",
			Plan: "pro",
		}
		jsonBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/workspaces", bytes.NewBuffer(jsonBytes))
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
		wsID = data["id"].(string)

		if data["name"] != "Enterprise Workspace" {
			t.Errorf("Expected name 'Enterprise Workspace', got %v", data["name"])
		}
	})

	t.Run("List Workspaces for user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/workspaces", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].([]interface{})
		if len(data) != 1 {
			t.Errorf("Expected 1 workspace, got %d", len(data))
		}
	})

	t.Run("Update Workspace details", func(t *testing.T) {
		reqBody := workspace.UpdateWorkspaceReq{
			Name: "Enterprise Updated",
			Settings: map[string]interface{}{
				"logo_url": "https://example.com/logo.png",
			},
		}
		jsonBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("PUT", "/api/workspaces/"+wsID, bytes.NewBuffer(jsonBytes))
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
		if data["name"] != "Enterprise Updated" {
			t.Errorf("Expected updated name 'Enterprise Updated', got %v", data["name"])
		}
	})

	t.Run("Add Member and Update Role", func(t *testing.T) {
		memberUserID := uuid.New()
		addReq := workspace.AddMemberReq{
			UserID: memberUserID.String(),
			Role:   "editor",
		}
		jsonBytes, _ := json.Marshal(addReq)

		req := httptest.NewRequest("POST", "/api/workspaces/"+wsID+"/members", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		// Update role to author
		updateReq := workspace.UpdateMemberRoleReq{
			Role: "author",
		}
		jsonBytes, _ = json.Marshal(updateReq)
		req = httptest.NewRequest("PUT", "/api/workspaces/"+wsID+"/members/"+memberUserID.String(), bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ = app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("TenantScoping Middleware Verification", func(t *testing.T) {
		appTenant := fiber.New()
		appTenant.Use(middleware.TenantScoping(svc))
		appTenant.Get("/tenant-endpoint", func(c *fiber.Ctx) error {
			wsIDLocal := c.Locals("workspace_id")
			return c.JSON(fiber.Map{
				"workspace_id": wsIDLocal,
			})
		})

		// 1. Test via Header
		req := httptest.NewRequest("GET", "/tenant-endpoint", nil)
		req.Header.Set("X-Workspace-ID", wsID)
		resp, _ := appTenant.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 via header, got %d", resp.StatusCode)
		}

		// 2. Test via Subdomain resolution
		reqSub := httptest.NewRequest("GET", "http://enterprise-ws.kontent.local/tenant-endpoint", nil)
		respSub, _ := appTenant.Test(reqSub, -1)
		if respSub.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 via subdomain, got %d", respSub.StatusCode)
		}

		// 3. Test invalid workspace
		reqInvalid := httptest.NewRequest("GET", "/tenant-endpoint", nil)
		reqInvalid.Header.Set("X-Workspace-ID", uuid.New().String())
		respInvalid, _ := appTenant.Test(reqInvalid, -1)
		if respInvalid.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 for invalid workspace, got %d", respInvalid.StatusCode)
		}
	})
}
