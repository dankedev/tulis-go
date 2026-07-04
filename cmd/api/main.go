package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dankedev/kontent/config"
	"github.com/dankedev/kontent/domain/media"
	"github.com/dankedev/kontent/domain/post"
	"github.com/dankedev/kontent/domain/user"
	"github.com/dankedev/kontent/domain/workspace"
	"github.com/dankedev/kontent/middleware"
	"github.com/dankedev/kontent/utils/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func SetupApp() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  fiber.StatusInternalServerError,
				"message": err.Error(),
			})
		},
	})

	app.Use(recover.New())
	app.Use(logger.New())

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  fiber.StatusOK,
			"message": "healthy",
			"time":    time.Now().Format(time.RFC3339),
		})
	})

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

		publicApi.Get("/posts", publicPostHandler.ListPosts)
		publicApi.Get("/posts/:slugOrId", publicPostHandler.GetPost)
		publicApi.Get("/taxonomies", publicPostHandler.ListTaxonomies)
		publicApi.Get("/media", publicMediaHandler.ListMedia)

		// ----------------------------------------------------
		// 2. ADMIN & AUTHENTICATED MANAGEMENT ROUTING
		// ----------------------------------------------------
		api := app.Group("/api")
		api.Post("/register", userHandler.Register)
		api.Post("/login", userHandler.Login)

		authGroup := api.Group("")
		authGroup.Use(middleware.AuthGuard(jwtSvc))

		// User profile (only requires authentication)
		authGroup.Get("/me", userHandler.Me)
		authGroup.Put("/me", userHandler.UpdateProfile)
		authGroup.Put("/me/password", userHandler.ChangePassword)

		// Workspace management (only requires authentication)
		authGroup.Post("/workspaces", wsHandler.Create)
		authGroup.Get("/workspaces", wsHandler.List)
		authGroup.Get("/workspaces/:id", wsHandler.GetByID)
		authGroup.Put("/workspaces/:id", wsHandler.Update)
		authGroup.Delete("/workspaces/:id", wsHandler.Delete)

		// Tenant-scoped group (requires both authentication and valid workspace context)
		tenantGroup := authGroup.Group("")
		tenantGroup.Use(middleware.TenantScoping(wsSvc))

		// Workspace members (requires workspace context)
		tenantGroup.Post("/workspaces/:id/members", wsHandler.AddMember)

		// Content CRUD & Custom Post Types (CPT)
		tenantGroup.Post("/posts", postHandler.Create)
		tenantGroup.Get("/posts", postHandler.List)
		tenantGroup.Get("/posts/:id", postHandler.GetByID)
		tenantGroup.Put("/posts/:id", postHandler.Update)
		tenantGroup.Delete("/posts/:id", postHandler.Delete)

		tenantGroup.Post("/post-types", postHandler.RegisterPostType)
		tenantGroup.Get("/post-types", postHandler.ListPostTypes)
		tenantGroup.Get("/post-types/:id", postHandler.GetPostTypeByID)
		tenantGroup.Delete("/post-types/:id", postHandler.DeletePostType)

		// Post Revisions
		tenantGroup.Get("/posts/:id/revisions", postHandler.ListRevisions)
		tenantGroup.Post("/posts/:id/revisions/:revisionId/restore", postHandler.RestoreRevision)

		// Post Taxonomies
		tenantGroup.Post("/taxonomies", postHandler.CreateTaxonomy)
		tenantGroup.Get("/taxonomies", postHandler.ListTaxonomies)
		tenantGroup.Get("/taxonomies/:id", postHandler.GetTaxonomyByID)
		tenantGroup.Put("/taxonomies/:id", postHandler.UpdateTaxonomy)
		tenantGroup.Delete("/taxonomies/:id", postHandler.DeleteTaxonomy)

		// Media Library Management
		tenantGroup.Post("/media/upload", mediaHandler.Upload)
		tenantGroup.Get("/media", mediaHandler.List)
		tenantGroup.Get("/media/:id", mediaHandler.GetByID)
		tenantGroup.Delete("/media/:id", mediaHandler.Delete)
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
