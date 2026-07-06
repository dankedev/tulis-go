package middleware

import (
	"strings"

	"github.com/dankedev/kontent/domain/workspace"
	"github.com/dankedev/kontent/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// TenantScoping extracts workspace from header or subdomain and injects it into locals
func TenantScoping(wsSvc workspace.WorkspaceService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip OPTIONS preflight requests for CORS
		if c.Method() == fiber.MethodOptions {
			return c.Next()
		}

		// 1. Try to extract workspace ID from X-Workspace-ID header
		wsIDStr := c.Get("X-Workspace-ID")
		if wsIDStr != "" {
			wsID, err := uuid.Parse(wsIDStr)
			if err == nil {
				ws, err := wsSvc.GetWorkspaceByID(c.Context(), wsID)
				if err == nil {
					c.Locals("workspace_id", ws.ID.String())
					c.Locals("workspace", ws)
					return c.Next()
				}
			}
		}

		// 2. Try to extract workspace slug from subdomain
		host := c.Hostname()
		parts := strings.Split(host, ".")
		// If subdomain is present (e.g., test.localhost or tenant.kontent.com)
		if len(parts) > 1 {
			slug := parts[0]
			if slug != "www" && slug != "api" && slug != "localhost" {
				ws, err := wsSvc.GetWorkspaceBySlug(c.Context(), slug)
				if err == nil {
					c.Locals("workspace_id", ws.ID.String())
					c.Locals("workspace", ws)
					return c.Next()
				}
			}
		}

		return response.Error(c, "BAD_REQUEST", "Workspace context missing or invalid", nil)
	}
}
