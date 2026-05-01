package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_System_RoleGating verifies that /admin/system/* endpoints reject
// every role except super_admin. Runs against the real Postgres+miniredis
// stack via setupE2E; auto-skipped when DB unreachable.
func TestE2E_System_RoleGating(t *testing.T) {
	env := setupE2E(t)
	defer env.cleanupTestData(t, "sysrole+%@example.com")

	cases := []struct {
		name       string
		promote    func(*testEnv, *testing.T, string)
		wantStatus int
	}{
		{name: "regular user", promote: func(*testEnv, *testing.T, string) {}, wantStatus: http.StatusForbidden},
		{name: "moderator", promote: (*testEnv).makeModerator, wantStatus: http.StatusForbidden},
		{name: "admin", promote: (*testEnv).makeAdmin, wantStatus: http.StatusForbidden},
		{name: "super_admin", promote: (*testEnv).makeSuperAdmin, wantStatus: http.StatusOK},
	}

	for i, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			email := fmt.Sprintf("sysrole+%d-%d@example.com", testNonce(t), i)
			toks := register(t, env, email, "Pass123!@#xyz")
			tc.promote(env, t, toks.UserID)

			req, _ := http.NewRequest(http.MethodGet, env.url("/api/v1/admin/system/build-info"), nil)
			req.Header.Set("Authorization", "Bearer "+toks.AccessToken)
			resp := env.do(req)
			body := readAndClose(t, resp)
			assert.Equal(t, tc.wantStatus, resp.StatusCode, "body=%s", body)
		})
	}
}

// TestE2E_System_FeatureFlagToggle exercises the full flag lifecycle: list
// returns seeded flags, PUT flips a flag, list reflects the new value.
func TestE2E_System_FeatureFlagToggle(t *testing.T) {
	env := setupE2E(t)
	defer env.cleanupTestData(t, "flagtoggle+%@example.com")

	// Idempotently seed the flag in case the migration's seed got wiped or
	// never ran on this DB. Uses ON CONFLICT to stay safe against re-runs.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := env.db.Pool.Exec(ctx, `
		INSERT INTO feature_flags (key, enabled, description) VALUES
			('registration_open', TRUE, 'Allow new user registrations.')
		ON CONFLICT (key) DO NOTHING
	`); err != nil {
		t.Fatalf("seed feature_flags: %v", err)
	}

	email := fmt.Sprintf("flagtoggle+%d@example.com", testNonce(t))
	toks := register(t, env, email, "Pass123!@#xyz")
	env.makeSuperAdmin(t, toks.UserID)
	auth := "Bearer " + toks.AccessToken

	// 1. List
	req, _ := http.NewRequest(http.MethodGet, env.url("/api/v1/admin/system/flags"), nil)
	req.Header.Set("Authorization", auth)
	resp := env.do(req)
	listBody := readAndClose(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, "flags list failed: %s", listBody)
	require.Contains(t, listBody, "registration_open", "seeded flag missing")

	// 2. Toggle registration_open OFF
	put, _ := http.NewRequest(http.MethodPut, env.url("/api/v1/admin/system/flags/registration_open"),
		bytes.NewBufferString(`{"enabled": false}`))
	put.Header.Set("Authorization", auth)
	put.Header.Set("Content-Type", "application/json")
	respPut := env.do(put)
	rawPut := readAndClose(t, respPut)
	require.Equal(t, http.StatusOK, respPut.StatusCode, "toggle failed: %s", rawPut)

	// 3. Re-list and parse to verify the new value stuck.
	req2, _ := http.NewRequest(http.MethodGet, env.url("/api/v1/admin/system/flags"), nil)
	req2.Header.Set("Authorization", auth)
	resp2 := env.do(req2)
	rawList := readAndClose(t, resp2)

	var listOut struct {
		Data struct {
			Flags []struct {
				Key     string `json:"Key"`
				Enabled bool   `json:"Enabled"`
			} `json:"flags"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(rawList), &listOut))
	var found bool
	for _, f := range listOut.Data.Flags {
		if f.Key == "registration_open" {
			assert.False(t, f.Enabled, "registration_open should be off after toggle")
			found = true
		}
	}
	assert.True(t, found, "registration_open not in list response")

	// 4. Restore for repeatability.
	restore, _ := http.NewRequest(http.MethodPut, env.url("/api/v1/admin/system/flags/registration_open"),
		bytes.NewBufferString(`{"enabled": true}`))
	restore.Header.Set("Authorization", auth)
	restore.Header.Set("Content-Type", "application/json")
	_ = readAndClose(t, env.do(restore))
}

// TestE2E_System_RejectUnknownFlag verifies that toggling a key not seeded
// via migration is rejected — the catalog of flags lives in source.
func TestE2E_System_RejectUnknownFlag(t *testing.T) {
	env := setupE2E(t)
	defer env.cleanupTestData(t, "unknownflag+%@example.com")

	email := fmt.Sprintf("unknownflag+%d@example.com", testNonce(t))
	toks := register(t, env, email, "Pass123!@#xyz")
	env.makeSuperAdmin(t, toks.UserID)

	req, _ := http.NewRequest(http.MethodPut,
		env.url("/api/v1/admin/system/flags/this_flag_does_not_exist"),
		strings.NewReader(`{"enabled": true}`))
	req.Header.Set("Authorization", "Bearer "+toks.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp := env.do(req)
	body := readAndClose(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "expected 400 for unknown flag, got body=%s", body)
}
