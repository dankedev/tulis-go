package importer

import (
	"strconv"

	"github.com/dankedev/kontent/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ImporterHandler struct {
	svc ImporterService
}

func NewImporterHandler(svc ImporterService) *ImporterHandler {
	return &ImporterHandler{svc: svc}
}

func (h *ImporterHandler) Upload(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	userIDStr := c.Locals("user_id")
	if userIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "File parameter is required", nil)
	}

	if fileHeader.Size > 50*1024*1024 {
		return response.Error(c, "BAD_REQUEST", "File size exceeds 50MB limit", nil)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Failed to read file", nil)
	}
	defer file.Close()

	result, err := h.svc.ImportWXR(c.Context(), workspaceID, authorID, file, fileHeader.Filename)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, result, "Import completed successfully")
}

func (h *ImporterHandler) ListLogs(c *fiber.Ctx) error {
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

	logs, total, err := h.svc.ListImportLogs(c.Context(), workspaceID, page, perPage)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", err.Error(), nil)
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
		"message": "Import logs retrieved successfully",
		"data":    logs,
		"meta":    meta,
	})
}

func (h *ImporterHandler) GetLog(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid log ID", nil)
	}

	log, err := h.svc.GetImportLog(c.Context(), id)
	if err != nil {
		return response.Error(c, "NOT_FOUND", "Import log not found", nil)
	}

	return response.Success(c, log, "Import log retrieved successfully")
}
