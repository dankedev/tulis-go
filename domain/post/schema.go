package post

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SchemaType represents the JSON-LD schema type for a post.
type SchemaType string

const (
	SchemaArticle       SchemaType = "Article"
	SchemaBlogPosting   SchemaType = "BlogPosting"
	SchemaFAQ           SchemaType = "FAQPage"
	SchemaBreadcrumb    SchemaType = "BreadcrumbList"
)

// GenerateJSONLD creates a JSON-LD structured data string for a post.
func GenerateJSONLD(post *Post, siteName, siteURL string) string {
	var builder strings.Builder

	// 1. BreadcrumbList
	breadcrumb := map[string]any{
		"@context":        "https://schema.org",
		"@type":           "BreadcrumbList",
		"itemListElement": []map[string]any{
			{
				"@type":    "ListItem",
				"position": 1,
				"name":     "Home",
				"item":     siteURL,
			},
			{
				"@type":    "ListItem",
				"position": 2,
				"name":     post.Title,
				"item":     fmt.Sprintf("%s/%s", siteURL, post.Slug),
			},
		},
	}
	if b, err := json.Marshal(breadcrumb); err == nil {
		builder.WriteString(`<script type="application/ld+json">`)
		builder.Write(b)
		builder.WriteString("</script>\n")
	}

	// 2. Article / BlogPosting
	articleType := string(SchemaBlogPosting)
	if post.PostType == "page" {
		articleType = string(SchemaArticle)
	}

	datePublished := post.CreatedAt.Format(time.RFC3339)
	dateModified := post.UpdatedAt.Format(time.RFC3339)
	if post.PublishedAt != nil {
		datePublished = post.PublishedAt.Format(time.RFC3339)
	}

	article := map[string]any{
		"@context":        "https://schema.org",
		"@type":           articleType,
		"headline":        post.Title,
		"description":     truncateStr(post.Excerpt, 160),
		"datePublished":   datePublished,
		"dateModified":    dateModified,
		"url":             fmt.Sprintf("%s/%s", siteURL, post.Slug),
		"wordCount":       len(strings.Fields(post.Content)),
	}

	if post.FeatureImage != "" {
		article["image"] = post.FeatureImage
	}

	// Author as Organization (no individual face needed)
	article["author"] = map[string]any{
		"@type": "Organization",
		"name":  siteName,
	}

	if b, err := json.Marshal(article); err == nil {
		builder.WriteString(`<script type="application/ld+json">`)
		builder.Write(b)
		builder.WriteString("</script>\n")
	}

	return builder.String()
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
