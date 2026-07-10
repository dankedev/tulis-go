package routes

import (
	"github.com/dankedev/tulis-go/domain/importer"
	"github.com/gofiber/fiber/v2"
)

func RegisterImporterRoutes(tenantGroup fiber.Router, importerHandler *importer.ImporterHandler) {
	tenantGroup.Post("/plugins/importer/upload", importerHandler.Upload)
	tenantGroup.Get("/plugins/importer/logs", importerHandler.ListLogs)
	tenantGroup.Get("/plugins/importer/logs/:id", importerHandler.GetLog)
	tenantGroup.Post("/plugins/importer/csv/upload", importerHandler.UploadCSV)
	tenantGroup.Post("/plugins/importer/csv/import", importerHandler.StartCSVImport)
	tenantGroup.Post("/plugins/importer/strapi/inspect", importerHandler.InspectStrapi)
	tenantGroup.Post("/plugins/importer/strapi/import", importerHandler.StartStrapiImport)
}
