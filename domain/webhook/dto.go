package webhook

import "github.com/google/uuid"

type CreateWebhookReq struct {
	URL    string `json:"url"`
	Events string `json:"events"` // "post.published post.updated post.deleted"
	Secret string `json:"secret,omitempty"`
}

type WebhookResponse struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Events    string    `json:"events"`
	IsActive  bool      `json:"is_active"`
	CreatedAt string    `json:"created_at"`
}

type DeliveryLogResponse struct {
	ID         uuid.UUID `json:"id"`
	Event      string    `json:"event"`
	Status     string    `json:"status"`
	StatusCode int       `json:"status_code"`
	CreatedAt  string    `json:"created_at"`
}

func (w *Webhook) ToResponse() WebhookResponse {
	return WebhookResponse{
		ID:        w.ID,
		URL:       w.URL,
		Events:    w.Events,
		IsActive:  w.IsActive,
		CreatedAt: w.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
