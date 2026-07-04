package plugin

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkspacePlugin struct {
	ID          uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	WorkspaceID uuid.UUID      `gorm:"type:char(36);not null;index:idx_ws_plugin,unique" json:"workspace_id"`
	PluginID    string         `gorm:"type:varchar(100);not null;index:idx_ws_plugin,unique" json:"plugin_id"`
	Enabled     bool           `gorm:"type:boolean;default:false" json:"enabled"`
	Settings    string         `gorm:"type:text" json:"settings"` // Serialized JSON settings
}

// PluginManifest defines the metadata and settings schema for available plugins
type PluginManifest struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Version        string                 `json:"version"`
	Enabled        bool                   `json:"enabled"`
	SettingsSchema map[string]FieldSchema `json:"settings_schema,omitempty"`
	Settings       map[string]interface{} `json:"settings,omitempty"`
}

type FieldSchema struct {
	Type     string      `json:"type"` // text, number, boolean
	Label    string      `json:"label"`
	Required bool        `json:"required"`
	Default  interface{} `json:"default"`
}

// System Available Plugins Registry
var AvailablePlugins = []PluginManifest{
	{
		ID:          "s3-storage",
		Name:        "AWS S3 GCS Storage Uploads",
		Description: "Directly offload all media uploads and assets from local filesystem to Cloud GCS or Amazon S3 buckets.",
		Version:     "1.0.4",
		SettingsSchema: map[string]FieldSchema{
			"bucket": {Type: "text", Label: "S3 Bucket Name", Required: true, Default: ""},
			"region": {Type: "text", Label: "AWS Region", Required: true, Default: "us-east-1"},
		},
	},
	{
		ID:          "slack-webhooks",
		Name:        "Slack Notifications Hook",
		Description: "Broadcast post publication updates and system revisions logs directly into your team channels.",
		Version:     "2.1.0",
		SettingsSchema: map[string]FieldSchema{
			"webhook_url": {Type: "text", Label: "Slack Webhook URL", Required: true, Default: ""},
		},
	},
	{
		ID:          "seo-analyzer",
		Name:        "SEO Engine Metrics Optimizer",
		Description: "Analyzes markdown body tags, keyword densities, and metadata values to output a premium scoring index.",
		Version:     "0.9.12",
		SettingsSchema: map[string]FieldSchema{
			"min_word_count": {Type: "number", Label: "Minimum Word Count", Required: false, Default: 300},
		},
	},
}
