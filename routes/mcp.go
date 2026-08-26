package routes

import (
	"github.com/dankedev/tulis-go/domain/apikey"
	"github.com/dankedev/tulis-go/domain/mcp"
	"github.com/dankedev/tulis-go/middleware"
	"github.com/gofiber/fiber/v2"
)

// RegisterMCPRoutes registers Streamable HTTP and SSE MCP endpoints
func RegisterMCPRoutes(app fiber.Router, handler *mcp.Handler, apiKeySvc *apikey.Service) {
	mcpGroup := app.Group("")
	if apiKeySvc != nil {
		mcpGroup.Use(middleware.ApiKeyAuth(apiKeySvc))
	}

	// Root /mcp endpoints (compatible with standard MCP client URLs like https://api.tulis.org/mcp)
	mcpGroup.Post("/mcp", handler.HandlePost)
	mcpGroup.Get("/mcp", handler.HandleSSE)
	mcpGroup.Get("/mcp/sse", handler.HandleSSE)
	mcpGroup.Post("/mcp/messages", handler.HandleSSEMessages)

	// Subpath /api/mcp aliases
	mcpGroup.Post("/api/mcp", handler.HandlePost)
	mcpGroup.Get("/api/mcp", handler.HandleSSE)
	mcpGroup.Get("/api/mcp/sse", handler.HandleSSE)
	mcpGroup.Post("/api/mcp/messages", handler.HandleSSEMessages)
}
