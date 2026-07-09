# Panduan Integrasi Tulis CMS Admin API — Overview

Tulis CMS Admin API menyediakan antarmuka terprogram untuk mengelola seluruh aspek internal CMS Anda, seperti autentikasi pengguna, manajemen workspace, manipulasi konten (CRUD), kustomisasi post types, dan manajemen media.

---

## 🔒 Alur Autentikasi (Authentication Flow)

Berbeda dengan Public API, seluruh endpoint Admin API berada di bawah prefix `/api` dan **wajib menyertakan otorisasi**.

### Langkah 1: Mendapatkan Token JWT (Login)
Lakukan POST request ke endpoint publik `/api/login` dengan kredensial akun Anda:

*   **URL:** `POST /api/login`
*   **Request Body:**
    ```json
    {
      "email": "admin@example.com",
      "password": "mysecurepassword123"
    }
    ```
*   **Respon Sukses (200):**
    ```json
    {
      "status": 200,
      "message": "Login successful",
      "data": {
        "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
        "user": {
          "id": "3ba7c1e3-25c7-4ab2-81e5-ef68930ef82e",
          "name": "Admin User",
          "email": "admin@example.com",
          "role": "admin"
        }
      }
    }
    ```

### Langkah 2: Menggunakan Token pada Request
Untuk setiap pemanggilan Admin API selanjutnya, sertakan token JWT tersebut di dalam Header HTTP `Authorization` sebagai **Bearer Token**:

*   **Header Name:** `Authorization`
*   **Header Value:** `Bearer <TOKEN_JWT_ANDA>`

---

## 📂 Konteks Tenant (Workspace Scoping)

Sebagian besar data di Tulis CMS (seperti posts, media, taxonomies, dan plugins) berada dalam lingkup **Workspace**. Agar server mengetahui data milik workspace mana yang ingin dimodifikasi, Anda wajib mengirimkan ID Workspace aktif.

*   **Header Name:** `X-Workspace-ID`
*   **Header Value:** `UUIDv4 Workspace` (Contoh: `8fa7c223-95c7-4ab2-81e5-ef68930ef82e`)

*Catatan: Endpoint umum seperti `GET /api/me` atau `GET /api/workspaces` tidak memerlukan header `X-Workspace-ID`.*

---

## 👥 Tingkat Hak Akses (Role-Based Access Control)

Tulis CMS menerapkan RBAC (Role-Based Access Control) di tingkat Workspace. Peran anggota tim menentukan endpoint mana saja yang dapat dipanggil:

*   **Super Admin / Admin:** Memiliki kontrol penuh atas workspace, dapat mengundang anggota tim baru, mengubah setting workspace, mengaktifkan plugin, dan mengelola seluruh tipe postingan.
*   **Editor:** Dapat membuat, menyunting, dan mempublikasikan seluruh postingan (milik sendiri maupun milik penulis lain).
*   **Author:** Dapat membuat, mengedit, dan mempublikasikan postingannya sendiri.
*   **Subscriber:** Hak akses baca saja (read-only) untuk lingkup dashboard internal.
