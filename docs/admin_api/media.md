# Panduan Integrasi Tulis CMS Admin API — Media Library

Modul Media Library digunakan untuk mengunggah berkas gambar, dokumen, atau aset digital lainnya secara terotentikasi, serta memperbarui metadatanya untuk keperluan optimasi SEO gambar (Alt Text).

---

## 📌 Endpoint Media

Endpoint berikut mewajibkan pengiriman header `Authorization: Bearer <token>` dan `X-Workspace-ID: <workspace_id>`.

### 1. Mengunggah Media Baru (`POST /api/media/upload`)
Endpoint ini menggunakan format data **Multipart Form Data** (`multipart/form-data`) untuk menerima file biner secara langsung.

#### Request Parameters (Form Data):
*   `file` (Binary File, Wajib): File gambar/dokumen yang akan diunggah.
*   `alt_text` (Text, Opsional): Deskripsi alternatif gambar untuk penanganan aksesibilitas screen reader dan robot SEO crawling.
*   `caption` (Text, Opsional): Teks takarir/keterangan yang muncul di bawah gambar.

---

### 2. List Berkas Media (`GET /api/media`)
Mendapatkan daftar data pustaka media yang ada di workspace terdaftar.

---

### 3. Memperbarui Metadata Media (`PUT /api/media/:id`)
Mengedit properti teks alternatif (`alt_text`) atau takarir (`caption`) media yang telah tersimpan.
*   **Request Body (JSON):**
    ```json
    {
      "alt_text": "Logo Tulis CMS Baru",
      "caption": "Dipasang pada header utama"
    }
    ```

---

### 4. Menghapus Media (`DELETE /api/media/:id`)
Menghapus rekaman media di database dan secara fisik menghapus file media dari disk penyimpanan lokal atau Cloudflare R2 bucket.

---

## 💡 Contoh Implementasi Upload File (Javascript)

Contoh kode frontend javascript untuk mengunggah file gambar ke API:

```javascript
async function uploadImage(fileObject) {
  const formData = new FormData();
  formData.append('file', fileObject);
  formData.append('alt_text', 'Ilustrasi artikel startup');
  formData.append('caption', 'Gambar pendukung bab 1');

  try {
    const response = await fetch('http://localhost:8080/api/media/upload', {
      method: 'POST',
      headers: {
        'Authorization': 'Bearer ' + localStorage.getItem('jwt_token'),
        'X-Workspace-ID': '8fa7c223-95c7-4ab2-81e5-ef68930ef82e'
        // CATATAN: Jangan definisikan header 'Content-Type' manual. 
        // Browser akan secara otomatis mengisi boundary multipart data.
      },
      body: formData
    });

    const result = await response.json();
    if (response.ok) {
      console.log('Upload Berhasil. URL File:', result.data.url);
      return result.data;
    } else {
      console.error('Gagal upload:', result.message);
    }
  } catch (error) {
    console.error('Error saat koneksi upload:', error);
  }
}
```
