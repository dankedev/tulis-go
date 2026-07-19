package post_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/webhook"
	"github.com/dankedev/tulis-go/domain/workspace"
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

	err = db.AutoMigrate(&post.Post{}, &post.PostType{}, &post.PostRevision{}, &post.Taxonomy{}, &post.PostTaxonomy{}, &workspace.WorkspaceMember{})
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	wsRepo := workspace.NewWorkspaceRepository(db)
	wsSvc := workspace.NewWorkspaceService(wsRepo)

	webhookRepo := webhook.NewRepository(db)
	webhookSvc := webhook.NewService(webhookRepo)

	repo := post.NewPostRepository(db)
	svc := post.NewPostService(repo, nil)
	handler := post.NewPostHandler(svc, wsSvc, webhookSvc)

	return db, svc, handler
}

func TestPostServiceAndHandler(t *testing.T) {
	db, svc, handler := setupTestPostDB(t)

	app := fiber.New()
	userID := uuid.New()
	wsID := uuid.New()

	// Insert test member so role validation checks pass
	err := db.Create(&workspace.WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        "superadmin",
	}).Error
	if err != nil {
		t.Fatalf("Failed to create test member: %v", err)
	}

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
	app.Get("/api/posts/:id/revisions", handler.ListRevisions)
	app.Post("/api/posts/:id/revisions/:revisionId/restore", handler.RestoreRevision)
	app.Post("/api/taxonomies", handler.CreateTaxonomy)
	app.Get("/api/taxonomies", handler.ListTaxonomies)
	app.Get("/api/taxonomies/:id", handler.GetTaxonomyByID)
	app.Put("/api/taxonomies/:id", handler.UpdateTaxonomy)
	app.Delete("/api/taxonomies/:id", handler.DeleteTaxonomy)

	pubHandler := post.NewPublicHandler(svc)
	app.Get("/v1/posts", pubHandler.ListPosts)
	app.Get("/v1/posts/:slugOrId", pubHandler.GetPost)

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

	t.Run("Post Revisions auto-save and restore", func(t *testing.T) {
		// 1. Create a post
		createReq := post.CreatePostReq{
			Title:   "Original Title",
			Content: "Original Content",
			Status:  "draft",
		}
		jsonBytes, _ := json.Marshal(createReq)
		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		var createRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&createRes)
		pData := createRes["data"].(map[string]interface{})
		pID := pData["id"].(string)

		// 2. Update the post to save a new revision
		updatedTitle := "Updated Title"
		updateReq := post.UpdatePostReq{
			Title: &updatedTitle,
		}
		jsonBytes, _ = json.Marshal(updateReq)
		req = httptest.NewRequest("PUT", "/api/posts/"+pID, bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		_, _ = app.Test(req, -1)

		// 3. List revisions -> Should have 2 revisions (1 initial, 1 updated)
		req = httptest.NewRequest("GET", "/api/posts/"+pID+"/revisions", nil)
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 listing revisions, got %d", resp.StatusCode)
		}

		var listRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&listRes)
		revisions := listRes["data"].([]interface{})
		if len(revisions) != 2 {
			t.Errorf("Expected 2 revisions, got %d", len(revisions))
		}

		// Revisions are order desc (newest first). The second revision (index 1) is the original state.
		origRev := revisions[1].(map[string]interface{})
		origRevID := origRev["id"].(string)

		// 4. Restore the post to the original state
		req = httptest.NewRequest("POST", "/api/posts/"+pID+"/revisions/"+origRevID+"/restore", nil)
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 restoring revision, got %d", resp.StatusCode)
		}

		// 5. Get the post -> Title should be back to 'Original Title'
		req = httptest.NewRequest("GET", "/api/posts/"+pID, nil)
		resp, _ = app.Test(req, -1)
		var postRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&postRes)
		postData := postRes["data"].(map[string]interface{})
		if postData["title"] != "Original Title" {
			t.Errorf("Expected restored title to be 'Original Title', got '%v'", postData["title"])
		}
	})

	t.Run("Taxonomy Lifecycle and Post Assignment", func(t *testing.T) {
		// 1. Create a Category taxonomy
		catReq := post.CreateTaxonomyReq{
			Name: "Tech News",
			Slug: "tech-news",
			Type: "category",
		}
		jsonBytes, _ := json.Marshal(catReq)
		req := httptest.NewRequest("POST", "/api/taxonomies", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 creating taxonomy, got %d", resp.StatusCode)
		}

		var catRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&catRes)
		catData := catRes["data"].(map[string]interface{})
		catID := catData["id"].(string)

		// 2. Create a Post and assign this taxonomy
		postReq := post.CreatePostReq{
			Title:       "Latest Tech Article",
			Content:     "Some tech content.",
			Status:      "draft",
			TaxonomyIDs: []string{catID},
		}
		jsonBytes, _ = json.Marshal(postReq)
		req = httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 creating post, got %d", resp.StatusCode)
		}

		var postRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&postRes)
		postData := postRes["data"].(map[string]interface{})
		postID := postData["id"].(string)
		taxonomies := postData["taxonomies"].([]interface{})

		if len(taxonomies) != 1 {
			t.Errorf("Expected 1 taxonomy assigned, got %d", len(taxonomies))
		}
		firstTax := taxonomies[0].(map[string]interface{})
		if firstTax["id"] != catID {
			t.Errorf("Expected assigned taxonomy ID to match catID %s, got %v", catID, firstTax["id"])
		}

		// 3. Update taxonomy details
		updateTax := post.UpdateTaxonomyReq{
			Name: "Tech News (Updated)",
		}
		jsonBytes, _ = json.Marshal(updateTax)
		req = httptest.NewRequest("PUT", "/api/taxonomies/"+catID, bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 updating taxonomy, got %d", resp.StatusCode)
		}

		// 4. Fetch the post again -> Taxonomy name should reflect update or be retrieved correctly
		req = httptest.NewRequest("GET", "/api/posts/"+postID, nil)
		resp, _ = app.Test(req, -1)
		json.NewDecoder(resp.Body).Decode(&postRes)
		postData = postRes["data"].(map[string]interface{})
		taxonomies = postData["taxonomies"].([]interface{})
		firstTax = taxonomies[0].(map[string]interface{})
		if firstTax["name"] != "Tech News (Updated)" {
			t.Errorf("Expected preloaded taxonomy name to be updated, got '%v'", firstTax["name"])
		}

		// 5. Delete taxonomy
		req = httptest.NewRequest("DELETE", "/api/taxonomies/"+catID, nil)
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 deleting taxonomy, got %d", resp.StatusCode)
		}
	})

	t.Run("Public REST API Headless Consumption", func(t *testing.T) {
		// 1. Create a published post
		pubPostReq := post.CreatePostReq{
			Title:   "Public Announcement",
			Content: "This is public.",
			Status:  "published",
		}
		jsonBytes, _ := json.Marshal(pubPostReq)
		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		var pubPostRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&pubPostRes)
		pubPostData := pubPostRes["data"].(map[string]interface{})
		pubSlug := pubPostData["slug"].(string)

		// 2. Create a draft post
		draftPostReq := post.CreatePostReq{
			Title:   "Draft Internal Info",
			Content: "This is private draft.",
			Status:  "draft",
		}
		jsonBytes, _ = json.Marshal(draftPostReq)
		req = httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ = app.Test(req, -1)

		var draftPostRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&draftPostRes)
		draftPostData := draftPostRes["data"].(map[string]interface{})
		draftSlug := draftPostData["slug"].(string)

		// 3. Consume via Public Endpoint (GET /v1/posts)
		req = httptest.NewRequest("GET", "/v1/posts", nil)
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 consuming public posts, got %d", resp.StatusCode)
		}

		var publicList map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&publicList)
		posts := publicList["data"].([]interface{})

		// Draft post should NOT be visible in public endpoint list
		for _, p := range posts {
			pMap := p.(map[string]interface{})
			if pMap["status"] != "published" {
				t.Errorf("Expected only published posts in public list, got status: %v", pMap["status"])
			}
			if pMap["slug"] == draftSlug {
				t.Errorf("Security violation: draft post slug '%s' found in public list", draftSlug)
			}
		}

		// 4. Retrieve published post directly via slug (GET /v1/posts/:slugOrId)
		req = httptest.NewRequest("GET", "/v1/posts/"+pubSlug, nil)
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 fetching public post by slug, got %d", resp.StatusCode)
		}

		// 5. Retrieve draft post directly via slug (GET /v1/posts/:slugOrId) -> Should return 404 Not Found
		req = httptest.NewRequest("GET", "/v1/posts/"+draftSlug, nil)
		resp, _ = app.Test(req, -1)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 fetching draft post via public endpoint, got %d", resp.StatusCode)
		}
	})

	// ---- feat-053: Post Type Filtering and Validation Tests ----

	t.Run("Create page post type", func(t *testing.T) {
		reqBody := post.CreatePostReq{
			Title:    "About Us Page",
			Content:  "This is the about page.",
			Status:   "draft",
			PostType: "page",
		}
		jsonBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 creating page post, got %d", resp.StatusCode)
		}
	})

	t.Run("Register custom post type 'portfolio'", func(t *testing.T) {
		cptReq := post.CreatePostTypeReq{
			Name:        "Portfolio Item",
			Slug:        "portfolio",
			Description: "Portfolio items CPT",
			Fields: []post.CustomFieldSchema{
				{Name: "url", Label: "Project URL", Type: "url", Required: false},
			},
		}
		jsonBytes, _ := json.Marshal(cptReq)
		req := httptest.NewRequest("POST", "/api/post-types", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 registering CPT, got %d", resp.StatusCode)
		}
	})

	t.Run("Create portfolio CPT post", func(t *testing.T) {
		reqBody := post.CreatePostReq{
			Title:    "My First Project",
			Content:  "Project description",
			Status:   "draft",
			PostType: "portfolio",
			CustomFields: map[string]interface{}{
				"url": "https://example.com/project",
			},
		}
		jsonBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 creating portfolio post, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /api/posts?type=post returns only posts with post_type='post'", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?type=post", nil)
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
		for i, p := range data {
			pMap := p.(map[string]interface{})
			if pMap["post_type"] != "post" {
				t.Errorf("Item %d: expected post_type='post', got '%v'", i, pMap["post_type"])
			}
		}
		meta := result["meta"].(map[string]interface{})
		t.Logf("Found %v posts with type=post", meta["total"])
	})

	t.Run("GET /api/posts?type=page returns only posts with post_type='page'", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?type=page", nil)
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
		for i, p := range data {
			pMap := p.(map[string]interface{})
			if pMap["post_type"] != "page" {
				t.Errorf("Item %d: expected post_type='page', got '%v'", i, pMap["post_type"])
			}
		}
		if len(data) != 1 {
			t.Errorf("Expected exactly 1 page post, got %d", len(data))
		}
		meta := result["meta"].(map[string]interface{})
		if meta["total"] != float64(1) {
			t.Errorf("Expected total=1, got %v", meta["total"])
		}
	})

	t.Run("GET /api/posts?type=portfolio returns only portfolio CPT posts", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?type=portfolio", nil)
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
		for i, p := range data {
			pMap := p.(map[string]interface{})
			if pMap["post_type"] != "portfolio" {
				t.Errorf("Item %d: expected post_type='portfolio', got '%v'", i, pMap["post_type"])
			}
		}
		if len(data) != 1 {
			t.Errorf("Expected exactly 1 portfolio post, got %d", len(data))
		}
		meta := result["meta"].(map[string]interface{})
		if meta["total"] != float64(1) {
			t.Errorf("Expected total=1, got %v", meta["total"])
		}
	})

	t.Run("Creating post with invalid/unregistered custom post_type returns validation error", func(t *testing.T) {
		reqBody := post.CreatePostReq{
			Title:    "Ghost Post",
			Content:  "This CPT is not registered.",
			Status:   "draft",
			PostType: "nonexistent_cpt",
		}
		jsonBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode == http.StatusOK {
			t.Error("Expected validation error when creating post with unregistered CPT, got 200 OK")
		}
	})

	t.Run("List handler passes authorID=nil (admin/editor sees all posts including other authors')", func(t *testing.T) {
		// Create a post owned by a different user to verify admin sees all
		otherUserID := uuid.New()
		db.Create(&workspace.WorkspaceMember{
			ID:          uuid.New(),
			WorkspaceID: wsID,
			UserID:      otherUserID,
			Role:        "author",
		})
		altPost, err := svc.CreatePost(context.Background(), post.CreatePostReq{
			Title:   "Other Author Post",
			Content: "Content by another author",
			Status:  "draft",
		}, otherUserID, wsID)
		if err != nil {
			t.Fatalf("Failed to create post by other author: %v", err)
		}

		// List all posts without author filter — handler passes authorID=nil,
		// so posts from all authors should be returned
		req := httptest.NewRequest("GET", "/api/posts", nil)
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

		// Verify we see the post created by the other author
		found := false
		for _, p := range data {
			pMap := p.(map[string]interface{})
			if pMap["id"] == altPost.ID.String() {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected admin/editor to see other author's post when authorID=nil, but it was not in the list")
		}

		// Clean up the alternate member we created
		db.Unscoped().Where("user_id = ?", otherUserID).Delete(&workspace.WorkspaceMember{})
	})

	t.Run("GET /api/posts?type=nonexistent returns empty list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?type=nonexistent_type", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 for nonexistent type filter, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		data := result["data"].([]interface{})
		if len(data) != 0 {
			t.Errorf("Expected empty list for nonexistent post type, got %d items", len(data))
		}
		meta := result["meta"].(map[string]interface{})
		if meta["total"] != float64(0) {
			t.Errorf("Expected total=0 for nonexistent type, got %v", meta["total"])
		}
	})
}

func TestPostPermissions(t *testing.T) {
	db, svc, handler := setupTestPostDB(t)

	app := fiber.New()

	// Dynamic auth context
	var currentUserID uuid.UUID
	wsID := uuid.New()

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", currentUserID.String())
		c.Locals("workspace_id", wsID.String())
		return c.Next()
	})

	app.Post("/api/posts", handler.Create)
	app.Put("/api/posts/:id", handler.Update)
	app.Delete("/api/posts/:id", handler.Delete)

	// Create test users in db
	subscriberID := uuid.New()
	author1ID := uuid.New()
	author2ID := uuid.New()
	editorID := uuid.New()

	// Insert workspace members
	members := []workspace.WorkspaceMember{
		{ID: uuid.New(), WorkspaceID: wsID, UserID: subscriberID, Role: "subscriber"},
		{ID: uuid.New(), WorkspaceID: wsID, UserID: author1ID, Role: "author"},
		{ID: uuid.New(), WorkspaceID: wsID, UserID: author2ID, Role: "author"},
		{ID: uuid.New(), WorkspaceID: wsID, UserID: editorID, Role: "editor"},
	}
	for _, m := range members {
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("Failed to create member: %v", err)
		}
	}

	// Create a post owned by author1 directly via service so we have a target post
	author1Post, err := svc.CreatePost(context.Background(), post.CreatePostReq{
		Title:   "Author 1 Post",
		Content: "Author 1 content",
		Status:  "draft",
	}, author1ID, wsID)
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}

	t.Run("Subscriber cannot create post", func(t *testing.T) {
		currentUserID = subscriberID
		reqBody := post.CreatePostReq{Title: "Sub Post", Content: "Sub content"}
		jsonBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}
	})

	t.Run("Subscriber cannot update post", func(t *testing.T) {
		currentUserID = subscriberID
		newTitle := "Updated Title"
		updateReq := post.UpdatePostReq{Title: &newTitle}
		jsonBytes, _ := json.Marshal(updateReq)
		req := httptest.NewRequest("PUT", "/api/posts/"+author1Post.ID.String(), bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}
	})

	t.Run("Author can update own post", func(t *testing.T) {
		currentUserID = author1ID
		newTitle := "Updated Title By Author 1"
		updateReq := post.UpdatePostReq{Title: &newTitle}
		jsonBytes, _ := json.Marshal(updateReq)
		req := httptest.NewRequest("PUT", "/api/posts/"+author1Post.ID.String(), bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}
	})

	t.Run("Author cannot update other author's post", func(t *testing.T) {
		currentUserID = author2ID
		newTitle := "Hack Title"
		updateReq := post.UpdatePostReq{Title: &newTitle}
		jsonBytes, _ := json.Marshal(updateReq)
		req := httptest.NewRequest("PUT", "/api/posts/"+author1Post.ID.String(), bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}
	})

	t.Run("Editor can update any post", func(t *testing.T) {
		currentUserID = editorID
		newTitle := "Updated Title By Editor"
		updateReq := post.UpdatePostReq{Title: &newTitle}
		jsonBytes, _ := json.Marshal(updateReq)
		req := httptest.NewRequest("PUT", "/api/posts/"+author1Post.ID.String(), bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}
	})

	t.Run("Author cannot delete other author's post", func(t *testing.T) {
		currentUserID = author2ID
		req := httptest.NewRequest("DELETE", "/api/posts/"+author1Post.ID.String(), nil)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}
	})

	t.Run("Author can delete own post", func(t *testing.T) {
		// First recreate post to delete
		author1PostToDel, _ := svc.CreatePost(context.Background(), post.CreatePostReq{
			Title:   "Author 1 Post To Delete",
			Content: "content",
			Status:  "draft",
		}, author1ID, wsID)

		currentUserID = author1ID
		req := httptest.NewRequest("DELETE", "/api/posts/"+author1PostToDel.ID.String(), nil)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}
	})
}

func TestPostTypeFiltering(t *testing.T) {
	db, _, handler := setupTestPostDB(t)

	app := fiber.New()
	userID := uuid.New()
	wsID := uuid.New()

	// Insert test member (superadmin so all permissions pass)
	err := db.Create(&workspace.WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        "superadmin",
	}).Error
	if err != nil {
		t.Fatalf("Failed to create test member: %v", err)
	}

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID.String())
		c.Locals("workspace_id", wsID.String())
		return c.Next()
	})

	app.Post("/api/posts", handler.Create)
	app.Get("/api/posts", handler.List)
	app.Post("/api/post-types", handler.RegisterPostType)

	// --- Register custom post type "project" ---
	cptReq := post.CreatePostTypeReq{
		Name:        "Project",
		Slug:        "project",
		Description: "Custom project CPT",
		Fields: []post.CustomFieldSchema{
			{Name: "deadline", Label: "Deadline", Type: "text", Required: false},
		},
	}
	jsonBytes, _ := json.Marshal(cptReq)
	req := httptest.NewRequest("POST", "/api/post-types", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to register custom post type: %d", resp.StatusCode)
	}

	// --- Create posts of different types ---
	// 1. A regular "post" type (default)
	postReq := post.CreatePostReq{
		Title:   "Regular Blog Post",
		Content: "A regular post content",
		Status:  "draft",
	}
	jsonBytes, _ = json.Marshal(postReq)
	req = httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to create regular post: %d", resp.StatusCode)
	}

	// 2. A "page" type
	pageReq := post.CreatePostReq{
		Title:    "About Us Page",
		Content:  "About page content",
		Status:   "draft",
		PostType: "page",
	}
	jsonBytes, _ = json.Marshal(pageReq)
	req = httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to create page: %d", resp.StatusCode)
	}

	// 3. A "project" CPT (registered above)
	projectReq := post.CreatePostReq{
		Title:    "Sample Project",
		Content:  "Project description",
		Status:   "draft",
		PostType: "project",
	}
	jsonBytes, _ = json.Marshal(projectReq)
	req = httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to create project: %d", resp.StatusCode)
	}

	t.Run("1. GET /api/posts?type=post returns only posts with post_type=post", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?type=post", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		data := result["data"].([]interface{})

		for i, item := range data {
			p := item.(map[string]interface{})
			if p["post_type"] != "post" {
				t.Errorf("Item %d: expected post_type 'post', got '%v'", i, p["post_type"])
			}
		}

		if len(data) != 1 {
			t.Errorf("Expected exactly 1 post with type 'post', got %d", len(data))
		}
	})

	t.Run("2. GET /api/posts?type=page returns only posts with post_type=page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?type=page", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		data := result["data"].([]interface{})

		for i, item := range data {
			p := item.(map[string]interface{})
			if p["post_type"] != "page" {
				t.Errorf("Item %d: expected post_type 'page', got '%v'", i, p["post_type"])
			}
		}

		if len(data) != 1 {
			t.Errorf("Expected exactly 1 post with type 'page', got %d", len(data))
		}
	})

	t.Run("3. GET /api/posts?type=project returns only posts with that custom CPT", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts?type=project", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		data := result["data"].([]interface{})

		for i, item := range data {
			p := item.(map[string]interface{})
			if p["post_type"] != "project" {
				t.Errorf("Item %d: expected post_type 'project', got '%v'", i, p["post_type"])
			}
		}

		if len(data) != 1 {
			t.Errorf("Expected exactly 1 post with type 'project', got %d", len(data))
		}
	})

	t.Run("4. Creating a post with invalid (unregistered) custom post_type returns error", func(t *testing.T) {
		badReq := post.CreatePostReq{
			Title:    "Invalid CPT Post",
			Content:  "This should fail",
			Status:   "draft",
			PostType: "unknown_cpt",
		}
		jsonBytes, _ := json.Marshal(badReq)
		req := httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected error when creating post with unregistered custom post type, got 200")
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		msg, hasMessage := result["message"]
		if hasMessage {
			msgStr := msg.(string)
			if msgStr != "custom post type 'unknown_cpt' is not registered in this workspace" &&
				!strings.Contains(msgStr, "not registered") {
				t.Logf("Got error message: %s", msgStr)
			}
		}
	})

	t.Run("5. List handler returns all posts (authorID=nil for admin/editor)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/posts", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		data := result["data"].([]interface{})

		// We created 3 posts (regular, page, project) — all should be visible
		if len(data) != 3 {
			t.Errorf("Expected 3 posts (all types), got %d", len(data))
		}

		// Verify all 3 post types are present
		typeSet := make(map[string]bool)
		for _, item := range data {
			p := item.(map[string]interface{})
			typeSet[p["post_type"].(string)] = true
		}
		if !typeSet["post"] {
			t.Error("Expected 'post' type in unfiltered list")
		}
		if !typeSet["page"] {
			t.Error("Expected 'page' type in unfiltered list")
		}
		if !typeSet["project"] {
			t.Error("Expected 'project' type in unfiltered list")
		}
	})
}
