# Panduan Integrasi Tulis CMS Public API — Overview

Selamat datang di Panduan Integrasi Tulis CMS Public API. API ini dirancang untuk memfasilitasi konsumsi data secara *headless* dengan performa tinggi, aman, dan modular.

---

## 🌐 Konsep Utama (Core Concepts)

Public API Tulis CMS beroperasi di bawah prefix path `/v1` (versi 1) dan bersifat read-only. Seluruh data disajikan dalam format JSON standar.

### 1. Multi-Tenancy & Identifikasi Workspace
Tulis CMS mendukung arsitektur multi-tenant. Setiap workspace memiliki kontainer datanya sendiri (seperti WordPress Multi-Site). Sebelum melakukan pemanggilan ke API, Anda wajib menentukan workspace mana yang ingin diakses.

#### Metode A: Menggunakan Header `X-Workspace-ID` (Direkomendasikan)
Header ini menerima UUID unik dari Workspace Anda. Sangat cocok digunakan untuk static site generator atau server-side rendering (SSR) di mana ID dapat disimpan secara aman di environment variables.

*   **Header Name:** `X-Workspace-ID`
*   **Value Format:** `UUIDv4` (Contoh: `8fa7c223-95c7-4ab2-81e5-ef68930ef82e`)

#### Metode B: Menggunakan Subdomain Scoping
Jika Tulis CMS di-deploy dengan dukungan subdomain dinamis (misalnya di domain utama `tulis.org` atau localhost dengan port mapping), Anda dapat mengakses API secara langsung menggunakan subdomain slug workspace.

*   **Format URL:** `http://[workspace-slug].[main-domain]:[port]/v1/[endpoints]`
*   **Contoh:** `http://techblog.localhost:8080/v1/posts`

---

## ⏱️ Keamanan & Batasan Akses (Rate Limiting)

*   **Tanpa Token (Unauthenticated):** Endpoint Public API (`/v1/...`) tidak memerlukan token autentikasi JWT (`Authorization: Bearer ...`). Anda bebas memanggilnya dari browser (client-side) maupun server-side.
*   **CORS Enabled:** Cross-Origin Resource Sharing (CORS) telah dikonfigurasi secara terbuka untuk header yang dibutuhkan seperti `X-Workspace-ID` dan `Authorization`.
*   **Rate Limiting:** Untuk mencegah penyalahgunaan dan serangan brute force, setiap alamat IP dibatasi maksimal **60 request per 1 menit**. Jika limit terlampaui, API akan mengembalikan status code `429 Too Many Requests`.

---

## ⚠️ Format Respon Error (Error Handling)

Setiap request yang gagal akan mengembalikan objek JSON standar dengan struktur berikut:

```json
{
  "status": 400,
  "message": "Workspace context required",
  "errors": null
}
```

### Daftar Kode Status HTTP yang Umum:

| Status Code | Nama | Deskripsi | Solusi |
| :--- | :--- | :--- | :--- |
| `400` | Bad Request | Parameter query tidak valid atau format header salah. | Periksa kecocokan UUID pada `X-Workspace-ID` atau query params. |
| `404` | Not Found | Resource tidak ditemukan (misalnya slug postingan tidak ada atau belum di-publish). | Pastikan status postingan sudah diset ke `published`. |
| `429` | Too Many Requests | Melebihi batas kuota pemanggilan API dalam 1 menit. | Implementasikan caching di sisi client/frontend untuk mengurangi request ke backend. |
| `500` | Internal Server Error | Kesalahan pada server database atau internal engine. | Hubungi sistem administrator atau periksa logs backend. |
