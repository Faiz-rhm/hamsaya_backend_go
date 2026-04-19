package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hamsaya/backend/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestEmailService(cfg *config.EmailConfig) *EmailService {
	svc := NewEmailService(cfg, zap.NewNop())
	return svc
}

func TestEmailService_SendEmail_NotConfigured(t *testing.T) {
	svc := newTestEmailService(&config.EmailConfig{})

	err := svc.sendEmail("to@example.com", "subject", "<p>body</p>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email not configured")
}

func TestEmailService_SendEmailResend_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "to@example.com", body["to"].([]interface{})[0])
		assert.Equal(t, "Test Subject", body["subject"])

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer ts.Close()

	cfg := &config.EmailConfig{ResendAPIKey: "test-api-key", From: "noreply@hamsaya.com"}
	svc := NewEmailService(cfg, zap.NewNop())
	// Override the Resend URL to point to test server by swapping httpClient target
	// Since the URL is hardcoded in sendEmailResend, test via full flow but intercept
	// at the transport level
	svc.httpClient = &http.Client{
		Transport: &rewriteTransport{target: ts.URL},
	}

	err := svc.sendEmailResend("to@example.com", "Test Subject", "<p>hello</p>")
	require.NoError(t, err)
}

func TestEmailService_SendEmailResend_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer ts.Close()

	cfg := &config.EmailConfig{ResendAPIKey: "bad-key"}
	svc := NewEmailService(cfg, zap.NewNop())
	svc.httpClient = &http.Client{
		Transport: &rewriteTransport{target: ts.URL},
	}

	err := svc.sendEmailResend("to@example.com", "subject", "<p>body</p>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestEmailService_RenderTemplate(t *testing.T) {
	svc := newTestEmailService(&config.EmailConfig{})

	data := EmailData{
		RecipientName:  "Test User",
		RecipientEmail: "test@example.com",
		Subject:        "Verify your email",
		Token:          "123456",
		ExpiresIn:      "24 hours",
		AppName:        "Hamsaya",
		AppURL:         "https://hamsaya.com",
		SupportEmail:   "support@hamsaya.com",
		Year:           "2026",
	}

	t.Run("verification template", func(t *testing.T) {
		html, err := svc.renderTemplate(verificationEmailTemplate, data)
		require.NoError(t, err)
		assert.Contains(t, html, "123456")
		assert.Contains(t, html, "Test User")
		assert.Contains(t, html, "24 hours")
	})

	t.Run("password reset template", func(t *testing.T) {
		html, err := svc.renderTemplate(passwordResetEmailTemplate, data)
		require.NoError(t, err)
		assert.Contains(t, html, "123456")
	})

	t.Run("welcome template", func(t *testing.T) {
		html, err := svc.renderTemplate(welcomeEmailTemplate, data)
		require.NoError(t, err)
		assert.Contains(t, html, "Test User")
		assert.Contains(t, html, "Hamsaya")
	})

	t.Run("password changed template", func(t *testing.T) {
		html, err := svc.renderTemplate(passwordChangedEmailTemplate, data)
		require.NoError(t, err)
		assert.Contains(t, html, "Test User")
	})
}

func TestEmailService_SendVerificationEmail_NoConfig(t *testing.T) {
	svc := newTestEmailService(&config.EmailConfig{})
	err := svc.SendVerificationEmail("user@example.com", "User", "123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email not configured")
}

func TestEmailService_SendPasswordResetEmail_NoConfig(t *testing.T) {
	svc := newTestEmailService(&config.EmailConfig{})
	err := svc.SendPasswordResetEmail("user@example.com", "User", "654321")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email not configured")
}

func TestEmailService_SendWelcomeEmail_NoConfig(t *testing.T) {
	svc := newTestEmailService(&config.EmailConfig{})
	err := svc.SendWelcomeEmail("user@example.com", "User")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email not configured")
}

func TestEmailService_SendPasswordChangedEmail_NoConfig(t *testing.T) {
	svc := newTestEmailService(&config.EmailConfig{})
	err := svc.SendPasswordChangedEmail("user@example.com", "User")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email not configured")
}

func TestEmailService_SendVerificationEmail_WithResend(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer ts.Close()

	cfg := &config.EmailConfig{ResendAPIKey: "test-key", From: "noreply@hamsaya.com"}
	svc := NewEmailService(cfg, zap.NewNop())
	svc.httpClient = &http.Client{Transport: &rewriteTransport{target: ts.URL}}

	err := svc.SendVerificationEmail("user@example.com", "Test User", "999888")
	require.NoError(t, err)
}

// rewriteTransport redirects all requests to a test server URL
type rewriteTransport struct {
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Host = req.URL.Host // keep path/query
	parsedTarget, _ := http.NewRequest("GET", t.target, nil)
	req.URL.Scheme = parsedTarget.URL.Scheme
	req.URL.Host = parsedTarget.URL.Host
	return http.DefaultTransport.RoundTrip(req)
}
