# Panduan Kustomisasi & Pengembangan (Customization Guide)

Tulis CMS dirancang menggunakan **Layered Architecture** (Arsitektur Berlapis) yang bersih dan terpisah secara domain. Panduan ini menjelaskan bagaimana cara memodifikasi, menambah fitur, atau menyesuaikan kode sumber [github.com/dankedev/tulis-go](https://github.com/dankedev/tulis-go) sesuai dengan kebutuhan proyek Anda.

---

## 🏗️ Struktur Folder Proyek (Project Structure)

Sebelum melakukan perubahan, pahami layout direktori backend berikut:

```
tulis-go/
├── cmd/
│   └── api/
│       └── main.go         # Entry point aplikasi, inisialisasi GORM, Fiber, Router, & Migrasi
├── config/
│   └── config.go       # Parsing environment variables & konfigurasi database
├── domain/             # Modul Domain / Bisnis (Setiap domain memiliki folder sendiri)
│   ├── post/           # Entitas, handler, repo, service, DTO untuk modul Post
│   ├── user/           # Autentikasi, profil user, notification scheduler
│   └── workspace/      # Multi-tenant workspace & manajemen tim
├── middleware/         # Custom Fiber Middlewares (auth guard, tenant scoping, RBAC, dll.)
├── routes/             # Pendaftaran/mapping endpoints API ke HTTP handler
├── storage/            # Abstraksi media storage (Local file system & Cloudflare R2)
└── utils/              # Helper utilities (JWT, Mail template, Standard response)
```

---

## 🗄️ 1. Menambah / Memodifikasi Entitas Database (GORM)

Semua entitas didefinisikan menggunakan struct Go dengan tag struct GORM untuk pemetaan kolom.

### Langkah 1: Modifikasi atau Tambah Entity Struct
Misalkan Anda ingin menambahkan field baru `phone_number` ke dalam profil user. Buka file `domain/user/user_entity.go` dan tambahkan field:

```go
type User struct {
	gorm.Model
	Email       string `gorm:"uniqueIndex;not null;size:191"`
	Name        string `gorm:"not null"`
	Password    string `gorm:"not null"`
	Role        string `gorm:"default:'subscriber'"`
	PhoneNumber string `gorm:"size:20"` // Field baru yang ditambahkan
    // ...
}
```

### Langkah 2: Jalankan Auto-Migration
Pendaftaran migrasi dilakukan secara otomatis di `cmd/api/main.go` pada fungsi `main()`:
```go
err := config.DB.AutoMigrate(
    &user.User{},
    &workspace.Workspace{},
    // ...
)
```
GORM akan mendeteksi penambahan field `phone_number` dan secara otomatis memperbarui tabel `users` pada database MySQL Anda saat server dijalankan kembali.

---

## 🔗 2. Menambah Endpoint API Baru (Layered Architecture)

Untuk menambahkan modul fitur baru (misalkan modul **"Komentar"** atau **"Comments"**), terapkan langkah berlapis berikut:

### Langkah 1: Buat Domain Folder & Entity (`domain/comment/comment_entity.go`)
```go
package comment

import (
	"gorm.io/gorm"
	"github.com/google/uuid"
)

type Comment struct {
	gorm.Model
	PostID      uuid.UUID `gorm:"type:char(36);not null"`
	AuthorName  string    `gorm:"not null"`
	Content     string    `gorm:"type:text;not null"`
}
```

### Langkah 2: Implementasikan Repository (`domain/comment/comment_repository.go`)
Berfungsi untuk interaksi query langsung ke database menggunakan GORM.
```go
package comment

import (
	"context"
	"gorm.io/gorm"
)

type CommentRepository interface {
	Create(ctx context.Context, comment *Comment) error
}

type commentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) CommentRepository {
	return &commentRepository{db: db}
}

func (r *commentRepository) Create(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}
```

### Langkah 3: Implementasikan Service (`domain/comment/comment_service.go`)
Berisi logika bisnis utama (validasi data, sanitasi, trigger email, dll.).
```go
package comment

import "context"

type CommentService struct {
	repo CommentRepository
}

func NewCommentService(repo CommentRepository) *CommentService {
	return &CommentService{repo: repo}
}

func (s *CommentService) AddComment(ctx context.Context, comment *Comment) error {
	// Logika bisnis tambahan (misal menyaring kata kasar)
	return s.repo.Create(ctx, comment)
}
```

### Langkah 4: Implementasikan Handler (`domain/comment/comment_handler.go`)
Berfungsi untuk memproses input HTTP request (body, query, params) dan mengembalikan format standard response JSON.
```go
package comment

import (
	"github.com/gofiber/fiber/v2"
	"github.com/dankedev/tulis-go/utils/response"
)

type CommentHandler struct {
	service *CommentService
}

func NewCommentHandler(service *CommentService) *CommentHandler {
	return &CommentHandler{service: service}
}

func (h *CommentHandler) Create(c *fiber.Ctx) error {
	var req Comment
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "BAD_REQUEST", "Invalid body", nil)
	}

	if err := h.service.AddComment(c.Context(), &req); err != nil {
		return response.Error(c, "INTERNAL_SERVER_ERROR", err.Error(), nil)
	}

	return response.Success(c, req, "Comment added successfully")
}
```

### Langkah 5: Registrasi Route (`routes/comment.go`)
```go
package routes

import (
	"github.com/dankedev/tulis-go/domain/comment"
	"github.com/gofiber/fiber/v2"
)

func RegisterCommentRoutes(api fiber.Router, handler *comment.CommentHandler) {
	api.Post("/comments", handler.Create)
}
```

### Langkah 6: Hubungkan Semuanya di `cmd/api/main.go`
1.  Jalankan auto-migrate untuk entitas baru `&comment.Comment{}`.
2.  Instansiasi instance repository, service, handler, dan daftarkan routernya:
    ```go
    commentRepo := comment.NewCommentRepository(config.DB)
    commentSvc := comment.NewCommentService(commentRepo)
    commentHandler := comment.NewCommentHandler(commentSvc)

    routes.RegisterCommentRoutes(api, commentHandler)
    ```

---

## 🛡️ 3. Menambahkan Middleware Kustom (Custom Middleware)

Middleware diletakkan pada folder `/middleware`. Untuk membuat middleware baru (misalnya middleware logging geografi IP):

1.  Buat file `middleware/geo.go`.
2.  Implementasikan signature middleware Fiber:
    ```go
    package middleware

    import "github.com/gofiber/fiber/v2"

    func GeoBlocker() fiber.Handler {
        return func(c *fiber.Ctx) error {
            // Contoh validasi IP/Negara
            if c.IP() == "blacklist-ip" {
                return c.Status(fiber.StatusForbidden).SendString("Akses ditolak")
            }
            return c.Next() // Lanjutkan ke handler berikutnya
        }
    }
    ```
3.  Pasang middleware di grup router tertentu pada `cmd/api/main.go`:
    ```go
    v1PublicApi.Use(middleware.GeoBlocker())
    ```

---

## 🔌 4. Menggunakan & Mengembangkan Sistem Event Hooks (Plugins)

Tulis CMS memiliki sistem event hooks dasar yang didefinisikan pada modul `domain/plugin`. Anda dapat menyisipkan kode kustom sebelum/sesudah event konten tertentu terjadi (seperti saat post dibuat, diupdate, atau dihapus).

Gunakan abstraksi ini untuk mengintegrasikan webhook eksternal, auto-share ke media sosial, atau pembersihan cache di CDN CDN (Cloudflare) Anda sendiri secara otomatis.

---

## 🧪 5. Menjalankan & Menulis Unit Test

Sebelum mengirimkan Pull Request (PR) atau men-deploy kustomisasi Anda ke server, pastikan semua tes berjalan dengan sukses untuk menjamin stabilitas fungsionalitas sistem.

```bash
# Menjalankan seluruh pengujian unit
cd backend
go test ./...

# Menjalankan pengujian tertentu dengan verbositas tinggi
go test -v ./domain/post/...
```
