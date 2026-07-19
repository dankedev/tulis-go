package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo       *Repository
	httpClient *http.Client
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *Service) Create(ctx context.Context, workspaceID uuid.UUID, req CreateWebhookReq) (*Webhook, error) {
	if req.URL == "" || req.Events == "" {
		return nil, fmt.Errorf("url and events are required")
	}
	w := &Webhook{
		WorkspaceID: workspaceID,
		URL:         req.URL,
		Events:      req.Events,
		Secret:      req.Secret,
		IsActive:    true,
	}
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Service) List(ctx context.Context, workspaceID uuid.UUID) ([]Webhook, error) {
	return s.repo.ListByWorkspace(ctx, workspaceID)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// Dispatch sends webhook payloads to all active subscribers for an event.
// Runs asynchronously — doesn't block the caller.
func (s *Service) Dispatch(ctx context.Context, workspaceID uuid.UUID, event string, payload any) {
	hooks, err := s.repo.ListActiveByEvent(ctx, workspaceID, event)
	if err != nil || len(hooks) == 0 {
		return
	}

	payloadBytes, _ := json.Marshal(payload)

	for _, hook := range hooks {
		go s.deliver(hook, event, payloadBytes)
	}
}

func (s *Service) deliver(hook Webhook, event string, payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", hook.URL, bytes.NewReader(payload))
	if err != nil {
		s.logDelivery(hook.ID, event, 0, "failed", err.Error(), payload)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tulis-Event", event)
	req.Header.Set("User-Agent", "TulisCMS-Webhook/1.0")

	// HMAC signature if secret is set
	if hook.Secret != "" {
		mac := hmac.New(sha256.New, []byte(hook.Secret))
		mac.Write(payload)
		req.Header.Set("X-Tulis-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logDelivery(hook.ID, event, 0, "failed", err.Error(), payload)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	status := "success"
	if resp.StatusCode >= 400 {
		status = "failed"
	}
	s.logDelivery(hook.ID, event, resp.StatusCode, status, string(body), payload)
}

func (s *Service) logDelivery(webhookID uuid.UUID, event string, statusCode int, status, response string, payload []byte) {
	log := &DeliveryLog{
		WebhookID:  webhookID,
		Event:      event,
		Status:     status,
		StatusCode: statusCode,
		Response:   response,
		Payload:    string(payload),
	}
	_ = s.repo.LogDelivery(context.Background(), log)
}

func (s *Service) GetDeliveryLogs(ctx context.Context, webhookID uuid.UUID, limit int) ([]DeliveryLog, error) {
	if limit < 1 || limit > 50 {
		limit = 20
	}
	return s.repo.ListDeliveryLogs(ctx, webhookID, limit)
}
