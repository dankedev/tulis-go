package collab

import (
	"time"

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

// AcquireLock godoc
// @Summary Acquire a content lock for editing
// @Tags Collaboration
// @Produce json
// @Security BearerAuth
// @Param id path string true "Post ID"
// @Router /api/collab/lock/:id [post]
func (h *Handler) AcquireLock(c *fiber.Ctx) error {
	postID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}
	userID := uuid.MustParse(c.Locals("user_id").(string))

	// Check if already locked
	var existing ContentLock
	if h.db.Where("post_id = ? AND expires_at > ?", postID, time.Now()).First(&existing).Error == nil {
		if existing.UserID != userID {
			return response.Error(c, "FORBIDDEN", "Post is being edited by "+existing.UserName, nil)
		}
		// Extend lock
		existing.ExpiresAt = time.Now().Add(5 * time.Minute)
		h.db.Save(&existing)
		return response.Success(c, existing, "Lock extended")
	}

	lock := ContentLock{
		PostID:    postID,
		UserID:    userID,
		UserName:  c.Locals("user_name").(string),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	if err := h.db.Create(&lock).Error; err != nil {
		return response.Error(c, "BAD_REQUEST", "Failed to acquire lock", nil)
	}
	return response.Success(c, lock, "Lock acquired")
}

// ReleaseLock godoc
// @Summary Release a content lock
// @Tags Collaboration
// @Produce json
// @Security BearerAuth
// @Param id path string true "Post ID"
// @Router /api/collab/unlock/:id [post]
func (h *Handler) ReleaseLock(c *fiber.Ctx) error {
	postID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}
	h.db.Where("post_id = ?", postID).Delete(&ContentLock{})
	return response.Success(c, nil, "Lock released")
}

// GetLock godoc
// @Summary Get current lock status for a post
// @Tags Collaboration
// @Produce json
// @Param id path string true "Post ID"
// @Router /api/collab/lock/:id [get]
func (h *Handler) GetLock(c *fiber.Ctx) error {
	postID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid post ID", nil)
	}
	var lock ContentLock
	if h.db.Where("post_id = ? AND expires_at > ?", postID, time.Now()).First(&lock).Error != nil {
		return response.Success(c, fiber.Map{"locked": false}, "No active lock")
	}
	return response.Success(c, fiber.Map{
		"locked":    true,
		"user_name": lock.UserName,
		"user_id":   lock.UserID,
		"expires_at": lock.ExpiresAt,
	}, "Lock status")
}
