package post

import "time"

type CreatePostReq struct {
	Title        string                 `json:"title"`
	Slug         string                 `json:"slug"` // optional
	Content      string                 `json:"content"`
	Excerpt      string                 `json:"excerpt"`
	Status       string                 `json:"status"` // draft, published, scheduled, archived
	PostType     string                 `json:"post_type"`
	PublishedAt  *time.Time             `json:"published_at"`
	CustomFields map[string]interface{} `json:"custom_fields"`
}

type UpdatePostReq struct {
	Title        *string                `json:"title"`
	Slug         *string                `json:"slug"`
	Content      *string                `json:"content"`
	Excerpt      *string                `json:"excerpt"`
	Status       *string                `json:"status"`
	PostType     *string                `json:"post_type"`
	PublishedAt  *time.Time             `json:"published_at"`
	CustomFields map[string]interface{} `json:"custom_fields"`
}

type CreatePostTypeReq struct {
	Name        string              `json:"name"`
	Slug        string              `json:"slug"`
	Description string              `json:"description"`
	Fields      []CustomFieldSchema `json:"fields"`
}
