package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TelegramUpdate struct {
	UpdateID int              `json:"update_id"`
	Message  *TelegramMessage `json:"message"`
}

type TelegramMessage struct {
	MessageID int           `json:"message_id"`
	From      *TelegramUser `json:"from"`
	Chat      *TelegramChat `json:"chat"`
	Text      string        `json:"text"`
	Date      int64         `json:"date"`
}

type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type TelegramChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type TelegramBotService struct {
	repo       Repository
	db         *gorm.DB
	postRepo   post.PostRepository
	wsRepo     workspace.WorkspaceRepository
	httpClient *http.Client
}

func NewTelegramBotService(repo Repository, db *gorm.DB, postRepo post.PostRepository, wsRepo workspace.WorkspaceRepository) *TelegramBotService {
	return &TelegramBotService{
		repo:       repo,
		db:         db,
		postRepo:   postRepo,
		wsRepo:     wsRepo,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SendMessage sends a message to a Telegram Chat ID using workspace bot token (or global fallback)
func (s *TelegramBotService) SendMessage(ctx context.Context, workspaceID uuid.UUID, chatID int64, text string) error {
	botToken := ""
	if workspaceID != uuid.Nil {
		cfg, err := s.repo.GetTelegramBotConfig(ctx, workspaceID)
		if err == nil && cfg.BotToken != "" {
			botToken = cfg.BotToken
		}
	}

	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured for workspace %s", workspaceID)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ProcessUpdate handles incoming webhook updates from Telegram Bot
func (s *TelegramBotService) ProcessUpdate(ctx context.Context, workspaceID uuid.UUID, update TelegramUpdate) error {
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}

	msg := update.Message
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	// Handle /start <code> pairing command
	if strings.HasPrefix(text, "/start") {
		parts := strings.Fields(text)
		if len(parts) > 1 {
			code := parts[1]
			return s.handlePairing(ctx, chatID, msg.From.Username, code, workspaceID)
		}
		return s.SendMessage(ctx, workspaceID, chatID, "⚡ <b>Tulis CMS Bot</b>\n\nTo pair your Telegram account, generate a pairing code in your Tulis CMS Dashboard Settings -> Notifications, then type:\n<code>/start &lt;PAIRING_CODE&gt;</code>")
	}

	// 1. Authenticate user from Chat ID
	binding, err := s.repo.GetTelegramBindingByChatID(ctx, chatID)
	if err != nil || binding == nil || !binding.IsVerified {
		return s.SendMessage(ctx, workspaceID, chatID, "⛔ <b>Akses Ditolak / Unlinked</b>\n\nAkun Telegram Anda belum terhubung dengan Tulis CMS. Dapatkan kode pairing dari Dashboard -> Settings -> Notifications, lalu jalankan:\n<code>/start &lt;KODE_PAIRING&gt;</code>")
	}

	// Determine active workspace context
	targetWorkspaceID := workspaceID
	if binding.ActiveWorkspaceID != nil && *binding.ActiveWorkspaceID != uuid.Nil {
		targetWorkspaceID = *binding.ActiveWorkspaceID
	}

	if targetWorkspaceID == uuid.Nil {
		return s.SendMessage(ctx, workspaceID, chatID, "❌ Tidak ada workspace aktif yang dipilih. Silakan hubungkan via Dashboard.")
	}

	// 2. Security Check: Resolve workspace role for user
	var member workspace.WorkspaceMember
	err = s.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", targetWorkspaceID, binding.UserID).First(&member).Error
	if err != nil {
		return s.SendMessage(ctx, workspaceID, chatID, "⛔ <b>Akses Ditolak</b>: Anda bukan anggota workspace ini.")
	}

	role := strings.ToLower(member.Role)

	// 3. Command Routing according to user role
	cmdParts := strings.Fields(text)
	cmd := cmdParts[0]

	switch cmd {
	case "/help":
		return s.handleHelpCommand(ctx, targetWorkspaceID, chatID, role)
	case "/posts":
		return s.handleListRecentPosts(ctx, targetWorkspaceID, chatID, binding.UserID, role, cmdParts)
	case "/list":
		return s.handleListByPeriod(ctx, targetWorkspaceID, chatID, binding.UserID, role, cmdParts)
	case "/newpost":
		return s.handleCreatePostCommand(ctx, targetWorkspaceID, chatID, binding.UserID, role, text)
	case "/update":
		return s.handleUpdatePostCommand(ctx, targetWorkspaceID, chatID, binding.UserID, role, text)
	default:
		return s.SendMessage(ctx, targetWorkspaceID, chatID, "❓ Perintah tidak dikenali. Ketik /help untuk melihat daftar perintah yang tersedia sesuai peran Anda.")
	}
}

func (s *TelegramBotService) handlePairing(ctx context.Context, chatID int64, username, code string, workspaceID uuid.UUID) error {
	binding, err := s.repo.GetTelegramBindingByCode(ctx, code)
	if err != nil || binding == nil {
		return s.SendMessage(ctx, workspaceID, chatID, "❌ Kode pairing tidak valid atau telah kadaluarsa. Silakan buat kode baru di Dashboard Tulis CMS.")
	}

	if binding.VerificationExpAt != nil && time.Now().After(*binding.VerificationExpAt) {
		return s.SendMessage(ctx, workspaceID, chatID, "⏳ Kode pairing telah kadaluarsa. Silakan buat kode baru.")
	}

	binding.TelegramChatID = chatID
	binding.TelegramUsername = username
	binding.IsVerified = true
	binding.VerificationCode = ""
	binding.VerificationExpAt = nil
	if workspaceID != uuid.Nil {
		binding.ActiveWorkspaceID = &workspaceID
	}

	if err := s.repo.SaveTelegramBinding(ctx, binding); err != nil {
		return s.SendMessage(ctx, workspaceID, chatID, "❌ Gagal mengaitkan akun Telegram.")
	}

	return s.SendMessage(ctx, workspaceID, chatID, "✅ <b>Berhasil Terhubung!</b>\n\nAkun Telegram Anda kini resmi terhubung dengan Tulis CMS. Ketik /help untuk melihat daftar perintah manajemen konten.")
}

func (s *TelegramBotService) handleHelpCommand(ctx context.Context, workspaceID uuid.UUID, chatID int64, role string) error {
	msg := fmt.Sprintf("📋 <b>Daftar Perintah Tulis CMS Bot</b>\nPeran Anda: <b>%s</b>\n\n", strings.ToUpper(role))

	if role == "subscriber" {
		msg += "<i>Sebagai Subscriber, Anda tidak memiliki izin untuk mengelola atau melihat konten backend.</i>"
		return s.SendMessage(ctx, workspaceID, chatID, msg)
	}

	msg += "• <code>/posts [limit] [post_type]</code> - Melihat post terbaru (Judul, Penulis, Kategori, Tanggal)\n"
	msg += "• <code>/list [today|week|month]</code> - Melihat daftar konten dalam rentang waktu tertentu\n"
	msg += "• <code>/newpost &lt;post_type&gt; | &lt;judul&gt; | &lt;konten&gt;</code> - Membuat post baru\n"
	if role == "author" {
		msg += "• <code>/update &lt;post_id&gt; | &lt;judul_baru&gt; | &lt;konten_baru&gt;</code> - Update konten (Hanya milik Anda)\n"
	} else {
		msg += "• <code>/update &lt;post_id&gt; | &lt;judul_baru&gt; | &lt;konten_baru&gt;</code> - Update konten workspace\n"
	}

	return s.SendMessage(ctx, workspaceID, chatID, msg)
}

func (s *TelegramBotService) handleListRecentPosts(ctx context.Context, workspaceID uuid.UUID, chatID int64, userID uuid.UUID, role string, args []string) error {
	if role == "subscriber" {
		return s.SendMessage(ctx, workspaceID, chatID, "⛔ Role Subscriber tidak memiliki akses melihat pos.")
	}

	limit := 5
	postType := "post"
	if len(args) > 1 {
		if val, err := strconv.Atoi(args[1]); err == nil && val > 0 && val <= 20 {
			limit = val
		}
	}
	if len(args) > 2 {
		postType = args[2]
	}

	posts, _, err := s.postRepo.List(ctx, workspaceID, post.PostFilter{PostType: postType}, limit, 0)
	if err != nil || len(posts) == 0 {
		return s.SendMessage(ctx, workspaceID, chatID, fmt.Sprintf("ℹ️ Tidak ada %s yang ditemukan.", postType))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📰 <b>%d Post Terbaru (%s)</b>\n\n", len(posts), postType))

	for i, p := range posts {
		// Get author name
		authorName := "Unknown"
		var authorUser struct{ Name, Email string }
		if err := s.db.WithContext(ctx).Table("users").Select("name, email").Where("id = ?", p.AuthorID).Scan(&authorUser).Error; err == nil {
			if authorUser.Name != "" {
				authorName = authorUser.Name
			} else {
				authorName = authorUser.Email
			}
		}

		// Get category
		catName := "-"
		if len(p.Taxonomies) > 0 {
			cats := []string{}
			for _, t := range p.Taxonomies {
				cats = append(cats, t.Name)
			}
			catName = strings.Join(cats, ", ")
		}

		dateStr := p.CreatedAt.Format("02 Jan 2006 15:04")
		sb.WriteString(fmt.Sprintf("%d. <b>%s</b>\n", i+1, p.Title))
		sb.WriteString(fmt.Sprintf("   🆔 <code>%s</code>\n", p.ID))
		sb.WriteString(fmt.Sprintf("   👤 Penulis: %s\n", authorName))
		sb.WriteString(fmt.Sprintf("   🏷️ Kategori: %s\n", catName))
		sb.WriteString(fmt.Sprintf("   📅 Tanggal: %s | Status: <i>%s</i>\n\n", dateStr, p.Status))
	}

	return s.SendMessage(ctx, workspaceID, chatID, sb.String())
}

func (s *TelegramBotService) handleListByPeriod(ctx context.Context, workspaceID uuid.UUID, chatID int64, userID uuid.UUID, role string, args []string) error {
	if role == "subscriber" {
		return s.SendMessage(ctx, workspaceID, chatID, "⛔ Role Subscriber tidak memiliki akses.")
	}

	period := "week"
	if len(args) > 1 {
		period = strings.ToLower(args[1])
	}

	var startTime time.Time
	now := time.Now()

	switch period {
	case "today":
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "month":
		startTime = now.AddDate(0, -1, 0)
	default: // week
		startTime = now.AddDate(0, 0, -7)
	}

	var posts []post.Post
	err := s.db.WithContext(ctx).Preload("Taxonomies").
		Where("workspace_id = ? AND created_at >= ?", workspaceID, startTime).
		Order("created_at desc").Limit(15).Find(&posts).Error

	if err != nil || len(posts) == 0 {
		return s.SendMessage(ctx, workspaceID, chatID, fmt.Sprintf("ℹ️ Tidak ada konten baru dalam periode <b>%s</b>.", period))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📅 <b>Daftar Konten (Periode: %s)</b>\n\n", period))

	for i, p := range posts {
		sb.WriteString(fmt.Sprintf("%d. <b>%s</b> (%s)\n", i+1, p.Title, p.Status))
		sb.WriteString(fmt.Sprintf("   📅 %s | ID: <code>%s</code>\n", p.CreatedAt.Format("02/01/2006"), p.ID))
	}

	return s.SendMessage(ctx, workspaceID, chatID, sb.String())
}

func (s *TelegramBotService) handleCreatePostCommand(ctx context.Context, workspaceID uuid.UUID, chatID int64, userID uuid.UUID, role string, fullText string) error {
	if role == "subscriber" {
		return s.SendMessage(ctx, workspaceID, chatID, "⛔ Role Subscriber tidak diizinkan membuat pos.")
	}

	// Format: /newpost <post_type> | <title> | <content>
	parts := strings.Split(fullText, "|")
	if len(parts) < 3 {
		return s.SendMessage(ctx, workspaceID, chatID, "💡 <b>Format Pembuatan Post:</b>\n<code>/newpost post_type | Judul Post | Isi Konten Ringkas</code>\n\nContoh:\n<code>/newpost post | Berita Terbaru | Ini adalah isi konten berita yang dibuat via Telegram bot.</code>")
	}

	firstPart := strings.Fields(parts[0])
	postType := "post"
	if len(firstPart) > 1 {
		postType = strings.TrimSpace(firstPart[1])
	}

	title := strings.TrimSpace(parts[1])
	content := strings.TrimSpace(parts[2])

	newPost := &post.Post{
		ID:          uuid.New(),
		Title:       title,
		Slug:        fmt.Sprintf("%s-%d", strings.ToLower(strings.ReplaceAll(title, " ", "-")), time.Now().Unix()),
		Content:     content,
		Status:      "draft",
		AuthorID:    userID,
		WorkspaceID: workspaceID,
		PostType:    postType,
		Language:    "id",
	}

	if err := s.postRepo.Create(ctx, newPost); err != nil {
		return s.SendMessage(ctx, workspaceID, chatID, "❌ Gagal membuat post: "+err.Error())
	}

	return s.SendMessage(ctx, workspaceID, chatID, fmt.Sprintf("✅ <b>Post Berhasil Dibuat!</b>\n\n📌 <b>%s</b> (Type: %s)\n🆔 <code>%s</code>\nStatus: <i>Draft</i>", newPost.Title, newPost.PostType, newPost.ID))
}

func (s *TelegramBotService) handleUpdatePostCommand(ctx context.Context, workspaceID uuid.UUID, chatID int64, userID uuid.UUID, role string, fullText string) error {
	if role == "subscriber" {
		return s.SendMessage(ctx, workspaceID, chatID, "⛔ Role Subscriber tidak diizinkan memperbarui pos.")
	}

	// Format: /update <post_id> | <new_title> | <new_content>
	parts := strings.Split(fullText, "|")
	if len(parts) < 3 {
		return s.SendMessage(ctx, workspaceID, chatID, "💡 <b>Format Update Post:</b>\n<code>/update post_id | Judul Baru | Konten Baru</code>")
	}

	firstPart := strings.Fields(parts[0])
	if len(firstPart) < 2 {
		return s.SendMessage(ctx, workspaceID, chatID, "❌ ID Post wajib diisi.")
	}

	postIDStr := strings.TrimSpace(firstPart[1])
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		return s.SendMessage(ctx, workspaceID, chatID, "❌ ID Post tidak valid UUID.")
	}

	existingPost, err := s.postRepo.FindByID(ctx, postID)
	if err != nil || existingPost == nil || existingPost.WorkspaceID != workspaceID {
		return s.SendMessage(ctx, workspaceID, chatID, "❌ Post tidak ditemukan di workspace ini.")
	}

	// Security check for author
	if role == "author" && existingPost.AuthorID != userID {
		return s.SendMessage(ctx, workspaceID, chatID, "⛔ <b>Akses Ditolak</b>: Role Author hanya dapat mengedit pos miliknya sendiri.")
	}

	newTitle := strings.TrimSpace(parts[1])
	newContent := strings.TrimSpace(parts[2])

	if newTitle != "" {
		existingPost.Title = newTitle
	}
	if newContent != "" {
		existingPost.Content = newContent
	}
	now := time.Now()
	existingPost.EditedAt = &now

	if err := s.postRepo.Update(ctx, existingPost); err != nil {
		return s.SendMessage(ctx, workspaceID, chatID, "❌ Gagal mengupdate post: "+err.Error())
	}

	return s.SendMessage(ctx, workspaceID, chatID, fmt.Sprintf("✅ <b>Post Berhasil Diupdate!</b>\n\n📌 <b>%s</b>\n🆔 <code>%s</code>", existingPost.Title, existingPost.ID))
}
