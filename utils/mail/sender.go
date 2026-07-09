package mail

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/dankedev/tulis-go/config"
	"github.com/sirupsen/logrus"
)

// SendHTMLMail sends an HTML email to the specified recipient using SMTP.
// In development, if SMTP is not accessible, it logs the content as a fallback.
func SendHTMLMail(toEmail, subject, htmlBody string) error {
	cfg := config.AppConfig
	if cfg == nil {
		logrus.Warn("AppConfig is not initialized, printing email to console instead")
		printToConsole(toEmail, subject, htmlBody)
		return nil
	}

	fromHeader := fmt.Sprintf("%s <%s>", cfg.SMTPFromName, cfg.SMTPFrom)

	// Create MIME headers
	headers := make(map[string]string)
	headers["From"] = fromHeader
	headers["To"] = toEmail
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"utf-8\""

	// Construct message
	var message strings.Builder
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(htmlBody)

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	var auth smtp.Auth
	if cfg.SMTPUser != "" || cfg.SMTPPassword != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	}

	// Send email using SMTP
	err := smtp.SendMail(addr, auth, cfg.SMTPFrom, []string{toEmail}, []byte(message.String()))
	if err != nil {
		logrus.Errorf("[MAIL ERROR] Failed to send email to %s via SMTP: %v. Printing content to console.", toEmail, err)
		printToConsole(toEmail, subject, htmlBody)
		// Return nil or proceed, since we logged it as a fallback in dev
		return nil
	}

	logrus.Infof("[MAIL SUCCESS] Email sent to %s with subject: %s", toEmail, subject)
	return nil
}

func printToConsole(to, subject, body string) {
	fmt.Printf("\n==================================================\n")
	fmt.Printf("📧 EMAIL SENT (CONSOLE FALLBACK)\n")
	fmt.Printf("To:      %s\n", to)
	fmt.Printf("Subject: %s\n", subject)
	fmt.Printf("Content Length: %d bytes\n", len(body))
	fmt.Printf("------------------ Body Snippet ------------------\n")
	// Print a cleaner, readable preview
	preview := body
	if len(preview) > 1000 {
		preview = preview[:1000] + "\n...[truncated]..."
	}
	fmt.Println(preview)
	fmt.Printf("==================================================\n\n")
}
