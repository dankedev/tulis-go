package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dankedev/tulis-go/config"
	"github.com/dankedev/tulis-go/domain/importer"
	"github.com/dankedev/tulis-go/domain/media"
	"github.com/dankedev/tulis-go/domain/plugin"
	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/setup"
	"github.com/dankedev/tulis-go/domain/user"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/dankedev/tulis-go/middleware"
	"github.com/dankedev/tulis-go/routes"
	"github.com/dankedev/tulis-go/storage"
	"github.com/dankedev/tulis-go/utils/jwt"
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
		AllowOrigins: config.AppConfig.CORSOrigins,
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
		oauthSvc := user.NewOAuthService(userRepo, wsRepo, jwtSvc)
		oauthHandler := user.NewOAuthHandler(oauthSvc)

		setupSvc := setup.NewSetupService(userRepo, userSvc, wsSvc)
		setupHandler := setup.NewSetupHandler(setupSvc)

		pluginRepo := plugin.NewRepository(config.DB)
		pluginSvc := plugin.NewService(pluginRepo)
		pluginHandler := plugin.NewHandler(pluginSvc)

		postRepo := post.NewPostRepository(config.DB)
		postSvc := post.NewPostService(postRepo, pluginSvc)
		postHandler := post.NewPostHandler(postSvc)

		mediaRepo := media.NewMediaRepository(config.DB)

		// Initialize storage (R2 if configured, otherwise local)
		var mediaStorage storage.Storage
		if config.AppConfig.R2AccountID != "" && config.AppConfig.R2AccessKey != "" && config.AppConfig.R2SecretKey != "" {
			r2Storage, err := storage.NewR2Storage(storage.R2Config{
				AccountID:  config.AppConfig.R2AccountID,
				AccessKey:  config.AppConfig.R2AccessKey,
				SecretKey:  config.AppConfig.R2SecretKey,
				BucketName: config.AppConfig.R2BucketName,
				PublicURL:  config.AppConfig.R2PublicURL,
			})
			if err != nil {
				log.Printf("Warning: Failed to initialize R2 storage: %v, falling back to local storage", err)
				mediaStorage = storage.NewLocalStorage("uploads")
			} else {
				mediaStorage = r2Storage
				log.Println("Using Cloudflare R2 for media storage")
			}
		} else {
			mediaStorage = storage.NewLocalStorage("uploads")
			log.Println("Using local storage for media (R2 not configured)")
		}

		mediaSvc := media.NewMediaService(mediaRepo, mediaStorage, pluginSvc)
		mediaHandler := media.NewMediaHandler(mediaSvc)

		importerSvc := importer.NewImporterService(config.DB, mediaSvc, postRepo, mediaRepo)
		importerHandler := importer.NewImporterHandler(importerSvc, pluginSvc)

		// Initialize Public Consumption Handlers
		publicPostHandler := post.NewPublicHandler(postSvc)
		publicMediaHandler := media.NewPublicHandler(mediaSvc)

		// Serve static uploads
		app.Static("/uploads", "./uploads")

		// ----------------------------------------------------
		// 1. PUBLIC API v1 ROUTING (Subdomain Guarded, Rate Limited & Tenant Scoped)
		// ----------------------------------------------------
		v1PublicApi := app.Group("/v1")
		v1PublicApi.Use(middleware.APISubdomainGuard(config.AppConfig.APIHost, config.AppConfig.AppEnv))
		v1PublicApi.Use(limiter.New(limiter.Config{
			Max:        60,
			Expiration: 1 * time.Minute,
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP()
			},
		}))
		v1PublicApi.Use(middleware.TenantScoping(wsSvc))

		// ----------------------------------------------------
		// 2. ADMIN & AUTHENTICATED MANAGEMENT ROUTING
		// ----------------------------------------------------
		api := app.Group("/api")
		routes.RegisterSetupRoutes(api, setupHandler)
		routes.RegisterUserPublicRoutes(api, userHandler)
		routes.RegisterWorkspacePublicRoutes(api, wsHandler)
		routes.RegisterOAuthRoutes(api, oauthHandler)

		authGroup := api.Group("")
		authGroup.Use(middleware.AuthGuard(jwtSvc))

		// Admin-scoped routes (requires authentication + superadmin)
		adminGroup := authGroup.Group("/admin")
		adminGroup.Use(middleware.RequireSystemSuperadmin(userSvc))
		routes.RegisterAdminRoutes(adminGroup, userHandler, wsHandler)

		// Register routes that ONLY require auth (no tenant context)
		routes.RegisterUserAuthRoutes(authGroup, userHandler)
		routes.RegisterWorkspaceRoutes(authGroup, wsHandler)

		// Tenant-scoped group (requires both authentication and valid workspace context)
		tenantGroup := authGroup.Group("")
		tenantGroup.Use(middleware.TenantScoping(wsSvc))

		// Register tenant-scoped routes using domain specific files in routes/
		routes.RegisterWorkspaceMemberRoutes(tenantGroup, wsHandler)
		routes.RegisterPostRoutes(v1PublicApi, tenantGroup, postHandler, publicPostHandler)
		routes.RegisterTaxonomyRoutes(v1PublicApi, tenantGroup, postHandler, publicPostHandler)
		routes.RegisterMediaRoutes(v1PublicApi, tenantGroup, mediaHandler, publicMediaHandler)
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
		&workspace.WorkspaceInvitation{},
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println("Database migration completed successfully")

	// Start background inactivity email notifications scheduler
	user.StartNotificationScheduler(config.DB)

	// 4. Initialize Fiber App
	app := SetupApp()

	// Setup shutdown channel
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	log.Printf("[STARTUP] Server listening on port %s, PID: %d", config.AppConfig.AppPort, os.Getpid())

	// Start server in goroutine
	go func() {
		port := config.AppConfig.AppPort
		log.Printf("[SERVER] Starting HTTP server on :%s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Printf("[SERVER] Server stopped serving: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-shutdown
	log.Printf("[SHUTDOWN] Received signal: %v", sig)

	// Create context with timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("[SHUTDOWN] Initiating graceful shutdown (timeout: 10s)...")

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("[SHUTDOWN] Graceful shutdown error: %v", err)
		if shutdownCtx.Err() == context.DeadlineExceeded {
			log.Printf("[SHUTDOWN] Shutdown timed out - forcing exit")
		}
	} else {
		log.Printf("[SHUTDOWN] All connections closed gracefully")
	}
	log.Printf("[SHUTDOWN] Server stopped")
}
