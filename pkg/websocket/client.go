package websocket

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 8192 // 8 KB
)

var (
	newline = []byte{'\n'}
)

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.Hub.logger.Error("WebSocket read error",
					zap.Error(err),
					zap.String("user_id", c.ID),
				)
			}
			break
		}

		// Reject non-JSON payloads before any further processing.
		// This prevents malformed/binary blobs from reaching business logic.
		if !json.Valid(message) {
			c.Hub.logger.Warn("Received invalid JSON WebSocket message, ignoring",
				zap.String("user_id", c.ID),
			)
			continue
		}

		// Handle incoming WS frames from the client. Currently handled:
		//   * `presence` — `{type:"presence", conversation_id:"<id>"}` —
		//     sets/clears the conversation the user is actively viewing so
		//     the chat service can suppress redundant push notifications
		//     while the recipient is on the screen. Empty conversation_id
		//     clears the active marker.
		var frame struct {
			Type           string `json:"type"`
			ConversationID string `json:"conversation_id"`
		}
		if err := json.Unmarshal(message, &frame); err != nil {
			c.Hub.logger.Debug("Unparseable WS frame, ignoring",
				zap.String("user_id", c.ID),
				zap.Error(err),
			)
			continue
		}
		switch frame.Type {
		case "presence":
			c.Hub.SetActiveConversation(c.ID, frame.ConversationID)
		default:
			c.Hub.logger.Debug("Received WebSocket message",
				zap.String("user_id", c.ID),
				zap.String("type", frame.Type),
			)
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write(newline)
				_, _ = w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
