package middleware

import (
	"strings"

	"github.com/dankedev/tulis-go/domain/apikey"
	"github.com/gofiber/fiber/v2"
)

// ApiKeyAuth validates X-API-Key header, Bearer token, or query parameter, and injects workspace context.
// Used for MCP and headless API access.
func ApiKeyAuth(apiKeySvc *apikey.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := strings.TrimSpace(c.Get("X-API-Key"))
		if key == "" {
			// Fallback: Authorization: Bearer tulis_sk_... (JWT tokens do not have this prefix and pass through to AuthGuard)
			auth := strings.TrimSpace(c.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(auth), "bearer tulis_sk_") {
				key = strings.TrimSpace(auth[7:])
			}
		}

		if key == "" && strings.Contains(c.Path(), "/mcp") {
			// Fallback for SSE/EventSource clients that cannot set headers. Only honoured on MCP
			// paths: elsewhere "token" collides with unrelated query params (e.g. email verification)
			// and would turn them into failed API-key logins.
			key = strings.TrimSpace(c.Query("apiKey"))
			if key == "" {
				key = strings.TrimSpace(c.Query("api_key"))
			}
		}

		if key == "" {
			// No API key — pass through to next handler (might use JWT or optional auth)
			return c.Next()
		}

		k, err := apiKeySvc.Validate(c.Context(), key)
		if err != nil {
			if strings.Contains(c.Path(), "mcp") {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"jsonrpc": "2.0",
					"error": fiber.Map{
						"code":    -32000,
						"message": "Invalid or expired API key",
					},
				})
			}
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  401,
				"message": "Invalid or expired API key",
			})
		}

		// Inject workspace context from the API key
		c.Locals("workspace_id", k.WorkspaceID.String())
		c.Locals("api_key_id", k.ID.String())
		c.Locals("api_key_scopes", k.Scopes)
		c.Locals("auth_method", "api_key")

		return c.Next()
	}
}
