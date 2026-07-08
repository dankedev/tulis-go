// Package media Tulis CMS Media API
//
//	Media library upload and management
//
//	Schemes: http
//	BasePath: /api
//	Version: 1.0.0
//
//	SecurityDefinitions:
//	Bearer:
//	     type: apiKey
//	     name: Authorization
//	     in: header
package media

import (
	"io"
	"strconv"

	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type MediaHandler struct {
	svc MediaService
}

func NewMediaHandler(svc MediaService) *MediaHandler {
	return &MediaHandler{svc: svc}
}

// Upload godoc
// @Summary Upload a media file
// @Description Uploads a file to the workspace media library (multipart/form-data)
// @Tags Media
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param file formData file true "Media file"
// @Param alt_text formData string false "Alt text"
// @Param caption formData string false "Caption"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/media/upload [post]
func (h *MediaHandler) Upload(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "File parameter is required", nil)
	}

	altText := c.FormValue("alt_text", "")
	caption := c.FormValue("caption", "")

	file, err := fileHeader.Open()
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Failed to read file", nil)
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Failed to read file contents", nil)
	}

	m, err := h.svc.SaveFile(
		c.Context(),
		workspaceID,
		fileHeader.Filename,
		fileData,
		fileHeader.Header.Get("Content-Type"),
		fileHeader.Size,
		altText,
		caption,
	)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, m, "File uploaded successfully")
}

// GetByID godoc
// @Summary Get media by ID
// @Description Returns a single media item metadata by its UUID
// @Tags Media
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Media UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/media/{id} [get]
func (h *MediaHandler) GetByID(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid media ID", nil)
	}

	m, err := h.svc.GetMediaByID(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", err.Error(), nil)
	}

	return response.Success(c, m, "Media metadata retrieved successfully")
}

// Delete godoc
// @Summary Delete a media item
// @Description Permanently deletes a media item and its file by UUID
// @Tags Media
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Media UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/media/{id} [delete]
func (h *MediaHandler) Delete(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid media ID", nil)
	}

	if err := h.svc.DeleteMedia(c.Context(), id); err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, nil, "Media deleted successfully")
}

// List godoc
// @Summary List media library
// @Description Returns a paginated list of media items in the workspace
// @Tags Media
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/media [get]
func (h *MediaHandler) List(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "10"))

	mediaList, total, err := h.svc.ListMedia(c.Context(), workspaceID, page, perPage)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	if totalPages == 0 {
		totalPages = 1
	}

	meta := &response.Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  fiber.StatusOK,
		"message": "Media library retrieved successfully",
		"data":    mediaList,
		"meta":    meta,
	})
}
