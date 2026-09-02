package helpers_test

import (
	"testing"

	"github.com/dankedev/tulis-go/utils/helpers"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Hello, World!", "hello-world"},
		{"test,.,@slug", "test-slug"},
		{"user@example.com", "user-example-com"},
		{"Post #1: What is Go?", "post-1-what-is-go"},
		{"   ---leading and trailing---   ", "leading-and-trailing"},
		{"multi---hyphens...and,,,dots", "multi-hyphens-and-dots"},
		{"slug_with_underscores", "slug-with-underscores"},
		{"123 456 789", "123-456-789"},
		{"special @!#$%^&*() characters", "special-characters"},
		{",.,@", ""},
		{"---", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := helpers.Slugify(tt.input)
		if got != tt.expected {
			t.Errorf("Slugify(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}
