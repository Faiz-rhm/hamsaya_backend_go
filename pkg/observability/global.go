package observability

import (
	"context"
	"sync/atomic"
)

// global holds the application-wide Metrics pointer. Services reach
// into this via the package-level Record* helpers so they don't have
// to pass *Metrics through every constructor. The pointer is atomic
// so test code can swap it without a data race.
//
// When global is nil (observability disabled, init failed, or never
// called SetGlobal), every Record* call is a no-op — services never
// have to nil-check before calling.
var global atomic.Pointer[Metrics]

// SetGlobal installs the application-wide metrics handle. Call once
// from main.go after NewTelemetry succeeds. Safe to call with nil
// (e.g. when telemetry is disabled) — Record* helpers become no-ops.
func SetGlobal(m *Metrics) {
	global.Store(m)
}

// loadGlobal returns the current Metrics pointer (may be nil).
func loadGlobal() *Metrics {
	return global.Load()
}

// RecordPostCreated bumps the posts_created_total counter. Skipped
// silently if metrics are disabled.
func RecordPostCreated(ctx context.Context, postType string) {
	if m := loadGlobal(); m != nil {
		m.RecordPostCreated(ctx, postType)
	}
}

// RecordUserCreated bumps the users_created_total counter. provider
// is the signup channel — "email", "google", "apple", "facebook".
func RecordUserCreated(ctx context.Context, provider string) {
	if m := loadGlobal(); m != nil {
		m.RecordUserCreated(ctx, provider)
	}
}

// RecordMessageCreated bumps the messages_created_total counter.
func RecordMessageCreated(ctx context.Context) {
	if m := loadGlobal(); m != nil {
		m.RecordMessageCreated(ctx)
	}
}

// WebSocketConnected increments the active-connections gauge.
func WebSocketConnected(ctx context.Context) {
	if m := loadGlobal(); m != nil {
		m.WebSocketConnected(ctx)
	}
}

// WebSocketDisconnected decrements the active-connections gauge.
func WebSocketDisconnected(ctx context.Context) {
	if m := loadGlobal(); m != nil {
		m.WebSocketDisconnected(ctx)
	}
}
