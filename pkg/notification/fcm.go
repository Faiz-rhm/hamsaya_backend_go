package notification

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

// normalizePEM accepts either a raw PEM string, a single-line PEM with literal
// "\n" escape sequences, or a base64-encoded PEM blob (no header line) and
// returns a PEM with real newlines. Base64 form is preferred when the value is
// pasted into env panels that mangle newlines (e.g. Dokploy compose .env).
func normalizePEM(value string) string {
	if strings.Contains(value, "-----BEGIN") {
		return strings.ReplaceAll(value, `\n`, "\n")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err == nil && strings.Contains(string(decoded), "-----BEGIN") {
		return string(decoded)
	}
	return value
}

// apnsExpirationSeconds is how long APNs should retry delivery if the device
// is offline or asleep. Without this header, APNs uses an opaque default and
// can silently drop messages — the root cause of inconsistent iOS push
// delivery (some notifications arrive, others never do). 1 hour is a safe
// default for chat/alert notifications where staleness >1h has no value.
const apnsExpirationSeconds = 3600

// FCMClient represents a Firebase Cloud Messaging client
type FCMClient struct {
	client *messaging.Client
	logger *zap.Logger
}

// FCMConfig holds credentials for initialising Firebase — either a file path or
// the three individual fields from environment variables.
type FCMConfig struct {
	CredentialsPath string // path to service-account JSON file (optional)
	ProjectID       string // FIREBASE_PROJECT_ID
	PrivateKey      string // FIREBASE_PRIVATE_KEY  (PEM, may use literal \n)
	ClientEmail     string // FIREBASE_CLIENT_EMAIL
}

// NewFCMClient creates a new FCM client.
// Priority: file path → individual env vars → error.
func NewFCMClient(cfg FCMConfig, logger *zap.Logger) (*FCMClient, error) {
	ctx := context.Background()

	var opt option.ClientOption

	if cfg.CredentialsPath != "" {
		opt = option.WithCredentialsFile(cfg.CredentialsPath)
	} else if cfg.ProjectID != "" && cfg.PrivateKey != "" && cfg.ClientEmail != "" {
		// Build a service-account JSON from individual env vars so no file is needed.
		// Accept PEM, \n-escaped PEM, or base64-encoded PEM (Dokploy-safe).
		privateKey := normalizePEM(cfg.PrivateKey)
		credJSON, err := json.Marshal(map[string]string{ //#nosec G101 -- credential field names, not values
			"type":                        "service_account",
			"project_id":                  cfg.ProjectID,
			"private_key":                 privateKey,
			"client_email":                cfg.ClientEmail,
			"token_uri":                   "https://oauth2.googleapis.com/token",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build Firebase credentials JSON: %w", err)
		}
		opt = option.WithCredentialsJSON(credJSON)
	} else {
		return nil, fmt.Errorf("firebase credentials not provided: set FIREBASE_CREDENTIALS_PATH or FIREBASE_PROJECT_ID + FIREBASE_PRIVATE_KEY + FIREBASE_CLIENT_EMAIL")
	}

	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get messaging client: %w", err)
	}

	logger.Info("FCM client initialized successfully")

	return &FCMClient{
		client: client,
		logger: logger,
	}, nil
}

// SendNotification sends a push notification to a single device
func (f *FCMClient) SendNotification(ctx context.Context, token string, payload *PushPayload) error {
	message := &messaging.Message{
		Token: token,
		Data:  payload.Data,
	}
	if !payload.Silent {
		message.Notification = &messaging.Notification{
			Title: payload.Title,
			Body:  payload.Body,
		}
		if payload.ImageURL != "" {
			message.Notification.ImageURL = payload.ImageURL
		}
	}

	// Android: always set high priority and channel ID so notifications wake the
	// device immediately and land in the correct channel (API 26+). Silent
	// pushes drop the AndroidNotification block — data-only payload still
	// fires onMessageReceived in the background isolate.
	channelID := payload.ChannelID
	if channelID == "" {
		channelID = "general"
	}
	apnsSound := payload.Sound
	if apnsSound == "" {
		apnsSound = "default"
	}
	if payload.Silent {
		message.Android = &messaging.AndroidConfig{Priority: "high"}
	} else {
		message.Android = &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID:   channelID,
				Sound:       payload.Sound,
				ClickAction: payload.ClickAction,
			},
		}
	}

	// iOS: alert vs background push types are encoded in apns-push-type +
	// content-available. Background pushes get priority 5 (Apple requires
	// non-immediate priority for content-available pushes).
	if payload.Silent {
		message.APNS = &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-push-type":  "background",
				"apns-priority":   "5",
				"apns-expiration": strconv.FormatInt(time.Now().Add(apnsExpirationSeconds*time.Second).Unix(), 10),
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					ContentAvailable: true,
				},
			},
		}
	} else {
		message.APNS = &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-push-type":  "alert",
				"apns-priority":   "10",
				"apns-expiration": strconv.FormatInt(time.Now().Add(apnsExpirationSeconds*time.Second).Unix(), 10),
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound:          apnsSound,
					MutableContent: true,
				},
			},
		}
		if payload.Badge != nil {
			message.APNS.Payload.Aps.Badge = payload.Badge
		}
	}

	// Send message
	response, err := f.client.Send(ctx, message)
	if err != nil {
		f.logger.Error("Failed to send FCM notification",
			zap.Error(err),
			zap.String("token", token),
		)
		// Surface stale-token errors so callers can prune the FCM token from
		// storage. messaging.IsRegistrationTokenNotRegistered covers both
		// classical "NotRegistered" (token revoked / app uninstalled) and
		// "InvalidArgument" / "InvalidRegistration" responses.
		if messaging.IsUnregistered(err) ||
			messaging.IsInvalidArgument(err) {
			return ErrTokenInvalid
		}
		return fmt.Errorf("failed to send notification: %w", err)
	}

	f.logger.Debug("FCM notification sent successfully",
		zap.String("message_id", response),
		zap.String("title", payload.Title),
	)

	return nil
}

// ErrTokenInvalid is returned by SendNotification when FCM reports the
// token is no longer valid (revoked, app uninstalled, or malformed).
// Callers should delete the stored token from their tokens table.
var ErrTokenInvalid = fmt.Errorf("fcm token is no longer valid")

// SendMulticast sends a push notification to multiple devices
func (f *FCMClient) SendMulticast(ctx context.Context, tokens []string, payload *PushPayload) (*messaging.BatchResponse, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens provided")
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: payload.Title,
			Body:  payload.Body,
		},
		Data: payload.Data,
	}

	// Set optional fields
	if payload.ImageURL != "" {
		message.Notification.ImageURL = payload.ImageURL
	}

	// Android: always set high priority and channel ID.
	multicastChannelID := payload.ChannelID
	if multicastChannelID == "" {
		multicastChannelID = "general"
	}
	multicastAPNSSound := payload.Sound
	if multicastAPNSSound == "" {
		multicastAPNSSound = "default"
	}
	message.Android = &messaging.AndroidConfig{
		Priority: "high",
		Notification: &messaging.AndroidNotification{
			ChannelID:   multicastChannelID,
			Sound:       payload.Sound,
			ClickAction: payload.ClickAction,
		},
	}

	// iOS: always include APNS headers for reliable immediate delivery.
	// `apns-expiration` ensures APNs retries delivery when device is offline.
	message.APNS = &messaging.APNSConfig{
		Headers: map[string]string{
			"apns-push-type":   "alert",
			"apns-priority":    "10",
			"apns-expiration":  strconv.FormatInt(time.Now().Add(apnsExpirationSeconds*time.Second).Unix(), 10),
		},
		Payload: &messaging.APNSPayload{
			Aps: &messaging.Aps{
				Sound:          multicastAPNSSound,
				MutableContent: true,
			},
		},
	}
	if payload.Badge != nil {
		message.APNS.Payload.Aps.Badge = payload.Badge
	}

	// Send to multiple devices
	response, err := f.client.SendEachForMulticast(ctx, message)
	if err != nil {
		f.logger.Error("Failed to send multicast FCM notification",
			zap.Error(err),
			zap.Int("token_count", len(tokens)),
		)
		return nil, fmt.Errorf("failed to send multicast notification: %w", err)
	}

	f.logger.Info("Multicast FCM notification sent",
		zap.Int("success_count", response.SuccessCount),
		zap.Int("failure_count", response.FailureCount),
		zap.String("title", payload.Title),
	)

	return response, nil
}

// SubscribeToTopic subscribes a token to a topic
func (f *FCMClient) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	response, err := f.client.SubscribeToTopic(ctx, tokens, topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	if response.FailureCount > 0 {
		f.logger.Warn("Some tokens failed to subscribe to topic",
			zap.String("topic", topic),
			zap.Int("failure_count", response.FailureCount),
		)
	}

	return nil
}

// UnsubscribeFromTopic unsubscribes a token from a topic
func (f *FCMClient) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	response, err := f.client.UnsubscribeFromTopic(ctx, tokens, topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %w", err)
	}

	if response.FailureCount > 0 {
		f.logger.Warn("Some tokens failed to unsubscribe from topic",
			zap.String("topic", topic),
			zap.Int("failure_count", response.FailureCount),
		)
	}

	return nil
}

// PushPayload represents the payload for push notifications
type PushPayload struct {
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Data        map[string]string `json:"data,omitempty"`
	ImageURL    string            `json:"image_url,omitempty"`
	Sound       string            `json:"sound,omitempty"`
	Badge       *int              `json:"badge,omitempty"`
	ClickAction string            `json:"click_action,omitempty"`
	ChannelID   string            `json:"channel_id,omitempty"` // Android notification channel
	// Silent: when true, send as data-only push (no Notification payload, no
	// banner). iOS adds `content-available: 1` so the OS wakes the app for a
	// background sync; Android relies on the data-only behaviour to fire
	// FirebaseMessagingService.onMessageReceived. Use for "refresh feed" or
	// "sync unread count" wakeups that shouldn't show a user-visible alert.
	Silent bool `json:"silent,omitempty"`
}
