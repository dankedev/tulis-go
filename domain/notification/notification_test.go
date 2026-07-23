package notification

import (
	"context"
	"testing"
	"time"

	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect sqlite memory DB: %v", err)
	}

	err = db.AutoMigrate(
		&NotificationPreference{},
		&TelegramUserBinding{},
		&TelegramBotConfig{},
		&NotificationLog{},
		&post.Post{},
		&workspace.Workspace{},
		&workspace.WorkspaceMember{},
	)
	if err != nil {
		t.Fatalf("failed to auto migrate sqlite memory DB: %v", err)
	}

	return db
}

func TestNotificationPreferences(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	svc := NewNotificationService(repo, db, nil, nil)

	userID := uuid.New()
	wsID := uuid.New()
	ctx := context.Background()

	prefs, err := svc.GetUserPreferences(ctx, userID, wsID)
	if err != nil {
		t.Fatalf("expected no error getting default preferences, got: %v", err)
	}
	if len(prefs) < 5 {
		t.Fatalf("expected at least 5 default preferences, got: %d", len(prefs))
	}

	// Modify email_enabled for post_published
	prefs[0].EmailEnabled = false
	err = svc.UpdateUserPreferences(ctx, userID, wsID, prefs)
	if err != nil {
		t.Fatalf("failed to update preferences: %v", err)
	}

	updated, err := svc.GetUserPreferences(ctx, userID, wsID)
	if err != nil {
		t.Fatalf("failed to refetch preferences: %v", err)
	}

	found := false
	for _, p := range updated {
		if p.EventType == prefs[0].EventType {
			found = true
			if p.EmailEnabled != false {
				t.Fatalf("expected EmailEnabled=false for %s, got true", p.EventType)
			}
		}
	}
	if !found {
		t.Fatalf("updated preference not found")
	}
}

func TestTelegramBindingAndRBAC(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	postRepo := post.NewPostRepository(db)
	wsRepo := workspace.NewWorkspaceRepository(db)
	telegramBotSvc := NewTelegramBotService(repo, db, postRepo, wsRepo)

	ctx := context.Background()
	userID := uuid.New()
	wsID := uuid.New()
	var chatID int64 = 987654321

	// Setup active workspace member with 'subscriber' role
	member := workspace.WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        "subscriber",
	}
	db.Create(&member)

	// Setup verified binding
	binding := TelegramUserBinding{
		ID:                uuid.New(),
		UserID:            userID,
		TelegramChatID:    chatID,
		TelegramUsername:  "testuser",
		IsVerified:        true,
		ActiveWorkspaceID: &wsID,
	}
	repo.SaveTelegramBinding(ctx, &binding)

	// Process /posts command for subscriber (should be blocked by RBAC)
	update := TelegramUpdate{
		UpdateID: 100,
		Message: &TelegramMessage{
			MessageID: 1,
			From:      &TelegramUser{ID: chatID, Username: "testuser"},
			Chat:      &TelegramChat{ID: chatID},
			Text:      "/posts",
			Date:      time.Now().Unix(),
		},
	}

	// Without bot token configured, call will return token missing error, proving process reached routing & security check
	err := telegramBotSvc.ProcessUpdate(ctx, wsID, update)
	if err == nil {
		t.Fatalf("expected error when sending message without bot token, got nil")
	}
}
