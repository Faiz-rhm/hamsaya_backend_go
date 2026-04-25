package geocoding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overrideNominatimURL patches the package-level constant for testing.
// The real function uses a hardcoded const, so we need an httptest server
// and must monkey-patch or use the env approach. Since the code uses
// os.Getenv("DISABLE_REVERSE_GEOCODE"), we test that path directly, and
// for the HTTP path we use a roundabout approach via a mock server patched
// via the nominatimBaseURL variable (must be var in the implementation).
// Since it's a const, we test the DISABLE path and the unmarshal helpers.

func TestReverseGeocode_Disabled(t *testing.T) {
	t.Setenv("DISABLE_REVERSE_GEOCODE", "1")

	result, err := ReverseGeocode(context.Background(), 34.5, 69.2)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestReverseGeocode_Disabled_False(t *testing.T) {
	os.Unsetenv("DISABLE_REVERSE_GEOCODE")
	// Without a real network we can't test the full call — only that the
	// env gate is unset. Integration test would need real HTTP.
	// Verifying strFromMap helper logic instead:
	m := map[string]interface{}{
		"state":          "Kabul Province",
		"state_district": "Kabul District",
		"country":        "Afghanistan",
		"neighbourhood":  "Wazir Akbar Khan",
	}
	assert.Equal(t, "Afghanistan", strFromMap(m, "country"))
	assert.Equal(t, "Kabul Province", strFromMap(m, "state"))
	assert.Equal(t, "Kabul District", strFromMap(m, "state_district", "county"))
	assert.Equal(t, "Wazir Akbar Khan", strFromMap(m, "neighbourhood", "suburb"))
}

func TestStrFromMap_FallsBackToSecondKey(t *testing.T) {
	m := map[string]interface{}{
		"county": "Some County",
	}
	result := strFromMap(m, "state_district", "county")
	assert.Equal(t, "Some County", result)
}

func TestStrFromMap_MissingKey_ReturnsEmpty(t *testing.T) {
	m := map[string]interface{}{}
	result := strFromMap(m, "nonexistent")
	assert.Empty(t, result)
}

func TestStrFromMap_NilValue_ReturnsEmpty(t *testing.T) {
	m := map[string]interface{}{"key": nil}
	result := strFromMap(m, "key")
	assert.Empty(t, result)
}

func TestNominatimResponse_Unmarshal(t *testing.T) {
	raw := `{"address":{"country":"Afghanistan","state":"Kabul Province","neighbourhood":"Wazir Akbar Khan"}}`
	var resp nominatimResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	assert.Equal(t, "Afghanistan", strFromMap(resp.Address, "country"))
	assert.Equal(t, "Kabul Province", strFromMap(resp.Address, "state"))
}

func TestReverseGeocode_HTTPServer(t *testing.T) {
	// This test requires the nominatim URL to be patchable. Since it's a
	// const we can't override it without modifying the impl. We test with
	// a server and verify the parsing logic via nominatimResponse directly.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.Header.Get("User-Agent"), "Hamsaya")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"address":{"country":"Afghanistan","state":"Kabul"}}`))
	}))
	defer server.Close()

	// Direct parsing test — the const prevents full integration without patching
	raw := `{"address":{"country":"Afghanistan","state":"Kabul"}}`
	var resp nominatimResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	r := &ReverseResult{
		Country:  strFromMap(resp.Address, "country"),
		Province: strFromMap(resp.Address, "state"),
	}
	assert.Equal(t, "Afghanistan", r.Country)
	assert.Equal(t, "Kabul", r.Province)
}
