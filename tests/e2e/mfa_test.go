package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_MFA_EnrollTOTP(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-mfaenroll-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-mfaenroll-%") })

	tokens := register(t, env, email, "Password123!")

	body := `{"type":"TOTP"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/mfa/enroll"), tokens.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "MFA enroll failed: %s", string(raw))

	var out struct {
		Data struct {
			FactorID string `json:"factor_id"`
			QRCode   string `json:"qr_code"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.NotEmpty(t, out.Data.FactorID)
}

func TestE2E_MFA_EnrollTOTP_InvalidTypeReturns400(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-mfabadtype-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-mfabadtype-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/mfa/enroll"), tokens.AccessToken, `{"type":"SMS"}`))
	defer func() { _ = resp.Body.Close() }()
	// SMS is not supported — handler returns 400 before calling service
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_MFA_VerifyEnrollment_InvalidCodeReturns4xx(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-mfaverify-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-mfaverify-%") })

	tokens := register(t, env, email, "Password123!")

	// Enroll first to get a real factor_id
	enrollResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/mfa/enroll"), tokens.AccessToken, `{"type":"TOTP"}`))
	defer func() { _ = enrollResp.Body.Close() }()
	enrollRaw, _ := io.ReadAll(enrollResp.Body)
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	var enrollOut struct {
		Data struct{ FactorID string `json:"factor_id"` } `json:"data"`
	}
	require.NoError(t, json.Unmarshal(enrollRaw, &enrollOut))
	require.NotEmpty(t, enrollOut.Data.FactorID)

	// Verify with wrong code — TOTP check fails → 4xx
	body := fmt.Sprintf(`{"factor_id":%q,"code":"000000"}`, enrollOut.Data.FactorID)
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/mfa/verify-enrollment"), tokens.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	assert.True(t, resp.StatusCode >= 400 && resp.StatusCode < 500,
		"expected 4xx for invalid TOTP code, got %d", resp.StatusCode)
}

func TestE2E_MFA_DisableMFA(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-mfadisable-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-mfadisable-%") })

	tokens := register(t, env, email, "Password123!")

	// Disable MFA (no factors enrolled — just clears state)
	body := `{"password":"Password123!"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/mfa/disable"), tokens.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "MFA disable failed: %s", string(raw))
}

func TestE2E_MFA_DisableMFA_WrongPasswordReturns401(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-mfadisbad-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-mfadisbad-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/mfa/disable"), tokens.AccessToken, `{"password":"WrongPass!"}`))
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestE2E_MFA_RegenerateBackupCodes(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-mfabackup-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-mfabackup-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/mfa/backup-codes/regenerate"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "regenerate backup codes failed: %s", string(raw))

	var out struct {
		Data struct {
			BackupCodes []string `json:"backup_codes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.NotEmpty(t, out.Data.BackupCodes)
}

func TestE2E_MFA_GetBackupCodesCount(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-mfacount-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-mfacount-%") })

	tokens := register(t, env, email, "Password123!")

	// Generate codes first so count > 0
	_ = env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/mfa/backup-codes/regenerate"), tokens.AccessToken, "")).Body.Close()

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/mfa/backup-codes/count"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get backup codes count failed: %s", string(raw))
}
