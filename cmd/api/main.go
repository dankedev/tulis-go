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
	"github.com/dankedev/kontent/domain/post"
	"github.com/dankedev/kontent/domain/user"
	"github.com/dankedev/kontent/domain/workspace"
	"github.com/gofiber/fiber/v2"
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
