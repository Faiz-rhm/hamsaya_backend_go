package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/smtp"
	"strconv"
	"time"

	"github.com/hamsaya/backend/config"
	"go.uber.org/zap"
)

// EmailService handles sending emails
type EmailService struct {
	cfg    *config.EmailConfig
	logger *zap.Logger
}

// NewEmailService creates a new email service
func NewEmailService(cfg *config.EmailConfig, logger *zap.Logger) *EmailService {
	return &EmailService{
		cfg:    cfg,
		logger: logger,
	}
}

// EmailData represents data for email templates
type EmailData struct {
	RecipientName  string
	RecipientEmail string
	Subject        string
	VerifyURL      string
	ResetURL       string
	Token          string
	ExpiresIn      string
	AppName        string
	AppURL         string
	SupportEmail   string
	Year           string // e.g. "2025" for footer
}

// SendVerificationEmail sends an email with a verification code (user enters code in the app)
func (s *EmailService) SendVerificationEmail(email, name, verificationCode string) error {
	s.logger.Info("Verification code generated (check server logs if email not configured)",
		zap.String("email", email),
		zap.String("code", verificationCode),
	)
	data := EmailData{
		RecipientName:  name,
		RecipientEmail: email,
		Subject:        "Your verification code",
		VerifyURL:      "",
		Token:          verificationCode,
		ExpiresIn:      "24 hours",
		AppName:        "Hamsaya",
		AppURL:         "https://hamsaya.com",
		SupportEmail:   "support@hamsaya.com",
		Year:           strconv.Itoa(time.Now().Year()),
	}

	htmlBody, err := s.renderTemplate(verificationEmailTemplate, data)
	if err != nil {
		s.logger.Error("Failed to render verification email template", zap.Error(err))
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return s.sendEmail(email, data.Subject, htmlBody)
}

// SendPasswordResetEmail sends a password reset code (user enters it in the app)
func (s *EmailService) SendPasswordResetEmail(email, name, resetCode string) error {
	s.logger.Info("Password reset code generated (check server logs if email not configured)",
		zap.String("email", email),
		zap.String("code", resetCode),
	)
	data := EmailData{
		RecipientName:  name,
		RecipientEmail: email,
		Subject:        "Your password reset code",
		Token:          resetCode,
		ExpiresIn:      "15 minutes",
		AppName:        "Hamsaya",
		AppURL:         "https://hamsaya.com",
		SupportEmail:   "support@hamsaya.com",
		Year:           strconv.Itoa(time.Now().Year()),
	}

	htmlBody, err := s.renderTemplate(passwordResetEmailTemplate, data)
	if err != nil {
		s.logger.Error("Failed to render password reset email template", zap.Error(err))
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return s.sendEmail(email, data.Subject, htmlBody)
}

// SendWelcomeEmail sends a welcome email after registration
func (s *EmailService) SendWelcomeEmail(email, name string) error {
	data := EmailData{
		RecipientName:  name,
		RecipientEmail: email,
		Subject:        "Welcome to Hamsaya!",
		AppName:        "Hamsaya",
		AppURL:         "https://hamsaya.com",
		SupportEmail:   "support@hamsaya.com",
		Year:           strconv.Itoa(time.Now().Year()),
	}

	htmlBody, err := s.renderTemplate(welcomeEmailTemplate, data)
	if err != nil {
		s.logger.Error("Failed to render welcome email template", zap.Error(err))
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return s.sendEmail(email, data.Subject, htmlBody)
}

// SendPasswordChangedEmail sends notification when password is changed
func (s *EmailService) SendPasswordChangedEmail(email, name string) error {
	data := EmailData{
		RecipientName:  name,
		RecipientEmail: email,
		Subject:        "Your Password Has Been Changed",
		AppName:        "Hamsaya",
		AppURL:         "https://hamsaya.com",
		SupportEmail:   "support@hamsaya.com",
		Year:           strconv.Itoa(time.Now().Year()),
	}

	htmlBody, err := s.renderTemplate(passwordChangedEmailTemplate, data)
	if err != nil {
		s.logger.Error("Failed to render password changed email template", zap.Error(err))
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return s.sendEmail(email, data.Subject, htmlBody)
}

// sendEmail sends an email using Resend API (if RESEND_API_KEY set) or SMTP.
// Returns an error if neither is configured so callers can report failure.
func (s *EmailService) sendEmail(to, subject, htmlBody string) error {
	if s.cfg.ResendAPIKey != "" {
		return s.sendEmailResend(to, subject, htmlBody)
	}
	if s.cfg.SMTPHost != "" && s.cfg.SMTPPort != "" {
		return s.sendEmailSMTP(to, subject, htmlBody)
	}
	return fmt.Errorf("email not configured: set RESEND_API_KEY or SMTP_HOST and SMTP_PORT to send emails")
}

// sendEmailResend sends an email via Resend API
func (s *EmailService) sendEmailResend(to, subject, htmlBody string) error {
	from := s.cfg.From
	if from == "" {
		from = "Hamsaya <onboarding@resend.dev>"
	}

	body := map[string]interface{}{
		"from":    from,
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal Resend request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create Resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.logger.Error("Resend API request failed", zap.String("to", to), zap.Error(err))
		return fmt.Errorf("failed to send email via Resend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody bytes.Buffer
		_, _ = errBody.ReadFrom(resp.Body) // read response body into errBody
		s.logger.Error("Resend API error",
			zap.String("to", to),
			zap.Int("status", resp.StatusCode),
			zap.String("body", errBody.String()),
		)
		return fmt.Errorf("Resend API returned status %d: %s", resp.StatusCode, errBody.String())
	}

	s.logger.Info("Email sent via Resend", zap.String("to", to), zap.String("subject", subject))
	return nil
}

// sendEmailSMTP sends an email using SMTP (caller must ensure SMTP is configured).
func (s *EmailService) sendEmailSMTP(to, subject, htmlBody string) error {
	from := s.cfg.From
	if from == "" {
		from = "noreply@hamsaya.com"
	}

	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + htmlBody

	auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.SMTPHost)
	addr := s.cfg.SMTPHost + ":" + s.cfg.SMTPPort
	err := smtp.SendMail(addr, auth, from, []string{to}, []byte(message))
	if err != nil {
		s.logger.Error("Failed to send email", zap.String("to", to), zap.String("subject", subject), zap.Error(err))
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("Email sent successfully", zap.String("to", to), zap.String("subject", subject))
	return nil
}

// renderTemplate renders an HTML template with data
func (s *EmailService) renderTemplate(tmpl string, data EmailData) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// Email templates — Hamsaya brand primary: #fc7b58
const verificationEmailTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Subject}}</title>
    <style>
        body { margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #1f2937; background: #f3f4f6; }
        .wrapper { max-width: 560px; margin: 0 auto; padding: 32px 16px; }
        .card { background: #ffffff; border-radius: 16px; padding: 40px 32px; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1); }
        .logo { font-size: 24px; font-weight: 700; color: #fc7b58; margin: 0 0 8px 0; letter-spacing: -0.5px; }
        .tagline { font-size: 14px; color: #6b7280; margin: 0 0 28px 0; }
        .content { margin-bottom: 28px; }
        .content h2 { font-size: 18px; font-weight: 600; color: #111827; margin: 0 0 16px 0; }
        .content p { margin: 0 0 12px 0; font-size: 15px; color: #374151; }
        .code-label { text-align: center; font-size: 13px; color: #6b7280; margin: 24px 0 8px 0; font-weight: 500; }
        .code-box { background: linear-gradient(135deg, #fff7ed 0%, #ffedd5 100%); border: 2px solid #fc7b58; border-radius: 12px; padding: 20px 24px; text-align: center; margin: 0 0 20px 0; }
        .code-box .code { font-size: 32px; font-weight: 700; letter-spacing: 10px; color: #c2410c; font-family: 'SF Mono', Monaco, 'Courier New', monospace; }
        .expiry { font-size: 14px; color: #6b7280; margin: 16px 0 0 0; }
        .footer { text-align: center; padding-top: 24px; border-top: 1px solid #e5e7eb; font-size: 13px; color: #9ca3af; }
        .footer a { color: #fc7b58; text-decoration: none; }
    </style>
</head>
<body>
    <div class="wrapper">
        <div class="card">
            <div class="content">
                <p class="logo">{{.AppName}}</p>
                <p class="tagline">Your neighborhood, connected.</p>
                <h2>Hi {{.RecipientName}},</h2>
                <p>Thanks for signing up. Use the code below in the app to verify your email and get started.</p>
                <p class="code-label">Your verification code</p>
                <div class="code-box"><span class="code">{{.Token}}</span></div>
                <p class="expiry"><strong>This code expires in {{.ExpiresIn}}.</strong> Enter it in the app before it expires.</p>
                <p style="margin-top: 20px; font-size: 14px; color: #6b7280;">If you didn't create an account with {{.AppName}}, you can safely ignore this email.</p>
            </div>
            <div class="footer">
                <p>Need help? <a href="mailto:{{.SupportEmail}}">Contact us</a></p>
                <p>&copy; {{.Year}} {{.AppName}}. All rights reserved.</p>
            </div>
        </div>
    </div>
</body>
</html>
`

const passwordResetEmailTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Subject}}</title>
    <style>
        body { margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #1f2937; background: #f3f4f6; }
        .wrapper { max-width: 560px; margin: 0 auto; padding: 32px 16px; }
        .card { background: #ffffff; border-radius: 16px; padding: 40px 32px; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1); }
        .logo { font-size: 24px; font-weight: 700; color: #fc7b58; margin: 0 0 8px 0; }
        .tagline { font-size: 14px; color: #6b7280; margin: 0 0 28px 0; }
        .content { margin-bottom: 28px; }
        .content h2 { font-size: 18px; font-weight: 600; color: #111827; margin: 0 0 16px 0; }
        .content p { margin: 0 0 12px 0; font-size: 15px; color: #374151; }
        .code-label { text-align: center; font-size: 13px; color: #6b7280; margin: 24px 0 8px 0; font-weight: 500; }
        .code-box { background: linear-gradient(135deg, #fff7ed 0%, #ffedd5 100%); border: 2px solid #fc7b58; border-radius: 12px; padding: 20px 24px; text-align: center; margin: 0 0 20px 0; }
        .code-box .code { font-size: 32px; font-weight: 700; letter-spacing: 10px; color: #c2410c; font-family: 'SF Mono', Monaco, 'Courier New', monospace; }
        .expiry { font-size: 14px; color: #6b7280; margin: 16px 0 0 0; }
        .warning { background: #fef2f2; border-left: 4px solid #dc2626; padding: 14px 16px; margin: 24px 0 0 0; border-radius: 0 8px 8px 0; font-size: 14px; color: #991b1b; }
        .footer { text-align: center; padding-top: 24px; border-top: 1px solid #e5e7eb; font-size: 13px; color: #9ca3af; }
        .footer a { color: #fc7b58; text-decoration: none; }
    </style>
</head>
<body>
    <div class="wrapper">
        <div class="card">
            <div class="content">
                <p class="logo">{{.AppName}}</p>
                <p class="tagline">Your neighborhood, connected.</p>
                <h2>Hi {{if .RecipientName}}{{.RecipientName}}{{else}}there{{end}},</h2>
                <p>We received a request to reset your password. Use the code below in the app to set a new password.</p>
                <p class="code-label">Your password reset code</p>
                <div class="code-box"><span class="code">{{.Token}}</span></div>
                <p class="expiry"><strong>This code expires in {{.ExpiresIn}}.</strong> Enter it in the app right away.</p>
                <div class="warning"><strong>Not you?</strong> If you didn't request a password reset, ignore this email. Your password will not change.</div>
            </div>
            <div class="footer">
                <p>Need help? <a href="mailto:{{.SupportEmail}}">Contact us</a></p>
                <p>&copy; {{.Year}} {{.AppName}}. All rights reserved.</p>
            </div>
        </div>
    </div>
</body>
</html>
`

const welcomeEmailTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Subject}}</title>
    <style>
        body { margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #1f2937; background: #f3f4f6; }
        .wrapper { max-width: 560px; margin: 0 auto; padding: 32px 16px; }
        .card { background: #ffffff; border-radius: 16px; padding: 40px 32px; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1); }
        .hero { text-align: center; margin-bottom: 28px; }
        .hero h1 { font-size: 26px; font-weight: 700; color: #111827; margin: 0 0 8px 0; }
        .hero .brand { color: #fc7b58; }
        .content { margin-bottom: 28px; }
        .content h2 { font-size: 18px; font-weight: 600; color: #111827; margin: 0 0 16px 0; }
        .content p { margin: 0 0 12px 0; font-size: 15px; color: #374151; }
        .features { background: #f9fafb; border-radius: 12px; padding: 20px 24px; margin: 20px 0; border: 1px solid #e5e7eb; }
        .features ul { margin: 0; padding-left: 20px; font-size: 15px; color: #4b5563; line-height: 1.8; }
        .cta { text-align: center; margin: 28px 0; }
        .cta a { display: inline-block; padding: 14px 28px; background: #fc7b58; color: #ffffff !important; text-decoration: none; border-radius: 10px; font-weight: 600; font-size: 16px; }
        .footer { text-align: center; padding-top: 24px; border-top: 1px solid #e5e7eb; font-size: 13px; color: #9ca3af; }
        .footer a { color: #fc7b58; text-decoration: none; }
    </style>
</head>
<body>
    <div class="wrapper">
        <div class="card">
            <div class="hero">
                <h1>Welcome to <span class="brand">{{.AppName}}</span></h1>
                <p style="font-size: 15px; color: #6b7280; margin: 0;">Your neighborhood, connected.</p>
            </div>
            <div class="content">
                <h2>Hi {{.RecipientName}},</h2>
                <p>You're all set. We're glad to have you in the community. Here's what you can do:</p>
                <div class="features">
                    <ul>
                        <li>Connect with neighbors in your area</li>
                        <li>Discover local events and activities</li>
                        <li>Buy and sell items in your neighborhood</li>
                        <li>Share updates and build your community</li>
                    </ul>
                </div>
                <div class="cta"><a href="{{.AppURL}}">Open {{.AppName}}</a></div>
                <p>If you have any questions, reply to this email or contact <a href="mailto:{{.SupportEmail}}" style="color: #fc7b58;">support</a>. We're here to help.</p>
            </div>
            <div class="footer">
                <p>Need help? <a href="mailto:{{.SupportEmail}}">Contact us</a></p>
                <p>&copy; {{.Year}} {{.AppName}}. All rights reserved.</p>
            </div>
        </div>
    </div>
</body>
</html>
`

const passwordChangedEmailTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Subject}}</title>
    <style>
        body { margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #1f2937; background: #f3f4f6; }
        .wrapper { max-width: 560px; margin: 0 auto; padding: 32px 16px; }
        .card { background: #ffffff; border-radius: 16px; padding: 40px 32px; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1); }
        .logo { font-size: 24px; font-weight: 700; color: #fc7b58; margin: 0 0 28px 0; }
        .content { margin-bottom: 28px; }
        .content h2 { font-size: 18px; font-weight: 600; color: #111827; margin: 0 0 16px 0; }
        .content p { margin: 0 0 12px 0; font-size: 15px; color: #374151; }
        .success { background: #f0fdf4; border-left: 4px solid #16a34a; padding: 16px 20px; margin: 20px 0; border-radius: 0 10px 10px 0; font-size: 15px; color: #166534; }
        .warning { background: #fef2f2; border-left: 4px solid #dc2626; padding: 16px 20px; margin: 20px 0 0 0; border-radius: 0 10px 10px 0; font-size: 14px; color: #991b1b; }
        .warning a { color: #dc2626; font-weight: 600; }
        .footer { text-align: center; padding-top: 24px; border-top: 1px solid #e5e7eb; font-size: 13px; color: #9ca3af; }
        .footer a { color: #fc7b58; text-decoration: none; }
    </style>
</head>
<body>
    <div class="wrapper">
        <div class="card">
            <div class="content">
                <p class="logo">{{.AppName}}</p>
                <h2>Hi {{.RecipientName}},</h2>
                <div class="success"><strong>Password changed successfully.</strong><br>Your {{.AppName}} account password was updated. If you made this change, you're all set.</div>
                <div class="warning"><strong>Didn't make this change?</strong><br>Contact us immediately at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a>. Your account may be at risk.</div>
            </div>
            <div class="footer">
                <p>Need help? <a href="mailto:{{.SupportEmail}}">Contact us</a></p>
                <p>&copy; {{.Year}} {{.AppName}}. All rights reserved.</p>
            </div>
        </div>
    </div>
</body>
</html>
`
