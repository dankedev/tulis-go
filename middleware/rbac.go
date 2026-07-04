package middleware

import (
	"github.com/dankedev/kontent/domain/workspace"
	"github.com/dankedev/kontent/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

var roleWeights = map[string]int{
	"superadmin": 5,
	"admin":      4,
	"editor":     3,
	"author":     2,
	"subscriber": 1,
}

// RequireRole checks if the user has at least the required role in the current workspace
func RequireRole(wsSvc workspace.WorkspaceService, requiredRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userIDStr := c.Locals("user_id")
		if userIDStr == nil {
			return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
		}

		userID, err := uuid.Parse(userIDStr.(string))
		if err != nil {
			return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
		}

		var wsID uuid.UUID
		wsIDStrLocal := c.Locals("workspace_id")
		if wsIDStrLocal != nil {
			wsID, err = uuid.Parse(wsIDStrLocal.(string))
		} else {
			// Fallback: try to read from URL params
			paramID := c.Params("id")
			if paramID != "" {
				wsID, err = uuid.Parse(paramID)
			}
		}

		if err != nil || wsID == uuid.Nil {
			return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
		}

		// Retrieve the membership to check the role
		member, err := wsSvc.GetMember(c.Context(), wsID, userID)
		if err != nil {
			return response.Error(c, "FORBIDDEN", "Access denied: you are not a member of this workspace", nil)
		}

		userWeight := roleWeights[member.Role]
		requiredWeight := roleWeights[requiredRole]

		if userWeight < requiredWeight {
			return response.Error(c, "FORBIDDEN", "Access denied: insufficient permissions", nil)
		}

		return c.Next()
	}
}
