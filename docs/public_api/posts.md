# Panduan Integrasi Tulis CMS Public API — Posts & Pages

Modul ini menjelaskan bagaimana cara mengambil data artikel (post), halaman statis (page), maupun tipe konten kustom (Custom Post Types) dari Tulis CMS.

---

## 📌 Endpoint Referensi

### 1. Menampilkan Daftar Tulisan (`GET /v1/posts`)
Mengambil daftar seluruh konten terpublikasi (*published*) di dalam workspace Anda.

#### Parameter Query:
| Nama | Tipe | Default | Deskripsi |
| :--- | :--- | :--- | :--- |
| `type` | String | `post` | Jenis tipe tulisan (misalnya: `post`, `page`, atau slug tipe kustom seperti `portfolio`, `book`). |
| `taxonomy` | String | - | Filter tulisan yang diasosiasikan dengan slug kategori atau tag tertentu (misal: `programming`). |
| `sort` | String | `published_at desc` | Pengurutan data. Contoh: `published_at desc`, `title asc`, `created_at asc`. |
| `page` | Integer | `1` | Halaman saat ini untuk kebutuhan pagination. |
| `per_page` | Integer | `10` | Jumlah data per halaman. |

---

### 2. Mengambil Detail Tulisan (`GET /v1/posts/:slugOrId`)
Mengambil detail lengkap satu postingan berdasarkan slug unik atau UUID postingan.

*   **Format Path:** `/v1/posts/:slugOrId`
*   **Contoh Path:** `/v1/posts/cara-belajar-golang-cepat` atau `/v1/posts/8fa7c223-95c7-4ab2-81e5-ef68930ef82e`

---

## 🏗️ Struktur Skema Data JSON (Post Object)

Setiap objek tulisan yang dikembalikan memiliki struktur berikut:

```json
{
  "id": "7ca6c1e3-85c7-4ab2-81e5-ef68930ef82e",
  "title": "Memulai Headless CMS",
  "slug": "memulai-headless-cms",
  "content": "<p>Isi tulisan HTML panjang di sini...</p>",
  "excerpt": "Panduan singkat memulai Tulis CMS...",
  "status": "published",
  "post_type": "post",
  "published_at": "2026-07-09T23:00:00+07:00",
  "feature_image": "http://localhost:8080/uploads/2026/07/banner.jpg",
  "author_id": "3ba7c1e3-25c7-4ab2-81e5-ef68930ef82e",
  "custom_fields": {
    "reading_time_minutes": 5,
    "sponsor_name": "DeepMind"
  },
  "taxonomies": [
    {
      "id": "1fa7c1e3-15c7-4ab2-81e5-ef68930ef82e",
      "name": "Teknologi",
      "slug": "teknologi",
      "type": "category"
    }
  ]
}
```

---

## 🛠️ Contoh Integrasi Kode

### 1. Mengambil Posts dengan Custom Post Type (cURL)
Mendapatkan 3 tulisan dengan tipe portofolio yang diurutkan dari yang terbaru:

```bash
curl -X GET "http://localhost:8080/v1/posts?type=portfolio&per_page=3&sort=published_at desc" \
  -H "X-Workspace-ID: 8fa7c223-95c7-4ab2-81e5-ef68930ef82e" \
  -H "Accept: application/json"
```

### 2. Integrasi React / Next.js dengan Server Component
```tsx
// app/blog/[slug]/page.tsx
import { notFound } from 'next/navigation';

interface PostDetail {
  title: string;
  content: string;
  published_at: string;
  feature_image: string;
}

async function getPostDetail(slug: string): Promise<PostDetail | null> {
  const res = await fetch(`http://localhost:8080/v1/posts/${slug}`, {
    headers: {
      'X-Workspace-ID': process.env.TULIS_WORKSPACE_ID || '',
    },
    next: { revalidate: 60 } // revalidate cache every 1 minute
  });

  if (res.status === 404) return null;
  const json = await res.json();
  return json.data;
}

export default async function BlogPostPage({ params }: { params: { slug: string } }) {
  const post = await getPostDetail(params.slug);

  if (!post) {
    notFound();
  }

  return (
    <article className="max-w-3xl mx-auto py-12 px-6">
      {post.feature_image && (
        <img src={post.feature_image} alt={post.title} className="w-full h-80 object-cover rounded-xl mb-8" />
      )}
      <h1 className="text-4xl font-bold">{post.title}</h1>
      <time className="text-sm text-gray-400 block mt-2 mb-6">
        {new Date(post.published_at).toLocaleDateString('id-ID', {
          year: 'numeric',
          month: 'long',
          day: 'numeric'
        })}
      </time>
      <div className="prose lg:prose-xl" dangerouslySetInnerHTML={{ __html: post.content }} />
    </article>
  );
}
```
