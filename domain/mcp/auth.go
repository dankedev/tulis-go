package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	ScopeRead  = "content:read"
	ScopeWrite = "content:write"
	ScopeAdmin = "admin"
)

// Principal describes the authenticated API key behind a request. It is carried in the
// context so the Service interface stays unchanged. A context without a Principal is a
// trusted in-process caller (e.g. cmd/mcp direct mode or tests) and is not restricted.
type Principal struct {
	WorkspaceID uuid.UUID
	Scopes      string // space-delimited, e.g. "content:read content:write"
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func principalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// toolScopes maps every tool to the scope it requires. `admin` satisfies any scope and
// `content:write` implies `content:read`.
var toolScopes = map[string]string{
	"tulis_list_posts":             ScopeRead,
	"tulis_get_post":               ScopeRead,
	"tulis_list_taxonomies":        ScopeRead,
	"tulis_get_taxonomy":           ScopeRead,
	"tulis_list_workspaces":        ScopeRead,
	"tulis_get_current_workspace":  ScopeRead,
	"tulis_switch_workspace":       ScopeRead,
	"tulis_create_post":            ScopeWrite,
	"tulis_update_post":            ScopeWrite,
	"tulis_delete_post":            ScopeWrite,
	"tulis_create_taxonomy":        ScopeWrite,
	"tulis_update_taxonomy":        ScopeWrite,
	"tulis_upload_media":           ScopeWrite,
	"tulis_list_workspace_members": ScopeAdmin,
}

func hasScope(granted, required string) bool {
	for _, s := range strings.Fields(granted) {
		switch {
		case s == ScopeAdmin, s == required:
			return true
		case s == ScopeWrite && required == ScopeRead:
			return true
		}
	}
	return false
}

// authorizeTool returns an error when the context carries a Principal lacking the tool's scope.
func authorizeTool(ctx context.Context, tool string) error {
	p, ok := principalFrom(ctx)
	if !ok {
		return nil
	}
	required, known := toolScopes[tool]
	if !known {
		return nil // unknown tools fall through to the "unknown tool" error
	}
	if !hasScope(p.Scopes, required) {
		return fmt.Errorf("forbidden: API key lacks the %q scope required by %s", required, tool)
	}
	return nil
}
