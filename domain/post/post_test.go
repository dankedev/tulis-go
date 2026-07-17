package post_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dankedev/tulis-go/domain/post"
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

	repo := post.NewPostRepository(db)
	svc := post.NewPostService(repo, nil)
	handler := post.NewPostHandler(svc, wsSvc)

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
