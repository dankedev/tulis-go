package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/dankedev/kontent/docs"
	"github.com/dankedev/kontent/config"
	"github.com/dankedev/kontent/domain/media"
	"github.com/dankedev/kontent/domain/post"
	"github.com/dankedev/kontent/domain/user"
	"github.com/dankedev/kontent/domain/workspace"
	"github.com/dankedev/kontent/domain/plugin"
	"github.com/dankedev/kontent/domain/importer"
	"github.com/dankedev/kontent/middleware"
	"github.com/dankedev/kontent/utils/jwt"
	"github.com/dankedev/kontent/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	swagger "github.com/swaggo/fiber-swagger"
)

func SetupApp() *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit: 50 * 1024 * 1024, // 50MB for file uploads
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  fiber.StatusInternalServerError,
				"message": err.Error(),
			})
		},
	})

	app.Use(recover.New())
	app.Use(logger.New())
	
	// Add CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000, http://127.0.0.1:3000", // Adjust to match your frontend URL (e.g. 5173 for Vite, 3000 for Next.js)
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Workspace-ID",
		AllowMethods: "GET, POST, HEAD, PUT, DELETE, PATCH, OPTIONS",
	}))

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  fiber.StatusOK,
			"message": "healthy",
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// Swagger UI
	app.Get("/swagger/*", swagger.FiberWrapHandler())

	if config.DB != nil {
		// Initialize Core Services & Handlers
		jwtSvc := jwt.NewJWTService(config.AppConfig.JWTSecret, time.Duration(config.AppConfig.JWTExpiryHours)*time.Hour)
		wsRepo := workspace.NewWorkspaceRepository(config.DB)
		wsSvc := workspace.NewWorkspaceService(wsRepo)
		wsHandler := workspace.NewWorkspaceHandler(wsSvc)

		userRepo := user.NewUserRepository(config.DB)
		userSvc := user.NewUserService(userRepo, wsRepo, jwtSvc)
		userHandler := user.NewAuthHandler(userSvc)

		postRepo := post.NewPostRepository(config.DB)
		postSvc := post.NewPostService(postRepo)
		postHandler := post.NewPostHandler(postSvc)

		mediaRepo := media.NewMediaRepository(config.DB)
		mediaSvc := media.NewMediaService(mediaRepo)
		mediaHandler := media.NewMediaHandler(mediaSvc)

		pluginRepo := plugin.NewRepository(config.DB)
		pluginSvc := plugin.NewService(pluginRepo)
		pluginHandler := plugin.NewHandler(pluginSvc)

		importerSvc := importer.NewImporterService(config.DB, mediaSvc, postRepo, mediaRepo)
		importerHandler := importer.NewImporterHandler(importerSvc)

		// Initialize Public Consumption Handlers
		publicPostHandler := post.NewPublicHandler(postSvc)
		publicMediaHandler := media.NewPublicHandler(mediaSvc)

		// Serve static uploads
		app.Static("/uploads", "./uploads")

		// ----------------------------------------------------
		// 1. PUBLIC API v1 ROUTING (Rate Limited & Tenant Scoped)
		// ----------------------------------------------------
		publicApi := app.Group("/api/v1/public")
		publicApi.Use(limiter.New(limiter.Config{
			Max:        60,
			Expiration: 1 * time.Minute,
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP()
			},
		}))
		publicApi.Use(middleware.TenantScoping(wsSvc))

		// ----------------------------------------------------
		// 2. ADMIN & AUTHENTICATED MANAGEMENT ROUTING
		// ----------------------------------------------------
		api := app.Group("/api")
		routes.RegisterUserPublicRoutes(api, userHandler)

		authGroup := api.Group("")
		authGroup.Use(middleware.AuthGuard(jwtSvc))

		// Register routes that ONLY require auth (no tenant context)
		routes.RegisterUserAuthRoutes(authGroup, userHandler)
		routes.RegisterWorkspaceRoutes(authGroup, wsHandler)

		// Tenant-scoped group (requires both authentication and valid workspace context)
		tenantGroup := authGroup.Group("")
		tenantGroup.Use(middleware.TenantScoping(wsSvc))

		// Register tenant-scoped routes using domain specific files in routes/
		routes.RegisterWorkspaceMemberRoutes(tenantGroup, wsHandler)
		routes.RegisterPostRoutes(publicApi, tenantGroup, postHandler, publicPostHandler)
		routes.RegisterTaxonomyRoutes(publicApi, tenantGroup, postHandler, publicPostHandler)
		routes.RegisterMediaRoutes(publicApi, tenantGroup, mediaHandler, publicMediaHandler)
		routes.RegisterPluginRoutes(tenantGroup, pluginHandler)
		routes.RegisterImporterRoutes(tenantGroup, importerHandler)
	}

	return app
}

func main() {
	// 1. Load configuration
	config.LoadConfig()

	// 2. Connect to database
	config.ConnectDB()

	// 3. Run Auto-migrations
	fmt.Println("Running database migrations...")
	err := config.DB.AutoMigrate(
		&user.User{},
		&workspace.Workspace{},
		&workspace.WorkspaceMember{},
		&post.Post{},
		&post.PostType{},
		&post.PostRevision{},
		&post.Taxonomy{},
		&post.PostTaxonomy{},
		&media.Media{},
		&plugin.WorkspacePlugin{},
		&importer.ImportLog{},
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println("Database migration completed successfully")

	// 4. Initialize Fiber App
	app := SetupApp()

	// Setup shutdown channel
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		port := config.AppConfig.AppPort
		if err := app.Listen(":" + port); err != nil {
			log.Printf("Server failed to serve: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-shutdown
	fmt.Println("Shutting down server gracefully...")

	// Create context with timeout for graceful shutdown
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Shutdown(); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}
	fmt.Println("Server gracefully stopped")
}
