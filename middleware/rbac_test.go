package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type mockWorkspaceServiceForRBAC struct {
	workspace.WorkspaceService
	roles map[string]string // key: workspaceID:userID, value: role
}

func (m *mockWorkspaceServiceForRBAC) GetMember(ctx context.Context, workspaceID, userID uuid.UUID) (*workspace.WorkspaceMember, error) {
	key := workspaceID.String() + ":" + userID.String()
	role, ok := m.roles[key]
	if !ok {
		return nil, workspace.ErrMemberNotFound
	}
	return &workspace.WorkspaceMember{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
	}, nil
}

func TestRequireRole(t *testing.T) {
	wsID := uuid.New()
	adminUserID := uuid.New()
	authorUserID := uuid.New()
	nonMemberUserID := uuid.New()

	mockSvc := &mockWorkspaceServiceForRBAC{
		roles: map[string]string{
			wsID.String() + ":" + adminUserID.String():  "admin",
			wsID.String() + ":" + authorUserID.String(): "author",
		},
	}

	app := fiber.New()

	// Inject active workspace details manually to simulate TenantScoping middleware
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("workspace_id", wsID.String())
		return c.Next()
	})

	// Setup protected route requiring 'admin' level or above
	app.Get("/admin-only", RequireRole(mockSvc, "admin"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	t.Run("Access Granted for Admin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin-only", nil)
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("user_id", adminUserID.String())
			return c.Next()
		})

		// Reset router state by creating a request specific test execution
		appAuth := fiber.New()
		appAuth.Use(func(c *fiber.Ctx) error {
			c.Locals("user_id", adminUserID.String())
			c.Locals("workspace_id", wsID.String())
			return c.Next()
		})
		appAuth.Get("/admin-only", RequireRole(mockSvc, "admin"), func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		resp, err := appAuth.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Access Denied for Author", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin-only", nil)
		appAuth := fiber.New()
		appAuth.Use(func(c *fiber.Ctx) error {
			c.Locals("user_id", authorUserID.String())
			c.Locals("workspace_id", wsID.String())
			return c.Next()
		})
		appAuth.Get("/admin-only", RequireRole(mockSvc, "admin"), func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		resp, err := appAuth.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("Access Denied for Non-Member", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin-only", nil)
		appAuth := fiber.New()
		appAuth.Use(func(c *fiber.Ctx) error {
			c.Locals("user_id", nonMemberUserID.String())
			c.Locals("workspace_id", wsID.String())
			return c.Next()
		})
		appAuth.Get("/admin-only", RequireRole(mockSvc, "admin"), func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		resp, err := appAuth.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", resp.StatusCode)
		}
	})
}
