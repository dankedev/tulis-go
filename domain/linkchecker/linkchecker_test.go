package linkchecker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testPostRepo is an in-memory stub of post.PostRepository for the service tests.
type testPostRepo struct {
	posts []post.Post
}

func (t *testPostRepo) FindByID(ctx context.Context, id uuid.UUID) (*post.Post, error) { return nil, nil }
func (t *testPostRepo) FindBySlug(ctx context.Context, wsID uuid.UUID, slug string) (*post.Post, error) {
	return nil, nil
}
func (t *testPostRepo) Update(ctx context.Context, p *post.Post) error { return nil }
func (t *testPostRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }
func (t *testPostRepo) Create(ctx context.Context, p *post.Post) error { return nil }
func (t *testPostRepo) CreatePostType(ctx context.Context, cpt *post.PostType) error { return nil }
func (t *testPostRepo) FindPostTypeByID(ctx context.Context, id uuid.UUID) (*post.PostType, error) { return nil, nil }
func (t *testPostRepo) FindPostTypeBySlug(ctx context.Context, wsID uuid.UUID, slug string) (*post.PostType, error) {
	return nil, nil
}
func (t *testPostRepo) ListPostTypes(ctx context.Context, wsID uuid.UUID) ([]post.PostType, error) { return nil, nil }
func (t *testPostRepo) DeletePostType(ctx context.Context, id uuid.UUID) error { return nil }
func (t *testPostRepo) CreateRevision(ctx context.Context, r *post.PostRevision) error { return nil }
func (t *testPostRepo) ListRevisions(ctx context.Context, postID uuid.UUID) ([]post.PostRevision, error) { return nil, nil }
func (t *testPostRepo) FindRevisionByID(ctx context.Context, id uuid.UUID) (*post.PostRevision, error) { return nil, nil }
func (t *testPostRepo) CreateTaxonomy(ctx context.Context, tx *post.Taxonomy) error { return nil }
func (t *testPostRepo) FindTaxonomyByID(ctx context.Context, id uuid.UUID) (*post.Taxonomy, error) { return nil, nil }
func (t *testPostRepo) FindTaxonomyBySlug(ctx context.Context, wsID uuid.UUID, slug, taxType string) (*post.Taxonomy, error) {
	return nil, nil
}
func (t *testPostRepo) UpdateTaxonomy(ctx context.Context, tx *post.Taxonomy) error { return nil }
func (t *testPostRepo) DeleteTaxonomy(ctx context.Context, id uuid.UUID) error { return nil }
func (t *testPostRepo) ListTaxonomies(ctx context.Context, wsID uuid.UUID, taxType string) ([]post.Taxonomy, error) { return nil, nil }
func (t *testPostRepo) AssignTaxonomies(ctx context.Context, postID uuid.UUID, ids []uuid.UUID) error { return nil }
func (t *testPostRepo) GetPostTaxonomies(ctx context.Context, postID uuid.UUID) ([]post.Taxonomy, error) { return nil, nil }
func (t *testPostRepo) ListPublic(ctx context.Context, wsID uuid.UUID, postType, taxonomySlug, sortBy string, limit, offset int) ([]post.Post, int64, error) {
	return nil, 0, nil
}
func (t *testPostRepo) List(ctx context.Context, wsID uuid.UUID, postType, status, search string, limit, offset int) ([]post.Post, int64, error) {
	if status == "published" {
		return t.posts, int64(len(t.posts)), nil
	}
	return nil, 0, nil
}

func setupLinkTestDB(t *testing.T) (Repository, Service, *testPostRepo) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&BrokenLink{}, &workspace.Workspace{}, &workspace.WorkspaceMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewRepository(db)
	pr := &testPostRepo{}
	svc := NewService(repo, pr, db, 0)
	return repo, svc, pr
}

func TestExtractLinks(t *testing.T) {
	html := `<p>Visit <a href="https://example.com/ok">us</a> and <a href="/relative">skip</a>.</p>` +
		`<img src="https://cdn.example.com/pic.png"/>` +
		`<a href="https://example.com/ok">dup</a>` // duplicate, should dedupe
	links := extractLinks(html)
	if len(links) != 2 {
		t.Fatalf("expected 2 unique external links, got %d: %v", len(links), links)
	}
}

func TestCheckWorkspaceDetectsBrokenAndHealthy(t *testing.T) {
	repo, svc, pr := setupLinkTestDB(t)

	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthy.Close()

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer dead.Close()

	wsID := uuid.New()
	pr.posts = []post.Post{
		{
			ID:      uuid.New(),
			Title:   "Test Post",
			Status:  "published",
			Content: `<a href="` + healthy.URL + `">ok</a><a href="` + dead.URL + `">broken</a>`,
		},
	}

	checked, broken, err := svc.CheckWorkspace(context.Background(), wsID)
	if err != nil {
		t.Fatalf("CheckWorkspace error: %v", err)
	}
	if checked != 1 {
		t.Errorf("expected 1 post checked, got %d", checked)
	}
	if broken != 1 {
		t.Errorf("expected 1 broken link, got %d", broken)
	}

	links, _ := repo.ListByWorkspace(context.Background(), wsID, true)
	if len(links) != 1 {
		t.Fatalf("expected 1 stored broken link, got %d", len(links))
	}
	if links[0].URL != dead.URL {
		t.Errorf("expected stored url to be dead server, got %s", links[0].URL)
	}
	if links[0].StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", links[0].StatusCode)
	}
	_ = time.Now
}
