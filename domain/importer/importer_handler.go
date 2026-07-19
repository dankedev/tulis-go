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
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dankedev/tulis-go/domain/plugin"
	"github.com/dankedev/tulis-go/utils/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ImporterHandler struct {
	svc       ImporterService
	pluginSvc plugin.Service
}

func NewImporterHandler(svc ImporterService, pluginSvc plugin.Service) *ImporterHandler {
	return &ImporterHandler{svc: svc, pluginSvc: pluginSvc}
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

	// Check if wordpress-import plugin is enabled
	plugins, err := h.pluginSvc.ListPlugins(c.Context(), workspaceID)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to verify plugin status", nil)
	}
	var importEnabled bool
	for _, p := range plugins {
		if p.ID == "wordpress-import" {
			importEnabled = p.Enabled
			break
		}
	}
	if !importEnabled {
		return response.Error(c, "FORBIDDEN", "Plugin WordPress XML Import must be enabled to perform this action", nil)
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
	}, "Import started successfully in the background")
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

// UploadCSV godoc
// @Summary Upload and parse CSV headers
// @Description Uploads a CSV file, saves it to storage, and extracts its headers for mapping
// @Tags Importer
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param file formData file true "CSV file"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/plugins/importer/csv/upload [post]
func (h *ImporterHandler) UploadCSV(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	// Check if wordpress-import plugin is enabled
	plugins, err := h.pluginSvc.ListPlugins(c.Context(), workspaceID)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to verify plugin status", nil)
	}
	var importEnabled bool
	for _, p := range plugins {
		if p.ID == "wordpress-import" {
			importEnabled = p.Enabled
			break
		}
	}
	if !importEnabled {
		return response.Error(c, "FORBIDDEN", "Plugin WordPress XML Import must be enabled to perform this action", nil)
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

	fileData, err := io.ReadAll(file)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Failed to read file contents", nil)
	}

	fileURL, headers, err := h.svc.UploadCSV(c.Context(), workspaceID, fileHeader.Filename, fileData)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, fiber.Map{
		"file_url": fileURL,
		"headers":  headers,
	}, "CSV parsed and uploaded successfully")
}

type StartCSVImportReq struct {
	FileURL         string            `json:"file_url"`
	Mapping         map[string]string `json:"mapping"`
	DefaultStatus   string            `json:"default_status"`
	DefaultPostType string            `json:"default_post_type"`
}

// StartCSVImport godoc
// @Summary Start background CSV import
// @Description Spawns a background process to import CSV rows based on field mappings
// @Tags Importer
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param req body StartCSVImportReq true "CSV import details and mapping configuration"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/plugins/importer/csv/import [post]
func (h *ImporterHandler) StartCSVImport(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	// Check if wordpress-import plugin is enabled
	plugins, err := h.pluginSvc.ListPlugins(c.Context(), workspaceID)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to verify plugin status", nil)
	}
	var importEnabled bool
	for _, p := range plugins {
		if p.ID == "wordpress-import" {
			importEnabled = p.Enabled
			break
		}
	}
	if !importEnabled {
		return response.Error(c, "FORBIDDEN", "Plugin WordPress XML Import must be enabled to perform this action", nil)
	}

	userIDStr := c.Locals("user_id")
	if userIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	var req StartCSVImportReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.FileURL == "" {
		return response.Error(c, "BAD_REQUEST", "file_url is required", nil)
	}

	// Create running log
	filename := filepath.Base(req.FileURL)
	if idx := strings.Index(filename, "?"); idx != -1 {
		filename = filename[:idx]
	}

	log := &ImportLog{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		AuthorID:    authorID,
		Filename:    filename,
		Status:      "running",
	}

	if err := h.svc.(*importerService).db.WithContext(c.Context()).Create(log).Error; err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to create import session log", nil)
	}

	// Start asynchronous background process
	go h.svc.ImportCSVBackground(context.Background(), workspaceID, authorID, log.ID, req.FileURL, req.Mapping, req.DefaultStatus, req.DefaultPostType)

	return response.Success(c, log, "CSV import started in the background")
}

type InspectStrapiReq struct {
	StrapiURL      string `json:"strapi_url"`
	APIToken       string `json:"api_token"`
	CollectionType string `json:"collection_type"`
}

type StartStrapiImportReq struct {
	StrapiURL       string            `json:"strapi_url"`
	APIToken        string            `json:"api_token"`
	CollectionType  string            `json:"collection_type"`
	Mapping         map[string]string `json:"mapping"`
	DefaultStatus   string            `json:"default_status"`
	DefaultPostType string            `json:"default_post_type"`
}

// InspectStrapi godoc
// @Summary Inspect Strapi schema fields
// @Description Fetches sample data from a Strapi endpoint and extracts its schema keys
// @Tags Importer
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param req body InspectStrapiReq true "Strapi connection details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/plugins/importer/strapi/inspect [post]
func (h *ImporterHandler) InspectStrapi(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	// Check if strapi-import plugin is enabled
	plugins, err := h.pluginSvc.ListPlugins(c.Context(), workspaceID)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to verify plugin status", nil)
	}
	var importEnabled bool
	for _, p := range plugins {
		if p.ID == "strapi-import" {
			importEnabled = p.Enabled
			break
		}
	}
	if !importEnabled {
		return response.Error(c, "FORBIDDEN", "Plugin Strapi API Import must be enabled to perform this action", nil)
	}

	var req InspectStrapiReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.StrapiURL == "" {
		return response.Error(c, "BAD_REQUEST", "strapi_url is required", nil)
	}
	if req.CollectionType == "" {
		return response.Error(c, "BAD_REQUEST", "collection_type is required", nil)
	}

	fields, err := h.svc.InspectStrapi(c.Context(), req.StrapiURL, req.APIToken, req.CollectionType)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, fiber.Map{
		"fields": fields,
	}, "Strapi collection fields retrieved successfully")
}

// StartStrapiImport godoc
// @Summary Start background Strapi content import
// @Description Fetches content from Strapi in the background and saves it to the database
// @Tags Importer
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param req body StartStrapiImportReq true "Strapi import details and field mappings"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/plugins/importer/strapi/import [post]
func (h *ImporterHandler) StartStrapiImport(c *fiber.Ctx) error {
	wsIDStr := c.Locals("workspace_id")
	if wsIDStr == nil {
		return response.Error(c, "BAD_REQUEST", "Workspace context required", nil)
	}
	workspaceID, err := uuid.Parse(wsIDStr.(string))
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid workspace ID", nil)
	}

	// Check if strapi-import plugin is enabled
	plugins, err := h.pluginSvc.ListPlugins(c.Context(), workspaceID)
	if err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to verify plugin status", nil)
	}
	var importEnabled bool
	for _, p := range plugins {
		if p.ID == "strapi-import" {
			importEnabled = p.Enabled
			break
		}
	}
	if !importEnabled {
		return response.Error(c, "FORBIDDEN", "Plugin Strapi API Import must be enabled to perform this action", nil)
	}

	userIDStr := c.Locals("user_id")
	if userIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

	var req StartStrapiImportReq
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid request body", nil)
	}

	if req.StrapiURL == "" {
		return response.Error(c, "BAD_REQUEST", "strapi_url is required", nil)
	}
	if req.CollectionType == "" {
		return response.Error(c, "BAD_REQUEST", "collection_type is required", nil)
	}

	log := &ImportLog{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		AuthorID:    authorID,
		Filename:    fmt.Sprintf("Strapi: %s", req.CollectionType),
		Status:      "running",
	}

	if err := h.svc.(*importerService).db.WithContext(c.Context()).Create(log).Error; err != nil {
		return response.Error(c, "INTERNAL_ERROR", "Failed to create import session log", nil)
	}

	// Start asynchronous background process
	go h.svc.ImportStrapiBackground(context.Background(), workspaceID, authorID, log.ID, req.StrapiURL, req.APIToken, req.CollectionType, req.Mapping, req.DefaultStatus, req.DefaultPostType)

	return response.Success(c, log, "Strapi import started in the background")
}

// ImportMarkdown godoc
// @Summary Import markdown files from a zip archive
// @Description Upload a zip of .md files organized in folders. Each folder becomes a category, each .md file becomes a post.
// @Tags Importer
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param Authorization header string true "Bearer token"
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param file formData file true "Zip file containing markdown files"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/plugins/importer/markdown/upload [post]
func (h *ImporterHandler) ImportMarkdown(c *fiber.Ctx) error {
	authUserIDStr := c.Locals("user_id")
	if authUserIDStr == nil {
		return response.Error(c, "UNAUTHORIZED", "Not authenticated", nil)
	}
	authorID, err := uuid.Parse(authUserIDStr.(string))
	if err != nil {
		return response.Error(c, "UNAUTHORIZED", "Invalid user ID", nil)
	}

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
		return response.Error(c, "BAD_REQUEST", "File is required", nil)
	}

	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".zip") {
		return response.Error(c, "BAD_REQUEST", "Only .zip files are accepted", nil)
	}

	if fileHeader.Size > 50*1024*1024 {
		return response.Error(c, "BAD_REQUEST", "File size exceeds 50MB limit", nil)
	}

	f, err := fileHeader.Open()
	if err != nil {
		return response.Error(c, "BAD_REQUEST", "Failed to open uploaded file", nil)
	}
	defer f.Close()

	importLog, err := h.svc.ImportMarkdown(c.Context(), workspaceID, authorID, f, fileHeader.Filename)
	if err != nil {
		return response.Error(c, "BAD_REQUEST", err.Error(), nil)
	}

	return response.Success(c, importLog, "Markdown import completed")
}


