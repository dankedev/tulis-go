# Panduan Integrasi Tulis CMS Admin API — Content & Custom Post Types

Modul ini digunakan untuk melakukan penulisan, pemutakhiran, penghapusan konten, registrasi Custom Post Types (CPT), serta pengelolaan riwayat revisi tulisan.

---

## 📌 Endpoint Post & Page CRUD

Semua endpoint berikut wajib menyertakan header `Authorization: Bearer <token>` dan `X-Workspace-ID: <workspace_id>`.

### 1. Membuat Postingan Baru (`POST /api/posts`)
*   **Request Body:**
    ```json
    {
      "title": "Judul Baru Portofolio",
      "slug": "judul-baru-portofolio", // Opsional, jika kosong akan di-generate otomatis
      "content": "<p>Detail isi proyek portofolio...</p>",
      "excerpt": "Kutipan singkat proyek",
      "status": "draft", // draft, published, scheduled, archived
      "post_type": "portfolio", // post, page, atau custom slug
      "taxonomy_ids": ["1fa7c1e3-15c7-4ab2-81e5-ef68930ef82e"], // ID kategori/tag
      "feature_image": "http://localhost:8080/uploads/cover.jpg",
      "custom_fields": {
        "client_name": "Google DeepMind",
        "project_url": "https://deepmind.google"
      }
    }
    ```

### 2. Menampilkan Daftar Posts Admin (`GET /api/posts`)
Mengambil seluruh daftar postingan (termasuk status draft, scheduled, dan archived).
*   **Query Params:** `page`, `per_page`.

### 3. Memperbarui Postingan (`PUT /api/posts/:id`)
*   **Request Body:**
    Mengirimkan parsial fields yang ingin diperbarui saja.
    ```json
    {
      "title": "Judul Proyek Diubah",
      "status": "published"
    }
    ```

### 4. Menghapus Postingan (`DELETE /api/posts/:id`)

---

## 🏗️ Custom Post Types (CPT)

Tulis CMS mengizinkan Admin membuat skema post kustom beserta validasi form custom fields yang terstruktur.

### Mendaftarkan Tipe Posting Baru (`POST /api/post-types`)
*   **Request Body:**
    ```json
    {
      "name": "Buku",
      "slug": "book",
      "description": "Koleksi resensi buku perpustakaan",
      "fields": [
        {
          "name": "isbn",
          "label": "Nomor ISBN",
          "type": "text",
          "required": true
        },
        {
          "name": "rating",
          "label": "Rating Bintang",
          "type": "number",
          "required": false
        }
      ]
    }
    ```

---

## ⏱️ Post Revisions (Riwayat Penyuntingan)

Setiap operasi pembaruan konten (`PUT /api/posts/:id`) secara otomatis menyimpan snapshot revisi ke dalam tabel `post_revisions`.

### 1. List Revisi Konten (`GET /api/posts/:id/revisions`)
Mengembalikan daftar riwayat modifikasi beserta siapa yang mengubah dan kapan waktu pengubahannya.

### 2. Memulihkan ke Revisi Tertentu (`POST /api/posts/:id/revisions/:revisionId/restore`)
Menimpa konten aktif saat ini dengan data historis dari UUID revisi yang ditentukan.
