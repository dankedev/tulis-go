# Panduan Integrasi Tulis CMS Public API — Taxonomies (Categories & Tags)

Taxonomy digunakan untuk mengelompokkan dan mengklasifikasikan postingan Anda. Tulis CMS mendukung dua kategori bawaan: **Category** (bertingkat/hierarkis) dan **Tag** (flat/non-hierarkis).

---

## 📌 Endpoint Referensi

### Menampilkan Daftar Taksonomi (`GET /v1/taxonomies`)
Mengembalikan seluruh taksonomi yang terdaftar dan aktif di workspace Anda.

#### Parameter Query:
| Nama | Tipe | Wajib | Deskripsi |
| :--- | :--- | :--- | :--- |
| `type` | String | Tidak | Filter tipe taksonomi: `category` atau `tag`. Jika dikosongkan, kedua tipe taksonomi akan dikembalikan. |

---

## 🏗️ Struktur Skema Data JSON (Taxonomy Object)

Format objek taxonomy yang dikembalikan:

```json
{
  "id": "1fa7c1e3-15c7-4ab2-81e5-ef68930ef82e",
  "name": "Pemrograman Go",
  "slug": "pemrograman-go",
  "type": "category",
  "parent_id": "0aa6c1e3-85c7-4ab2-81e5-ef68930ef82e" // UUID kategori induk jika merupakan sub-kategori
}
```

*   `type` bernilai `category` atau `tag`.
*   `parent_id` akan bernilai `null` jika kategori berada di tingkat teratas (root) atau bertipe `tag`.

---

## 🎯 Hubungan Banyak-ke-Banyak (Post & Taxonomies)
Ketika mengambil tulisan melalui `GET /v1/posts`, setiap post objek menyertakan field array `taxonomies` yang berisi kategori dan tag yang terkait dengan postingan tersebut.

Anda juga bisa melakukan pencarian balik dengan menyaring postingan berdasarkan slug taksonomi tertentu menggunakan query parameter `taxonomy` di endpoint `GET /v1/posts`.

---

## 🛠️ Contoh Integrasi Kode

### 1. Mengambil Daftar Kategori Saja (cURL)
```bash
curl -X GET "http://localhost:8080/v1/taxonomies?type=category" \
  -H "X-Workspace-ID: 8fa7c223-95c7-4ab2-81e5-ef68930ef82e" \
  -H "Accept: application/json"
```

### 2. Membangun Struktur Navigasi Menu Kategori Bertingkat di Javascript
Jika Anda menggunakan kategori hierarkis, Anda dapat menyusunnya menjadi tree structure di client side:

```javascript
async function fetchCategoryTree() {
  const response = await fetch('http://localhost:8080/v1/taxonomies?type=category', {
    headers: { 'X-Workspace-ID': '8fa7c223-95c7-4ab2-81e5-ef68930ef82e' }
  });
  const result = await response.json();
  const categories = result.data;

  // Membuat map untuk mempermudah pencarian parent
  const categoryMap = {};
  categories.forEach(cat => {
    categoryMap[cat.id] = { ...cat, children: [] };
  });

  const roots = [];
  categories.forEach(cat => {
    if (cat.parent_id) {
      if (categoryMap[cat.parent_id]) {
        categoryMap[cat.parent_id].children.push(categoryMap[cat.id]);
      }
    } else {
      roots.push(categoryMap[cat.id]);
    }
  });

  console.log('Hierarchical Categories:', roots);
  return roots;
}

fetchCategoryTree();
```
