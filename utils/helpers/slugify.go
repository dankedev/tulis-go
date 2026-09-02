package helpers

import (
	"regexp"
	"strings"
)

var (
	nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9\-]+`)
	multipleHyphensRegex = regexp.MustCompile(`-+`)
)

// Slugify sanitizes and formats a string into a strict URL-friendly slug.
// It converts text to lowercase, replaces spaces, underscores, dots, commas, @,
// and all other non-alphanumeric characters with hyphens, collapses multiple hyphens,
// and trims leading and trailing hyphens. Only letters (a-z), digits (0-9), and hyphens (-) remain.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlphanumericRegex.ReplaceAllString(s, "-")
	s = multipleHyphensRegex.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
