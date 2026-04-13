package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

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
		// Replace literal "\n" sequences (common when storing PEM in env vars) with real newlines.
		privateKey := strings.ReplaceAll(cfg.PrivateKey, `\n`, "\n")
		credJSON, err := json.Marshal(map[string]string{
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
		return nil, fmt.Errorf("Firebase credentials not provided: set FIREBASE_CREDENTIALS_PATH or FIREBASE_PROJECT_ID + FIREBASE_PRIVATE_KEY + FIREBASE_CLIENT_EMAIL")
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

	// Android specific config
	if payload.Sound != "" || payload.ClickAction != "" {
		message.Android = &messaging.AndroidConfig{
			Notification: &messaging.AndroidNotification{
				Sound:       payload.Sound,
				ClickAction: payload.ClickAction,
			},
		}
	}

	// iOS specific config
	if payload.Badge != nil || payload.Sound != "" {
		message.APNS = &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: payload.Sound,
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
		return fmt.Errorf("failed to send notification: %w", err)
	}

	f.logger.Debug("FCM notification sent successfully",
		zap.String("message_id", response),
		zap.String("title", payload.Title),
	)

	return nil
}

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

	// Android specific config
	if payload.Sound != "" || payload.ClickAction != "" {
		message.Android = &messaging.AndroidConfig{
			Notification: &messaging.AndroidNotification{
				Sound:       payload.Sound,
				ClickAction: payload.ClickAction,
			},
		}
	}

	// iOS specific config
	if payload.Badge != nil || payload.Sound != "" {
		message.APNS = &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: payload.Sound,
				},
			},
		}
		if payload.Badge != nil {
			message.APNS.Payload.Aps.Badge = payload.Badge
		}
	}

	// Send to multiple devices
	response, err := f.client.SendMulticast(ctx, message)
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
	Title       string                 `json:"title"`
	Body        string                 `json:"body"`
	Data        map[string]string      `json:"data,omitempty"`
	ImageURL    string                 `json:"image_url,omitempty"`
	Sound       string                 `json:"sound,omitempty"`
	Badge       *int                   `json:"badge,omitempty"`
	ClickAction string                 `json:"click_action,omitempty"`
}
