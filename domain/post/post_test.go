package post_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dankedev/kontent/domain/post"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestPostDB(t *testing.T) (*gorm.DB, post.PostService, *post.PostHandler) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&post.Post{}, &post.PostType{})
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := post.NewPostRepository(db)
	svc := post.NewPostService(repo)
	handler := post.NewPostHandler(svc)

	return db, svc, handler
}

func TestPostServiceAndHandler(t *testing.T) {
	_, _, handler := setupTestPostDB(t)

	app := fiber.New()
	userID := uuid.New()
	wsID := uuid.New()

	// Inject auth and tenant contexts manually
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID.String())
		c.Locals("workspace_id", wsID.String())
		return c.Next()
	})

	// Map routes
	app.Post("/api/posts", handler.Create)
	app.Get("/api/posts", handler.List)
	app.Get("/api/posts/:id", handler.GetByID)
	app.Put("/api/posts/:id", handler.Update)
	app.Delete("/api/posts/:id", handler.Delete)
	app.Post("/api/post-types", handler.RegisterPostType)
	app.Get("/api/post-types", handler.ListPostTypes)
	app.Get("/api/post-types/:id", handler.GetPostTypeByID)
	app.Delete("/api/post-types/:id", handler.DeletePostType)

	var firstPostID string
	var firstPostSlug string

	t.Run("Create Draft Post with auto slugify", func(t *testing.T) {
		reqBody := post.CreatePostReq{
			Title:   "My First Blog Post",
			Content: "This is the content of my first post.",
			Status:  "draft",
		}
		jsonBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		firstPostID = data["id"].(string)
		firstPostSlug = data["slug"].(string)

		if firstPostSlug != "my-first-blog-post" {
			t.Errorf("Expected slug 'my-first-blog-post', got '%s'", firstPostSlug)
		}
		if data["status"] != "draft" {
			t.Errorf("Expected status 'draft', got '%s'", data["status"])
		}
	})

	t.Run("Create duplicate post resolves slug collision", func(t *testing.T) {
		reqBody := post.CreatePostReq{
			Title:   "My First Blog Post",
			Content: "Another post with the exact same title.",
			Status:  "draft",
		}
		jsonBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		duplicateSlug := data["slug"].(string)

		if duplicateSlug != "my-first-blog-post-1" {
			t.Errorf("Expected collision resolved slug to be 'my-first-blog-post-1', got '%s'", duplicateSlug)
		}
	})

	t.Run("Create Scheduled Post requires published_at", func(t *testing.T) {
		reqBody := post.CreatePostReq{
			Title:   "Future scheduled post",
			Content: "This will go live later.",
			Status:  "scheduled",
		}
		jsonBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error when creating scheduled post without published_at")
		}

		// Fix with date
		futureDate := time.Now().Add(24 * time.Hour)
		reqBody.PublishedAt = &futureDate
		jsonBytes, _ = json.Marshal(reqBody)
		req = httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ = app.Test(req, -1)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Get Post by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts/"+firstPostID, nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		if data["title"] != "My First Blog Post" {
			t.Errorf("Expected title 'My First Blog Post', got '%v'", data["title"])
		}
	})

	t.Run("Update Post content and publish status", func(t *testing.T) {
		newTitle := "My First Blog Post (Updated)"
		newStatus := "published"
		updateReq := post.UpdatePostReq{
			Title:  &newTitle,
			Status: &newStatus,
		}
		jsonBytes, _ := json.Marshal(updateReq)

		req := httptest.NewRequest("PUT", "/api/posts/"+firstPostID, bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		if data["title"] != "My First Blog Post (Updated)" {
			t.Errorf("Expected title update, got '%v'", data["title"])
		}
		if data["status"] != "published" {
			t.Errorf("Expected status to be 'published', got '%v'", data["status"])
		}
		if data["published_at"] == nil {
			t.Error("Expected published_at to be auto-populated upon publishing")
		}
	})

	t.Run("List Posts with pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?type=post&page=1&per_page=5", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].([]interface{})
		// We created 3 posts successfully (first draft -> updated, duplicate draft, and scheduled post)
		if len(data) != 3 {
			t.Errorf("Expected 3 posts, got %d", len(data))
		}

		meta := result["meta"].(map[string]interface{})
		if meta["total"] != float64(3) {
			t.Errorf("Expected total metadata to be 3, got %v", meta["total"])
		}
	})

	t.Run("Delete Post", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/posts/"+firstPostID, nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		// Verify deletion
		reqGet := httptest.NewRequest("GET", "/api/posts/"+firstPostID, nil)
		respGet, _ := app.Test(reqGet, -1)
		if respGet.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 after deletion, got %d", respGet.StatusCode)
		}
	})

	t.Run("Custom Post Type Lifecycle and Custom Fields Validation", func(t *testing.T) {
		// 1. Register a new custom post type 'book' with a required field 'isbn' and default field 'genre'
		fields := []post.CustomFieldSchema{
			{Name: "isbn", Label: "ISBN", Type: "text", Required: true},
			{Name: "genre", Label: "Genre", Type: "text", Required: false, DefaultVal: "Fiction"},
		}
		cptReq := post.CreatePostTypeReq{
			Name:        "Book CPT",
			Slug:        "book",
			Description: "Books library CPT",
			Fields:      fields,
		}
		jsonBytes, _ := json.Marshal(cptReq)
		req := httptest.NewRequest("POST", "/api/post-types", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected CPT registration status 200, got %d", resp.StatusCode)
		}

		var cptResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&cptResult)
		cptData := cptResult["data"].(map[string]interface{})
		cptID := cptData["id"].(string)

		// 2. Try to create a Book post without required 'isbn' field -> Should fail
		postReqFail := post.CreatePostReq{
			Title:    "Learn Go in 24 Hours",
			PostType: "book",
			Status:   "draft",
		}
		jsonBytes, _ = json.Marshal(postReqFail)
		req = httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ = app.Test(req, -1)
		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error when missing required custom field 'isbn'")
		}

		// 3. Create a Book post with valid 'isbn' field -> Should succeed and apply 'genre' default value
		postReqSuccess := post.CreatePostReq{
			Title:    "Learn Go in 24 Hours",
			PostType: "book",
			Status:   "draft",
			CustomFields: map[string]interface{}{
				"isbn": "978-3-16-148410-0",
			},
		}
		jsonBytes, _ = json.Marshal(postReqSuccess)
		req = httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var postResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&postResult)
		postData := postResult["data"].(map[string]interface{})
		cf := postData["custom_fields"].(map[string]interface{})

		if cf["isbn"] != "978-3-16-148410-0" {
			t.Errorf("Expected custom field isbn to be populated, got %v", cf["isbn"])
		}
		if cf["genre"] != "Fiction" {
			t.Errorf("Expected default custom field genre to be 'Fiction', got %v", cf["genre"])
		}

		// 4. Delete the custom post type
		reqDel := httptest.NewRequest("DELETE", "/api/post-types/"+cptID, nil)
		respDel, _ := app.Test(reqDel, -1)
		if respDel.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 on deleting CPT, got %d", respDel.StatusCode)
		}
	})
}
