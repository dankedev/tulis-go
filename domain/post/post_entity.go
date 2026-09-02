package post

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Post struct {
	ID           uuid.UUID              `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	DeletedAt    gorm.DeletedAt         `gorm:"index" json:"-"`
	Title        string                 `gorm:"type:varchar(255);not null" json:"title"`
	Slug         string                 `gorm:"type:varchar(255);index:idx_slug_ws;not null" json:"slug"`
	Content      string                 `gorm:"type:text" json:"content"`
	Excerpt      string                 `gorm:"type:text" json:"excerpt"`
	Status       string                 `gorm:"type:varchar(50);default:'draft';not null" json:"status"` // draft, published, scheduled, archived
	AuthorID     uuid.UUID              `gorm:"type:char(36);not null;index" json:"author_id"`
	WorkspaceID  uuid.UUID              `gorm:"type:char(36);not null;index:idx_slug_ws" json:"workspace_id"`
	PostType     string                 `gorm:"type:varchar(50);default:'post';not null;index" json:"post_type"` // 'post', 'page', CPTs
	PublishedAt  *time.Time             `json:"published_at"`
	CustomFields map[string]interface{} `gorm:"serializer:json" json:"custom_fields"`
	Taxonomies   []Taxonomy             `gorm:"many2many:post_taxonomies;" json:"taxonomies"`
	FeatureImage string                 `gorm:"type:varchar(255)" json:"feature_image"`
	EditedAt     *time.Time             `json:"edited_at,omitempty"`
	SeoTitle     string                 `gorm:"type:varchar(255)" json:"seo_title,omitempty"`
	SeoDesc      string                 `gorm:"type:text" json:"seo_desc,omitempty"`
	FocusKeyword string                 `gorm:"type:varchar(100)" json:"focus_keyword,omitempty"`
	OgpTitle     string                 `gorm:"type:varchar(255)" json:"ogp_title,omitempty"`
	OgpDesc      string                 `gorm:"type:text" json:"ogp_desc,omitempty"`
	OgpImage     string                 `gorm:"type:text" json:"ogp_image,omitempty"`
	SeoScore     int                    `gorm:"type:integer;default:0" json:"seo_score"`
	ReadingTime  int                    `gorm:"type:integer;default:0" json:"reading_time"` // estimated minutes
	Language     string                 `gorm:"type:varchar(10);default:'id';index" json:"language"`
	Visibility   string                 `gorm:"type:varchar(20);default:'public'" json:"visibility"` // public, members, tier_slug
	Order        int                    `gorm:"type:integer;default:0" json:"order"`
}

func (p *Post) GetTitle() string { return p.Title }
func (p *Post) GetContent() string { return p.Content }
func (p *Post) GetSlug() string { return p.Slug }
func (p *Post) GetFocusKeyword() string { return p.FocusKeyword }
func (p *Post) GetSeoDesc() string { return p.SeoDesc }
func (p *Post) GetOgpImage() string { return p.OgpImage }
func (p *Post) SetSeoScore(score int) { p.SeoScore = score }
