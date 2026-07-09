package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// APISubdomainGuard restricts access to a specific API host (e.g. api.tulis.org),
// but allows bypass on localhost/127.0.0.1 when APP_ENV is development.
func APISubdomainGuard(apiHost string, appEnv string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		host := c.Hostname()

		// Bypass subdomain check in development environment if accessing via localhost/127.0.0.1
		if appEnv == "development" && (host == "localhost" || host == "127.0.0.1") {
			return c.Next()
		}

		if host != apiHost {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  fiber.StatusNotFound,
				"message": fmt.Sprintf("Cannot %s %s", c.Method(), c.Path()),
			})
		}

		return c.Next()
	}
}
