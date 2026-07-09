# Panduan Integrasi Tulis CMS Admin API — Workspaces & Members

Modul ini menjelaskan cara mengelola workspace (situs multi-tenant) Anda, mengundang anggota tim baru, mengubah peran/akses anggota, dan menghentikan keanggotaan.

---

## 🏢 Manajemen Workspace

Semua endpoint workspace memerlukan header `Authorization: Bearer <token>`.

### 1. Membuat Workspace Baru (`POST /api/workspaces`)
*   **Request Body:**
    ```json
    {
      "name": "Developer Hub",
      "slug": "dev-hub",
      "plan": "free"
    }
    ```

### 2. List Workspace Saya (`GET /api/workspaces`)
Menampilkan seluruh workspace di mana user aktif tergabung sebagai anggota.

### 3. Memperbarui Detail Workspace (`PUT /api/workspaces/:id`)
Mengubah nama workspace, slug, atau konfigurasi json settings kustom.
*   **Request Body:**
    ```json
    {
      "name": "DevHub Updated",
      "slug": "devhub-new",
      "settings": {
        "allow_registration": false,
        "site_theme": "dark"
      }
    }
    ```

---

## 👥 Manajemen Anggota (Team Members)

Endpoint berikut memerlukan header `X-Workspace-ID`.

### 1. List Anggota Workspace (`GET /api/workspaces/:id/members`)
Mengembalikan daftar seluruh user yang tergabung ke dalam workspace beserta perannya (admin, editor, author, subscriber).

### 2. Mengubah Peran Anggota (`PUT /api/workspaces/:id/members/:userId`)
*   **Request Body:**
    ```json
    {
      "role": "editor" // admin, editor, author, subscriber
    }
    ```

### 3. Menghapus Anggota (`DELETE /api/workspaces/:id/members/:userId`)
Mengeluarkan user dari keanggotaan workspace secara permanen.

---

## ✉️ Alur Undangan (Workspace Invitations)

Tulis CMS mendukung alur kolaborasi tim terpadu di mana Anda bisa mengundang pengguna yang belum memiliki akun terdaftar untuk bergabung.

### 1. Mengirim Undangan Baru (`POST /api/workspaces/:id/invitations`)
Mengirim email undangan pendaftaran ke email target. Jika email belum terdaftar di sistem, sistem akan mengarahkan user ke alur pendaftaran registrasi undangan khusus.
*   **Request Body:**
    ```json
    {
      "email": "colleague@example.com",
      "role": "author"
    }
    ```

### 2. List Undangan Pending (`GET /api/workspaces/:id/invitations`)
Menampilkan seluruh undangan terkirim yang belum dikonfirmasi atau telah kedaluwarsa.

### 3. Membatalkan/Revoke Undangan (`DELETE /api/workspaces/:id/invitations/:inviteId`)
Membatalkan token undangan aktif sehingga token tersebut tidak dapat digunakan lagi untuk bergabung.
