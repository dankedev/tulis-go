package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Generate creates a new API key, returning the raw key exactly once.
func (s *Service) Generate(ctx context.Context, workspaceID uuid.UUID, req CreateApiKeyReq) (*ApiKeyResponse, error) {
	rawKey, prefix, err := generateRawKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	var expiresAt *time.Time
	if req.ExpiresInDays > 0 {
		t := time.Now().AddDate(0, 0, req.ExpiresInDays)
		expiresAt = &t
	}

	scopes := req.Scopes
	if scopes == "" {
		scopes = "content:read"
	}

	k := &ApiKey{
		WorkspaceID: workspaceID,
		Name:        req.Name,
		KeyPrefix:   prefix,
		KeyHash:     sha256Hex(rawKey),
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
		IsActive:    true,
	}

	if err := s.repo.Create(ctx, k); err != nil {
		return nil, err
	}

	resp := &ApiKeyResponse{
		ID:        k.ID,
		Name:      k.Name,
		KeyPrefix: k.KeyPrefix,
		RawKey:    rawKey,
		Scopes:    k.Scopes,
		CreatedAt: k.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		IsActive:  k.IsActive,
	}
	if k.ExpiresAt != nil {
		s := k.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.ExpiresAt = &s
	}
	return resp, nil
}

// Validate checks an incoming raw key and returns the ApiKey entity if valid.
func (s *Service) Validate(ctx context.Context, rawKey string) (*ApiKey, error) {
	k, err := s.repo.GetByHash(ctx, sha256Hex(rawKey))
	if err != nil {
		return nil, fmt.Errorf("invalid or expired API key")
	}
	// Update last used
	now := time.Now()
	k.LastUsedAt = &now
	_ = s.repo.Update(ctx, k)
	return k, nil
}

func (s *Service) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]ApiKey, error) {
	return s.repo.ListByWorkspace(ctx, workspaceID)
}

func (s *Service) Revoke(ctx context.Context, id uuid.UUID) error {
	k, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("api key not found")
	}
	k.IsActive = false
	return s.repo.Update(ctx, k)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// generateRawKey creates a random key like "tulis_sk_a1b2c3d4e5f6..."
func generateRawKey() (rawKey, prefix string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	rawKey = "tulis_sk_" + hex.EncodeToString(b)
	prefix = rawKey[:12] // "tulis_sk_a1b2" — fits varchar(12) column
	return rawKey, prefix, nil
}

// sha256Hex hashes the raw key for deterministic lookup.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
