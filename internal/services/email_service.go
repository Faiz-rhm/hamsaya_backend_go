package services

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"

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
}

// SendVerificationEmail sends an email verification link
func (s *EmailService) SendVerificationEmail(email, name, verificationToken string) error {
	data := EmailData{
		RecipientName:  name,
		RecipientEmail: email,
		Subject:        "Verify Your Email Address",
		VerifyURL:      fmt.Sprintf("https://app.hamsaya.com/verify-email?token=%s", verificationToken),
		Token:          verificationToken,
		ExpiresIn:      "24 hours",
		AppName:        "Hamsaya",
		AppURL:         "https://hamsaya.com",
		SupportEmail:   "support@hamsaya.com",
	}

	htmlBody, err := s.renderTemplate(verificationEmailTemplate, data)
	if err != nil {
		s.logger.Error("Failed to render verification email template", zap.Error(err))
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return s.sendEmail(email, data.Subject, htmlBody)
}

// SendPasswordResetEmail sends a password reset link
func (s *EmailService) SendPasswordResetEmail(email, name, resetToken string) error {
	data := EmailData{
		RecipientName:  name,
		RecipientEmail: email,
		Subject:        "Reset Your Password",
		ResetURL:       fmt.Sprintf("https://app.hamsaya.com/reset-password?token=%s", resetToken),
		Token:          resetToken,
		ExpiresIn:      "15 minutes",
		AppName:        "Hamsaya",
		AppURL:         "https://hamsaya.com",
		SupportEmail:   "support@hamsaya.com",
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
	}

	htmlBody, err := s.renderTemplate(passwordChangedEmailTemplate, data)
	if err != nil {
		s.logger.Error("Failed to render password changed email template", zap.Error(err))
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return s.sendEmail(email, data.Subject, htmlBody)
}

// sendEmail sends an email using SMTP
func (s *EmailService) sendEmail(to, subject, htmlBody string) error {
	// Skip if SMTP is not configured
	if s.cfg.SMTPHost == "" || s.cfg.SMTPPort == "" {
		s.logger.Warn("SMTP not configured, skipping email send",
			zap.String("to", to),
			zap.String("subject", subject),
		)
		return nil
	}

	// Prepare email headers and body
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

	// Build message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + htmlBody

	// SMTP authentication
	auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.SMTPHost)

	// Send email
	addr := s.cfg.SMTPHost + ":" + s.cfg.SMTPPort
	err := smtp.SendMail(addr, auth, from, []string{to}, []byte(message))
	if err != nil {
		s.logger.Error("Failed to send email",
			zap.String("to", to),
			zap.String("subject", subject),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("Email sent successfully",
		zap.String("to", to),
		zap.String("subject", subject),
	)

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

// Email templates
const verificationEmailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Subject}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .header h1 {
            color: #2563eb;
            margin: 0;
        }
        .content {
            margin-bottom: 30px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #2563eb;
            color: #ffffff !important;
            text-decoration: none;
            border-radius: 6px;
            font-weight: 600;
            text-align: center;
        }
        .button:hover {
            background-color: #1d4ed8;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e5e7eb;
            font-size: 14px;
            color: #6b7280;
        }
        .code {
            background-color: #f3f4f6;
            padding: 12px;
            border-radius: 4px;
            font-family: monospace;
            font-size: 16px;
            text-align: center;
            margin: 20px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.AppName}}</h1>
        </div>
        <div class="content">
            <h2>Hi {{.RecipientName}},</h2>
            <p>Thank you for signing up with {{.AppName}}! We're excited to have you on board.</p>
            <p>To complete your registration and verify your email address, please click the button below:</p>
            <p style="text-align: center; margin: 30px 0;">
                <a href="{{.VerifyURL}}" class="button">Verify Email Address</a>
            </p>
            <p>Or copy and paste this link into your browser:</p>
            <div class="code">{{.VerifyURL}}</div>
            <p><strong>This link will expire in {{.ExpiresIn}}.</strong></p>
            <p>If you didn't create an account with {{.AppName}}, you can safely ignore this email.</p>
        </div>
        <div class="footer">
            <p>Need help? Contact us at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a></p>
            <p>&copy; 2024 {{.AppName}}. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`

const passwordResetEmailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Subject}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .header h1 {
            color: #2563eb;
            margin: 0;
        }
        .content {
            margin-bottom: 30px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #dc2626;
            color: #ffffff !important;
            text-decoration: none;
            border-radius: 6px;
            font-weight: 600;
            text-align: center;
        }
        .button:hover {
            background-color: #b91c1c;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e5e7eb;
            font-size: 14px;
            color: #6b7280;
        }
        .warning {
            background-color: #fef2f2;
            border-left: 4px solid #dc2626;
            padding: 12px;
            margin: 20px 0;
        }
        .code {
            background-color: #f3f4f6;
            padding: 12px;
            border-radius: 4px;
            font-family: monospace;
            font-size: 16px;
            text-align: center;
            margin: 20px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.AppName}}</h1>
        </div>
        <div class="content">
            <h2>Hi {{.RecipientName}},</h2>
            <p>We received a request to reset your password for your {{.AppName}} account.</p>
            <p>To reset your password, please click the button below:</p>
            <p style="text-align: center; margin: 30px 0;">
                <a href="{{.ResetURL}}" class="button">Reset Password</a>
            </p>
            <p>Or copy and paste this link into your browser:</p>
            <div class="code">{{.ResetURL}}</div>
            <p><strong>This link will expire in {{.ExpiresIn}}.</strong></p>
            <div class="warning">
                <strong>⚠️ Security Notice:</strong><br>
                If you didn't request a password reset, please ignore this email or contact our support team immediately. Your password will remain unchanged.
            </div>
        </div>
        <div class="footer">
            <p>Need help? Contact us at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a></p>
            <p>&copy; 2024 {{.AppName}}. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`

const welcomeEmailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Subject}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .header h1 {
            color: #2563eb;
            margin: 0;
        }
        .content {
            margin-bottom: 30px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #2563eb;
            color: #ffffff !important;
            text-decoration: none;
            border-radius: 6px;
            font-weight: 600;
            text-align: center;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e5e7eb;
            font-size: 14px;
            color: #6b7280;
        }
        .features {
            background-color: #f9fafb;
            padding: 20px;
            border-radius: 6px;
            margin: 20px 0;
        }
        .features ul {
            margin: 0;
            padding-left: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to {{.AppName}}! 🎉</h1>
        </div>
        <div class="content">
            <h2>Hi {{.RecipientName}},</h2>
            <p>Welcome to {{.AppName}}! We're thrilled to have you join our community.</p>
            <div class="features">
                <p><strong>Here's what you can do with {{.AppName}}:</strong></p>
                <ul>
                    <li>Connect with neighbors in your area</li>
                    <li>Discover local events and activities</li>
                    <li>Buy and sell items in your neighborhood</li>
                    <li>Share updates and build your community</li>
                </ul>
            </div>
            <p style="text-align: center; margin: 30px 0;">
                <a href="{{.AppURL}}" class="button">Get Started</a>
            </p>
            <p>If you have any questions or need assistance, our support team is here to help!</p>
        </div>
        <div class="footer">
            <p>Need help? Contact us at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a></p>
            <p>&copy; 2024 {{.AppName}}. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`

const passwordChangedEmailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Subject}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .header h1 {
            color: #2563eb;
            margin: 0;
        }
        .content {
            margin-bottom: 30px;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e5e7eb;
            font-size: 14px;
            color: #6b7280;
        }
        .warning {
            background-color: #fef2f2;
            border-left: 4px solid #dc2626;
            padding: 12px;
            margin: 20px 0;
        }
        .success {
            background-color: #f0fdf4;
            border-left: 4px solid #16a34a;
            padding: 12px;
            margin: 20px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.AppName}}</h1>
        </div>
        <div class="content">
            <h2>Hi {{.RecipientName}},</h2>
            <div class="success">
                <strong>✓ Password Changed Successfully</strong><br>
                Your password has been changed successfully.
            </div>
            <p>This is a confirmation that your password for your {{.AppName}} account has been changed.</p>
            <p>If you made this change, no further action is required.</p>
            <div class="warning">
                <strong>⚠️ Didn't make this change?</strong><br>
                If you did not change your password, please contact our support team immediately at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a>. Your account security may be compromised.
            </div>
        </div>
        <div class="footer">
            <p>Need help? Contact us at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a></p>
            <p>&copy; 2024 {{.AppName}}. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`
