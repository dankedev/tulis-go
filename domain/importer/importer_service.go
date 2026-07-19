package importer

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dankedev/tulis-go/domain/media"
	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/utils/helpers"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ImporterService interface {
	ImportWXR(ctx context.Context, workspaceID, authorID uuid.UUID, file multipart.File, filename string) (*ImportLog, error)
	GetImportLog(ctx context.Context, id uuid.UUID) (*ImportLog, error)
	ListImportLogs(ctx context.Context, workspaceID uuid.UUID, page, perPage int) ([]ImportLog, int64, error)
	ParseCSVHeaders(ctx context.Context, file io.Reader) ([]string, error)
	UploadCSV(ctx context.Context, workspaceID uuid.UUID, filename string, fileData []byte) (string, []string, error)
	ImportCSVBackground(ctx context.Context, workspaceID, authorID, logID uuid.UUID, fileURL string, mapping map[string]string, defaultStatus, defaultPostType string)
	InspectStrapi(ctx context.Context, urlStr, token, collectionType string) ([]string, error)
	ImportStrapiBackground(ctx context.Context, workspaceID, authorID, logID uuid.UUID, urlStr, token, collectionType string, mapping map[string]string, defaultStatus, defaultPostType string)
	ImportMarkdown(ctx context.Context, workspaceID, authorID uuid.UUID, file multipart.File, filename string, opts ImportMarkdownOpts) (*ImportLog, error)
	PreviewMarkdown(ctx context.Context, workspaceID uuid.UUID, file multipart.File) ([]MdFilePreview, error)
}

type MdFilePreview struct {
	Path       string  `json:"path"`
	Title      string  `json:"title"`
	Slug       string  `json:"slug"`
	Category   string  `json:"category"`
	AlreadyExists bool `json:"already_exists"`
	Status     string  `json:"status"` // "new", "duplicate"
}

type importerService struct {
	db        *gorm.DB
	mediaSvc  media.MediaService
	postRepo  postRepoHelper
	mediaRepo mediaRepoHelper
}

type ImportResult struct {
	PostsCount   int      `json:"posts_count"`
	PagesCount   int      `json:"pages_count"`
	MediaCount   int      `json:"media_count"`
	TaxCount     int      `json:"tax_count"`
	SkippedCount int      `json:"skipped_count"`
	Errors       []string `json:"errors"`
}

func NewImporterService(db *gorm.DB, mediaSvc media.MediaService, postRepo postRepoHelper, mediaRepo mediaRepoHelper) ImporterService {
	return &importerService{
		db:        db,
		mediaSvc:  mediaSvc,
		postRepo:  postRepo,
		mediaRepo: mediaRepo,
	}
}

type importSession struct {
	workspaceID uuid.UUID
	authorID    uuid.UUID
	result      ImportResult
	errors      []string
	urlMap      map[string]string
	taxMap      map[string]uuid.UUID
	log         *ImportLog
}

func (s *importerService) ImportWXR(ctx context.Context, workspaceID, authorID uuid.UUID, file multipart.File, filename string) (*ImportLog, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	wxr, err := ParseWXR(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WXR XML: %w", err)
	}

	if wxr.Channel.WxrVersion != "1.2" {
		return nil, fmt.Errorf("unsupported WXR version: %s (supported: 1.2)", wxr.Channel.WxrVersion)
	}

	log := &ImportLog{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		AuthorID:    authorID,
		Filename:    filename,
		Status:      "running",
	}
	if err := s.db.WithContext(ctx).Create(log).Error; err != nil {
		return nil, fmt.Errorf("failed to create import log: %w", err)
	}

	// Process import in the background
	go func() {
		bgCtx := context.Background()
		session := &importSession{
			workspaceID: workspaceID,
			authorID:    authorID,
			result:      ImportResult{},
			errors:      []string{},
			urlMap:      make(map[string]string),
			taxMap:      make(map[string]uuid.UUID),
			log:         log,
		}

		if err := s.importTaxonomies(bgCtx, wxr.Channel, session); err != nil {
			session.errors = append(session.errors, fmt.Sprintf("taxonomy import error: %v", err))
		}

		if err := s.importAttachments(bgCtx, wxr.Channel.Items, session); err != nil {
			session.errors = append(session.errors, fmt.Sprintf("media import error: %v", err))
		}

		if err := s.importPosts(bgCtx, wxr.Channel.Items, session); err != nil {
			session.errors = append(session.errors, fmt.Sprintf("post import error: %v", err))
		}

		session.log.Status = "completed"
		session.log.PostsCount = session.result.PostsCount
		session.log.PagesCount = session.result.PagesCount
		session.log.MediaCount = session.result.MediaCount
		session.log.TaxCount = session.result.TaxCount
		session.log.SkippedCount = session.result.SkippedCount

		errorsJSON, _ := json.Marshal(session.errors)
		session.log.Errors = string(errorsJSON)

		summaryJSON, _ := json.Marshal(session.result)
		session.log.Summary = string(summaryJSON)

		s.db.Save(session.log)
	}()

	return log, nil
}

func (s *importerService) importTaxonomies(ctx context.Context, ch WXRChannel, session *importSession) error {
	for _, cat := range ch.Categories {
		if cat.NiceName == "" {
			continue
		}
		slug := cat.NiceName
		if slug == "" {
			slug = helpers.Slugify(cat.Name)
		}
		existing, _ := s.postRepo.FindTaxonomyBySlug(ctx, session.workspaceID, slug, "category")
		if existing != nil {
			session.taxMap["category:"+slug] = existing.ID
			session.result.SkippedCount++
			continue
		}
		tax := &post.Taxonomy{
			ID:          uuid.New(),
			WorkspaceID: session.workspaceID,
			Name:        cat.Name,
			Slug:        slug,
			Type:        "category",
		}
		if cat.Parent != "" {
			if parentID, ok := session.taxMap["category:"+cat.Parent]; ok {
				tax.ParentID = &parentID
			}
		}
		if err := s.postRepo.CreateTaxonomy(ctx, tax); err != nil {
			session.errors = append(session.errors, fmt.Sprintf("create category %s: %v", cat.Name, err))
			continue
		}
		session.taxMap["category:"+slug] = tax.ID
		session.result.TaxCount++
	}

	for _, tag := range ch.Tags {
		if tag.Slug == "" {
			continue
		}
		existing, _ := s.postRepo.FindTaxonomyBySlug(ctx, session.workspaceID, tag.Slug, "tag")
		if existing != nil {
			session.taxMap["tag:"+tag.Slug] = existing.ID
			session.result.SkippedCount++
			continue
		}
		tax := &post.Taxonomy{
			ID:          uuid.New(),
			WorkspaceID: session.workspaceID,
			Name:        tag.Name,
			Slug:        tag.Slug,
			Type:        "tag",
		}
		if err := s.postRepo.CreateTaxonomy(ctx, tax); err != nil {
			session.errors = append(session.errors, fmt.Sprintf("create tag %s: %v", tag.Name, err))
			continue
		}
		session.taxMap["tag:"+tag.Slug] = tax.ID
		session.result.TaxCount++
	}

	for _, term := range ch.Terms {
		if term.Slug == "" || term.TermTaxonomy == "" {
			continue
		}
		existing, _ := s.postRepo.FindTaxonomyBySlug(ctx, session.workspaceID, term.Slug, term.TermTaxonomy)
		if existing != nil {
			session.taxMap[term.TermTaxonomy+":"+term.Slug] = existing.ID
			session.result.SkippedCount++
			continue
		}
		tax := &post.Taxonomy{
			ID:          uuid.New(),
			WorkspaceID: session.workspaceID,
			Name:        term.TermName,
			Slug:        term.Slug,
			Type:        term.TermTaxonomy,
		}
		if term.TermParent != "" {
			if parentID, ok := session.taxMap[term.TermTaxonomy+":"+term.TermParent]; ok {
				tax.ParentID = &parentID
			}
		}
		if err := s.postRepo.CreateTaxonomy(ctx, tax); err != nil {
			session.errors = append(session.errors, fmt.Sprintf("create term %s: %v", term.TermName, err))
			continue
		}
		session.taxMap[term.TermTaxonomy+":"+term.Slug] = tax.ID
		session.result.TaxCount++
	}

	return nil
}

func (s *importerService) importAttachments(ctx context.Context, items []WXRItem, session *importSession) error {
	type attachTask struct {
		item  WXRItem
		index int
	}

	var tasks []attachTask
	for i, item := range items {
		if item.PostType == "attachment" && item.AttachmentURL != "" {
			tasks = append(tasks, attachTask{item: item, index: i})
		}
	}

	if len(tasks) == 0 {
		return nil
	}

	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, t := range tasks {
		wg.Add(1)
		go func(tk attachTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			data, mimeType, err := s.downloadImage(ctx, tk.item.AttachmentURL)
			if err != nil {
				mu.Lock()
				session.errors = append(session.errors, fmt.Sprintf("download media %s: %v", tk.item.AttachmentURL, err))
				mu.Unlock()
				return
			}

			filename := filepath.Base(tk.item.AttachmentURL)
			if filename == "" || !strings.Contains(filename, ".") {
				ext := s.mimeToExt(mimeType)
				filename = fmt.Sprintf("attachment-%d%s", tk.item.PostID, ext)
			}

			mime := mimeType
			if mime == "" {
				mime = "application/octet-stream"
			}

			mediaRec, err := s.mediaSvc.SaveFile(ctx, session.workspaceID, filename, data, mime, int64(len(data)), "", "")
			if err != nil {
				mu.Lock()
				session.errors = append(session.errors, fmt.Sprintf("save media %s: %v", tk.item.AttachmentURL, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			session.urlMap[tk.item.AttachmentURL] = mediaRec.Path
			session.result.MediaCount++
			mu.Unlock()
		}(t)
	}

	wg.Wait()
	return nil
}

func (s *importerService) downloadImage(ctx context.Context, url string) ([]byte, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	mimeType := resp.Header.Get("Content-Type")
	return data, mimeType, nil
}

func (s *importerService) mimeToExt(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".jpg"
	}
}

func (s *importerService) importPosts(ctx context.Context, items []WXRItem, session *importSession) error {
	for _, item := range items {
		if item.PostType == "attachment" || item.PostType == "nav_menu_item" {
			continue
		}

		if item.Status == "inherit" {
			session.result.SkippedCount++
			continue
		}

		slug := item.PostName
		if slug == "" {
			slug = helpers.Slugify(item.Title)
		}

		originalSlug := slug
		counter := 1
		for {
			existing, _ := s.postRepo.FindBySlug(ctx, session.workspaceID, slug)
			if existing == nil {
				break
			}
			slug = fmt.Sprintf("%s-%d", originalSlug, counter)
			counter++
		}

		status := s.mapStatus(item.Status)
		if status == "" {
			session.result.SkippedCount++
			continue
		}

		var publishedAt *time.Time
		if status == "published" {
			pt, err := ParsePostDate(item.PostDate)
			if err == nil {
				publishedAt = &pt
			}
		}

		customFields := make(map[string]interface{})
		customFields["_wxr_post_id"] = item.PostID
		for _, pm := range item.PostMetas {
			if pm.Key != "" {
				customFields["_meta_"+pm.Key] = pm.Value
			}
		}

		content := s.transformContent(item.Content, session)
		excerpt := s.transformContent(item.Excerpt, session)

		postType := item.PostType
		if postType == "" {
			postType = "post"
		}

		p := &post.Post{
			ID:           uuid.New(),
			Title:        item.Title,
			Slug:         slug,
			Content:      content,
			Excerpt:      excerpt,
			Status:       status,
			AuthorID:     session.authorID,
			WorkspaceID:  session.workspaceID,
			PostType:     postType,
			PublishedAt:  publishedAt,
			CustomFields: customFields,
		}

		if err := s.postRepo.Create(ctx, p); err != nil {
			session.errors = append(session.errors, fmt.Sprintf("create post %s: %v", item.Title, err))
			continue
		}

		var taxIDs []uuid.UUID
		for _, cat := range item.Categories {
			if cat.Domain == "category" && cat.NiceName != "" {
				if taxID, ok := session.taxMap["category:"+cat.NiceName]; ok {
					taxIDs = append(taxIDs, taxID)
				}
			} else if cat.Domain == "post_tag" && cat.NiceName != "" {
				if taxID, ok := session.taxMap["tag:"+cat.NiceName]; ok {
					taxIDs = append(taxIDs, taxID)
				}
			} else if cat.Domain != "" && cat.NiceName != "" {
				if taxID, ok := session.taxMap[cat.Domain+":"+cat.NiceName]; ok {
					taxIDs = append(taxIDs, taxID)
				}
			}
		}

		if len(taxIDs) > 0 {
			s.postRepo.AssignTaxonomies(ctx, p.ID, taxIDs)
		}

		revision := &post.PostRevision{
			ID:           uuid.New(),
			PostID:       p.ID,
			Title:        p.Title,
			Content:      p.Content,
			Excerpt:      p.Excerpt,
			CustomFields: p.CustomFields,
			AuthorID:     session.authorID,
		}
		s.postRepo.CreateRevision(ctx, revision)

		if postType == "post" {
			session.result.PostsCount++
		} else {
			session.result.PagesCount++
		}
	}

	return nil
}

func (s *importerService) mapStatus(wpStatus string) string {
	switch wpStatus {
	case "publish":
		return "published"
	case "draft":
		return "draft"
	case "pending":
		return "draft"
	case "private":
		return "draft"
	case "future":
		return "scheduled"
	default:
		return "draft"
	}
}

func (s *importerService) transformContent(content string, session *importSession) string {
	if content == "" {
		return content
	}

	// Step 1: Convert Gutenberg blocks to HTML
	content = convertGutenbergToHTML(content)

	// Step 2: Replace old media URLs with new paths (for all img src and href attributes)
	content = replaceMediaURLs(content, session.urlMap)

	// Step 3: Convert HTML to Markdown (Pure Markdown storage)
	content = htmlToMarkdown(content)

	return content
}

func convertGutenbergToHTML(content string) string {
	// Replace common Gutenberg blocks with HTML equivalents

	// Paragraph: <!-- wp:paragraph -->...<!-- /wp:paragraph -->
	content = regexp.MustCompile(`(?s)<!--\s*wp:paragraph\s*-->\s*<p[^>]*>(.*?)</p>\s*<!--\s*/wp:paragraph\s*-->`).ReplaceAllStringFunc(content, func(match string) string {
		// Extract content between <p> tags
		re := regexp.MustCompile(`<p[^>]*>(.*?)</p>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) > 1 {
			return "<p>" + matches[1] + "</p>"
		}
		return match
	})

	// Image block: <!-- wp:image -->...<!-- /wp:image --> with figure and img
	content = regexp.MustCompile(`(?s)<!--\s*wp:image\s*-->\s*<figure[^>]*>\s*<img\s+src="([^"]+)"[^>]*/>\s*</figure>\s*<!--\s*/wp:image\s*-->`).ReplaceAllString(content, `<img src="$1" />`)

	// Image block with link: convert linked images
	content = regexp.MustCompile(`(?s)<!--\s*wp:image\s*-->\s*<figure[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>\s*<img\s+src="([^"]+)"[^>]*/>\s*</a>\s*</figure>\s*<!--\s*/wp:image\s*-->`).ReplaceAllString(content, `<a href="$1"><img src="$2" /></a>`)

	// Heading blocks: <!-- wp:heading {"level":2} -->...<!-- /wp:heading -->
	content = regexp.MustCompile(`(?s)<!--\s*wp:heading\s*(?:\{[^}]*\})?\s*-->\s*<h([1-6])[^>]*>(.*?)</h\1>\s*<!--\s*/wp:heading\s*-->`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`<h([1-6])[^>]*>(.*?)</h\1>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) > 2 {
			return "<" + matches[1] + ">" + matches[2] + "</" + matches[1] + ">"
		}
		return match
	})

	// List blocks: <!-- wp:list -->...<!-- /wp:list -->
	content = regexp.MustCompile(`(?s)<!--\s*wp:list\s*-->\s*<ul[^>]*>(.*?)</ul>\s*<!--\s*/wp:list\s*-->`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`<ul[^>]*>(.*?)</ul>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) > 1 {
			return "<ul>" + matches[1] + "</ul>"
		}
		return match
	})

	// Quote blocks: <!-- wp:quote -->...<!-- /wp:quote -->
	content = regexp.MustCompile(`(?s)<!--\s*wp:quote\s*-->\s*<blockquote[^>]*>(.*?)</blockquote>\s*<!--\s*/wp:quote\s*-->`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`<blockquote[^>]*>(.*?)</blockquote>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) > 1 {
			return "<blockquote>" + matches[1] + "</blockquote>"
		}
		return match
	})

	// Code blocks: <!-- wp:code -->...<!-- /wp:code -->
	content = regexp.MustCompile(`(?s)<!--\s*wp:code\s*-->\s*<pre[^>]*><code[^>]*>(.*?)</code></pre>\s*<!--\s*/wp:code\s*-->`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`<code[^>]*>(.*?)</code>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) > 1 {
			return "<pre><code>" + matches[1] + "</code></pre>"
		}
		return match
	})

	// Separator: <!-- wp:separator -->
	content = regexp.MustCompile(`<!--\s*wp:separator\s*-->`).ReplaceAllString(content, "<hr />")

	// More block: <!-- more -->
	content = regexp.MustCompile(`<!--\s*more\s*-->`).ReplaceAllString(content, "<!--more-->")

	// Gallery: <!-- wp:gallery -->...<!-- /wp:gallery -->
	content = regexp.MustCompile(`(?s)<!--\s*wp:gallery\s*-->(.*?)<!--\s*/wp:gallery\s*-->`).ReplaceAllStringFunc(content, func(match string) string {
		// Extract individual image src from gallery
		re := regexp.MustCompile(`<img\s+src="([^"]+)"`)
		matches := re.FindAllStringSubmatch(match, -1)
		var imgs []string
		for _, m := range matches {
			if len(m) > 1 {
				imgs = append(imgs, `<img src="`+m[1]+`" />`)
			}
		}
		if len(imgs) > 0 {
			return `<div class="gallery">` + strings.Join(imgs, "") + `</div>`
		}
		return match
	})

	// Cover block: <!-- wp:cover -->...<!-- /wp:cover -->
	content = regexp.MustCompile(`(?s)<!--\s*wp:cover\s*-->\s*<div[^>]*background-image:[^>]*url\(([^)]+)\)[^>]*>.*?</div>\s*<!--\s*/wp:cover\s*-->`).ReplaceAllString(content, `<div class="cover" style="background-image:url($1)"></div>`)

	// Pullquote: <!-- wp:pullquote -->...<!-- /wp:pullquote -->
	content = regexp.MustCompile(`(?s)<!--\s*wp:pullquote\s*-->\s*<blockquote[^>]*>(.*?)</blockquote>\s*<!--\s*/wp:pullquote\s*-->`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`<blockquote[^>]*>(.*?)</blockquote>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) > 1 {
			return `<blockquote class="pullquote">` + matches[1] + `</blockquote>`
		}
		return match
	})

	// Button block inside group
	content = regexp.MustCompile(`<!--\s*wp:buttons\s*-->(.*?)<!--\s*/wp:buttons\s*-->`).ReplaceAllStringFunc(content, func(match string) string {
		re := regexp.MustCompile(`<div[^>]*class="[^"]*wp-block-button[^"]*"[^>]*>(.*?)</div>`)
		matches := re.FindStringSubmatch(match)
		if len(matches) > 1 {
			return matches[0]
		}
		return match
	})

	// Columns: <!-- wp:columns -->...<!-- /wp:columns -->
	content = regexp.MustCompile(`(?s)<!--\s*wp:columns\s*-->\s*(<div[^>]*class="[^"]*wp-block-columns[^"]*"[^>]*>)(.*?)(</div>)\s*<!--\s*/wp:columns\s*-->`).ReplaceAllString(content, `<div class="columns">$2</div>`)

	// Group block
	content = regexp.MustCompile(`(?s)<!--\s*wp:group\s*-->\s*(<div[^>]*>)(.*?)(</div>)\s*<!--\s*/wp:group\s*-->`).ReplaceAllString(content, `<div class="group">$2</div>`)

	// Spacer: <!-- wp:spacer -->
	content = regexp.MustCompile(`<!--\s*wp:spacer\s*-->`).ReplaceAllString(content, `<div class="spacer"></div>`)

	// Embed blocks - convert to figures with captions
	content = regexp.MustCompile(`(?s)<!--\s*wp:embed\s*-->\s*(<figure[^>]*>.*?</figure>)\s*<!--\s*/wp:embed\s*-->`).ReplaceAllString(content, "$1")

	// Remove remaining Gutenberg comments: <!-- wp:xxx --> and <!-- /wp:xxx -->
	content = regexp.MustCompile(`<!--\s*/?wp:[a-z0-9-]+\s*(?:\{[^}]*\})?\s*-->`).ReplaceAllString(content, "")

	// Clean up empty paragraphs
	content = regexp.MustCompile(`<p>\s*</p>`).ReplaceAllString(content, "")

	// Clean up double spaces
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")

	// Trim whitespace around block elements
	content = strings.TrimSpace(content)

	return content
}

func htmlToMarkdown(html string) string {
	if html == "" {
		return html
	}

	content := html

	// Headers: h1-h6
	for i := 1; i <= 6; i++ {
		tag := fmt.Sprintf("h%d", i)
		closeTag := fmt.Sprintf("/h%d", i)
		re := regexp.MustCompile(`<` + tag + `[^>]*>(.*?)</` + closeTag + `>`)
		prefix := strings.Repeat("#", i) + " "
		content = re.ReplaceAllStringFunc(content, func(match string) string {
			re2 := regexp.MustCompile(`<` + tag + `[^>]*>(.*?)</` + closeTag + `>`)
			matches := re2.FindStringSubmatch(match)
			if len(matches) > 1 {
				return prefix + stripTags(matches[1]) + "\n\n"
			}
			return match
		})
	}

	// Bold/strong
	reBold := regexp.MustCompile(`<(?:strong|b)[^>]*>(.*?)</(?:strong|b)>`)
	content = reBold.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<(?:strong|b)[^>]*>(.*?)</(?:strong|b)>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 1 {
			return "**" + matches[1] + "**"
		}
		return match
	})

	// Italic/em
	reItalic := regexp.MustCompile(`<(?:em|i)[^>]*>(.*?)</(?:em|i)>`)
	content = reItalic.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<(?:em|i)[^>]*>(.*?)</(?:em|i)>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 1 {
			return "*" + matches[1] + "*"
		}
		return match
	})

	// Links
	reLink := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	content = reLink.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 2 {
			return "[" + stripTags(matches[2]) + "](href:" + matches[1] + ")"
		}
		return match
	})

	// Images: <img src="..." alt="..." />
	reImg := regexp.MustCompile(`<img[^>]*src="([^"]+)"[^>]*>`)
	content = reImg.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<img[^>]*src="([^"]+)"[^>]*alt="([^"]*)"[^>]*>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 2 {
			return "![" + matches[2] + "](href:" + matches[1] + ")"
		}
		re3 := regexp.MustCompile(`<img[^>]*src="([^"]+)"[^>]*>`)
		matches2 := re3.FindStringSubmatch(match)
		if len(matches2) > 1 {
			return "![](href:" + matches2[1] + ")"
		}
		return match
	})

	// Code blocks: <pre><code>...</code></pre>
	reCodeBlock := regexp.MustCompile(`<pre[^>]*><code[^>]*>(.*?)</code></pre>`)
	content = reCodeBlock.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<pre[^>]*><code[^>]*>(.*?)</code></pre>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 1 {
			return "```\n" + matches[1] + "\n```\n\n"
		}
		return match
	})

	// Inline code: <code>...</code>
	reInlineCode := regexp.MustCompile(`<code[^>]*>(.*?)</code>`)
	content = reInlineCode.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<code[^>]*>(.*?)</code>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 1 {
			return "`" + matches[1] + "`"
		}
		return match
	})

	// Blockquote
	reBlockquote := regexp.MustCompile(`<blockquote[^>]*>(.*?)</blockquote>`)
	content = reBlockquote.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<blockquote[^>]*>(.*?)</blockquote>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 1 {
			lines := strings.Split(matches[1], "\n")
			var result []string
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					result = append(result, "> "+line)
				}
			}
			return strings.Join(result, "\n") + "\n\n"
		}
		return match
	})

	// Horizontal rule: <hr />
	reHr := regexp.MustCompile(`<hr\s*/?>`)
	content = reHr.ReplaceAllString(content, "\n---\n\n")

	// Line breaks: <br />
	reBr := regexp.MustCompile(`<br\s*/?>`)
	content = reBr.ReplaceAllString(content, "\n")

	// Lists: <ul><li>...</li></ul>
	reUl := regexp.MustCompile(`<ul[^>]*>(.*?)</ul>`)
	content = reUl.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<ul[^>]*>(.*?)</ul>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 1 {
			reLi := regexp.MustCompile(`<li[^>]*>(.*?)</li>`)
			items := reLi.FindAllStringSubmatch(matches[1], -1)
			var result []string
			for _, item := range items {
				if len(item) > 1 {
					result = append(result, "- "+stripTags(item[1]))
				}
			}
			return strings.Join(result, "\n") + "\n\n"
		}
		return match
	})

	// Ordered lists: <ol><li>...</li></ol>
	reOl := regexp.MustCompile(`<ol[^>]*>(.*?)</ol>`)
	content = reOl.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<ol[^>]*>(.*?)</ol>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 1 {
			reLi := regexp.MustCompile(`<li[^>]*>(.*?)</li>`)
			items := reLi.FindAllStringSubmatch(matches[1], -1)
			var result []string
			for i, item := range items {
				if len(item) > 1 {
					result = append(result, fmt.Sprintf("%d. %s", i+1, stripTags(item[1])))
				}
			}
			return strings.Join(result, "\n") + "\n\n"
		}
		return match
	})

	// Paragraphs: <p>...</p>
	reP := regexp.MustCompile(`<p[^>]*>(.*?)</p>`)
	content = reP.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<p[^>]*>(.*?)</p>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 1 {
			return stripTags(matches[1]) + "\n\n"
		}
		return match
	})

	// Divs to newlines: <div>...</div>
	reDiv := regexp.MustCompile(`<div[^>]*>(.*?)</div>`)
	content = reDiv.ReplaceAllStringFunc(content, func(match string) string {
		re2 := regexp.MustCompile(`<div[^>]*>(.*?)</div>`)
		matches := re2.FindStringSubmatch(match)
		if len(matches) > 1 {
			return stripTags(matches[1]) + "\n\n"
		}
		return match
	})

	// Clean up remaining HTML tags
	content = stripRemainingTags(content)

	// Clean up excessive newlines
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")

	// Trim whitespace
	content = strings.TrimSpace(content)

	return content
}

func stripTags(html string) string {
	if html == "" {
		return html
	}
	re := regexp.MustCompile(`<[^>]+>`)
	result := re.ReplaceAllString(html, "")
	// Decode common HTML entities
	result = strings.ReplaceAll(result, "&nbsp;", " ")
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&quot;", "\"")
	result = strings.ReplaceAll(result, "&#39;", "'")
	return strings.TrimSpace(result)
}

func stripRemainingTags(html string) string {
	if html == "" {
		return html
	}
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(html, "")
}

func replaceMediaURLs(content string, urlMap map[string]string) string {
	if content == "" || len(urlMap) == 0 {
		return content
	}

	result := content

	// Replace URLs in img src attributes
	imgRe := regexp.MustCompile(`(<img[^>]*src=")([^"]+)("[^>]*>)`)
	result = imgRe.ReplaceAllStringFunc(result, func(match string) string {
		submatches := imgRe.FindStringSubmatch(match)
		if len(submatches) > 3 {
			oldURL := submatches[2]
			if newPath, ok := urlMap[oldURL]; ok {
				return submatches[1] + newPath + submatches[3]
			}
			// Try partial match (in case URL has query params)
			for oldBase, newPath := range urlMap {
				if strings.Contains(oldURL, oldBase) {
					return submatches[1] + newPath + submatches[3]
				}
			}
		}
		return match
	})

	// Replace URLs in href attributes (for image links)
	hrefRe := regexp.MustCompile(`(<a[^>]*href=")([^"]+)("[^>]*>)`)
	result = hrefRe.ReplaceAllStringFunc(result, func(match string) string {
		submatches := hrefRe.FindStringSubmatch(match)
		if len(submatches) > 3 {
			oldURL := submatches[2]
			if newPath, ok := urlMap[oldURL]; ok {
				return submatches[1] + newPath + submatches[3]
			}
			for oldBase, newPath := range urlMap {
				if strings.Contains(oldURL, oldBase) {
					return submatches[1] + newPath + submatches[3]
				}
			}
		}
		return match
	})

	// Replace URLs in style attributes (for background-image)
	styleRe := regexp.MustCompile(`(style="[^"]*url\()([^)]+)(\)[^"]*")`)
	result = styleRe.ReplaceAllStringFunc(result, func(match string) string {
		submatches := styleRe.FindStringSubmatch(match)
		if len(submatches) > 3 {
			oldURL := submatches[2]
			// Remove quotes if present
			oldURL = strings.Trim(oldURL, `"'`)
			if newPath, ok := urlMap[oldURL]; ok {
				return submatches[1] + `"` + newPath + `"` + submatches[3]
			}
			for oldBase, newPath := range urlMap {
				if strings.Contains(oldURL, oldBase) {
					return submatches[1] + `"` + newPath + `"` + submatches[3]
				}
			}
		}
		return match
	})

	// Replace URLs in srcset attributes
	srcsetRe := regexp.MustCompile(`(srcset=")([^"]+)("[^>]*>)`)
	result = srcsetRe.ReplaceAllStringFunc(result, func(match string) string {
		submatches := srcsetRe.FindStringSubmatch(match)
		if len(submatches) > 3 {
			srcset := submatches[2]
			for oldURL, newPath := range urlMap {
				if strings.Contains(srcset, oldURL) {
					srcset = strings.ReplaceAll(srcset, oldURL, newPath)
				}
			}
			return submatches[1] + srcset + submatches[3]
		}
		return match
	})

	// Generic URL replacement for any remaining URLs (for Gutenberg JSON attrs)
	for oldURL, newPath := range urlMap {
		// Replace exact URL
		result = strings.Replace(result, `"`+oldURL+`"`, `"`+newPath+`"`, -1)
		result = strings.Replace(result, `'`+oldURL+`'`, `'`+newPath+`'`, -1)
	}

	return result
}

func (s *importerService) GetImportLog(ctx context.Context, id uuid.UUID) (*ImportLog, error) {
	var log ImportLog
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (s *importerService) ListImportLogs(ctx context.Context, workspaceID uuid.UUID, page, perPage int) ([]ImportLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	offset := (page - 1) * perPage

	var logs []ImportLog
	var total int64

	query := s.db.WithContext(ctx).Model(&ImportLog{}).Where("workspace_id = ?", workspaceID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at desc").Limit(perPage).Offset(offset).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (s *importerService) ParseCSVHeaders(ctx context.Context, file io.Reader) ([]string, error) {
	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}
	return headers, nil
}

func (s *importerService) readImportFile(ctx context.Context, fileURL string) ([]byte, error) {
	if strings.HasPrefix(fileURL, "/uploads/") {
		localPath := filepath.Join("uploads", strings.TrimPrefix(fileURL, "/uploads/"))
		return os.ReadFile(localPath)
	}
	// Fallback to HTTP download
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download file, HTTP status: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (s *importerService) ImportCSVBackground(ctx context.Context, workspaceID, authorID, logID uuid.UUID, fileURL string, mapping map[string]string, defaultStatus, defaultPostType string) {
	bgCtx := context.Background()

	log := &ImportLog{}
	if err := s.db.Where("id = ?", logID).First(log).Error; err != nil {
		return
	}

	updateLogStatus := func(status string, result ImportResult, errorsList []string) {
		log.Status = status
		log.PostsCount = result.PostsCount
		log.PagesCount = result.PagesCount
		log.MediaCount = result.MediaCount
		log.TaxCount = result.TaxCount
		log.SkippedCount = result.SkippedCount

		errorsJSON, _ := json.Marshal(errorsList)
		log.Errors = string(errorsJSON)

		summaryJSON, _ := json.Marshal(result)
		log.Summary = string(summaryJSON)

		s.db.Save(log)
	}

	result := ImportResult{}
	var errorsList []string

	// 1. Fetch file
	data, err := s.readImportFile(bgCtx, fileURL)
	if err != nil {
		errorsList = append(errorsList, fmt.Sprintf("Failed to read file from %s: %v", fileURL, err))
		updateLogStatus("failed", result, errorsList)
		return
	}

	// 2. Parse CSV
	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		errorsList = append(errorsList, fmt.Sprintf("Failed to parse CSV records: %v", err))
		updateLogStatus("failed", result, errorsList)
		return
	}

	if len(records) < 2 {
		errorsList = append(errorsList, "CSV file has no data rows")
		updateLogStatus("failed", result, errorsList)
		return
	}

	headers := records[0]
	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.TrimSpace(h)] = i
	}

	// Helper to extract column value by field name using mapping
	getVal := func(row []string, field string) string {
		colName, ok := mapping[field]
		if !ok || colName == "" {
			return ""
		}
		idx, ok := headerMap[colName]
		if !ok || idx >= len(row) {
			return ""
		}
		return row[idx]
	}

	// Process each row
	for rowIndex := 1; rowIndex < len(records); rowIndex++ {
		row := records[rowIndex]
		if len(row) == 0 {
			continue
		}

		title := getVal(row, "title")
		if title == "" {
			result.SkippedCount++
			errorsList = append(errorsList, fmt.Sprintf("Row %d: title is empty, skipped", rowIndex+1))
			continue
		}

		content := getVal(row, "content")
		excerpt := getVal(row, "excerpt")
		slug := getVal(row, "slug")
		status := getVal(row, "status")
		postType := getVal(row, "post_type")
		publishedAtStr := getVal(row, "published_at")
		featureImageCol := getVal(row, "feature_image")

		if status == "" {
			status = defaultStatus
		}
		if status == "" {
			status = "draft"
		}
		if status != "draft" && status != "published" && status != "scheduled" && status != "archived" {
			status = "draft"
		}

		if postType == "" {
			postType = defaultPostType
		}
		if postType == "" {
			postType = "post"
		}

		if slug == "" {
			slug = helpers.Slugify(title)
		}

		originalSlug := slug
		counter := 1
		for {
			existing, _ := s.postRepo.FindBySlug(bgCtx, workspaceID, slug)
			if existing == nil {
				break
			}
			slug = fmt.Sprintf("%s-%d", originalSlug, counter)
			counter++
		}

		var publishedAt *time.Time
		if publishedAtStr != "" {
			// Try various date formats
			formats := []string{
				time.RFC3339,
				"2006-01-02 15:04:05",
				"2006-01-02 15:04",
				"2006-01-02",
				"01/02/2006",
				"02/01/2006",
			}
			for _, fmtStr := range formats {
				t, err := time.Parse(fmtStr, publishedAtStr)
				if err == nil {
					publishedAt = &t
					break
				}
			}
		}

		if publishedAt == nil && status == "published" {
			now := time.Now()
			publishedAt = &now
		}

		// Handle Feature Image download
		featureImageURL := ""
		if featureImageCol != "" && (strings.HasPrefix(featureImageCol, "http://") || strings.HasPrefix(featureImageCol, "https://")) {
			imgData, imgMime, err := s.downloadImage(bgCtx, featureImageCol)
			if err != nil {
				errorsList = append(errorsList, fmt.Sprintf("Row %d: failed to download feature image %s: %v", rowIndex+1, featureImageCol, err))
			} else {
				u, parseErr := url.Parse(featureImageCol)
				filename := "feature_image"
				if parseErr == nil {
					filename = filepath.Base(u.Path)
				}
				if ext := s.mimeToExt(imgMime); ext != "" && !strings.HasSuffix(filename, ext) {
					filename = filename + ext
				}

				m, err := s.mediaSvc.SaveFile(bgCtx, workspaceID, filename, imgData, imgMime, int64(len(imgData)), "Featured Image for "+title, "")
				if err != nil {
					errorsList = append(errorsList, fmt.Sprintf("Row %d: failed to save feature image to storage: %v", rowIndex+1, err))
				} else {
					featureImageURL = m.Path
					result.MediaCount++
				}
			}
		} else if featureImageCol != "" {
			featureImageURL = featureImageCol
		}

		p := &post.Post{
			ID:           uuid.New(),
			Title:        title,
			Slug:         slug,
			Content:      content,
			Excerpt:      excerpt,
			Status:       status,
			AuthorID:     authorID,
			WorkspaceID:  workspaceID,
			PostType:     postType,
			PublishedAt:  publishedAt,
			FeatureImage: featureImageURL,
		}

		if err := s.postRepo.Create(bgCtx, p); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("Row %d: failed to create post: %v", rowIndex+1, err))
			continue
		}

		revision := &post.PostRevision{
			ID:           uuid.New(),
			PostID:       p.ID,
			Title:        p.Title,
			Content:      p.Content,
			Excerpt:      p.Excerpt,
			AuthorID:     authorID,
			FeatureImage: p.FeatureImage,
		}
		s.postRepo.CreateRevision(bgCtx, revision)

		if postType == "post" {
			result.PostsCount++
		} else {
			result.PagesCount++
		}
	}

	updateLogStatus("completed", result, errorsList)
}

func (s *importerService) UploadCSV(ctx context.Context, workspaceID uuid.UUID, filename string, fileData []byte) (string, []string, error) {
	// 1. Parse headers first to validate
	headers, err := s.ParseCSVHeaders(ctx, bytes.NewReader(fileData))
	if err != nil {
		return "", nil, fmt.Errorf("invalid CSV file: %w", err)
	}

	// 2. Save file using media service
	m, err := s.mediaSvc.SaveFile(ctx, workspaceID, filename, fileData, "text/csv", int64(len(fileData)), "Import source CSV: "+filename, "")
	if err != nil {
		return "", nil, fmt.Errorf("failed to save CSV to storage: %w", err)
	}

	return m.Path, headers, nil
}

func (s *importerService) InspectStrapi(ctx context.Context, urlStr, token, collectionType string) ([]string, error) {
	strapiURL := strings.TrimSuffix(urlStr, "/")
	endpoint := collectionType
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	if !strings.HasPrefix(endpoint, "/api/") && !strings.HasPrefix(endpoint, "/api") {
		endpoint = "/api" + endpoint
	}

	reqURL := fmt.Sprintf("%s%s?pagination[pageSize]=1", strapiURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Strapi endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("strapi returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	dataVal, exists := result["data"]
	if !exists {
		return nil, fmt.Errorf("invalid Strapi response structure (missing 'data' key)")
	}

	var dataList []interface{}
	if list, ok := dataVal.([]interface{}); ok {
		dataList = list
	} else if item, ok := dataVal.(map[string]interface{}); ok {
		dataList = []interface{}{item}
	}

	if len(dataList) == 0 {
		return nil, fmt.Errorf("no content found in collection type '%s' to inspect fields", collectionType)
	}

	firstItem, ok := dataList[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid format for data item")
	}

	fieldsMap := make(map[string]bool)
	// Add root fields
	for k := range firstItem {
		if k != "attributes" {
			fieldsMap[k] = true
		}
	}

	// Add attributes fields (Strapi v4)
	if attrs, ok := firstItem["attributes"].(map[string]interface{}); ok {
		for k := range attrs {
			fieldsMap[k] = true
		}
	}

	fields := make([]string, 0, len(fieldsMap))
	for k := range fieldsMap {
		fields = append(fields, k)
	}

	return fields, nil
}

func (s *importerService) ImportStrapiBackground(ctx context.Context, workspaceID, authorID, logID uuid.UUID, urlStr, token, collectionType string, mapping map[string]string, defaultStatus, defaultPostType string) {
	bgCtx := context.Background()

	logEntity := &ImportLog{}
	if err := s.db.Where("id = ?", logID).First(logEntity).Error; err != nil {
		return
	}

	updateLogStatus := func(status string, result ImportResult, errorsList []string) {
		logEntity.Status = status
		logEntity.PostsCount = result.PostsCount
		logEntity.PagesCount = result.PagesCount
		logEntity.MediaCount = result.MediaCount
		logEntity.TaxCount = result.TaxCount
		logEntity.SkippedCount = result.SkippedCount

		errorsJSON, _ := json.Marshal(errorsList)
		logEntity.Errors = string(errorsJSON)

		summaryJSON, _ := json.Marshal(result)
		logEntity.Summary = string(summaryJSON)

		s.db.Save(logEntity)
	}

	result := ImportResult{}
	var errorsList []string

	strapiURL := strings.TrimSuffix(urlStr, "/")
	endpoint := collectionType
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	if !strings.HasPrefix(endpoint, "/api/") && !strings.HasPrefix(endpoint, "/api") {
		endpoint = "/api" + endpoint
	}

	page := 1
	pageSize := 25
	hasMore := true

	getVal := func(itemMap map[string]interface{}, field string) interface{} {
		key, ok := mapping[field]
		if !ok || key == "" {
			return nil
		}
		// First check in attributes if available (v4)
		if attrs, ok := itemMap["attributes"].(map[string]interface{}); ok {
			if val, ok := attrs[key]; ok {
				return val
			}
		}
		// Fallback to root map
		return itemMap[key]
	}

	getStrapiMediaURL := func(val interface{}, strapiURL string) string {
		if val == nil {
			return ""
		}
		if str, ok := val.(string); ok {
			if str == "" {
				return ""
			}
			if !strings.HasPrefix(str, "http://") && !strings.HasPrefix(str, "https://") {
				return strings.TrimSuffix(strapiURL, "/") + "/" + strings.TrimPrefix(str, "/")
			}
			return str
		}
		if m, ok := val.(map[string]interface{}); ok {
			if urlVal, ok := m["url"].(string); ok && urlVal != "" {
				if !strings.HasPrefix(urlVal, "http://") && !strings.HasPrefix(urlVal, "https://") {
					return strings.TrimSuffix(strapiURL, "/") + "/" + strings.TrimPrefix(urlVal, "/")
				}
				return urlVal
			}
			if dataVal, ok := m["data"].(map[string]interface{}); ok {
				if attrs, ok := dataVal["attributes"].(map[string]interface{}); ok {
					if urlVal, ok := attrs["url"].(string); ok && urlVal != "" {
						if !strings.HasPrefix(urlVal, "http://") && !strings.HasPrefix(urlVal, "https://") {
							return strings.TrimSuffix(strapiURL, "/") + "/" + strings.TrimPrefix(urlVal, "/")
						}
						return urlVal
					}
				}
				if urlVal, ok := dataVal["url"].(string); ok && urlVal != "" {
					if !strings.HasPrefix(urlVal, "http://") && !strings.HasPrefix(urlVal, "https://") {
						return strings.TrimSuffix(strapiURL, "/") + "/" + strings.TrimPrefix(urlVal, "/")
					}
					return urlVal
				}
			}
		}
		return ""
	}

	for hasMore {
		reqURL := fmt.Sprintf("%s%s?pagination[page]=%d&pagination[pageSize]=%d", strapiURL, endpoint, page, pageSize)
		req, err := http.NewRequestWithContext(bgCtx, "GET", reqURL, nil)
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("Failed to create request: %v", err))
			updateLogStatus("failed", result, errorsList)
			return
		}

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("Failed to fetch Strapi page %d: %v", page, err))
			updateLogStatus("failed", result, errorsList)
			return
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			errorsList = append(errorsList, fmt.Sprintf("Strapi API returned status %d on page %d: %s", resp.StatusCode, page, string(bodyBytes)))
			updateLogStatus("failed", result, errorsList)
			return
		}

		var apiResp map[string]interface{}
		decodeErr := json.NewDecoder(resp.Body).Decode(&apiResp)
		resp.Body.Close()
		if decodeErr != nil {
			errorsList = append(errorsList, fmt.Sprintf("Failed to decode JSON from Strapi on page %d: %v", page, decodeErr))
			updateLogStatus("failed", result, errorsList)
			return
		}

		dataVal, exists := apiResp["data"]
		if !exists {
			errorsList = append(errorsList, fmt.Sprintf("No 'data' field found in response on page %d", page))
			updateLogStatus("failed", result, errorsList)
			return
		}

		var items []interface{}
		if list, ok := dataVal.([]interface{}); ok {
			items = list
		} else if item, ok := dataVal.(map[string]interface{}); ok {
			items = []interface{}{item}
		}

		if len(items) == 0 {
			hasMore = false
			break
		}

		for itemIdx, itemVal := range items {
			itemMap, ok := itemVal.(map[string]interface{})
			if !ok {
				continue
			}

			titleVal := getVal(itemMap, "title")
			title := ""
			if titleVal != nil {
				title = fmt.Sprintf("%v", titleVal)
			}
			if title == "" {
				result.SkippedCount++
				errorsList = append(errorsList, fmt.Sprintf("Page %d item %d: title is empty, skipped", page, itemIdx+1))
				continue
			}

			contentVal := getVal(itemMap, "content")
			content := ""
			if contentVal != nil {
				content = fmt.Sprintf("%v", contentVal)
			}

			excerptVal := getVal(itemMap, "excerpt")
			excerpt := ""
			if excerptVal != nil {
				excerpt = fmt.Sprintf("%v", excerptVal)
			}

			slugVal := getVal(itemMap, "slug")
			slug := ""
			if slugVal != nil {
				slug = fmt.Sprintf("%v", slugVal)
			}

			statusVal := getVal(itemMap, "status")
			status := ""
			if statusVal != nil {
				status = strings.ToLower(fmt.Sprintf("%v", statusVal))
			}

			postTypeVal := getVal(itemMap, "post_type")
			postType := ""
			if postTypeVal != nil {
				postType = strings.ToLower(fmt.Sprintf("%v", postTypeVal))
			}

			publishedAtVal := getVal(itemMap, "published_at")
			publishedAtStr := ""
			if publishedAtVal != nil {
				publishedAtStr = fmt.Sprintf("%v", publishedAtVal)
			}

			featureImageVal := getVal(itemMap, "feature_image")

			if status == "" {
				status = defaultStatus
			}
			if status == "" {
				status = "draft"
			}
			if status != "draft" && status != "published" && status != "scheduled" && status != "archived" {
				status = "draft"
			}

			if postType == "" {
				postType = defaultPostType
			}
			if postType == "" {
				postType = "post"
			}

			if slug == "" {
				slug = helpers.Slugify(title)
			}

			originalSlug := slug
			counter := 1
			for {
				existing, _ := s.postRepo.FindBySlug(bgCtx, workspaceID, slug)
				if existing == nil {
					break
				}
				slug = fmt.Sprintf("%s-%d", originalSlug, counter)
				counter++
			}

			var publishedAt *time.Time
			if publishedAtStr != "" {
				formats := []string{
					time.RFC3339,
					"2006-01-02 15:04:05",
					"2006-01-02 15:04",
					"2006-01-02",
				}
				for _, fmtStr := range formats {
					t, err := time.Parse(fmtStr, publishedAtStr)
					if err == nil {
						publishedAt = &t
						break
					}
				}
			}
			if publishedAt == nil && status == "published" {
				now := time.Now()
				publishedAt = &now
			}

			featureImageURL := ""
			resolvedImgURL := getStrapiMediaURL(featureImageVal, urlStr)
			if resolvedImgURL != "" {
				imgData, imgMime, err := s.downloadImage(bgCtx, resolvedImgURL)
				if err != nil {
					errorsList = append(errorsList, fmt.Sprintf("Page %d item %d: failed to download feature image %s: %v", page, itemIdx+1, resolvedImgURL, err))
				} else {
					u, parseErr := url.Parse(resolvedImgURL)
					filename := "feature_image"
					if parseErr == nil {
						filename = filepath.Base(u.Path)
					}
					if ext := s.mimeToExt(imgMime); ext != "" && !strings.HasSuffix(filename, ext) {
						filename = filename + ext
					}

					m, err := s.mediaSvc.SaveFile(bgCtx, workspaceID, filename, imgData, imgMime, int64(len(imgData)), "Featured Image for "+title, "")
					if err != nil {
						errorsList = append(errorsList, fmt.Sprintf("Page %d item %d: failed to save feature image to storage: %v", page, itemIdx+1, err))
					} else {
						featureImageURL = m.Path
						result.MediaCount++
					}
				}
			}

			p := &post.Post{
				ID:           uuid.New(),
				Title:        title,
				Slug:         slug,
				Content:      content,
				Excerpt:      excerpt,
				Status:       status,
				AuthorID:     authorID,
				WorkspaceID:  workspaceID,
				PostType:     postType,
				PublishedAt:  publishedAt,
				FeatureImage: featureImageURL,
			}

			if err := s.postRepo.Create(bgCtx, p); err != nil {
				errorsList = append(errorsList, fmt.Sprintf("Page %d item %d: failed to create post: %v", page, itemIdx+1, err))
				continue
			}

			revision := &post.PostRevision{
				ID:           uuid.New(),
				PostID:       p.ID,
				Title:        p.Title,
				Content:      p.Content,
				Excerpt:      p.Excerpt,
				AuthorID:     authorID,
				FeatureImage: p.FeatureImage,
			}
			s.postRepo.CreateRevision(bgCtx, revision)

			if postType == "post" {
				result.PostsCount++
			} else {
				result.PagesCount++
			}
		}

		if len(items) < pageSize {
			hasMore = false
		} else {
			if metaVal, exists := apiResp["meta"]; exists {
				if metaMap, ok := metaVal.(map[string]interface{}); ok {
					if pagVal, ok := metaMap["pagination"].(map[string]interface{}); ok {
						if pageCountVal, ok := pagVal["pageCount"]; ok {
							var pageCount int
							switch v := pageCountVal.(type) {
							case float64:
								pageCount = int(v)
							case int:
								pageCount = v
							}
							if page >= pageCount {
								hasMore = false
							}
						}
					}
				}
			}
			page++
		}
	}

	updateLogStatus("completed", result, errorsList)
}

// ImportMarkdown imports a zip of markdown files as posts with folder-based categories.
type ImportMarkdownOpts struct {
	PostType      string
	SkipExisting  bool
}

func (s *importerService) ImportMarkdown(ctx context.Context, workspaceID, authorID uuid.UUID, file multipart.File, _ string, opts ImportMarkdownOpts) (*ImportLog, error) {
	fileData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read uploaded file: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip file: %w", err)
	}

	createdAt := time.Now()
	logID := uuid.New()
	log := &ImportLog{
		ID:          logID,
		WorkspaceID: workspaceID,
		AuthorID:    authorID,
		Filename:    "markdown-import.zip",
		Status:      "running",
		CreatedAt:   createdAt,
	}
	if err := s.db.WithContext(ctx).Create(log).Error; err != nil {
		return nil, fmt.Errorf("failed to create import log: %w", err)
	}

	var result ImportResult
	var errorsList []string
	categoryCache := make(map[string]uuid.UUID)

	for _, zf := range zipReader.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(zf.Name), ".md") {
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("failed to open %s: %v", zf.Name, err))
			continue
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("failed to read %s: %v", zf.Name, err))
			continue
		}

		body := strings.TrimSpace(string(content))
		if body == "" {
			result.SkippedCount++
			continue
		}

		title := extractMdTitle(body)
		if title == "" {
			result.SkippedCount++
			errorsList = append(errorsList, fmt.Sprintf("skipped %s: no title found", zf.Name))
			continue
		}

		mdBody := extractMdBody(body)
		dir := filepath.Dir(zf.Name)

		slug := parseMdSlug(zf.Name, title)

		// Skip duplicate slugs only when opted in
		existingPost, _ := s.postRepo.FindBySlug(ctx, workspaceID, slug)
		if existingPost != nil {
			if opts.SkipExisting {
				result.SkippedCount++
				errorsList = append(errorsList, fmt.Sprintf("skipped %s: slug '%s' already exists", zf.Name, slug))
				continue
			}
			// Append random suffix to avoid collision
			slug = slug + "-" + uuid.New().String()[:8]
		}

		// Nested folder → hierarchy of categories
		var categoryIDs []uuid.UUID
		if dir != "." && dir != "" {
			parts := strings.Split(dir, "/")
			parentID := uuid.Nil
			catPath := ""
			for _, part := range parts {
				if catPath != "" {
					catPath += "/"
				}
				catPath += part
				catSlug := helpers.Slugify(part)
				if catSlug == "" {
					continue
				}

				var catID uuid.UUID
				if existingID, ok := categoryCache[catPath]; ok {
					catID = existingID
				} else {
					catName := strings.ReplaceAll(part, "-", " ")
					catName = strings.ReplaceAll(catName, "_", " ")
					if len(catName) > 0 {
						words := strings.Fields(catName)
						for i, w := range words {
							if len(w) > 0 {
								words[i] = strings.ToUpper(w[:1]) + w[1:]
							}
						}
						catName = strings.Join(words, " ")
					}

					existingTax, _ := s.postRepo.FindTaxonomyBySlug(ctx, workspaceID, catSlug, "category")
					if existingTax != nil {
						catID = existingTax.ID
					} else {
						var parentPtr *uuid.UUID
						if parentID != uuid.Nil {
							parentPtr = &parentID
						}
						category := &post.Taxonomy{
							ID:          uuid.New(),
							WorkspaceID: workspaceID,
							Name:        catName,
							Slug:        catSlug,
							Type:        "category",
							ParentID:    parentPtr,
						}
						if err := s.postRepo.CreateTaxonomy(ctx, category); err != nil {
							errorsList = append(errorsList, fmt.Sprintf("failed to create category '%s': %v", catName, err))
							break
						}
						catID = category.ID
						result.TaxCount++
					}
					categoryCache[catPath] = catID
				}
				categoryIDs = append(categoryIDs, catID)
				parentID = catID
			}
		}

		postType := opts.PostType
		if postType == "" {
			postType = "post"
		}

		now := time.Now()
		postRecord := &post.Post{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			Title:       title,
			Slug:        slug,
			Content:     mdBody,
			Status:      "draft",
			PostType:    postType,
			AuthorID:    authorID,
			CreatedAt:   now,
			EditedAt:    &now,
		}
		if err := s.postRepo.Create(ctx, postRecord); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("failed to create post '%s': %v", title, err))
			result.SkippedCount++
			continue
		}
		result.PostsCount++

		// Assign lowest-level category (most specific)
		if len(categoryIDs) > 0 {
			lowest := categoryIDs[len(categoryIDs)-1]
			_ = s.postRepo.AssignTaxonomies(ctx, postRecord.ID, []uuid.UUID{lowest})
		}
	}

	finishedAt := time.Now()
	if len(errorsList) > 0 {
		result.Errors = errorsList
	}

	s.db.WithContext(ctx).Model(log).Updates(map[string]interface{}{
		"status":        "completed",
		"posts_count":   result.PostsCount,
		"tax_count":     result.TaxCount,
		"skipped_count": result.SkippedCount,
		"errors":        strings.Join(errorsList, "; "),
		"finished_at":   finishedAt,
	})

	log.Status = "completed"
	log.PostsCount = result.PostsCount
	log.TaxCount = result.TaxCount
	log.SkippedCount = result.SkippedCount
	log.Errors = strings.Join(errorsList, "; ")
	log.FinishedAt = &finishedAt

	return log, nil
}

func extractMdTitle(md string) string {
	lines := strings.SplitN(md, "\n", 2)
	firstLine := strings.TrimSpace(lines[0])
	trimmed := strings.TrimPrefix(firstLine, "# ")
	if trimmed != firstLine {
		return strings.TrimSpace(trimmed)
	}
	trimmed = strings.TrimPrefix(firstLine, "#")
	if trimmed != firstLine {
		return strings.TrimSpace(trimmed)
	}
	return ""
}

func extractMdBody(md string) string {
	lines := strings.SplitN(md, "\n", 2)
	if len(lines) < 2 {
		return ""
	}
	return strings.TrimSpace(lines[1])
}

func parseMdSlug(zfName string, title string) string {
	dir := filepath.Dir(zfName)
	baseName := strings.TrimSuffix(filepath.Base(zfName), ".md")
	slugParts := []string{}
	if dir != "." && dir != "" {
		for _, part := range strings.Split(dir, "/") {
			p := helpers.Slugify(part)
			if p != "" {
				slugParts = append(slugParts, p)
			}
		}
	}
	slugParts = append(slugParts, helpers.Slugify(baseName))
	slug := strings.Join(slugParts, "-")
	if slug == "" {
		slug = helpers.Slugify(title)
	}
	return slug
}

func (s *importerService) PreviewMarkdown(ctx context.Context, workspaceID uuid.UUID, file multipart.File) ([]MdFilePreview, error) {
	fileData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip file: %w", err)
	}

	var previews []MdFilePreview
	for _, zf := range zipReader.File {
		if zf.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(zf.Name), ".md") {
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			continue
		}
		content, _ := io.ReadAll(rc)
		rc.Close()

		body := strings.TrimSpace(string(content))
		if body == "" {
			continue
		}

		title := extractMdTitle(body)
		if title == "" {
			continue
		}

		slug := parseMdSlug(zf.Name, title)
		dir := filepath.Dir(zf.Name)
		catName := ""
		if dir != "." && dir != "" {
			catName = strings.ReplaceAll(dir, "/", " / ")
		}

		exists, _ := s.postRepo.FindBySlug(ctx, workspaceID, slug)
		already := exists != nil
		status := "new"
		if already {
			status = "duplicate"
		}

		previews = append(previews, MdFilePreview{
			Path:          zf.Name,
			Title:         title,
			Slug:          slug,
			Category:      catName,
			AlreadyExists: already,
			Status:        status,
		})
	}

	return previews, nil
}


