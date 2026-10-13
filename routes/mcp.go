package routes

import (
	"time"

	"github.com/dankedev/tulis-go/domain/apikey"
	"github.com/dankedev/tulis-go/domain/mcp"
	"github.com/dankedev/tulis-go/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RegisterMCPRoutes registers Streamable HTTP and SSE MCP endpoints
func RegisterMCPRoutes(app fiber.Router, handler *mcp.Handler, apiKeySvc *apikey.Service) {
	mcpGroup := app.Group("")
	if apiKeySvc != nil {
		mcpGroup.Use(middleware.ApiKeyAuth(apiKeySvc))
	}

	// Rate limit per API key (falls back to IP). Applied per route, not via Use: a group-level Use
	// on an empty prefix would also capture routes main.go registers on app after this call.
	rateLimit := limiter.New(limiter.Config{
		Max:        120,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			if id, ok := c.Locals("api_key_id").(string); ok && id != "" {
				return id
			}
			return c.IP()
		},
	})

	// Root /mcp endpoints (compatible with standard MCP client URLs like https://api.tulis.org/mcp)
	mcpGroup.Post("/mcp", rateLimit, handler.HandlePost)
	mcpGroup.Delete("/mcp", rateLimit, handler.HandleDelete)
	mcpGroup.Get("/mcp", rateLimit, handler.HandleSSE)
	mcpGroup.Get("/mcp/sse", rateLimit, handler.HandleSSE)
	mcpGroup.Post("/mcp/messages", rateLimit, handler.HandleSSEMessages)

	// Subpath /api/mcp aliases
	mcpGroup.Post("/api/mcp", rateLimit, handler.HandlePost)
	mcpGroup.Delete("/api/mcp", rateLimit, handler.HandleDelete)
	mcpGroup.Get("/api/mcp", rateLimit, handler.HandleSSE)
	mcpGroup.Get("/api/mcp/sse", rateLimit, handler.HandleSSE)
	mcpGroup.Post("/api/mcp/messages", rateLimit, handler.HandleSSEMessages)
}
