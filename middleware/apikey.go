package middleware

import (
	"strings"

	"github.com/dankedev/tulis-go/domain/apikey"
	"github.com/gofiber/fiber/v2"
)

// ApiKeyAuth validates X-API-Key header and injects workspace context.
// Used for MCP and headless API access.
func ApiKeyAuth(apiKeySvc *apikey.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := c.Get("X-API-Key")
		if key == "" {
			// Fallback: Authorization: Bearer tulis_sk_...
			auth := c.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer tulis_sk_") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if key == "" {
			// No API key — pass through to next handler (might use JWT)
			return c.Next()
		}

		k, err := apiKeySvc.Validate(c.Context(), key)
		if err != nil {
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
