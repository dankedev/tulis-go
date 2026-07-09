package user

import (
	"context"
	"testing"
	"time"

	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/dankedev/tulis-go/utils/jwt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEmailVerificationAndResetPassword(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&User{}, &workspace.Workspace{})
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewUserRepository(db)
	wsRepo := &mockWorkspaceRepo{db: db}
	jwtSvc := jwt.NewJWTService("secret", 1*time.Hour)
	svc := NewUserService(repo, wsRepo, jwtSvc)

	ctx := context.Background()

	// 1. Test Registration Verification Token Generation
	u, err := svc.Register(ctx, &User{
		Name:  "Test User",
		Email: "test@example.com",
	}, "password123")

	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	if u.VerificationToken == "" {
		t.Error("Expected VerificationToken to be generated, got empty string")
	}

	if u.EmailVerifiedAt != nil {
		t.Error("Expected EmailVerifiedAt to be nil initially")
	}

	// 2. Test Verification
	err = svc.VerifyEmail(ctx, u.VerificationToken)
	if err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	// Fetch user again
	updatedUser, err := svc.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if updatedUser.EmailVerifiedAt == nil {
		t.Error("Expected EmailVerifiedAt to be populated after verification")
	}

	if updatedUser.VerificationToken != "" {
		t.Error("Expected VerificationToken to be cleared after verification")
	}

	// 3. Test Password Reset Request
	err = svc.RequestPasswordReset(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("RequestPasswordReset failed: %v", err)
	}

	// Fetch user again
	resetUser, err := svc.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if resetUser.ResetPasswordToken == "" {
		t.Error("Expected ResetPasswordToken to be generated")
	}

	if resetUser.ResetPasswordExpiresAt == nil || resetUser.ResetPasswordExpiresAt.Before(time.Now()) {
		t.Error("Expected valid ResetPasswordExpiresAt")
	}

	// 4. Test Reset Password
	err = svc.ResetPassword(ctx, resetUser.ResetPasswordToken, "newpassword123")
	if err != nil {
		t.Fatalf("ResetPassword failed: %v", err)
	}

	finalUser, err := svc.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if finalUser.ResetPasswordToken != "" {
		t.Error("Expected ResetPasswordToken to be cleared")
	}

	if finalUser.ResetPasswordExpiresAt != nil {
		t.Error("Expected ResetPasswordExpiresAt to be nil")
	}
}
