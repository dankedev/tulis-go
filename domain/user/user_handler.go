package user

import (
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

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.Name == "" || req.Email == "" || req.Password == "" {
		return response.Error(c, "VALIDATION_ERROR", "Name, email, and password are required", nil)
	}

	user, token, workspace, err := h.userSvc.RegisterWithWorkspace(c.Context(), &User{
		Name:  req.Name,
		Email: req.Email,
	}, req.Password)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
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
	if workspace != nil {
		data["workspace"] = fiber.Map{
			"id":   workspace.ID,
			"name": workspace.Name,
			"slug": workspace.Slug,
		}
	}

	return response.Success(c, data, "User registered successfully")
}

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
