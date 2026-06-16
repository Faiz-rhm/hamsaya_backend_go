package notification

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

// Why this exists: the Flutter app runs in Afghanistan where Google endpoints
// (fcmtoken.googleapis.com) are DNS-blocked, so iOS devices cannot mint an FCM
// token without a VPN — and therefore never receive push. APNs delivery,
// however, rides Apple's own infrastructure (api.push.apple.com), which is
// reachable. This client talks to APNs directly using a .p8 auth key, bypassing
// Firebase entirely for iOS. The on-device APNs token (getAPNSToken) is minted
// by iOS without any Google round-trip, so registration works offline of Google.
//
// Android continues to use FCM (Play Services has its own transport).

const (
	apnsProdHost    = "https://api.push.apple.com"
	apnsSandboxHost = "https://api.sandbox.push.apple.com"

	// Provider tokens are valid up to 60 min; Apple rejects tokens older than
	// that and rate-limits regeneration. Refresh a little under the limit.
	apnsJWTTTL = 50 * time.Minute
)

// ErrAPNsTokenInvalid is returned when APNs rejects the device token as
// unregistered/bad on BOTH environments, so the caller can prune it.
var ErrAPNsTokenInvalid = errors.New("apns device token invalid")

// APNsConfig holds the .p8 auth-key credentials. KeyP8 accepts a raw PEM, a
// single-line PEM with literal \n escapes, or a base64-encoded PEM blob
// (Dokploy-safe — same handling as the Firebase private key).
type APNsConfig struct {
	KeyP8      string // APNS_KEY_P8       — contents of the AuthKey_XXXX.p8
	KeyID      string // APNS_KEY_ID       — 10-char Key ID (e.g. M4YGKTH4JY)
	TeamID     string // APNS_TEAM_ID      — 10-char Team ID (e.g. 37T82WQTNV)
	BundleID   string // APNS_BUNDLE_ID    — apns-topic (e.g. af.hamsaya)
	Production bool   // APNS_PRODUCTION   — true for App Store/TestFlight tokens
}

// APNsClient sends pushes straight to Apple Push Notification service over
// HTTP/2 using JWT (ES256) provider authentication.
type APNsClient struct {
	cfg    APNsConfig
	key    *ecdsa.PrivateKey
	http   *http.Client
	logger *zap.Logger

	mu        sync.Mutex
	jwtCache  string
	jwtExpiry time.Time
}

// NewAPNsClient parses the .p8 key and prepares an HTTP/2 client. Returns an
// error if the key/IDs are malformed so startup can disable APNs cleanly.
func NewAPNsClient(cfg APNsConfig, logger *zap.Logger) (*APNsClient, error) {
	if cfg.KeyP8 == "" || cfg.KeyID == "" || cfg.TeamID == "" || cfg.BundleID == "" {
		return nil, fmt.Errorf("apns: KeyP8, KeyID, TeamID and BundleID are all required")
	}

	// Accept three shapes for the key:
	//   1. a full PEM (-----BEGIN PRIVATE KEY----- ... ), incl. \n-escaped,
	//   2. base64 of the whole PEM file (normalizePEM decodes it back to #1),
	//   3. the bare base64 PKCS8 DER body — the inner lines of the .p8 with the
	//      header/footer stripped (common when only the key material is pasted).
	pemStr := normalizePEM(cfg.KeyP8)
	var der []byte
	if block, _ := pem.Decode([]byte(pemStr)); block != nil {
		der = block.Bytes
	} else {
		decoded, dErr := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(pemStr), ""))
		if dErr != nil {
			return nil, fmt.Errorf("apns: key is neither PEM nor base64 DER: %w", dErr)
		}
		der = decoded
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("apns: failed to parse PKCS8 .p8 key: %w", err)
	}
	ecKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("apns: .p8 key is not an ECDSA key")
	}

	// APNs requires HTTP/2. Build a transport that negotiates h2.
	tr := &http.Transport{ForceAttemptHTTP2: true}
	if err := http2.ConfigureTransport(tr); err != nil {
		return nil, fmt.Errorf("apns: failed to configure HTTP/2 transport: %w", err)
	}

	logger.Info("APNs client initialized",
		zap.String("key_id", cfg.KeyID),
		zap.String("bundle", cfg.BundleID),
		zap.Bool("production", cfg.Production),
	)

	return &APNsClient{
		cfg:    cfg,
		key:    ecKey,
		http:   &http.Client{Transport: tr, Timeout: 15 * time.Second},
		logger: logger,
	}, nil
}

// providerToken returns a cached JWT, regenerating it when near expiry. APNs
// accepts the same token across requests for up to an hour, so we sign at most
// ~once per 50 minutes.
func (a *APNsClient) providerToken() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.jwtCache != "" && time.Now().Before(a.jwtExpiry) {
		return a.jwtCache, nil
	}

	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": a.cfg.TeamID,
		"iat": now.Unix(),
	})
	tok.Header["kid"] = a.cfg.KeyID

	signed, err := tok.SignedString(a.key)
	if err != nil {
		return "", fmt.Errorf("apns: failed to sign provider token: %w", err)
	}
	a.jwtCache = signed
	a.jwtExpiry = now.Add(apnsJWTTTL)
	return signed, nil
}

// SendNotification delivers a push to a single APNs device token. It tries the
// configured environment first and, on an environment-mismatch rejection,
// retries the other gateway once — so a TestFlight (production) token still
// works even if the server defaults to sandbox, and vice versa. Returns
// ErrAPNsTokenInvalid when the token is dead on both environments.
func (a *APNsClient) SendNotification(ctx context.Context, deviceToken string, payload *PushPayload) error {
	body, err := buildAPNsPayload(payload)
	if err != nil {
		return err
	}

	primary, secondary := apnsProdHost, apnsSandboxHost
	if !a.cfg.Production {
		primary, secondary = apnsSandboxHost, apnsProdHost
	}

	err = a.send(ctx, primary, deviceToken, body, payload)
	if errors.Is(err, errAPNsWrongEnv) {
		// Token belongs to the other environment — retry once there.
		return a.send(ctx, secondary, deviceToken, body, payload)
	}
	return err
}

// errAPNsWrongEnv signals the device token is valid but was used against the
// wrong gateway; SendNotification retries on the opposite host.
var errAPNsWrongEnv = errors.New("apns wrong environment")

func (a *APNsClient) send(ctx context.Context, host, deviceToken string, body []byte, payload *PushPayload) error {
	jwtTok, err := a.providerToken()
	if err != nil {
		return err
	}

	url := host + "/3/device/" + deviceToken
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apns: build request: %w", err)
	}
	req.Header.Set("authorization", "bearer "+jwtTok)
	req.Header.Set("apns-topic", a.cfg.BundleID)
	req.Header.Set("content-type", "application/json")

	pushType := "alert"
	priority := "10"
	if payload.Silent {
		pushType = "background"
		priority = "5"
	}
	req.Header.Set("apns-push-type", pushType)
	req.Header.Set("apns-priority", priority)
	req.Header.Set("apns-expiration", strconv.FormatInt(time.Now().Add(apnsExpirationSeconds*time.Second).Unix(), 10))

	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("apns: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	var apnsErr struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(respBody, &apnsErr)

	switch apnsErr.Reason {
	case "Unregistered":
		// 410 — the device token is permanently dead (app uninstalled). Prune it.
		return ErrAPNsTokenInvalid
	case "BadDeviceToken", "DeviceTokenNotForTopic", "BadEnvironmentKeyInToken":
		// The token belongs to the OTHER APNs environment (a sandbox token sent
		// to production, or vice versa). A dev/Xcode build mints a sandbox token
		// while APNS_PRODUCTION=true targets production → "BadEnvironmentKeyInToken";
		// a TestFlight token against sandbox → the same class of error. Signal a
		// one-shot retry on the opposite gateway so both build types deliver
		// regardless of the configured default.
		return errAPNsWrongEnv
	case "ExpiredProviderToken", "InvalidProviderToken", "MissingProviderToken":
		// Our JWT is stale/bad — drop the cache so the next call re-signs.
		a.mu.Lock()
		a.jwtCache = ""
		a.mu.Unlock()
		return fmt.Errorf("apns: provider token rejected (%s)", apnsErr.Reason)
	default:
		return fmt.Errorf("apns: push rejected status=%d reason=%q", resp.StatusCode, apnsErr.Reason)
	}
}

// buildAPNsPayload renders the aps dictionary plus custom data keys. Mirrors the
// FCM APNSConfig the app already expects (alert vs background, sound, badge,
// and flat string data keys read by the Flutter notification handler).
func buildAPNsPayload(p *PushPayload) ([]byte, error) {
	aps := map[string]any{}
	if p.Silent {
		aps["content-available"] = 1
	} else {
		alert := map[string]string{"title": p.Title, "body": p.Body}
		aps["alert"] = alert
		sound := p.Sound
		if sound == "" {
			sound = "default"
		}
		aps["sound"] = sound
	}
	if p.Badge != nil {
		aps["badge"] = *p.Badge
	}

	root := map[string]any{"aps": aps}
	for k, v := range p.Data {
		// Don't let a stray "aps" data key clobber the reserved dictionary.
		if k == "aps" {
			continue
		}
		root[k] = v
	}

	// FCM identifier so the firebase_messaging Flutter plugin recognises this
	// direct-APNs push as one of its messages. Without `gcm.message_id` the
	// plugin ignores the notification — onMessage / onMessageOpenedApp /
	// getInitialMessage never fire, so a tap doesn't deep-link to the target
	// screen. The custom data keys (type, post_id, …) are already at the root,
	// which the plugin maps into RemoteMessage.data for the tap router.
	if _, exists := root["gcm.message_id"]; !exists {
		id := p.Data["notification_id"]
		if id == "" {
			id = p.Data["type"]
		}
		root["gcm.message_id"] = id
	}

	out, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("apns: marshal payload: %w", err)
	}
	return out, nil
}
