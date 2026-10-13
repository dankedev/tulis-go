package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// A nil service panics if Validate is reached, which proves the query param was ignored.
func TestApiKeyAuthIgnoresQueryKeyOutsideMCP(t *testing.T) {
	app := fiber.New()
	app.Use(ApiKeyAuth(nil))
	app.Get("/api/verify-email", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	for _, q := range []string{"token=abc", "apiKey=abc", "api_key=abc"} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/verify-email?"+q, nil))
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: expected request to pass through, got %d", q, resp.StatusCode)
		}
	}
}
