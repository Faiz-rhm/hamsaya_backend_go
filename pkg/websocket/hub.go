package websocket

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Client represents a WebSocket client connection
type Client struct {
	ID     string          // User ID
	Conn   *websocket.Conn
	Hub    *Hub
	Send   chan []byte // Buffered channel for outbound messages
	mu     sync.Mutex
	closed bool
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Registered clients (userID -> *Client)
	clients map[string]*Client

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast message to specific user
	broadcast chan *BroadcastMessage

	// Shutdown signal
	done chan struct{}

	// Mutex for thread-safe access to clients map
	mu sync.RWMutex

	// Logger
	logger *zap.Logger
}

// BroadcastMessage represents a message to be sent to a specific user
type BroadcastMessage struct {
	UserID  string
	Message []byte
}

// NewHub creates a new WebSocket hub
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage),
		done:       make(chan struct{}),
		logger:     logger,
	}
}

// Run starts the hub's main loop. It exits when Shutdown() is called.
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			// Graceful shutdown: close all client connections
			h.mu.Lock()
			for _, client := range h.clients {
				client.close()
			}
			h.clients = make(map[string]*Client)
			h.mu.Unlock()
			h.logger.Info("WebSocket hub shut down gracefully")
			return

		case client := <-h.register:
			h.mu.Lock()
			// Close existing connection if user is already connected
			if existingClient, exists := h.clients[client.ID]; exists {
				h.logger.Info("Replacing existing connection",
					zap.String("user_id", client.ID),
				)
				existingClient.close()
			}
			h.clients[client.ID] = client
			h.mu.Unlock()
			h.logger.Info("Client connected",
				zap.String("user_id", client.ID),
				zap.Int("total_clients", len(h.clients)),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				client.close()
				h.logger.Info("Client disconnected",
					zap.String("user_id", client.ID),
					zap.Int("total_clients", len(h.clients)),
				)
			}
			h.mu.Unlock()

		case broadcast := <-h.broadcast:
			h.mu.RLock()
			client, exists := h.clients[broadcast.UserID]
			h.mu.RUnlock()

			if exists {
				select {
				case client.Send <- broadcast.Message:
					h.logger.Debug("Message sent to client",
						zap.String("user_id", broadcast.UserID),
					)
				default:
					// Client's send buffer is full, close the connection
					h.logger.Warn("Client send buffer full, closing connection",
						zap.String("user_id", broadcast.UserID),
					)
					h.unregister <- client
				}
			} else {
				h.logger.Debug("User not connected, message not sent",
					zap.String("user_id", broadcast.UserID),
				)
			}
		}
	}
}

// Shutdown gracefully stops the hub, closing all client connections.
func (h *Hub) Shutdown() {
	close(h.done)
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// SendToUser sends a message to a specific user
func (h *Hub) SendToUser(userID string, message interface{}) error {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal WebSocket message",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return err
	}

	h.broadcast <- &BroadcastMessage{
		UserID:  userID,
		Message: messageBytes,
	}

	return nil
}

// IsUserConnected checks if a user is currently connected
func (h *Hub) IsUserConnected(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.clients[userID]
	return exists
}

// GetConnectedUserIDs returns a list of all connected user IDs
func (h *Hub) GetConnectedUserIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userIDs := make([]string, 0, len(h.clients))
	for userID := range h.clients {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

// close safely closes a client connection
func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		c.closed = true
		close(c.Send)
		if c.Conn != nil {
			c.Conn.Close()
		}
	}
}

// IsClosed returns whether the client connection is closed
func (c *Client) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}
