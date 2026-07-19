package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Provider is the AI service provider interface.
type Provider interface {
	Chat(ctx context.Context, prompt string) (string, error)
}

// Config holds AI provider configuration.
type Config struct {
	Provider string // openai, claude, or custom base URL
	APIKey   string
	Model    string
	BaseURL  string
}

// LoadConfigFromEnv reads AI configuration from environment variables.
func LoadConfigFromEnv() *Config {
	return &Config{
		Provider: getEnv("AI_PROVIDER", ""),
		APIKey:   getEnv("AI_API_KEY", ""),
		Model:    getEnv("AI_MODEL", "gpt-4o-mini"),
		BaseURL:  getEnv("AI_BASE_URL", "https://api.openai.com/v1"),
	}
}

// IsConfigured returns true if AI provider is set up.
func (c *Config) IsConfigured() bool {
	return c.APIKey != "" && c.Provider != ""
}

type openAIProvider struct {
	config *Config
	client *http.Client
}

func NewProvider(config *Config) Provider {
	if config == nil || !config.IsConfigured() {
		return nil
	}
	return &openAIProvider{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *openAIProvider) Chat(ctx context.Context, prompt string) (string, error) {
	req := chatRequest{
		Model: p.config.Model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(p.config.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result chatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if result.Error != nil {
		return "", fmt.Errorf("AI error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	return result.Choices[0].Message.Content, nil
}

// GenerateTitles generates SEO-optimized title suggestions for a post topic.
func GenerateTitles(ctx context.Context, provider Provider, topic, keywords string) ([]string, error) {
	if provider == nil {
		return nil, fmt.Errorf("AI not configured")
	}

	prompt := fmt.Sprintf(`You are an SEO expert. Generate 5 SEO-optimized blog post titles in Indonesian for the topic: "%s".

Keywords to include if relevant: %s

Requirements:
- Each title under 70 characters
- Include power words
- Return ONLY the titles, one per line, no numbering, no quotes
- Make them engaging and click-worthy`, topic, keywords)

	result, err := provider.Chat(ctx, prompt)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(result), "\n")
	titles := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "• ")
		if line != "" && len(line) < 200 {
			titles = append(titles, line)
		}
	}
	if len(titles) == 0 {
		return nil, fmt.Errorf("no valid titles generated")
	}
	return titles, nil
}

// GenerateMetaDescription generates a meta description for a post.
func GenerateMetaDescription(ctx context.Context, provider Provider, title, content string) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("AI not configured")
	}

	excerpt := content
	if len(excerpt) > 500 {
		excerpt = excerpt[:500]
	}

	prompt := fmt.Sprintf(`Write an SEO meta description for this blog post in Indonesian.
Title: %s
Content preview: %s

Requirements:
- Maximum 160 characters
- Include main keywords naturally
- Compelling and click-worthy
- Return ONLY the description, no quotes, no labels`, title, excerpt)

	result, err := provider.Chat(ctx, prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

// SuggestTaxonomies suggests categories and tags based on post content.
func SuggestTaxonomies(ctx context.Context, provider Provider, title, content string) (categories []string, tags []string, err error) {
	if provider == nil {
		return nil, nil, fmt.Errorf("AI not configured")
	}

	excerpt := content
	if len(excerpt) > 800 {
		excerpt = excerpt[:800]
	}

	prompt := fmt.Sprintf(`Analyze this blog post in Indonesian and suggest taxonomies.
Title: %s
Content preview: %s

Return ONLY valid JSON in this exact format:
{
  "categories": ["Category1", "Category2"],
  "tags": ["tag1", "tag2", "tag3"]
}

Rules:
- 2-3 categories maximum
- 3-5 tags maximum
- All in Indonesian
- Categories should be broad topics
- Tags should be specific keywords`, title, excerpt)

	result, err := provider.Chat(ctx, prompt)
	if err != nil {
		return nil, nil, err
	}

	// Extract JSON from response
	result = extractJSON(result)

	var taxonomyResp struct {
		Categories []string `json:"categories"`
		Tags       []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(result), &taxonomyResp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse AI taxonomy suggestions: %w", err)
	}

	return taxonomyResp.Categories, taxonomyResp.Tags, nil
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
