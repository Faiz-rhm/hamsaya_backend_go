package notification

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"

	"go.uber.org/zap"
)

// freshP256Base64DER returns a valid PKCS8 P-256 key encoded two ways: a full
// PEM and the bare base64 DER body (no header lines) — the two forms a user may
// paste into APNS_KEY_P8.
func freshP256Base64DER(t *testing.T) (pemStr, bareB64 string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	pemStr = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	bareB64 = base64.StdEncoding.EncodeToString(der)
	return pemStr, bareB64
}

func TestNewAPNsClient_AcceptsPEM(t *testing.T) {
	pemStr, _ := freshP256Base64DER(t)
	_, err := NewAPNsClient(APNsConfig{
		KeyP8: pemStr, KeyID: "JGXW594YC8", TeamID: "37T82WQTNV", BundleID: "af.hamsaya",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("expected PEM key to parse, got: %v", err)
	}
}

func TestNewAPNsClient_AcceptsBareBase64DER(t *testing.T) {
	_, bare := freshP256Base64DER(t)
	_, err := NewAPNsClient(APNsConfig{
		KeyP8: bare, KeyID: "JGXW594YC8", TeamID: "37T82WQTNV", BundleID: "af.hamsaya",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("expected bare base64 DER to parse, got: %v", err)
	}
}

func TestBuildAPNsPayload_Alert(t *testing.T) {
	badge := 3
	out, err := buildAPNsPayload(&PushPayload{
		Title: "Hello",
		Body:  "World",
		Sound: "",
		Badge: &badge,
		Data:  map[string]string{"type": "MESSAGE", "notification_id": "abc"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	aps, ok := root["aps"].(map[string]any)
	if !ok {
		t.Fatalf("aps dict missing: %s", out)
	}
	alert, ok := aps["alert"].(map[string]any)
	if !ok {
		t.Fatalf("alert dict missing: %s", out)
	}
	if alert["title"] != "Hello" || alert["body"] != "World" {
		t.Errorf("alert wrong: %v", alert)
	}
	if aps["sound"] != "default" {
		t.Errorf("expected default sound, got %v", aps["sound"])
	}
	if aps["badge"].(float64) != 3 {
		t.Errorf("expected badge 3, got %v", aps["badge"])
	}
	// Custom data keys must be promoted to the JSON root for the Flutter handler.
	if root["type"] != "MESSAGE" || root["notification_id"] != "abc" {
		t.Errorf("data keys not at root: %s", out)
	}
}

func TestBuildAPNsPayload_Silent(t *testing.T) {
	out, err := buildAPNsPayload(&PushPayload{Silent: true, Data: map[string]string{"type": "SYNC"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var root map[string]any
	_ = json.Unmarshal(out, &root)
	aps := root["aps"].(map[string]any)
	if aps["content-available"].(float64) != 1 {
		t.Errorf("silent push must set content-available=1: %s", out)
	}
	if _, hasAlert := aps["alert"]; hasAlert {
		t.Errorf("silent push must not carry an alert: %s", out)
	}
}

func TestBuildAPNsPayload_DataCannotClobberAps(t *testing.T) {
	out, _ := buildAPNsPayload(&PushPayload{
		Title: "t", Body: "b",
		Data: map[string]string{"aps": "malicious"},
	})
	var root map[string]any
	_ = json.Unmarshal(out, &root)
	// The reserved "aps" key must remain the dictionary, not the data string.
	if _, ok := root["aps"].(map[string]any); !ok {
		t.Errorf("data key 'aps' clobbered the reserved dictionary: %s", out)
	}
}

func TestNewAPNsClient_RejectsBadKey(t *testing.T) {
	_, err := NewAPNsClient(APNsConfig{
		KeyP8:    "not-a-pem",
		KeyID:    "ABCDE12345",
		TeamID:   "FGHIJ67890",
		BundleID: "af.hamsaya",
	}, zap.NewNop())
	if err == nil {
		t.Fatal("expected error for malformed .p8 key")
	}
}

func TestNewAPNsClient_RequiresAllFields(t *testing.T) {
	_, err := NewAPNsClient(APNsConfig{KeyP8: "x"}, zap.NewNop())
	if err == nil {
		t.Fatal("expected error when KeyID/TeamID/BundleID are missing")
	}
}
