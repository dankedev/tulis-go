package plugin

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, Service) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&WorkspacePlugin{})
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewRepository(db)
	svc := NewService(repo)

	return db, svc
}

func TestPluginService_ListPlugins(t *testing.T) {
	_, svc := setupTestDB(t)
	workspaceID := uuid.New()

	ctx := context.Background()
	plugins, err := svc.ListPlugins(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Failed to list plugins: %v", err)
	}

	if len(plugins) != len(AvailablePlugins) {
		t.Errorf("Expected %d plugins, got %d", len(AvailablePlugins), len(plugins))
	}

	// Default should be disabled
	for _, p := range plugins {
		if p.Enabled {
			t.Errorf("Expected plugin %s to be disabled by default", p.ID)
		}
	}
}

func TestPluginService_TogglePlugin(t *testing.T) {
	_, svc := setupTestDB(t)
	workspaceID := uuid.New()
	pluginID := "slack-webhooks"

	ctx := context.Background()
	err := svc.TogglePlugin(ctx, workspaceID, pluginID, true)
	if err != nil {
		t.Fatalf("Failed to toggle plugin: %v", err)
	}

	plugins, err := svc.ListPlugins(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Failed to list plugins: %v", err)
	}

	found := false
	for _, p := range plugins {
		if p.ID == pluginID {
			found = true
			if !p.Enabled {
				t.Errorf("Expected plugin %s to be enabled", pluginID)
			}
		}
	}

	if !found {
		t.Errorf("Plugin %s not found in list", pluginID)
	}
}

func TestPluginService_SaveSettings(t *testing.T) {
	_, svc := setupTestDB(t)
	workspaceID := uuid.New()
	pluginID := "slack-webhooks"

	ctx := context.Background()
	settings := map[string]interface{}{
		"webhook_url": "https://hooks.slack.com/services/test",
	}

	err := svc.SaveSettings(ctx, workspaceID, pluginID, settings)
	if err != nil {
		t.Fatalf("Failed to save settings: %v", err)
	}

	plugins, err := svc.ListPlugins(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Failed to list plugins: %v", err)
	}

	found := false
	for _, p := range plugins {
		if p.ID == pluginID {
			found = true
			url, ok := p.Settings["webhook_url"].(string)
			if !ok || url != "https://hooks.slack.com/services/test" {
				t.Errorf("Expected settings webhook_url to match, got %v", p.Settings["webhook_url"])
			}
		}
	}

	if !found {
		t.Errorf("Plugin %s not found in list", pluginID)
	}
}
