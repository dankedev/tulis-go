package membership

import (
	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// CreateTier godoc
// @Summary Create a subscription tier
// @Tags Membership
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body SubscriptionTier true "Tier data"
// @Router /api/membership/tiers [post]
func (h *Handler) CreateTier(c *fiber.Ctx) error {
	var tier SubscriptionTier
	if err := c.BodyParser(&tier); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request", nil)
	}
	wsID := c.Locals("workspace_id").(string)
	tier.WorkspaceID = uuid.MustParse(wsID)
	if err := h.db.Create(&tier).Error; err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}
	return response.Success(c, tier, "Tier created")
}

// ListTiers godoc
// @Summary List subscription tiers
// @Tags Membership
// @Produce json
// @Router /api/membership/tiers [get]
func (h *Handler) ListTiers(c *fiber.Ctx) error {
	wsID := c.Locals("workspace_id").(string)
	var tiers []SubscriptionTier
	h.db.Where("workspace_id = ?", wsID).Find(&tiers)
	return response.Success(c, tiers, "Tiers retrieved")
}

// Subscribe godoc
// @Summary Subscribe a user to a tier
// @Tags Membership
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body map[string]string true "JSON with tier_id"
// @Router /api/membership/subscribe [post]
func (h *Handler) Subscribe(c *fiber.Ctx) error {
	var req struct {
		TierID string `json:"tier_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request", nil)
	}
	userID := c.Locals("user_id").(string)
	sub := UserSubscription{
		UserID: uuid.MustParse(userID),
		TierID: uuid.MustParse(req.TierID),
		Status: "active",
	}
	if err := h.db.Create(&sub).Error; err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}
	return response.Success(c, sub, "Subscribed successfully")
}
