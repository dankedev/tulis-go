package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTenantTestApp(t *testing.T, uid uuid.UUID, wsID uuid.UUID, injectUserID bool) *fiber.App {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&workspace.Workspace{}, &workspace.WorkspaceMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if wsID != uuid.Nil {
		if err := db.Create(&workspace.Workspace{ID: wsID, Name: "WS", Slug: "ws"}).Error; err != nil {
			t.Fatalf("seed ws: %v", err)
		}
	}
	if uid != uuid.Nil && wsID != uuid.Nil {
		if err := db.Create(&workspace.WorkspaceMember{ID: uuid.New(), WorkspaceID: wsID, UserID: uid, Role: "owner"}).Error; err != nil {
			t.Fatalf("seed member: %v", err)
		}
	}

	svc := workspace.NewWorkspaceService(workspace.NewWorkspaceRepository(db))
	app := fiber.New()
	if injectUserID {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("user_id", uid.String())
			return c.Next()
		})
	}
	app.Use(TenantScoping(svc))
	app.Get("/me", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"workspace_id": c.Locals("workspace_id")})
	})
	return app
}

// Authenticated request with no X-Workspace-ID header falls back to the user's
// first workspace (fixes 400 "Workspace context missing" on fresh/non-localhost sessions).
func TestTenantScopingUserWorkspaceFallback(t *testing.T) {
	uid := uuid.New()
	wsID := uuid.New()
	app := newTenantTestApp(t, uid, wsID, true)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Host = "api.example.com" // non-localhost so localhost fallback does not apply
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 via user-workspace fallback, got %d", resp.StatusCode)
	}
}

// Invalid workspace UUID with no authenticated user still returns 400 (no silent cross-tenant access).
func TestTenantScopingInvalidUUIDStill400(t *testing.T) {
	app := newTenantTestApp(t, uuid.Nil, uuid.Nil, false)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Host = "api.example.com"
	req.Header.Set("X-Workspace-ID", uuid.New().String()) // random, non-existent
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid workspace, got %d", resp.StatusCode)
	}
}
