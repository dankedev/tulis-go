package mcp

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNegotiateProtocol(t *testing.T) {
	for in, want := range map[string]string{
		`{"protocolVersion":"2024-11-05"}`: "2024-11-05",
		`{"protocolVersion":"2025-06-18"}`: "2025-06-18",
		`{"protocolVersion":"1999-01-01"}`: "2025-06-18",
		``:                                 "2025-06-18",
	} {
		if got := negotiateProtocol(json.RawMessage(in)); got != want {
			t.Errorf("negotiateProtocol(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestValidateArgs(t *testing.T) {
	cases := []struct {
		tool   string
		params map[string]any
		ok     bool
	}{
		{"tulis_list_posts", map[string]any{"limit": float64(10)}, true},
		{"tulis_list_posts", map[string]any{"limit": "10"}, false},
		{"tulis_list_posts", map[string]any{"page": 1.5}, false},
		{"tulis_create_post", map[string]any{}, false},
		{"tulis_create_post", map[string]any{"title": "t", "taxonomy_ids": "abc"}, false},
		{"tulis_create_post", map[string]any{"title": "t", "taxonomy_ids": []any{float64(1)}}, false},
		{"tulis_create_post", map[string]any{"title": "t", "taxonomy_ids": []any{"a"}}, true},
		{"tulis_update_post", map[string]any{"id": "x", "taxonomy_ids": []any{}}, true},
		{"tulis_get_post", map[string]any{"id_or_slug": float64(5)}, false},
	}
	for _, c := range cases {
		if err := validateArgs(c.tool, c.params); (err == nil) != c.ok {
			t.Errorf("validateArgs(%s, %v) err=%v, want ok=%v", c.tool, c.params, err, c.ok)
		}
	}
}

func TestFormatResultErrorHidesInternalDetails(t *testing.T) {
	res := formatResultError(errors.New(`Error 1054: Unknown column 'x' in 'field list' (SQL: select * from posts)`))
	if got := res.Content[0].Text; got != "Error: internal error while processing the request" {
		t.Fatalf("internal error leaked: %q", got)
	}
	res = formatResultError(errors.New("title is required"))
	if got := res.Content[0].Text; got != "Error: title is required" {
		t.Fatalf("validation error must pass through, got %q", got)
	}
}

func TestToolsHaveSchemaItemsForArrays(t *testing.T) {
	for _, tool := range toolDefs {
		for name, p := range tool.InputSchema.Properties {
			if p.Type == "array" && p.Items == nil {
				t.Errorf("%s.%s is an array without items", tool.Name, name)
			}
		}
		if _, ok := toolScopes[tool.Name]; !ok {
			t.Errorf("tool %s has no scope mapping", tool.Name)
		}
	}
}
