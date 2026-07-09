package mail

import (
	"fmt"
	"strings"
)

// getLayout wraps html content in our premium Tulis CMS branding layout.
func getLayout(title, content string) string {
	brandName := "Tulis CMS"
	layout := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background-color: #f8fafc;
            color: #1e293b;
            margin: 0;
            padding: 0;
            -webkit-font-smoothing: antialiased;
        }
        .container {
            max-width: 600px;
            margin: 40px auto;
            background-color: #ffffff;
            border-radius: 12px;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.05);
            border: 1px solid #e2e8f0;
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #0f172a 0%, #1e293b 100%);
            padding: 32px;
            text-align: center;
        }
        .header h1 {
            color: #ffffff;
            margin: 0;
            font-size: 24px;
            font-weight: 700;
            letter-spacing: -0.025em;
        }
        .content {
            padding: 40px;
            line-height: 1.6;
        }
        .content h2 {
            font-size: 20px;
            font-weight: 600;
            color: #0f172a;
            margin-top: 0;
            margin-bottom: 16px;
        }
        .content p {
            margin-top: 0;
            margin-bottom: 20px;
            color: #475569;
        }
        .action-box {
            text-align: center;
            margin: 32px 0;
        }
        .btn {
            display: inline-block;
            background-color: #6366f1;
            color: #ffffff !important;
            text-decoration: none;
            padding: 12px 28px;
            border-radius: 8px;
            font-weight: 600;
            text-align: center;
            box-shadow: 0 4px 6px -1px rgba(99, 102, 241, 0.2);
        }
        .btn-secondary {
            display: inline-block;
            background-color: #e2e8f0;
            color: #334155 !important;
            text-decoration: none;
            padding: 12px 28px;
            border-radius: 8px;
            font-weight: 600;
            text-align: center;
            margin-left: 10px;
        }
        .footer {
            background-color: #f8fafc;
            padding: 24px;
            text-align: center;
            border-top: 1px solid #e2e8f0;
            font-size: 12px;
            color: #64748b;
        }
        .footer a {
            color: #6366f1;
            text-decoration: none;
        }
        .steps-box {
            background-color: #f1f5f9;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 24px;
            border: 1px solid #e2e8f0;
        }
        .steps-box h3 {
            margin-top: 0;
            font-size: 14px;
            font-weight: 600;
            color: #334155;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .steps-list {
            padding-left: 20px;
            margin: 0;
            color: #475569;
        }
        .steps-list li {
            margin-bottom: 8px;
        }
        .badge {
            display: inline-block;
            padding: 4px 10px;
            border-radius: 9999px;
            font-size: 11px;
            font-weight: 600;
            background-color: #f0fdf4;
            color: #166534;
            margin-bottom: 12px;
            border: 1px solid #bbf7d0;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.BrandName}}</h1>
        </div>
        <div class="content">
            {{.Content}}
        </div>
        <div class="footer">
            <p>Sent with ❤️ by {{.BrandName}} Team</p>
            <p>If you have any questions, contact us at <a href="mailto:support@tulis.org">support@tulis.org</a></p>
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
		<h2>Verify Your Email Address</h2>
		<p>Hi %s,</p>
		<p>Thank you for registering at Tulis CMS! We're excited to have you on board. Please click the button below to verify your email address and get started.</p>
		<div class="action-box">
			<a href="%s" class="btn" target="_blank">Verify Email</a>
		</div>
		<p>If the button doesn't work, copy and paste this URL into your browser:</p>
		<p style="word-break: break-all; font-size: 13px; color: #64748b;"><a href="%s">%s</a></p>
	`, name, link, link, link)
	return getLayout("Verify Email", content)
}

// GetInvitationEmail returns the template for workspace invitations.
func GetInvitationEmail(workspaceName, inviterName, link, registrationLink string) string {
	content := fmt.Sprintf(`
		<div class="badge">Undangan Workspace</div>
		<h2>You've Been Invited to Join %s</h2>
		<p>Hi there,</p>
		<p><strong>%s</strong> has invited you to collaborate in their workspace on Tulis CMS.</p>
		
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
	return getLayout("Workspace Invitation", content)
}

// GetPasswordResetEmail returns the template for password resets.
func GetPasswordResetEmail(name, link string) string {
	content := fmt.Sprintf(`
		<div class="badge" style="background-color: #fef3c7; color: #92400e; border-color: #fde68a;">Keamanan Akun</div>
		<h2>Reset Your Password</h2>
		<p>Hi %s,</p>
		<p>We received a request to reset your password for your Tulis CMS account. Click the button below to choose a new password.</p>
		<div class="action-box">
			<a href="%s" class="btn" target="_blank">Reset Password</a>
		</div>
		<p>If you did not request this, you can safely ignore this email. Your password will remain unchanged.</p>
		<p>This link is valid for 1 hour. If the button doesn't work, copy and paste this URL:</p>
		<p style="word-break: break-all; font-size: 13px; color: #64748b;"><a href="%s">%s</a></p>
	`, name, link, link, link)
	return getLayout("Reset Password", content)
}

// Get7DaysInactiveEmail returns the template for users inactive for 7 days.
func Get7DaysInactiveEmail(name string) string {
	content := fmt.Sprintf(`
		<div class="badge" style="background-color: #ecfdf5; color: #065f46; border-color: #a7f3d0;">Kami Merindukanmu</div>
		<h2>Lama Tidak Berjumpa, %s!</h2>
		<p>Hi %s,</p>
		<p>Sudah 7 hari sejak terakhir kali Anda login ke Tulis CMS. Kami merindukan kehadiran Anda di platform kami!</p>
		<p>Ada banyak konten dan pembaruan baru yang menunggu Anda di workspace Anda. Mari login kembali untuk melanjutkan menulis atau melihat perkembangan terbaru.</p>
		<div class="action-box">
			<a href="https://app.tulis.org/login" class="btn" target="_blank">Masuk ke Dashboard</a>
		</div>
	`, name, name)
	return getLayout("We miss you!", content)
}

// Get30DaysNoWriteEmail returns the template for users with no posts for 30 days.
func Get30DaysNoWriteEmail(name string) string {
	content := fmt.Sprintf(`
		<div class="badge" style="background-color: #eff6ff; color: #1e40af; border-color: #bfdbfe;">Ayo Menulis</div>
		<h2>Yuk, Bagikan Ide Barumu!</h2>
		<p>Hi %s,</p>
		<p>Sudah 30 hari Anda tidak menulis postingan baru di Tulis CMS. Keyboard Anda mungkin sudah mulai berdebu! 😉</p>
		<p>Menulis secara konsisten adalah cara terbaik untuk berinteraksi dengan audiens Anda dan membagikan keahlian Anda. Kami memiliki editor Markdown & Rich Text yang sangat responsif dan siap membantu Anda menumpahkan ide-ide brilian Anda.</p>
		<div class="action-box">
			<a href="https://app.tulis.org/posts/new" class="btn" target="_blank">Mulai Menulis Post Baru</a>
		</div>
	`, name)
	return getLayout("Share your new ideas!", content)
}

// GetGeneralNotificationEmail returns a general notification template.
func GetGeneralNotificationEmail(name, title, message string) string {
	content := fmt.Sprintf(`
		<h2>%s</h2>
		<p>Hi %s,</p>
		<p>%s</p>
	`, title, name, strings.ReplaceAll(message, "\n", "<br>"))
	return getLayout(title, content)
}
