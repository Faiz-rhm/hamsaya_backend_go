package notification

import (
	"context"
	"fmt"

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

// NewFCMClient creates a new FCM client
func NewFCMClient(credentialsPath string, logger *zap.Logger) (*FCMClient, error) {
	ctx := context.Background()

	// Initialize Firebase app
	opt := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	// Get messaging client
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
