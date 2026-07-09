# Konfigurasi Server & Environment Variables

Halaman ini berisi spesifikasi kebutuhan server, penjelasan detail variabel lingkungan (`.env`), konfigurasi database MySQL, dan penyimpanan eksternal menggunakan Cloudflare R2.

---

## 🖥️ Spesifikasi Kebutuhan Server (Specifications)

Berkat efisiensi bahasa Go, Tulis CMS dapat berjalan dengan performa tinggi pada spesifikasi server minimal.

### Spesifikasi Minimum (Cocok untuk skala kecil/personal blog):
*   **CPU:** 1 vCPU
*   **RAM:** 512 MB (1 GB direkomendasikan jika menjalankan database di VPS yang sama)
*   **Storage:** 10 GB SSD/NVMe
*   **OS:** Ubuntu 20.04+, Debian 10+, CentOS 8+, atau Docker Environment.

### Spesifikasi Rekomendasi (Skala Enterprise / Ratusan Tenant):
*   **CPU:** 2 vCPU atau lebih
*   **RAM:** 2 GB - 4 GB
*   **Storage:** 50 GB SSD (disarankan menggunakan cloud storage terpisah seperti R2/S3 untuk aset media)

---

## ⚙️ Variabel Lingkungan (Environment Variables)

Salin file `.env.example` menjadi `.env` di folder root backend dan sesuaikan nilainya:

```env
# ==============================================================================
# APLIKASI UTAMA (APP CONFIG)
# ==============================================================================
APP_ENV=development         # production, development, test
APP_PORT=8080               # Port tempat Go Fiber mendengarkan request HTTP
API_HOST=api.tulis.org      # Domain utama API publik (diperlukan untuk Subdomain Guard)
CORS_ORIGINS=http://localhost:3000,https://admin.tulis.org  # Domain admin frontend yang diperbolehkan CORS

# JWT CONFIGURATION
JWT_SECRET=super-secret-key-tulis-cms-api-change-me # Kunci rahasia signing token JWT
JWT_EXPIRY_HOURS=24                                  # Masa aktif token JWT (jam)

# REGISTRATION CONTROLS
ALLOW_REGISTRATION=true     # Set ke false untuk menonaktifkan registrasi user umum baru
WORKSPACE_RESTRICTED=false  # Set ke true agar registrasi baru harus meminta persetujuan admin untuk workspace

# ==============================================================================
# DATABASE (MYSQL CONFIG)
# ==============================================================================
DB_HOST=127.0.0.1           # Alamat server database
DB_PORT=3306                # Port database
DB_USER=root                # Username database
DB_PASSWORD=secretpassword  # Password database
DB_NAME=tulis_db            # Nama database yang akan digunakan

# ==============================================================================
# SMTP (EMAIL CONFIG)
# ==============================================================================
SMTP_HOST=smtp.mailtrap.io  # Server SMTP hosting (eg. Mailgun, Sendgrid, Mailtrap)
SMTP_PORT=587               # Port SMTP (biasanya 587 atau 465)
SMTP_USERNAME=my-smtp-user  # Username SMTP
SMTP_PASSWORD=my-smtp-pass  # Password SMTP
SMTP_FROM_EMAIL=no-reply@tulis.org # Email pengirim default
SMTP_FROM_NAME="Tulis CMS"  # Nama pengirim default

# ==============================================================================
# CLOUDFLARE R2 (S3-COMPATIBLE STORAGE)
# ==============================================================================
R2_ACCOUNT_ID=              # Account ID Cloudflare (dari dashboard Cloudflare R2)
R2_ACCESS_KEY_ID=           # Access Key dari Cloudflare R2 API token
R2_SECRET_ACCESS_KEY=       # Secret Access Key dari Cloudflare R2 API token
R2_BUCKET_NAME=             # Nama bucket penyimpanan R2 Anda
R2_PUBLIC_URL=              # URL publik bucket R2 Anda (misal https://pub-xxx.r2.dev)
```

---

## 🗄️ Konfigurasi Database (MySQL Setup)

Secara default, GORM di Tulis CMS akan mencoba melakukan **Auto-Migrate** skema tabel database secara otomatis saat aplikasi pertama kali dijalankan.

### Langkah Inisialisasi Database:
1.  Buat database kosong baru pada MySQL server:
    ```sql
    CREATE DATABASE tulis_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
    ```
2.  Pastikan user database Anda memiliki hak akses penuh (`ALL PRIVILEGES`) ke database tersebut.
3.  Konfigurasikan detail database Anda di `.env`. Saat Tulis CMS pertama kali dijalankan, sistem akan membuat 12 tabel dasar (termasuk `users`, `workspaces`, `posts`, dsb.) secara otomatis.

---

## ☁️ Integrasi Cloudflare R2 (Storage Setup)

Tulis CMS mendukung penyimpanan media terdistribusi menggunakan **Cloudflare R2** dengan protokol S3-compatible.

### Cara Konfigurasi Cloudflare R2:
1.  **Buat Bucket Baru:** Masuk ke dashboard Cloudflare -> R2 -> *Create Bucket*. Beri nama bucket Anda (misal `tulis-assets`).
2.  **Buat API Token:** Di tab R2, klik *Manage R2 API Tokens* -> *Create API Token*.
    *   Beri izin akses: **Read & Write**.
    *   Salin **Access Key ID** dan **Secret Access Key**.
3.  **Dapatkan Account ID:** ID Akun Anda terletak di URL dashboard Cloudflare R2 atau di halaman utama R2 API Tokens.
4.  **Konfigurasi Public Domain:** Di pengaturan bucket R2, aktifkan *Custom Domain* atau *R2.dev Subdomain* agar aset dapat diakses oleh publik. Salin domain tersebut.
5.  **Pasang di `.env`:** Masukkan nilai-nilai tersebut ke variabel R2 di file `.env`.

Jika variabel R2 dikosongkan, Tulis CMS akan secara otomatis menggunakan fallback **Local Storage** dan menyimpan media di dalam direktori `backend/uploads`.
