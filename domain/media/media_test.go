package media_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dankedev/kontent/domain/media"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestMediaDB(t *testing.T) (*gorm.DB, media.MediaService, *media.MediaHandler) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&media.Media{})
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := media.NewMediaRepository(db)
	svc := media.NewMediaService(repo)
	handler := media.NewMediaHandler(svc)

	return db, svc, handler
}

func generateTestImageBytes(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		t.Fatalf("Failed to generate test image: %v", err)
	}
	return buf.Bytes()
}

func TestMediaServiceAndHandler(t *testing.T) {
	_, _, handler := setupTestMediaDB(t)

	// Clean up uploads folder if any tests write to it
	defer os.RemoveAll("uploads")

	app := fiber.New()
	userID := uuid.New()
	wsID := uuid.New()

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID.String())
		c.Locals("workspace_id", wsID.String())
		return c.Next()
	})

	app.Post("/api/media/upload", handler.Upload)
	app.Get("/api/media", handler.List)
	app.Get("/api/media/:id", handler.GetByID)
	app.Delete("/api/media/:id", handler.Delete)

	var uploadedMediaID string
	var uploadedPath string

	t.Run("Upload Image and Auto-Generate Thumbnail", func(t *testing.T) {
		imageBytes := generateTestImageBytes(t)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "test_avatar.png")
		if err != nil {
			t.Fatal(err)
		}
		part.Write(imageBytes)

		_ = writer.WriteField("alt_text", "Avatar Alt")
		_ = writer.WriteField("caption", "Avatar Caption")
		writer.Close()

		req := httptest.NewRequest("POST", "/api/media/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 upload, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		data := result["data"].(map[string]interface{})
		uploadedMediaID = data["id"].(string)
		uploadedPath = data["path"].(string)

		if data["alt_text"] != "Avatar Alt" {
			t.Errorf("Expected alt_text 'Avatar Alt', got '%v'", data["alt_text"])
		}

		// Verify files exist on local disk
		localPath := strings.TrimPrefix(uploadedPath, "/")
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			t.Errorf("Expected uploaded file to exist at %s, but not found", localPath)
		}

		thumbPath := filepath.Join("uploads", "thumb_"+filepath.Base(localPath))
		if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
			t.Errorf("Expected thumbnail file to exist at %s, but not found", thumbPath)
		}
	})

	t.Run("Get Media Metadata by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/media/"+uploadedMediaID, nil)
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
		if data["filename"] != "test_avatar.png" {
			t.Errorf("Expected filename 'test_avatar.png', got '%v'", data["filename"])
		}
	})

	t.Run("List Media in library", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/media?page=1&per_page=5", nil)
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
		if len(data) != 1 {
			t.Errorf("Expected 1 media item, got %d", len(data))
		}
	})

	t.Run("Delete Media and cleanup disk", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/media/"+uploadedMediaID, nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 delete, got %d", resp.StatusCode)
		}

		// Verify files are cleaned up from local disk
		localPath := strings.TrimPrefix(uploadedPath, "/")
		if _, err := os.Stat(localPath); !os.IsNotExist(err) {
			t.Errorf("Expected original file %s to be deleted, but still exists", localPath)
		}

		thumbPath := filepath.Join("uploads", "thumb_"+filepath.Base(localPath))
		if _, err := os.Stat(thumbPath); !os.IsNotExist(err) {
			t.Errorf("Expected thumbnail file %s to be deleted, but still exists", thumbPath)
		}

		// Verify database metadata is gone
		reqGet := httptest.NewRequest("GET", "/api/media/"+uploadedMediaID, nil)
		respGet, _ := app.Test(reqGet, -1)
		if respGet.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 after deletion, got %d", respGet.StatusCode)
		}
	})
}
