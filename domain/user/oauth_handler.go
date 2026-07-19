package user

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"

	"github.com/dankedev/tulis-go/config"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/gofiber/fiber/v2"
)

type OAuthHandler struct {
	oauthSvc OAuthService
}

func NewOAuthHandler(oauthSvc OAuthService) *OAuthHandler {
	return &OAuthHandler{oauthSvc: oauthSvc}
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (h *OAuthHandler) OAuthRedirect(c *fiber.Ctx) error {
	provider := c.Params("provider")
	if provider == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  fiber.StatusBadRequest,
			"message": "Provider is required",
		})
	}

	state := generateState()
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    state,
		HTTPOnly: true,
		Secure:   config.AppConfig.AppEnv == "production",
		SameSite: "Lax",
	})

	authURL, err := h.oauthSvc.GetAuthURL(provider, state)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  fiber.StatusBadRequest,
			"message": err.Error(),
		})
	}

	return c.Redirect(authURL, fiber.StatusTemporaryRedirect)
}

func (h *OAuthHandler) OAuthCallback(c *fiber.Ctx) error {
	provider := c.Params("provider")
	if provider == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  fiber.StatusBadRequest,
			"message": "Provider is required",
		})
	}

	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  fiber.StatusBadRequest,
			"message": "Authorization code is required",
		})
	}

	state := c.Query("state")
	storedState := c.Cookies("oauth_state")
	if storedState == "" || state != storedState {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  fiber.StatusBadRequest,
			"message": "Invalid state parameter",
		})
	}

	c.ClearCookie("oauth_state")

	user, token, ws, err := h.oauthSvc.HandleCallback(c.Context(), provider, code, state)
	if err != nil {
		frontendBase := "http://localhost:3000"
		if config.AppConfig != nil && config.AppConfig.AppEnv == "production" {
			frontendBase = config.AppConfig.FrontURL
		}
		if errors.Is(err, ErrRegistrationDisabled) {
			return c.Redirect(frontendBase+"/auth/callback?error=registration_disabled", fiber.StatusTemporaryRedirect)
		}
		return c.Redirect(frontendBase+"/auth/callback?error=oauth_failed", fiber.StatusTemporaryRedirect)
	}

	frontendURL := getFrontendCallbackURL(token, user, ws)
	return c.Redirect(frontendURL, fiber.StatusTemporaryRedirect)
}

func getFrontendCallbackURL(token string, user *User, ws *workspace.Workspace) string {
	frontendBase := "http://localhost:3000"
	if config.AppConfig != nil {
		if config.AppConfig.AppEnv == "production" {
			frontendBase = config.AppConfig.FrontURL
		}
	}

	callbackURL := fmt.Sprintf("%s/auth/callback?token=%s&user_id=%s&user_email=%s&user_name=%s&user_role=%s",
		frontendBase,
		token,
		user.ID,
		user.Email,
		urlEncode(user.Name),
		user.Role,
	)

	if ws != nil {
		callbackURL += fmt.Sprintf("&workspace_id=%s&workspace_name=%s&workspace_slug=%s",
			ws.ID,
			urlEncode(ws.Name),
			ws.Slug,
		)
	}

	return callbackURL
}

func urlEncode(s string) string {
	return url.QueryEscape(s)
}
