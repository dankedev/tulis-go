package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dankedev/kontent/domain/media"
	"github.com/dankedev/kontent/domain/post"
	"github.com/dankedev/kontent/utils/helpers"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ImporterService interface {
	ImportWXR(ctx context.Context, workspaceID, authorID uuid.UUID, file multipart.File, filename string) (*ImportLog, error)
	GetImportLog(ctx context.Context, id uuid.UUID) (*ImportLog, error)
	ListImportLogs(ctx context.Context, workspaceID uuid.UUID, page, perPage int) ([]ImportLog, int64, error)
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
	urlMap     map[string]string
	taxMap     map[string]uuid.UUID
	log        *ImportLog
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

	session := &importSession{
		workspaceID: workspaceID,
		authorID:    authorID,
		result:      ImportResult{},
		errors:     []string{},
		urlMap:     make(map[string]string),
		taxMap:    make(map[string]uuid.UUID),
		log:       log,
	}

	if err := s.importTaxonomies(ctx, wxr.Channel, session); err != nil {
		session.errors = append(session.errors, fmt.Sprintf("taxonomy import error: %v", err))
	}

	if err := s.importAttachments(ctx, wxr.Channel.Items, session); err != nil {
		session.errors = append(session.errors, fmt.Sprintf("media import error: %v", err))
	}

	if err := s.importPosts(ctx, wxr.Channel.Items, session); err != nil {
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

	s.db.WithContext(ctx).Save(session.log)

	return session.log, nil
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
