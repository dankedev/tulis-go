package importer

import (
	"encoding/xml"
	"time"
)

type WXR struct {
	Channel WXRChannel `xml:"channel"`
}

type WXRChannel struct {
	Title       string      `xml:"title"`
	Link        string      `xml:"link"`
	Description string      `xml:"description"`
	PubDate     string      `xml:"pubDate"`
	Language    string      `xml:"language"`
	WxrVersion  string      `xml:"wxr_version"`
	BaseSiteURL string      `xml:"base_site_url"`
	BaseBlogURL string      `xml:"base_blog_url"`
	Generator   string      `xml:"generator"`
	Authors     []WXRAuthor `xml:"author"`
	Categories  []WXRCategory `xml:"category"`
	Tags        []WXRTag     `xml:"tag"`
	Terms       []WXRTerm   `xml:"term"`
	Items       []WXRItem   `xml:"item"`
}

type WXRAuthor struct {
	ID          int32  `xml:"author_id"`
	Login       string `xml:"author_login"`
	Email       string `xml:"author_email"`
	DisplayName string `xml:"author_display_name"`
	FirstName   string `xml:"author_first_name"`
	LastName    string `xml:"author_last_name"`
}

type WXRCategory struct {
	ID          int32  `xml:"term_id"`
	NiceName    string `xml:"category_nicename"`
	Parent      string `xml:"category_parent"`
	Name        string `xml:"cat_name"`
	Description string `xml:"category_description"`
}

type WXRTag struct {
	ID          int32  `xml:"term_id"`
	Slug        string `xml:"tag_slug"`
	Name        string `xml:"tag_name"`
	Description string `xml:"tag_description"`
}

type WXRTerm struct {
	TermID       int32  `xml:"term_id"`
	TermTaxonomy string `xml:"term_taxonomy"`
	Slug         string `xml:"term_slug"`
	TermParent   string `xml:"term_parent"`
	TermName     string `xml:"term_name"`
	Description  string `xml:"term_description"`
}

type WXRItem struct {
	XMLName xml.Name `xml:"item"`

	Title        string          `xml:"title"`
	Link         string          `xml:"link"`
	PubDate      string          `xml:"pubDate"`
	GUID         string          `xml:"guid"`
	Desc         string          `xml:"description"`
	Content      string          `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	Excerpt      string          `xml:"http://wordpress.org/export/1.2/excerpt/ encoded"`
	Creator      string          `xml:"http://purl.org/dc/elements/1.1/ creator"`
	PostID       int32           `xml:"http://wordpress.org/export/1.2/ post_id"`
	PostDate     string          `xml:"http://wordpress.org/export/1.2/ post_date"`
	PostDateGmt  string          `xml:"http://wordpress.org/export/1.2/ post_date_gmt"`
	CommentStatus string         `xml:"http://wordpress.org/export/1.2/ comment_status"`
	PingStatus   string          `xml:"http://wordpress.org/export/1.2/ ping_status"`
	PostName     string          `xml:"http://wordpress.org/export/1.2/ post_name"`
	Status       string          `xml:"http://wordpress.org/export/1.2/ status"`
	PostParent   int32           `xml:"http://wordpress.org/export/1.2/ post_parent"`
	MenuOrder    int32           `xml:"http://wordpress.org/export/1.2/ menu_order"`
	PostType     string          `xml:"http://wordpress.org/export/1.2/ post_type"`
	PostPassword string          `xml:"http://wordpress.org/export/1.2/ post_password"`
	IsSticky     bool            `xml:"http://wordpress.org/export/1.2/ is_sticky"`
	AttachmentURL string         `xml:"http://wordpress.org/export/1.2/ attachment_url"`
	Categories   []ItemCategory `xml:"category"`
	PostMetas    []PostMeta     `xml:"http://wordpress.org/export/1.2/ postmeta"`
	Comments     []WXRComment   `xml:"http://wordpress.org/export/1.2/ comment"`
}

type ItemCategory struct {
	Domain      string `xml:"domain,attr"`
	NiceName    string `xml:"nicename,attr"`
	DisplayName string `xml:",chardata"`
}

type PostMeta struct {
	Key   string `xml:"http://wordpress.org/export/1.2/ meta_key"`
	Value string `xml:"http://wordpress.org/export/1.2/ meta_value"`
}

type WXRComment struct {
	ID         int32  `xml:"http://wordpress.org/export/1.2/ comment_id"`
	Author     string `xml:"http://wordpress.org/export/1.2/ comment_author"`
	AuthorEmail string `xml:"http://wordpress.org/export/1.2/ comment_author_email"`
	AuthorURL  string `xml:"http://wordpress.org/export/1.2/ comment_author_url"`
	AuthorIP   string `xml:"http://wordpress.org/export/1.2/ comment_author_ip"`
	Date       string `xml:"http://wordpress.org/export/1.2/ comment_date"`
	DateGmt    string `xml:"http://wordpress.org/export/1.2/ comment_date_gmt"`
	Content    string `xml:"http://wordpress.org/export/1.2/ comment_content"`
	Approved   bool   `xml:"http://wordpress.org/export/1.2/ comment_approved"`
	Type       string `xml:"http://wordpress.org/export/1.2/ comment_type"`
	Parent     int32  `xml:"http://wordpress.org/export/1.2/ comment_parent"`
	UserID     int32  `xml:"http://wordpress.org/export/1.2/ comment_user_id"`
}

func ParseWXR(data []byte) (*WXR, error) {
	var wxr WXR
	if err := xml.Unmarshal(data, &wxr); err != nil {
		return nil, err
	}
	return &wxr, nil
}

func ParsePostDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", s)
}

func ParsePubDate(s string) (time.Time, error) {
	return time.Parse(time.RFC1123Z, s)
}
