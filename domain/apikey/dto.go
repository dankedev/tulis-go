package apikey

import "github.com/google/uuid"

// CreateApiKeyReq is the request body for generating a new API key.
type CreateApiKeyReq struct {
	Name    string `json:"name"`
	Scopes  string `json:"scopes"` // "content:read content:write admin"
	ExpiresInDays int `json:"expires_in_days,omitempty"` // 0 = never
}

// ApiKeyResponse is the response when creating a key — includes the raw key ONCE.
type ApiKeyResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	KeyPrefix string    `json:"key_prefix"`
	RawKey    string    `json:"raw_key,omitempty"` // only returned on creation
	Scopes    string    `json:"scopes"`
	CreatedAt string    `json:"created_at"`
	ExpiresAt *string   `json:"expires_at,omitempty"`
	IsActive  bool      `json:"is_active"`
}

// ApiKeyListResponse is the response for listing keys (never includes raw key).
type ApiKeyListResponse struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	KeyPrefix  string    `json:"key_prefix"`
	Scopes     string    `json:"scopes"`
	CreatedAt  string    `json:"created_at"`
	LastUsedAt *string   `json:"last_used_at,omitempty"`
	ExpiresAt  *string   `json:"expires_at,omitempty"`
	IsActive   bool      `json:"is_active"`
}

// ToListResponse converts an ApiKey entity to a safe list response.
func (k *ApiKey) ToListResponse() ApiKeyListResponse {
	resp := ApiKeyListResponse{
		ID:        k.ID,
		Name:      k.Name,
		KeyPrefix: k.KeyPrefix,
		Scopes:    k.Scopes,
		CreatedAt: k.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		IsActive:  k.IsActive,
	}
	if k.LastUsedAt != nil {
		s := k.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.LastUsedAt = &s
	}
	if k.ExpiresAt != nil {
		s := k.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.ExpiresAt = &s
	}
	return resp
}
