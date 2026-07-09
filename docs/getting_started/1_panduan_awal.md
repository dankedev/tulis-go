# Panduan Awal — Mengenal Tulis CMS (Backend Go)

Tulis CMS adalah platform Headless Content Management System (CMS) open-source performa tinggi yang dirancang menggunakan bahasa pemrograman **Go (Fiber & GORM)**. Repository open-source Tulis CMS dapat diakses secara publik melalui [github.com/dankedev/tulis-go](https://github.com/dankedev/tulis-go).

Tulis CMS menawarkan kapabilitas multi-tenancy bawaan yang memungkinkan pengelolaan banyak situs/workspace dalam satu instalasi backend tunggal.

---

## 🎨 Filosofi Tulis CMS

Tulis CMS dibangun di atas empat pilar filosofis utama:

1.  **Multi-Tenancy First (Bawaan):**
    Sama seperti WordPress Multi-Site (MU), Tulis CMS memperlakukan ruang kerja (*workspace*) sebagai warga kelas satu. Anda dapat membuat ratusan situs independen di bawah subdomain yang berbeda menggunakan satu server database dan satu aplikasi backend.
2.  **Performa Maksimal:**
    Menggunakan Go dan web framework Fiber yang sangat cepat dan hemat resource. Memori footprint sangat kecil jika dibandingkan dengan CMS tradisional berbasis monolitik.
3.  **Headless & Konten Terpisah:**
    Tulis CMS hanya fokus pada penyediaan data konten terstruktur (melalui JSON API). Layout dan visualisasi website sepenuhnya didelegasikan ke frontend modern pilihan Anda (Next.js, Nuxt, Svelte, Mobile App).
4.  **Extensibility (Kemudahan Ekstensi):**
    Mendukung pendaftaran Custom Post Types (CPT) secara dinamis langsung dari UI/API beserta sistem plugin/hooks berbasis event untuk memodifikasi siklus hidup konten.

---

## 🚀 Quick Start (Memulai dengan Cepat)

Ikuti langkah-langkah di bawah ini untuk mengunduh dan menjalankan server backend Tulis CMS di mesin lokal Anda dalam mode development.

### 📋 Prasyarat (Prerequisites)
Pastikan sistem Anda sudah terinstall:
*   Go (versi 1.21 atau lebih tinggi)
*   Air (opsional, untuk live-reload otomatis saat kode berubah)
*   MySQL 8.0+

### 🛠️ Langkah Inisialisasi Backend
1.  **Clone Repository:**
    Clone kode sumber resmi dari GitHub:
    ```bash
    git clone https://github.com/dankedev/tulis-go.git
    cd tulis-go
    ```
2.  **Salin Konfigurasi Environment:**
    Salin konfigurasi default file environment:
    ```bash
    cp .env.example .env
    ```
3.  **Konfigurasi File `.env`:**
    Buka file `.env` dan konfigurasikan akses ke server MySQL Anda (isi `DB_HOST`, `DB_USER`, `DB_PASSWORD`, dan `DB_NAME`).
4.  **Jalankan Backend Server:**
    ```bash
    # Jika menggunakan Air untuk auto-reload saat development:
    air

    # Atau jika ingin menjalankan Go compiler secara langsung:
    go run cmd/api/main.go
    ```
    Server backend Anda sekarang berhasil berjalan dan mendengarkan request HTTP di `http://localhost:8080`.

---

## 🌐 Cara Deploy ke Server Produksi

### Opsi A: Menggunakan Docker Compose (Sangat Direkomendasikan)
Tulis CMS sudah dilengkapi dengan konfigurasi Docker multi-stage yang siap pakai untuk production.

1.  Sesuaikan environment variables pada file `docker-compose.yml` di folder root project.
2.  Jalankan compose build untuk memaketkan dan menjalankan container:
    ```bash
    docker-compose up -d --build
    ```
3.  Docker akan menginstansiasi container MySQL 8.0 dan service Go API yang berjalan di belakang port mapping server Anda.

### Opsi B: Deployment Manual & Systemd (Linux/Ubuntu)
Untuk mendeploy backend Go tanpa Docker:

1.  **Build binary Go di server:**
    ```bash
    go build -ldflags="-s -w" -o tulis-api cmd/api/main.go
    ```
2.  **Konfigurasi Systemd Service:**
    Buat file service systemd `/etc/systemd/system/tulis-api.service`:
    ```ini
    [Unit]
    Description=Tulis CMS Go Backend API
    After=network.target

    [Service]
    Type=simple
    User=www-data
    WorkingDirectory=/var/www/tulis-go
    ExecStart=/var/www/tulis-go/tulis-api
    Restart=always
    RestartSec=5
    Environment=APP_ENV=production

    [Install]
    WantedBy=multi-user.target
    ```
3.  **Start & Enable Service:**
    ```bash
    systemctl daemon-reload
    systemctl start tulis-api
    systemctl enable tulis-api
    ```
4.  **Konfigurasi Reverse Proxy (Nginx/Caddy):**
    Pasang SSL (Let's Encrypt) dan arahkan domain API Anda (misal `api.tulis.org`) ke port `8080` (port internal aplikasi).
