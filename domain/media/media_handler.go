package media

import (
	"io"
	"strconv"

	"github.com/dankedev/kontent/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type MediaHandler struct {
	svc MediaService
}

func NewMediaHandler(svc MediaService) *MediaHandler {
	return &MediaHandler{svc: svc}
}

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
