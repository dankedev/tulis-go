# Panduan Integrasi Tulis CMS Public API — Media Library

Modul Media digunakan untuk menyajikan data file gambar, dokumen, atau lampiran yang diunggah ke workspace. File media dapat disimpan di local server atau di Cloudflare R2 (S3-compatible).

---

## 📌 Endpoint Referensi

### Menampilkan Daftar Media (`GET /v1/media`)
Mengambil seluruh metadata aset media yang bersifat publik di dalam workspace.

---

## 🏗️ Struktur Skema Data JSON (Media Object)

Setiap objek media memiliki format berikut:

```json
{
  "id": "5fa7c1e3-25c7-4ab2-81e5-ef68930ef82e",
  "filename": "weekly-stats.png",
  "path": "workspace-8fa7c223/media/2026/07/weekly-stats.png",
  "url": "http://localhost:8080/uploads/workspace-8fa7c223/media/2026/07/weekly-stats.png",
  "mime_type": "image/png",
  "size": 154032,
  "alt_text": "Grafik Statistik Mingguan",
  "caption": "Pertumbuhan traffic tulisan kuartal 2"
}
```

*   `url`: Alamat URL absolut lengkap untuk mengakses file. Jika Cloudflare R2 digunakan, field `url` secara otomatis mengarah ke Cloudflare public URL bucket R2 Anda.
*   `path`: Lokasi relatif file di dalam storage.

---

## 🖼️ Pengolahan Gambar & Thumbnails
Secara default, saat mengunggah gambar ke Tulis CMS:
1.  Sistem menyimpan file asli.
2.  Sistem membuat salinan thumbnail/ukuran kecil jika tipe file adalah gambar (`image/*`).
3.  Thumbnail disimpan dengan nama file berakhiran `_thumb` (misalnya `weekly-stats_thumb.png`).

Anda dapat memanggil thumbnail secara langsung di frontend untuk optimasi performa loading gambar dengan mengganti nama file pada url:

```javascript
// Contoh mengubah image URL menjadi thumbnail URL
function getThumbnailUrl(originalUrl) {
  if (!originalUrl) return '';
  const lastDotIndex = originalUrl.lastIndexOf('.');
  if (lastDotIndex === -1) return originalUrl;
  
  const pathWithoutExt = originalUrl.substring(0, lastDotIndex);
  const ext = originalUrl.substring(lastDotIndex);
  
  return `${pathWithoutExt}_thumb${ext}`;
}

const original = "http://localhost:8080/uploads/media/2026/07/stats.png";
console.log(getThumbnailUrl(original)); 
// Output: http://localhost:8080/uploads/media/2026/07/stats_thumb.png
```

---

## 🛠️ Contoh Integrasi Kode (cURL)
```bash
curl -X GET "http://localhost:8080/v1/media" \
  -H "X-Workspace-ID: 8fa7c223-95c7-4ab2-81e5-ef68930ef82e" \
  -H "Accept: application/json"
```
