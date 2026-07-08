// Package importer Tulis CMS Importer API
//
//	WordPress WXR import management
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
package importer

import (
	"strconv"

	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ImporterHandler struct {
	svc ImporterService
}

func NewImporterHandler(svc ImporterService) *ImporterHandler {
	return &ImporterHandler{svc: svc}
}

// Upload godoc
// @Summary Upload WordPress WXR file
// @Description Imports a WordPress WXR (XML) file, creating posts, media, and taxonomies
// @Tags Importer
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param file formData file true "WXR file (.xml)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/plugins/importer/upload [post]
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

	importLog, err := h.svc.ImportWXR(c.Context(), workspaceID, authorID, file, fileHeader.Filename)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, fiber.Map{
		"id":            importLog.ID,
		"filename":      importLog.Filename,
		"status":        importLog.Status,
		"posts_count":   importLog.PostsCount,
		"pages_count":   importLog.PagesCount,
		"media_count":   importLog.MediaCount,
		"tax_count":     importLog.TaxCount,
		"skipped_count": importLog.SkippedCount,
		"error_message": importLog.Errors,
		"created_at":    importLog.CreatedAt,
		"finished_at":   importLog.UpdatedAt,
	}, "Import completed successfully")
}

// ListLogs godoc
// @Summary List import logs
// @Description Returns a paginated list of WXR import logs for the workspace
// @Tags Importer
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/plugins/importer/logs [get]
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

// GetLog godoc
// @Summary Get import log by ID
// @Description Returns a single import log by its UUID
// @Tags Importer
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param id path string true "Import log UUID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/plugins/importer/logs/{id} [get]
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
