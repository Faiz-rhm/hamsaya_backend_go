// Package alert provides best-effort out-of-band incident notifications via a
// webhook (Slack / Discord / any endpoint accepting a JSON body). It exists so
// the API is not blind to crashes: panics are logged durably AND pushed to a
// channel a human watches. No external SDK / vendoring required.
//
// Enable by setting ALERT_WEBHOOK_URL. When unset, Alert is a no-op.
package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Alerter posts short incident messages to a webhook. Safe to use when nil or
// when the URL is empty (both no-op), so callers never need to guard.
type Alerter struct {
	url    string
	client *http.Client
	logger *zap.Logger
}

// New returns an Alerter. A nil/empty url yields a no-op alerter.
func New(webhookURL string, logger *zap.Logger) *Alerter {
	return &Alerter{
		url:    webhookURL,
		client: &http.Client{Timeout: 5 * time.Second},
		logger: logger,
	}
}

// Enabled reports whether a webhook is configured.
func (a *Alerter) Enabled() bool { return a != nil && a.url != "" }

// Alert sends a best-effort notification. It never blocks the caller for long
// (5s client timeout) and never returns an error — delivery failures are
// logged, not propagated, because alerting must not break the request path.
// The body carries both "text" (Slack) and "content" (Discord) so a single
// payload works with either provider, plus structured fields for generic sinks.
func (a *Alerter) Alert(ctx context.Context, title, detail string) {
	if !a.Enabled() {
		return
	}
	msg := title
	if detail != "" {
		msg = title + "\n" + detail
	}
	payload, err := json.Marshal(map[string]string{
		"text":    msg, // Slack
		"content": msg, // Discord
		"title":   title,
		"detail":  detail,
	})
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		if a.logger != nil {
			a.logger.Warn("alert webhook delivery failed", zap.Error(err))
		}
		return
	}
	_ = resp.Body.Close()
}
