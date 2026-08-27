package linkchecker

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dankedev/tulis-go/config"
	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/dankedev/tulis-go/utils/mail"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/html"
	"gorm.io/gorm"
)

// Service scans published posts for dead external links and records them.
type Service interface {
	CheckWorkspace(ctx context.Context, workspaceID uuid.UUID) (checked int, broken int, err error)
	ListBrokenLinks(ctx context.Context, workspaceID uuid.UUID, onlyUnresolved bool) ([]BrokenLink, error)
	CountBrokenLinks(ctx context.Context, workspaceID uuid.UUID) (int64, error)
	MarkResolved(ctx context.Context, id uuid.UUID) error
}

type service struct {
	repo       Repository
	postRepo   post.PostRepository
	db         *gorm.DB
	httpClient *http.Client
	threshold  int
}

// NewService builds a link-checker service.
// threshold = number of broken links that triggers an admin email alert (0 = never email).
func NewService(repo Repository, postRepo post.PostRepository, db *gorm.DB, threshold int) Service {
	if threshold < 0 {
		threshold = 0
	}
	return &service{
		repo:     repo,
		postRepo: postRepo,
		db:       db,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		threshold: threshold,
	}
}

// CheckWorkspace scans every published post in the workspace for broken external links.
func (s *service) CheckWorkspace(ctx context.Context, workspaceID uuid.UUID) (int, int, error) {
	const batch = 50
	offset := 0
	checked := 0
	totalBroken := 0

	for {
		posts, _, err := s.postRepo.List(ctx, workspaceID, post.PostFilter{Status: "published"}, batch, offset)
		if err != nil {
			return checked, totalBroken, err
		}
		if len(posts) == 0 {
			break
		}

		for _, p := range posts {
			checked++
			urls := extractLinks(p.Content)
			for _, u := range urls {
				code := s.checkURL(u)
				// 0 = unreachable; <200 or >=400 = broken
				if code == 0 || code >= 400 || code < 200 {
					bl := &BrokenLink{
						ID:            uuid.New(),
						WorkspaceID:   workspaceID,
						PostID:        p.ID,
						PostTitle:     p.Title,
						URL:           u,
						StatusCode:    code,
						LastCheckedAt: time.Now(),
					}
					if err := s.repo.Upsert(ctx, bl); err != nil {
						logrus.Errorf("[LINKCHECKER] failed to store broken link %s: %v", u, err)
						continue
					}
					totalBroken++
				} else {
					// link is healthy again -> clear any prior record
					_ = s.repo.DeleteByURL(ctx, workspaceID, u)
				}
			}
		}

		offset += batch
		if len(posts) < batch {
			break
		}
	}

	// Alert admins if broken-link count exceeds threshold.
	if s.threshold > 0 {
		count, err := s.repo.CountUnresolvedByWorkspace(ctx, workspaceID)
		if err == nil && int(count) >= s.threshold {
			s.alertAdmins(workspaceID, int(count))
		}
	}

	return checked, totalBroken, nil
}

// extractLinks pulls all external http(s) href/src values from an HTML document.
func extractLinks(content string) []string {
	var links []string
	seen := map[string]bool{}
	z := html.NewTokenizer(strings.NewReader(content))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if z.Err() == io.EOF {
				break
			}
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		tn, hasAttr := z.TagName()
		tag := strings.ToLower(string(tn))
		if !hasAttr || (tag != "a" && tag != "img" && tag != "link" && tag != "script" && tag != "iframe") {
			continue
		}
		for {
			k, v, more := z.TagAttr()
			key := strings.ToLower(string(k))
			if (tag == "a" && key == "href") || ((tag == "img" || tag == "link" || tag == "script" || tag == "iframe") && key == "src") {
				url := strings.TrimSpace(string(v))
				if isValidExternalURL(url) && !seen[url] {
					seen[url] = true
					links = append(links, url)
				}
			}
			if !more {
				break
			}
		}
	}
	return links
}

func isValidExternalURL(u string) bool {
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return false
	}
	// Skip localhost by hostname (IP literals like 127.0.0.1 are still checked,
	// since an unreachable local link is genuinely broken from the server's view).
	if strings.Contains(u, "localhost") {
		return false
	}
	return true
}

// checkURL performs a HEAD request (falling back to GET) and returns the HTTP status code.
// Returns 0 if the host is unreachable / DNS fails / request errors.
func (s *service) checkURL(url string) int {
	do := func(method string) int {
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			return 0
		}
		req.Header.Set("User-Agent", "TulisCMS-LinkChecker/1.0")
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return 0
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}

	if code := do(http.MethodHead); code != 0 {
		return code
	}
	return do(http.MethodGet)
}

// alertAdmins emails workspace admins/superadmins when broken links exceed threshold.
func (s *service) alertAdmins(workspaceID uuid.UUID, count int) {
	var members []workspace.WorkspaceMember
	if err := s.db.
		Where("workspace_id = ? AND role IN ?", workspaceID, []string{"superadmin", "admin"}).
		Find(&members).Error; err != nil || len(members) == 0 {
		return
	}

	var userIDs []uuid.UUID
	for _, m := range members {
		userIDs = append(userIDs, m.UserID)
	}
	var users []struct {
		Email string
		Name  string
	}
	if err := s.db.Table("users").
		Select("email", "name").
		Where("id IN ?", userIDs).
		Find(&users).Error; err != nil {
		return
	}

	for _, u := range users {
		body := mail.GetBrokenLinkAlertEmail(u.Name, count, config.AppConfig.FrontURL)
		if err := mail.SendHTMLMail(u.Email, "Peringatan: Tautan Rusak Terdeteksi", body); err != nil {
			logrus.Errorf("[LINKCHECKER] failed to alert admin %s: %v", u.Email, err)
		}
	}
}

func (s *service) ListBrokenLinks(ctx context.Context, workspaceID uuid.UUID, onlyUnresolved bool) ([]BrokenLink, error) {
	return s.repo.ListByWorkspace(ctx, workspaceID, onlyUnresolved)
}

func (s *service) CountBrokenLinks(ctx context.Context, workspaceID uuid.UUID) (int64, error) {
	return s.repo.CountUnresolvedByWorkspace(ctx, workspaceID)
}

func (s *service) MarkResolved(ctx context.Context, id uuid.UUID) error {
	return s.repo.MarkResolved(ctx, id)
}
