package comment

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&Comment{}))
	return db
}

func TestCommentRepository_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	postID := uuid.New()
	wsID := uuid.New()

	c := &Comment{
		PostID:      postID,
		WorkspaceID: wsID,
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		Content:     "Great post!",
		Status:      "approved",
	}

	err := repo.Create(ctx, c)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, c.ID)

	got, err := repo.GetByID(ctx, c.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Test User", got.AuthorName)
}

func TestCommentRepository_ListByPost(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	postID := uuid.New()
	wsID := uuid.New()

	// Create comments
	for i := 0; i < 3; i++ {
		c := &Comment{
			PostID:      postID,
			WorkspaceID: wsID,
			AuthorName:  "User",
			AuthorEmail: "u@e.com",
			Content:     "Content",
			Status:      "approved",
		}
		assert.NoError(t, repo.Create(ctx, c))
	}

	comments, total, err := repo.ListByPost(ctx, postID, "approved", 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, comments, 3)
}

func TestCommentRepository_NestedComments(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	postID := uuid.New()
	wsID := uuid.New()

	parent := &Comment{
		PostID:      postID,
		WorkspaceID: wsID,
		AuthorName:  "Parent",
		AuthorEmail: "p@e.com",
		Content:     "Parent comment",
		Status:      "approved",
	}
	assert.NoError(t, repo.Create(ctx, parent))

	childID := parent.ID
	child := &Comment{
		PostID:      postID,
		WorkspaceID: wsID,
		AuthorName:  "Child",
		AuthorEmail: "c@e.com",
		Content:     "Reply",
		Status:      "approved",
		ParentID:    &childID,
	}
	assert.NoError(t, repo.Create(ctx, child))

	got, err := repo.GetByID(ctx, parent.ID)
	assert.NoError(t, err)
	assert.Len(t, got.Children, 1)
	assert.Equal(t, "Reply", got.Children[0].Content)
}

func TestCommentService_StatusUpdate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	c := &Comment{
		PostID:      uuid.New(),
		WorkspaceID: uuid.New(),
		AuthorName:  "User",
		AuthorEmail: "u@e.com",
		Content:     "Test",
		Status:      "pending",
	}
	assert.NoError(t, repo.Create(ctx, c))

	err := svc.UpdateStatus(ctx, c.ID, "approved")
	assert.NoError(t, err)

	got, _ := repo.GetByID(ctx, c.ID)
	assert.Equal(t, "approved", got.Status)

	err = svc.UpdateStatus(ctx, c.ID, "invalid")
	assert.Error(t, err)
}
