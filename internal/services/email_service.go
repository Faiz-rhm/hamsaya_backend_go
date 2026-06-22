package services

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"image/jpeg"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/hamsaya/backend/config"
	"go.uber.org/zap"
)

//go:embed assets/icon.jpg
var emailIconJPG []byte

// EmailIconBytes returns the resized JPEG payload for the brand icon used in
// transactional emails. Resized to 128x128 once at package init so the HTTP
// static handler can serve it without re-encoding per request.
//
// Hosted via a public URL (not a data URI) because Gmail and other major
// webmail clients strip `data:` image sources for security. Recipients only
// see the brand mark when the icon is reachable over plain HTTP/S.
var emailIconBytes = buildIconBytes()

func buildIconBytes() []byte {
	img, err := jpeg.Decode(bytes.NewReader(emailIconJPG))
	if err != nil {
		return emailIconJPG
	}
	resized := imaging.Resize(img, 128, 128, imaging.Lanczos)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 82}); err != nil {
		return emailIconJPG
	}
	return buf.Bytes()
}

// EmailIconBytes exposes the resized icon for the HTTP static handler.
func EmailIconBytes() []byte { return emailIconBytes }

// EmailService handles sending emails
type EmailService struct {
	cfg        *config.EmailConfig
	logger     *zap.Logger
	httpClient *http.Client
	iconURL    string
}

// NewEmailService creates a new email service
func NewEmailService(cfg *config.EmailConfig, logger *zap.Logger) *EmailService {
	return &EmailService{
		cfg:     cfg,
		logger:  logger,
		iconURL: deriveIconURL(cfg.EmailVerifyBaseURL),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// deriveIconURL builds the absolute URL where the email icon is served. Empty
// string disables the icon (template skips the <img>) — preferable to
// rendering a broken image when no public base URL is configured.
func deriveIconURL(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		return ""
	}
	return base + "/email-icon.jpg"
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
	IconURL        template.URL
}

// transportConfigured reports whether a real email transport (Resend or SMTP)
// is wired. When false, codes are logged as a dev fallback; when true (prod),
// codes are NEVER logged — they're delivered by email only.
func (s *EmailService) transportConfigured() bool {
	return s.cfg.ResendAPIKey != "" || (s.cfg.SMTPHost != "" && s.cfg.SMTPPort != "")
}

// SendVerificationEmail sends an email with a verification code (user enters code in the app)
func (s *EmailService) SendVerificationEmail(email, name, verificationCode string) error {
	if !s.transportConfigured() {
		// Dev fallback only — no email transport configured, so surface the
		// code in logs. In production (Resend/SMTP set) this never runs, so
		// verification codes are not leaked to the log pipeline.
		s.logger.Warn("Email transport not configured — verification code in logs (dev only)",
			zap.String("email", email),
			zap.String("code", verificationCode),
		)
	}
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
		IconURL:        template.URL(s.iconURL),
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
	if !s.transportConfigured() {
		s.logger.Warn("Email transport not configured — password reset code in logs (dev only)",
			zap.String("email", email),
			zap.String("code", resetCode),
		)
	}
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
		IconURL:        template.URL(s.iconURL),
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
		IconURL:        template.URL(s.iconURL),
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
		IconURL:        template.URL(s.iconURL),
	}

	htmlBody, err := s.renderTemplate(passwordChangedEmailTemplate, data)
	if err != nil {
		s.logger.Error("Failed to render password changed email template", zap.Error(err))
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return s.sendEmail(email, data.Subject, htmlBody)
}

// summaryLine builds the plain-text subhead, e.g. "1 unread message and 3
// unread notifications waiting for you."
func summaryLine(unreadMessages, unreadNotifications int) string {
	plural := func(n int, word string) string {
		if n == 1 {
			return fmt.Sprintf("%d unread %s", n, word)
		}
		return fmt.Sprintf("%d unread %ss", n, word)
	}
	var parts []string
	if unreadMessages > 0 {
		parts = append(parts, plural(unreadMessages, "message"))
	}
	if unreadNotifications > 0 {
		parts = append(parts, plural(unreadNotifications, "notification"))
	}
	return strings.Join(parts, " and ") + " waiting for you."
}

// SendUnreadDigestEmail nudges a user who has unread messages and/or
// notifications that have sat unread for 2+ days. Backend-driven re-engagement
// that works regardless of push delivery (notably in Afghanistan, where push
// can be unreliable). Keeps the copy short and links back to the app.
func (s *EmailService) SendUnreadDigestEmail(email, name string, unreadNotifications, unreadMessages int) error {
	if strings.TrimSpace(name) == "" {
		name = "there"
	}

	if unreadMessages <= 0 && unreadNotifications <= 0 {
		return nil // nothing to nudge about
	}

	// Title mirrors what's actually waiting.
	var title string
	switch {
	case unreadMessages > 0 && unreadNotifications > 0:
		title = "You have unread activity on Hamsaya"
	case unreadMessages > 0:
		if unreadMessages == 1 {
			title = "1 new message awaits your response"
		} else {
			title = fmt.Sprintf("%d new messages await your response", unreadMessages)
		}
	default:
		if unreadNotifications == 1 {
			title = "You have 1 new notification"
		} else {
			title = fmt.Sprintf("You have %d new notifications", unreadNotifications)
		}
	}

	// Small unread badges (chat + bell) for the header, LinkedIn-style. Inline
	// red count chips — no absolute positioning, which many email clients strip.
	var badges strings.Builder
	if unreadMessages > 0 {
		badges.WriteString(fmt.Sprintf(`<span style="font-size:18px;margin-left:14px;white-space:nowrap;">&#128172; <span style="background:#cc1016;color:#fff;border-radius:10px;padding:1px 6px;font-size:12px;font-weight:bold;">%d</span></span>`, unreadMessages))
	}
	if unreadNotifications > 0 {
		badges.WriteString(fmt.Sprintf(`<span style="font-size:18px;margin-left:14px;white-space:nowrap;">&#128276; <span style="background:#cc1016;color:#fff;border-radius:10px;padding:1px 6px;font-size:12px;font-weight:bold;">%d</span></span>`, unreadNotifications))
	}

	// Smart deep link: AppsFlyer OneLink opens the app if installed, else the
	// store. Falls back to the website when APP_DEEP_LINK_URL isn't configured.
	openURL := s.cfg.AppLink
	if strings.TrimSpace(openURL) == "" {
		openURL = "https://hamsaya.af"
	}
	storeIOS := s.cfg.StoreURLIOS
	if strings.TrimSpace(storeIOS) == "" {
		storeIOS = openURL
	}
	storeAndroid := s.cfg.StoreURLAndroid
	if strings.TrimSpace(storeAndroid) == "" {
		storeAndroid = openURL
	}

	iconHTML := `<span style="font-size:22px;font-weight:bold;color:#2563eb;">Hamsaya</span>`
	if s.iconURL != "" {
		iconHTML = fmt.Sprintf(`<img src="%s" width="40" height="40" alt="Hamsaya" style="border-radius:9px;display:block;">`, s.iconURL)
	}

	const tmpl = `<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;padding:0;background:#f3f2ef;font-family:Helvetica,Arial,sans-serif;color:#1a1a1a;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f3f2ef;padding:24px 12px;"><tr><td align="center">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:512px;">
    <tr><td style="background:#ffffff;border-radius:10px;padding:24px;">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr>
        <td align="left">{{ICON}}</td>
        <td align="right" style="white-space:nowrap;">{{BADGES}}</td>
      </tr></table>
      <h1 style="font-size:21px;text-align:center;margin:28px 0 8px;">{{TITLE}}</h1>
      <p style="text-align:center;color:#555;margin:0 0 24px;font-size:15px;">{{SUMMARY}}</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr><td align="center">
        <a href="{{URL}}" style="background:#2563eb;color:#ffffff;text-decoration:none;padding:13px 32px;border-radius:24px;display:inline-block;font-weight:bold;font-size:16px;">Open Hamsaya</a>
      </td></tr></table>
    </td></tr>
    <tr><td align="center" style="padding:28px 0 8px;">
      <p style="color:#2c5d63;font-weight:bold;font-size:16px;margin:0 0 14px;">Get the Hamsaya app</p>
      <a href="{{STORE_IOS}}" style="text-decoration:none;"><img src="https://tools.applemediaservices.com/api/badges/download-on-the-app-store/black/en-us?size=250x83" height="40" alt="Download on the App Store" style="margin:0 4px;vertical-align:middle;"></a>
      <a href="{{STORE_ANDROID}}" style="text-decoration:none;"><img src="https://play.google.com/intl/en_us/badges/static/images/badges/en_badge_web_generic.png" height="40" alt="Get it on Google Play" style="margin:0 4px;vertical-align:middle;"></a>
    </td></tr>
    <tr><td style="padding:24px 8px 0;border-top:1px solid #e0e0e0;">
      <p style="color:#888;font-size:12px;margin:8px 0;">Hi {{NAME}} — you're receiving this because you have unread activity on Hamsaya. If you've already caught up, you can ignore it.</p>
      <p style="color:#aaa;font-size:12px;margin:8px 0;">&copy; {{YEAR}} Hamsaya</p>
    </td></tr>
  </table>
</td></tr></table>
</body></html>`

	htmlBody := strings.NewReplacer(
		"{{ICON}}", iconHTML,
		"{{BADGES}}", badges.String(),
		"{{TITLE}}", template.HTMLEscapeString(title),
		"{{SUMMARY}}", template.HTMLEscapeString(summaryLine(unreadMessages, unreadNotifications)),
		"{{URL}}", template.HTMLEscapeString(openURL),
		"{{STORE_IOS}}", template.HTMLEscapeString(storeIOS),
		"{{STORE_ANDROID}}", template.HTMLEscapeString(storeAndroid),
		"{{NAME}}", template.HTMLEscapeString(name),
		"{{YEAR}}", strconv.Itoa(time.Now().Year()),
	).Replace(tmpl)

	return s.sendEmail(email, "You have unread activity on Hamsaya", htmlBody)
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

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Error("Resend API request failed", zap.String("to", to), zap.Error(err))
		return fmt.Errorf("failed to send email via Resend: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody bytes.Buffer
		_, _ = errBody.ReadFrom(resp.Body) // read response body into errBody
		s.logger.Error("Resend API error",
			zap.String("to", to),
			zap.Int("status", resp.StatusCode),
			zap.String("body", errBody.String()),
		)
		return fmt.Errorf("resend API returned status %d: %s", resp.StatusCode, errBody.String())
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
        .brand-icon { display: block; width: 64px; height: 64px; margin: 0 0 12px 0; border-radius: 14px; }
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
                {{if .IconURL}}<img class="brand-icon" src="{{.IconURL}}" alt="{{.AppName}}" width="64" height="64">{{end}}
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

//#nosec G101 -- HTML email template; "password" appears in copy, not as a credential
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
        .brand-icon { display: block; width: 64px; height: 64px; margin: 0 0 12px 0; border-radius: 14px; }
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
                {{if .IconURL}}<img class="brand-icon" src="{{.IconURL}}" alt="{{.AppName}}" width="64" height="64">{{end}}
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
        .brand-icon { display: inline-block; width: 72px; height: 72px; margin: 0 auto 14px auto; border-radius: 16px; }
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
                {{if .IconURL}}<img class="brand-icon" src="{{.IconURL}}" alt="{{.AppName}}" width="72" height="72">{{end}}
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

//#nosec G101 -- HTML email template; "password" appears in copy, not as a credential
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
        .brand-icon { display: block; width: 64px; height: 64px; margin: 0 0 12px 0; border-radius: 14px; }
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
                {{if .IconURL}}<img class="brand-icon" src="{{.IconURL}}" alt="{{.AppName}}" width="64" height="64">{{end}}
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
