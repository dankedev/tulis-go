package routes

import (
	"github.com/dankedev/tulis-go/domain/user"
	"github.com/gofiber/fiber/v2"
)

func RegisterUserPublicRoutes(api fiber.Router, userHandler *user.AuthHandler) {
	api.Post("/register", userHandler.Register)
	api.Post("/login", userHandler.Login)
	api.Get("/verify-email", userHandler.VerifyEmail)
	api.Post("/forgot-password", userHandler.RequestPasswordReset)
	api.Post("/reset-password", userHandler.ResetPassword)
	api.Post("/register/invitation", userHandler.RegisterByInvitation)
}

func RegisterUserAuthRoutes(authGroup fiber.Router, userHandler *user.AuthHandler) {
	authGroup.Get("/me", userHandler.Me)
	authGroup.Put("/me", userHandler.UpdateProfile)
	authGroup.Put("/me/password", userHandler.ChangePassword)
}

func RegisterOAuthRoutes(api fiber.Router, oauthHandler *user.OAuthHandler) {
	auth := api.Group("/auth")
	auth.Get("/:provider", oauthHandler.OAuthRedirect)
	auth.Get("/:provider/callback", oauthHandler.OAuthCallback)
}
