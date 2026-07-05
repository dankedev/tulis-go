package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
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
	ImportWXR(ctx context.Context, workspaceID, authorID uuid.UUID, file multipart.File, filename string) (*ImportResult, error)
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

func (s *importerService) ImportWXR(ctx context.Context, workspaceID, authorID uuid.UUID, file multipart.File, filename string) (*ImportResult, error) {
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

	return &session.result, nil
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

		content := s.rewriteContentURLs(item.Content, session)
		excerpt := s.rewriteContentURLs(item.Excerpt, session)

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

func (s *importerService) rewriteContentURLs(content string, session *importSession) string {
	if content == "" || len(session.urlMap) == 0 {
		return content
	}
	result := content
	for oldURL, newPath := range session.urlMap {
		result = strings.ReplaceAll(result, oldURL, newPath)
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
