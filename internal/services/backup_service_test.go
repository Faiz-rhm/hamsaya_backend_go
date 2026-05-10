package services

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyTier walks the calendar logic. classifyTier picks:
//   adhoc  → triggeredBy == "admin"   (always wins)
//   monthly → date is the 1st of any month
//   weekly  → date is a Sunday
//   daily   → everything else
// Tier classification is the only piece of policy-defining logic in
// the backup service that runs without a DB, so it's the highest-value
// pure-logic test.
func TestClassifyTier(t *testing.T) {
	cases := []struct {
		name        string
		ts          time.Time
		triggeredBy string
		want        string
	}{
		{
			name:        "admin trigger always adhoc",
			ts:          time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC), // Sunday + 1st-of-month-ish
			triggeredBy: "admin",
			want:        "adhoc",
		},
		{
			name:        "admin trigger on 1st still adhoc",
			ts:          time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			triggeredBy: "admin",
			want:        "adhoc",
		},
		{
			name:        "1st of month, cron → monthly",
			ts:          time.Date(2026, 7, 1, 3, 0, 0, 0, time.UTC), // Wed
			triggeredBy: "cron",
			want:        "monthly",
		},
		{
			name:        "Sunday non-1st, cron → weekly",
			ts:          time.Date(2026, 5, 10, 3, 0, 0, 0, time.UTC), // Sunday May 10
			triggeredBy: "cron",
			want:        "weekly",
		},
		{
			name:        "Tuesday non-1st, cron → daily",
			ts:          time.Date(2026, 5, 12, 3, 0, 0, 0, time.UTC),
			triggeredBy: "cron",
			want:        "daily",
		},
		{
			name:        "Monday non-1st, cron → daily",
			ts:          time.Date(2026, 5, 11, 3, 0, 0, 0, time.UTC),
			triggeredBy: "cron",
			want:        "daily",
		},
		{
			name:        "1st of month + Sunday → monthly (1st takes precedence)",
			ts:          time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC), // Sunday
			triggeredBy: "cron",
			want:        "monthly",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyTier(tc.ts, tc.triggeredBy)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestWritePassphraseFile_RoundTrip(t *testing.T) {
	pass := "this-is-a-secret-passphrase-32bytes!!!"
	path, err := writePassphraseFile(pass)
	require.NoError(t, err)
	defer os.Remove(path)

	// File must exist with the exact passphrase content.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, pass, string(data))

	// Permissions must be 0600 — gpg's --passphrase-file argument is
	// only secure when the file isn't world-readable. If a refactor
	// breaks this contract, dumps could leak passphrases on a shared
	// host.
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWritePassphraseFile_Empty(t *testing.T) {
	// Empty passphrase still writes a file (gpg will reject at runtime
	// but the writer's job is just to produce a 0600 file).
	path, err := writePassphraseFile("")
	require.NoError(t, err)
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "", string(data))
}

func TestDefaultRetention_Sane(t *testing.T) {
	// Sanity — protect against accidental edits that would let the GFS
	// rotation drift to absurd values. Industry-standard targets:
	//  ≤ 14 daily / ≤ 8 weekly / ≤ 24 monthly.
	r := defaultRetention
	assert.Greater(t, r.Daily, 0)
	assert.LessOrEqual(t, r.Daily, 14)
	assert.Greater(t, r.Weekly, 0)
	assert.LessOrEqual(t, r.Weekly, 8)
	assert.Greater(t, r.Monthly, 0)
	assert.LessOrEqual(t, r.Monthly, 24)
	assert.Greater(t, r.FailedDays, 0)
}
