package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type Service interface {
	ListPlugins(ctx context.Context, workspaceID uuid.UUID) ([]PluginManifest, error)
	TogglePlugin(ctx context.Context, workspaceID uuid.UUID, pluginID string, enabled bool) error
	SaveSettings(ctx context.Context, workspaceID uuid.UUID, pluginID string, settings map[string]interface{}) error
	TriggerHook(ctx context.Context, workspaceID uuid.UUID, hookName string, payload interface{}) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListPlugins(ctx context.Context, workspaceID uuid.UUID) ([]PluginManifest, error) {
	wps, err := s.repo.GetWorkspacePlugins(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	wpMap := make(map[string]WorkspacePlugin)
	for _, wp := range wps {
		wpMap[wp.PluginID] = wp
	}

	result := make([]PluginManifest, len(AvailablePlugins))
	for i, ap := range AvailablePlugins {
		manifest := ap
		if wp, exists := wpMap[ap.ID]; exists {
			manifest.Enabled = wp.Enabled
			if wp.Settings != "" {
				var settings map[string]interface{}
				if err := json.Unmarshal([]byte(wp.Settings), &settings); err == nil {
					manifest.Settings = settings
				}
			}
		}
		result[i] = manifest
	}

	return result, nil
}

func (s *service) TogglePlugin(ctx context.Context, workspaceID uuid.UUID, pluginID string, enabled bool) error {
	wp, err := s.repo.GetWorkspacePlugin(ctx, workspaceID, pluginID)
	if err != nil {
		return err
	}

	if wp == nil {
		wp = &WorkspacePlugin{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			PluginID:    pluginID,
			Enabled:     enabled,
			Settings:    "{}",
		}
	} else {
		wp.Enabled = enabled
	}

	return s.repo.SaveWorkspacePlugin(ctx, wp)
}

func (s *service) SaveSettings(ctx context.Context, workspaceID uuid.UUID, pluginID string, settings map[string]interface{}) error {
	wp, err := s.repo.GetWorkspacePlugin(ctx, workspaceID, pluginID)
	if err != nil {
		return err
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	if wp == nil {
		wp = &WorkspacePlugin{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			PluginID:    pluginID,
			Enabled:     true,
			Settings:    string(settingsJSON),
		}
	} else {
		wp.Settings = string(settingsJSON)
	}

	return s.repo.SaveWorkspacePlugin(ctx, wp)
}

func (s *service) TriggerHook(ctx context.Context, workspaceID uuid.UUID, hookName string, payload interface{}) error {
	plugins, err := s.ListPlugins(ctx, workspaceID)
	if err != nil {
		return err
	}

	for _, p := range plugins {
		if !p.Enabled {
			continue
		}

		// Handle specific plugin actions based on Hook Name
		switch p.ID {
		case "slack-webhooks":
			if hookName == "after_create_post" || hookName == "after_update_post" {
				webhookURL, ok := p.Settings["webhook_url"].(string)
				if ok && webhookURL != "" {
					go func(url string, name string) {
						payloadStr := fmt.Sprintf(`{"text": "Post event triggered: %s. Workspace: %s. Payload: %+v"}`, hookName, workspaceID, payload)
						req, err := http.NewRequest("POST", url, strings.NewReader(payloadStr))
						if err == nil {
							req.Header.Set("Content-Type", "application/json")
							resp, err := http.DefaultClient.Do(req)
							if err == nil {
								resp.Body.Close()
							}
						}
					}(webhookURL, p.Name)
				}
			}
		case "seo-analyzer":
			if hookName == "before_create_post" || hookName == "before_update_post" {
				if seoPost, ok := payload.(SeoPost); ok {
					minWordCount := 300
					if p.Settings != nil {
						if mwcVal, exists := p.Settings["min_word_count"]; exists {
							switch v := mwcVal.(type) {
							case float64:
								minWordCount = int(v)
							case int:
								minWordCount = v
							}
						}
					}
					score := CalculateSeoScore(seoPost, minWordCount)
					seoPost.SetSeoScore(score)
					log.Printf("[SEO Plugin] Calculated SEO score for post: %d", score)
				}
			}
		}
	}

	return nil
}

type SeoPost interface {
	GetTitle() string
	GetContent() string
	GetSlug() string
	GetFocusKeyword() string
	GetSeoDesc() string
	GetOgpImage() string
	SetSeoScore(score int)
}

func CalculateSeoScore(post SeoPost, minWordCount int) int {
	score := 0
	content := post.GetContent()
	title := post.GetTitle()
	slug := post.GetSlug()
	focusKeyword := post.GetFocusKeyword()
	seoDesc := post.GetSeoDesc()
	ogpImage := post.GetOgpImage()

	wordCount := len(strings.Fields(content))

	// 1. Word count rule (up to 30 points)
	if wordCount >= minWordCount {
		score += 30
	} else if wordCount > 0 && minWordCount > 0 {
		score += (wordCount * 30) / minWordCount
	}

	// 2. Focus keyword rules (up to 50 points)
	if focusKeyword != "" {
		lowerKeyword := strings.ToLower(focusKeyword)
		if strings.Contains(strings.ToLower(title), lowerKeyword) {
			score += 20
		}
		if strings.Contains(strings.ToLower(slug), lowerKeyword) {
			score += 15
		}
		if strings.Contains(strings.ToLower(content), lowerKeyword) {
			score += 15
		}
	}

	// 3. Meta completeness rules (up to 20 points)
	if seoDesc != "" {
		score += 10
	}
	if ogpImage != "" {
		score += 10
	}

	return score
}
