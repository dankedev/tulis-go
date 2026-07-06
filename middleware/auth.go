package middleware

import (
	"strings"

	"github.com/dankedev/kontent/utils/jwt"
	"github.com/dankedev/kontent/utils/response"
	"github.com/gofiber/fiber/v2"
)

// AuthGuard validates the JWT token in Authorization header and injects user_id into locals
func AuthGuard(jwtSvc jwt.JWTService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip OPTIONS preflight requests for CORS
		if c.Method() == fiber.MethodOptions {
			return c.Next()
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Error(c, "UNAUTHORIZED", "Missing authorization header", nil)
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return response.Error(c, "UNAUTHORIZED", "Invalid authorization header format", nil)
		}

		tokenString := parts[1]
		userID, err := jwtSvc.GetUserIDFromToken(tokenString)
		if err != nil {
			return response.Error(c, "UNAUTHORIZED", "Invalid or expired token", nil)
		}

		// Inject user_id into Fiber context locals
		c.Locals("user_id", userID.String())

		return c.Next()
	}
}
