package services

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/gomail.v2"
)

// EmailService handles email operations
type EmailService struct {
	smtpHost     string
	smtpPort     int
	smtpUsername string
	smtpPassword string
	fromEmail    string
	fromName     string
}

// EmailData represents email content
type EmailData struct {
	To      string
	Subject string
	Body    string
}

// NewEmailService creates a new email service
func NewEmailService() (*EmailService, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found in email service package, using system env")
	}

	// Get SMTP configuration from environment
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		smtpHost = "smtp.gmail.com"
	}

	smtpPort := 587
	if port := os.Getenv("SMTP_PORT"); port != "" {
		if p, err := fmt.Sscanf(port, "%d", &smtpPort); err != nil || p != 1 {
			smtpPort = 587
		}
	}

	smtpUsername := os.Getenv("SMTP_USERNAME")
	if smtpUsername == "" {
		return nil, fmt.Errorf("SMTP_USERNAME is required")
	}

	smtpPassword := os.Getenv("SMTP_PASSWORD")
	if smtpPassword == "" {
		return nil, fmt.Errorf("SMTP_PASSWORD is required")
	}

	fromEmail := os.Getenv("FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = smtpUsername
	}

	fromName := os.Getenv("FROM_NAME")
	if fromName == "" {
		fromName = "ZACloth"
	}

	return &EmailService{
		smtpHost:     smtpHost,
		smtpPort:     smtpPort,
		smtpUsername: smtpUsername,
		smtpPassword: smtpPassword,
		fromEmail:    fromEmail,
		fromName:     fromName,
	}, nil
}

// SendOTPEmail sends OTP verification email
func (es *EmailService) SendOTPEmail(to, username, otp string) error {
	subject := "Verifikasi Email - ZACloth"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .otp-code { background: #667eea; color: white; font-size: 32px; font-weight: bold; padding: 20px; text-align: center; border-radius: 8px; margin: 20px 0; letter-spacing: 5px; }
        .footer { text-align: center; margin-top: 30px; color: #666; font-size: 14px; }
        .button { background: #667eea; color: white; padding: 12px 24px; text-decoration: none; border-radius: 5px; display: inline-block; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎉 Selamat Datang di ZACloth!</h1>
        </div>
        <div class="content">
            <h2>Halo %s!</h2>
            <p>Terima kasih telah mendaftar di ZACloth. Untuk melengkapi proses pendaftaran, silakan verifikasi email Anda dengan kode OTP berikut:</p>
            
            <div class="otp-code">%s</div>
            
            <p><strong>Kode ini berlaku selama 10 menit.</strong></p>
            
            <p>Jika Anda tidak mendaftar di ZACloth, silakan abaikan email ini.</p>
            
            <p>Terima kasih,<br>Tim ZACloth</p>
        </div>
        <div class="footer">
            <p>Email ini dikirim secara otomatis, mohon tidak membalas email ini.</p>
        </div>
    </div>
</body>
</html>`, subject, username, otp)

	return es.SendEmail(EmailData{
		To:      to,
		Subject: subject,
		Body:    body,
	})
}

// SendWelcomeEmail sends welcome email after verification
func (es *EmailService) SendWelcomeEmail(to, username string) error {
	subject := "Selamat! Akun Anda Telah Terverifikasi - ZACloth"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .footer { text-align: center; margin-top: 30px; color: #666; font-size: 14px; }
        .button { background: #667eea; color: white; padding: 12px 24px; text-decoration: none; border-radius: 5px; display: inline-block; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎉 Selamat Datang di ZACloth!</h1>
        </div>
        <div class="content">
            <h2>Halo %s!</h2>
            <p>Selamat! Email Anda telah berhasil diverifikasi. Akun ZACloth Anda sekarang sudah aktif dan siap digunakan.</p>
            
            <p>Anda sekarang dapat:</p>
            <ul>
                <li>✅ Login ke akun Anda</li>
                <li>🛍️ Berbelanja produk terbaru</li>
                <li>💳 Mengelola profil dan preferensi</li>
                <li>📱 Mengakses semua fitur ZACloth</li>
            </ul>
            
            <p>Terima kasih telah bergabung dengan ZACloth!</p>
            
            <p>Terima kasih,<br>Tim ZACloth</p>
        </div>
        <div class="footer">
            <p>Email ini dikirim secara otomatis, mohon tidak membalas email ini.</p>
        </div>
    </div>
</body>
</html>`, subject, username)

	return es.SendEmail(EmailData{
		To:      to,
		Subject: subject,
		Body:    body,
	})
}

// SendEmail sends a generic email
func (es *EmailService) SendEmail(emailData EmailData) error {
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", es.fromName, es.fromEmail))
	m.SetHeader("To", emailData.To)
	m.SetHeader("Subject", emailData.Subject)
	m.SetBody("text/html", emailData.Body)

	d := gomail.NewDialer(es.smtpHost, es.smtpPort, es.smtpUsername, es.smtpPassword)

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("✅ Email sent successfully to: %s", emailData.To)
	return nil
}

// HealthCheck checks if email service is properly configured
func (es *EmailService) HealthCheck() error {
	if es.smtpHost == "" || es.smtpUsername == "" || es.smtpPassword == "" {
		return fmt.Errorf("email service not properly configured")
	}
	return nil
}
