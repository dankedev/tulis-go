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
	TaxonomyIDs  []string               `json:"taxonomy_ids"` // IDs of categories/tags to assign
	FeatureImage string                 `json:"feature_image"`
	SeoTitle     string                 `json:"seo_title"`
	SeoDesc      string                 `json:"seo_desc"`
	FocusKeyword string                 `json:"focus_keyword"`
	OgpTitle     string                 `json:"ogp_title"`
	OgpDesc      string                 `json:"ogp_desc"`
	OgpImage     string                 `json:"ogp_image"`
	Language     string                 `json:"language"` // ISO 639-1: id, en, ar
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
	TaxonomyIDs  *[]string              `json:"taxonomy_ids"` // optional taxonomy assignment updates
	FeatureImage *string                `json:"feature_image"`
	AuthorID     *string                `json:"author_id"`     // optional author ID update
	SeoTitle     *string                `json:"seo_title"`
	SeoDesc      *string                `json:"seo_desc"`
	FocusKeyword *string                `json:"focus_keyword"`
	OgpTitle     *string                `json:"ogp_title"`
	OgpDesc      *string                `json:"ogp_desc"`
	OgpImage     *string                `json:"ogp_image"`
}

type CreatePostTypeReq struct {
	Name        string              `json:"name"`
	Slug        string              `json:"slug"`
	Description string              `json:"description"`
	Icon        string              `json:"icon"`
	MenuOrder   int                 `json:"menu_order"`
	IsActive    *bool               `json:"is_active"`
	Fields      []CustomFieldSchema `json:"fields"`
}

type CreateTaxonomyReq struct {
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	Type     string  `json:"type"` // 'category' or 'tag'
	ParentID *string `json:"parent_id"` // optional parent ID for hierarchical categories
}

type UpdateTaxonomyReq struct {
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	ParentID *string `json:"parent_id"`
}
