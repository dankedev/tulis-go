package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dankedev/tulis-go/utils/jwt"
	"github.com/gofiber/fiber/v2"
)

func TestAuthGuard(t *testing.T) {
	// Initialize JWT service with a test secret
	secret := "test-secret-key"
	jwtSvc := jwt.NewJWTService(secret, 5*time.Second)

	app := fiber.New()

	// Protected endpoint that returns the user ID from locals
	app.Get("/protected", AuthGuard(jwtSvc), func(c *fiber.Ctx) error {
		userID := c.Locals("user_id")
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  fiber.StatusOK,
			"user_id": userID,
		})
	})

	t.Run("Missing Authorization Header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("Invalid Authorization Format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "InvalidFormat token123")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("Valid Token", func(t *testing.T) {
		userID := "88888888-4444-4444-4444-121212121212"
		token, err := jwtSvc.GenerateToken(userID)
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Expired Token", func(t *testing.T) {
		expiredSvc := jwt.NewJWTService(secret, -5*time.Second)
		userID := "88888888-4444-4444-4444-121212121212"
		token, err := expiredSvc.GenerateToken(userID)
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401, got %d", resp.StatusCode)
		}
	})
}
