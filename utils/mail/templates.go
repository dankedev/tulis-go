package mail

import (
	"fmt"
	"strings"

	"github.com/dankedev/tulis-go/config"
)

// getLayout wraps html content in our premium Tulis CMS branding layout.
func getLayout(title, content string) string {
	brandName := "Tulis CMS"
	layout := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="color-scheme" content="light dark">
    <meta name="supported-color-schemes" content="light dark">
    <title>{{.Title}}</title>
    <style>
        @import url('https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700&display=swap');
        body {
            font-family: 'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Arial, sans-serif;
            background-color: #f8fafc;
            color: #334155;
            margin: 0;
            padding: 0;
            -webkit-font-smoothing: antialiased;
        }
        .container {
            max-width: 600px;
            margin: 40px auto;
            background-color: #ffffff;
            border-radius: 16px;
            box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.03), 0 8px 10px -6px rgba(0, 0, 0, 0.03);
            border: 1px solid #f1f5f9;
            overflow: hidden;
        }
        .header {
            background-color: #ffffff;
            padding: 36px 32px 16px 32px;
            text-align: center;
            border-bottom: 1px solid #f1f5f9;
        }
        .header img {
            height: 72px;
            max-height: 72px;
            width: auto;
            margin: 0 auto;
            display: block;
        }
        .content {
            padding: 48px 40px;
            line-height: 1.75;
        }
        .content h2 {
            font-size: 24px;
            font-weight: 700;
            color: #0f172a;
            margin-top: 0;
            margin-bottom: 20px;
            letter-spacing: -0.025em;
        }
        .content p {
            margin-top: 0;
            margin-bottom: 24px;
            color: #1e293b;
            font-size: 16px;
        }
        .action-box {
            text-align: center;
            margin: 36px 0;
        }
        .btn {
            display: inline-block;
            background-color: #2563eb;
            color: #ffffff !important;
            text-decoration: none;
            padding: 14px 32px;
            border-radius: 10px;
            font-weight: 600;
            font-size: 16px;
            text-align: center;
            box-shadow: 0 4px 12px rgba(37, 99, 235, 0.2);
            transition: all 0.2s ease;
        }
        .btn-secondary {
            display: inline-block;
            background-color: #f1f5f9;
            color: #0f172a !important;
            text-decoration: none;
            padding: 14px 32px;
            border-radius: 10px;
            font-weight: 600;
            font-size: 16px;
            text-align: center;
            margin-left: 12px;
            transition: all 0.2s ease;
        }
        .footer {
            background-color: #f8fafc;
            padding: 32px;
            text-align: center;
            border-top: 1px solid #f1f5f9;
            font-size: 14px;
            color: #64748b;
            line-height: 1.6;
        }
        .footer p {
            margin: 0 0 10px 0;
        }
        .footer p:last-child {
            margin: 0;
        }
        .footer a {
            color: #2563eb;
            text-decoration: none;
            font-weight: 500;
        }
        .steps-box {
            background-color: #f8fafc;
            padding: 24px;
            border-radius: 12px;
            margin-bottom: 28px;
            border: 1px solid #f1f5f9;
        }
        .steps-box h3 {
            margin-top: 0;
            font-size: 13px;
            font-weight: 700;
            color: #475569;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 12px;
        }
        .steps-list {
            padding-left: 20px;
            margin: 0;
            color: #1e293b;
            font-size: 15px;
            line-height: 1.7;
        }
        .steps-list li {
            margin-bottom: 10px;
        }
        .badge {
            display: inline-block;
            padding: 6px 14px;
            border-radius: 9999px;
            font-size: 12px;
            font-weight: 600;
            background-color: #eff6ff;
            color: #1e40af;
            margin-bottom: 16px;
            border: 1px solid #bfdbfe;
        }

        @media (prefers-color-scheme: dark) {
            body {
                background-color: #090d16 !important;
                color: #cbd5e1 !important;
            }
            .container {
                background-color: #0f172a !important;
                border-color: #1e293b !important;
                box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3) !important;
            }
            .header {
                background-color: #0f172a !important;
                border-bottom-color: #1e293b !important;
            }
            .header img {
                filter: invert(1) brightness(2) !important;
            }
            .content h2 {
                color: #f8fafc !important;
            }
            .content p {
                color: #cbd5e1 !important;
            }
            .steps-box {
                background-color: #090d16 !important;
                border-color: #1e293b !important;
            }
            .steps-box h3 {
                color: #94a3b8 !important;
            }
            .steps-list {
                color: #cbd5e1 !important;
            }
            .footer {
                background-color: #0f172a !important;
                border-top-color: #1e293b !important;
                color: #64748b !important;
            }
            .btn-secondary {
                background-color: #1e293b !important;
                color: #f8fafc !important;
            }
            .badge {
                background-color: #1e293b !important;
                border-color: #3b82f6 !important;
                color: #60a5fa !important;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <img src="https://s3.tulis.org/brand/tulis-logo.png" alt="Tulis CMS" />
        </div>
        <div class="content">
            {{.Content}}
        </div>
        <div class="footer">
            <p>Dikirim dengan ❤️ oleh Tim {{.BrandName}}</p>
            <p>Jika Anda memiliki pertanyaan, hubungi kami di <a href="mailto:halo@tulis.org">halo@tulis.org</a></p>
        </div>
    </div>
</body>
</html>`

	layout = strings.ReplaceAll(layout, "{{.Title}}", title)
	layout = strings.ReplaceAll(layout, "{{.BrandName}}", brandName)
	layout = strings.ReplaceAll(layout, "{{.Content}}", content)
	return layout
}

// GetVerificationEmail returns the template for registration verification.
func GetVerificationEmail(name, link string) string {
	content := fmt.Sprintf(`
		<div class="badge">Selamat Datang</div>
		<h2>Verifikasi Alamat Email Anda</h2>
		<p>Halo %s,</p>
		<p>Terima kasih telah mendaftar di Tulis CMS! Kami sangat senang menyambut Anda. Silakan klik tombol di bawah ini untuk memverifikasi alamat email Anda dan memulai.</p>
		<div class="action-box">
			<a href="%s" class="btn" target="_blank">Verifikasi Email</a>
		</div>
		<p>Jika tombol di atas tidak berfungsi, salin dan tempel URL berikut ke peramban Anda:</p>
		<p style="word-break: break-all; font-size: 13px; color: #64748b;"><a href="%s">%s</a></p>
	`, name, link, link, link)
	return getLayout("Verifikasi Email", content)
}

// GetInvitationEmail returns the template for workspace invitations.
func GetInvitationEmail(workspaceName, inviterName, link, registrationLink string) string {
	content := fmt.Sprintf(`
		<div class="badge">Undangan Workspace</div>
		<h2>Anda Telah Diundang untuk Bergabung dengan %s</h2>
		<p>Halo,</p>
		<p><strong>%s</strong> telah mengundang Anda untuk berkolaborasi di workspace mereka di Tulis CMS.</p>
		
		<div class="steps-box">
			<h3>Cara Bergabung:</h3>
			<ul class="steps-list">
				<li><strong>Sudah memiliki akun?</strong> Klik tombol "Terima Undangan" di bawah, login, lalu konfirmasi undangan Anda.</li>
				<li><strong>Belum memiliki akun?</strong> Daftarkan akun Anda terlebih dahulu menggunakan link Registrasi, kemudian klik link undangan ini kembali untuk bergabung.</li>
			</ul>
		</div>

		<div class="action-box">
			<a href="%s" class="btn" target="_blank">Terima Undangan</a>
			<a href="%s" class="btn-secondary" target="_blank">Buat Akun</a>
		</div>

		<p>Link undangan ini berlaku selama 7 hari. Jika tombol di atas tidak berfungsi, gunakan link berikut:</p>
		<p style="word-break: break-all; font-size: 13px; color: #64748b;"><a href="%s">%s</a></p>
	`, workspaceName, inviterName, link, registrationLink, link, link)
	return getLayout("Undangan Workspace", content)
}

// GetPasswordResetEmail returns the template for password resets.
func GetPasswordResetEmail(name, link string) string {
	content := fmt.Sprintf(`
		<div class="badge" style="background-color: #fef3c7; color: #92400e; border-color: #fde68a;">Keamanan Akun</div>
		<h2>Atur Ulang Kata Sandi Anda</h2>
		<p>Halo %s,</p>
		<p>Kami menerima permintaan untuk mengatur ulang kata sandi akun Tulis CMS Anda. Klik tombol di bawah ini untuk memilih kata sandi baru.</p>
		<div class="action-box">
			<a href="%s" class="btn" target="_blank">Atur Ulang Kata Sandi</a>
		</div>
		<p>Jika Anda tidak meminta ini, Anda dapat mengabaikan email ini dengan aman. Kata sandi Anda tidak akan berubah.</p>
		<p>Tautan ini berlaku selama 1 jam. Jika tombol tidak berfungsi, salin dan tempel URL berikut:</p>
		<p style="word-break: break-all; font-size: 13px; color: #64748b;"><a href="%s">%s</a></p>
	`, name, link, link, link)
	return getLayout("Atur Ulang Kata Sandi", content)
}

// Get7DaysInactiveEmail returns the template for users inactive for 7 days.
func Get7DaysInactiveEmail(name string) string {
	content := fmt.Sprintf(`
		<div class="badge" style="background-color: #ecfdf5; color: #065f46; border-color: #a7f3d0;">Kami Merindukanmu</div>
		<h2>Lama Tidak Berjumpa, %s!</h2>
		<p>Halo %s,</p>
		<p>Sudah 7 hari sejak terakhir kali Anda login ke Tulis CMS. Kami merindukan kehadiran Anda di platform kami!</p>
		<p>Ada banyak konten dan pembaruan baru yang menunggu Anda di workspace Anda. Mari login kembali untuk melanjutkan menulis atau melihat perkembangan terbaru.</p>
		<div class="action-box">
			<a href="%s/login" class="btn" target="_blank">Masuk ke Dashboard</a>
		</div>
	`, name, name, config.AppConfig.FrontURL)
	return getLayout("Kami merindukanmu!", content)
}

// Get30DaysNoWriteEmail returns the template for users with no posts for 30 days.
func Get30DaysNoWriteEmail(name string) string {
	content := fmt.Sprintf(`
		<div class="badge" style="background-color: #eff6ff; color: #1e40af; border-color: #bfdbfe;">Ayo Menulis</div>
		<h2>Yuk, Bagikan Ide Barumu!</h2>
		<p>Halo %s,</p>
		<p>Sudah 30 hari Anda tidak menulis postingan baru di Tulis CMS. Keyboard Anda mungkin sudah mulai berdebu! 😉</p>
		<p>Menulis secara konsisten adalah cara terbaik untuk berinteraksi dengan audiens Anda dan membagikan keahlian Anda. Kami memiliki editor Markdown & Rich Text yang sangat responsif dan siap membantu Anda menumpahkan ide-ide brilian Anda.</p>
		<div class="action-box">
			<a href="%s/posts/new" class="btn" target="_blank">Mulai Menulis Post Baru</a>
		</div>
	`, name, config.AppConfig.FrontURL)
	return getLayout("Bagikan ide barumu!", content)
}

// GetGeneralNotificationEmail returns a general notification template.
func GetGeneralNotificationEmail(name, title, message string) string {
	content := fmt.Sprintf(`
		<h2>%s</h2>
		<p>Halo %s,</p>
		<p>%s</p>
	`, title, name, strings.ReplaceAll(message, "\n", "<br>"))
	return getLayout(title, content)
}

// GetBrokenLinkAlertEmail notifies workspace admins that broken links were detected.
func GetBrokenLinkAlertEmail(name string, count int, frontURL string) string {
	content := fmt.Sprintf(`
		<div class="badge" style="background-color: #fef2f2; color: #b91c1c; border-color: #fecaca;">Peringatan Tautan Rusak</div>
		<h2>Tautan Rusak Terdeteksi</h2>
		<p>Halo %s,</p>
		<p>Pemindai tautan otomatis Tulis CMS menemukan <strong>%d tautan rusak</strong> pada postingan di workspace Anda. Tautan yang rusak dapat merusak SEO dan pengalaman pembaca.</p>
		<p>Silakan buka dashboard untuk meninjau dan memperbaiki tautan tersebut agar konten tetap berkualitas.</p>
		<div class="action-box">
			<a href="%s/dashboard/links" class="btn" target="_blank">Tinjau Tautan Rusak</a>
		</div>
	`, name, count, frontURL)
	return getLayout("Peringatan Tautan Rusak", content)
}

// GetNotificationEmailTemplate returns general notification email content.
func GetNotificationEmailTemplate(title, message string) string {
	content := fmt.Sprintf(`
		<div class="badge">Notifikasi Tulis CMS</div>
		<h2>%s</h2>
		<p>%s</p>
	`, title, strings.ReplaceAll(message, "\n", "<br>"))
	return getLayout(title, content)
}

