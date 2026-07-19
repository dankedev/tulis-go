package apikey

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
	assert.NoError(t, db.AutoMigrate(&ApiKey{}))
	return db
}

func TestApiKeyService_GenerateAndValidate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	wsID := uuid.New()
	resp, err := svc.Generate(ctx, wsID, CreateApiKeyReq{
		Name:   "Test Key",
		Scopes: "content:read content:write",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.RawKey)
	assert.Contains(t, resp.RawKey, "tulis_sk_")
	assert.Equal(t, "content:read content:write", resp.Scopes)

	// Validate the key
	k, err := svc.Validate(ctx, resp.RawKey)
	assert.NoError(t, err)
	assert.Equal(t, "Test Key", k.Name)
	assert.True(t, k.IsActive)

	// Validate with wrong key
	_, err = svc.Validate(ctx, "tulis_sk_wrong")
	assert.Error(t, err)
}

func TestApiKeyService_Revoke(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	resp, _ := svc.Generate(ctx, uuid.New(), CreateApiKeyReq{Name: "Revoke Me", Scopes: "content:read"})

	err := svc.Revoke(ctx, resp.ID)
	assert.NoError(t, err)

	_, err = svc.Validate(ctx, resp.RawKey)
	assert.Error(t, err, "revoked key should fail validation")
}

func TestApiKeyService_ListByWorkspace(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	wsID := uuid.New()
	otherWsID := uuid.New()

	svc.Generate(ctx, wsID, CreateApiKeyReq{Name: "K1", Scopes: "content:read"})
	svc.Generate(ctx, wsID, CreateApiKeyReq{Name: "K2", Scopes: "content:write"})
	svc.Generate(ctx, otherWsID, CreateApiKeyReq{Name: "K3", Scopes: "admin"})

	keys, err := svc.ListByWorkspace(ctx, wsID)
	assert.NoError(t, err)
	assert.Len(t, keys, 2)
}

func TestApiKeyService_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	resp, _ := svc.Generate(ctx, uuid.New(), CreateApiKeyReq{Name: "Delete Me", Scopes: "content:read"})

	err := svc.Delete(ctx, resp.ID)
	assert.NoError(t, err)

	_, err = svc.Validate(ctx, resp.RawKey)
	assert.Error(t, err)
}
