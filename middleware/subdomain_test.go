package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAPISubdomainGuard(t *testing.T) {
	apiHost := "api.tulis.org"

	tests := []struct {
		name           string
		appEnv         string
		hostHeader     string
		expectedStatus int
	}{
		{
			name:           "Dev - Allowed via localhost",
			appEnv:         "development",
			hostHeader:     "localhost",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Dev - Allowed via 127.0.0.1",
			appEnv:         "development",
			hostHeader:     "127.0.0.1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Dev - Allowed via api.tulis.org",
			appEnv:         "development",
			hostHeader:     "api.tulis.org",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Dev - Blocked via engine.tulis.org",
			appEnv:         "development",
			hostHeader:     "engine.tulis.org",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Prod - Allowed via api.tulis.org",
			appEnv:         "production",
			hostHeader:     "api.tulis.org",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Prod - Blocked via localhost",
			appEnv:         "production",
			hostHeader:     "localhost",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Prod - Blocked via engine.tulis.org",
			appEnv:         "production",
			hostHeader:     "engine.tulis.org",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/v1/posts", APISubdomainGuard(apiHost, tt.appEnv), func(c *fiber.Ctx) error {
				return c.SendStatus(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/v1/posts", nil)
			req.Host = tt.hostHeader
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatal(err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}
