// Package userhandlers Kontent CMS User & Auth API
//
//	User and authentication management for Kontent CMS
//
//	Schemes: http
//	BasePath: /api
//	Version: 1.0.0
//	License: MIT
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
//	Security:
//	- Bearer
//
//	SecurityDefinitions:
//	Bearer:
//	     type: apiKey
//	     name: Authorization
//	     in: header
//
package user

import (
	"github.com/dankedev/kontent/config"
	"github.com/dankedev/kontent/domain/workspace"
	"github.com/dankedev/kontent/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type AuthHandler struct {
	userSvc UserService
}

func NewAuthHandler(userSvc UserService) *AuthHandler {
	return &AuthHandler{userSvc: userSvc}
}

// Register godoc
// @Summary Register a new user
// @Description Creates a new user account. Registration is disabled when ALLOW_REGISTRATION=false. When WORKSPACE_RESTRICTED=false (default), a personal workspace is auto-created. When WORKSPACE_RESTRICTED=true, the user must be assigned to an existing workspace by an admin.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	if config.AppConfig != nil && !config.AppConfig.AllowRegistration {
		return c.SendStatus(fiber.StatusNotFound)
	}

	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.Name == "" || req.Email == "" || req.Password == "" {
		return response.Error(c, "VALIDATION_ERROR", "Name, email, and password are required", nil)
	}

	var user *User
	var token string
	var ws *workspace.Workspace
	var err error

	restricted := config.AppConfig != nil && config.AppConfig.WorkspaceRestricted

	if restricted {
		user, err = h.userSvc.Register(c.Context(), &User{
			Name:  req.Name,
			Email: req.Email,
		}, req.Password)
		if err != nil {
			return response.Error(c, "BAD_REQUEST", err.Error(), nil)
		}

		_, token, err = h.userSvc.Login(c.Context(), req.Email, req.Password)
		if err != nil {
			return response.Error(c, "INTERNAL_ERROR", "Registration successful but login failed", nil)
		}
	} else {
		user, token, ws, err = h.userSvc.RegisterWithWorkspace(c.Context(), &User{
			Name:  req.Name,
			Email: req.Email,
		}, req.Password)
		if err != nil {
			return response.Error(c, "BAD_REQUEST", err.Error(), nil)
		}
	}

	data := fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":         user.ID,
			"name":       user.Name,
			"email":      user.Email,
			"role":       user.Role,
			"created_at": user.CreatedAt,
		},
	}
	if ws != nil {
		data["workspace"] = fiber.Map{
			"id":   ws.ID,
			"name": ws.Name,
			"slug": ws.Slug,
		}
	}

	return response.Success(c, data, "User registered successfully")
}

// Login godoc
// @Summary Login user
// @Description Authenticates a user and returns a JWT token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	user, token, err := h.userSvc.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid credentials", nil)
	}

	return response.Success(c, fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":         user.ID,
			"name":       user.Name,
			"email":      user.Email,
			"role":       user.Role,
			"created_at": user.CreatedAt,
		},
	}, "Login successful")
}

// Me godoc
// @Summary Get current authenticated user
// @Description Returns the profile of the currently authenticated user
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/me [get]
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userIDStr := c.Locals("user_id")
	if userIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	user, err := h.userSvc.GetByID(c.Context(), userID)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "User not found", nil)
	}

	return response.Success(c, fiber.Map{
		"id":         user.ID,
		"name":       user.Name,
		"email":      user.Email,
		"avatar_url": user.AvatarURL,
		"role":       user.Role,
	}, "User data retrieved successfully")
}

// GetUserByID godoc
// @Summary Get user by ID
// @Description Returns a single user by their UUID
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "User UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/users/{id} [get]
func (h *AuthHandler) GetUserByID(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid user ID", nil)
	}

	user, err := h.userSvc.GetByID(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "User not found", nil)
	}

	return response.Success(c, fiber.Map{
		"id":         user.ID,
		"name":       user.Name,
		"email":      user.Email,
		"avatar_url": user.AvatarURL,
		"role":       user.Role,
		"created_at": user.CreatedAt,
	}, "User retrieved successfully")
}

// UpdateProfile godoc
// @Summary Update user profile
// @Description Updates the name and avatar URL of the authenticated user (own profile only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "User UUID"
// @Param request body UpdateUserRequest true "Profile update fields"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/users/{id} [put]
func (h *AuthHandler) UpdateProfile(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid user ID", nil)
	}

	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	if authUserIDStr.(string) != idStr {
		return response.Error(c, "FORBIDDEN", "You can only update your own profile", nil)
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	user, err := h.userSvc.GetByID(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "User not found", nil)
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}

	if err := h.userSvc.Update(c.Context(), user); err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, fiber.Map{
		"id":         user.ID,
		"name":       user.Name,
		"email":      user.Email,
		"avatar_url": user.AvatarURL,
		"role":       user.Role,
		"updated_at": user.UpdatedAt,
	}, "User profile updated successfully")
}

// ChangePassword godoc
// @Summary Change password
// @Description Changes the password of the authenticated user
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param request body map[string]string true "Old and new password"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/me/password [put]
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}

	userID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	type ChangePasswordReq struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	var req ChangePasswordReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return response.Error(c, "VALIDATION_ERROR", "Old password and new password are required", nil)
	}

	if err := h.userSvc.ChangePassword(c.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, nil, "Password changed successfully")
}
