package websocket

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// numShards controls the parallelism of the hub. Each shard owns its own
// goroutine, clients map, and channels — register/unregister/broadcast on
// different shards run concurrently. 16 keeps goroutine count low while
// removing the single-broadcaster bottleneck identified in BACKEND_REVIEW.
const numShards = 16

// Client represents a WebSocket client connection
type Client struct {
	ID     string // User ID
	Conn   *websocket.Conn
	Hub    *Hub
	Send   chan []byte // Buffered channel for outbound messages
	mu     sync.Mutex
	closed bool
}

// hubShard is one slice of the connection map. Each shard runs an
// independent select loop so concurrent SendToUser calls to *different*
// shards never serialize. Within a shard, register/unregister/broadcast
// are still strictly ordered (which is required for connection-replace
// semantics in register).
type hubShard struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
	done       chan struct{}
	mu         sync.RWMutex
	logger     *zap.Logger
}

// Hub maintains the set of active clients and routes messages across
// numShards parallel shards.
type Hub struct {
	shards [numShards]*hubShard
	logger *zap.Logger

	// Optional cross-instance fanout. nil = single-pod mode.
	fanout *Fanout
}

// AttachFanout wires a Redis pub/sub fanout to this hub. Called once at
// boot when DB_REDIS multi-instance mode is enabled. Safe to leave nil.
func (h *Hub) AttachFanout(f *Fanout) {
	h.fanout = f
}

// BroadcastMessage represents a message to be sent to a specific user
type BroadcastMessage struct {
	UserID  string
	Message []byte
}

// NewHub creates a new WebSocket hub
func NewHub(logger *zap.Logger) *Hub {
	h := &Hub{logger: logger}
	for i := 0; i < numShards; i++ {
		h.shards[i] = &hubShard{
			clients:    make(map[string]*Client),
			register:   make(chan *Client),
			unregister: make(chan *Client),
			broadcast:  make(chan *BroadcastMessage),
			done:       make(chan struct{}),
			logger:     logger,
		}
	}
	return h
}

// shardFor returns the shard responsible for userID. fnv32 is fast and
// low-collision enough for this use case (hot path on every send).
func (h *Hub) shardFor(userID string) *hubShard {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(userID))
	return h.shards[hash.Sum32()%numShards]
}

// Run launches one goroutine per shard. Returns when Shutdown is called
// (each shard's done channel is closed).
func (h *Hub) Run() {
	var wg sync.WaitGroup
	for i := range h.shards {
		wg.Add(1)
		s := h.shards[i]
		go func() {
			defer wg.Done()
			s.run()
		}()
	}
	wg.Wait()
	h.logger.Info("WebSocket hub shut down gracefully")
}

func (s *hubShard) run() {
	for {
		select {
		case <-s.done:
			s.mu.Lock()
			for _, client := range s.clients {
				client.close()
			}
			s.clients = make(map[string]*Client)
			s.mu.Unlock()
			return

		case client := <-s.register:
			s.mu.Lock()
			if existingClient, exists := s.clients[client.ID]; exists {
				s.logger.Info("Replacing existing connection",
					zap.String("user_id", client.ID),
				)
				existingClient.close()
			}
			s.clients[client.ID] = client
			s.mu.Unlock()
			s.logger.Info("Client connected",
				zap.String("user_id", client.ID),
				zap.Int("shard_clients", len(s.clients)),
			)

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client.ID]; ok {
				delete(s.clients, client.ID)
				client.close()
				s.logger.Info("Client disconnected",
					zap.String("user_id", client.ID),
					zap.Int("shard_clients", len(s.clients)),
				)
			}
			s.mu.Unlock()

		case broadcast := <-s.broadcast:
			s.mu.RLock()
			client, exists := s.clients[broadcast.UserID]
			s.mu.RUnlock()

			if exists {
				select {
				case client.Send <- broadcast.Message:
					s.logger.Debug("Message sent to client",
						zap.String("user_id", broadcast.UserID),
					)
				default:
					s.logger.Warn("Client send buffer full, closing connection",
						zap.String("user_id", broadcast.UserID),
					)
					// Re-enqueue unregister on the same shard.
					s.unregister <- client
				}
			} else {
				s.logger.Debug("User not connected, message not sent",
					zap.String("user_id", broadcast.UserID),
				)
			}
		}
	}
}

// Shutdown gracefully stops every shard, closing all client connections.
func (h *Hub) Shutdown() {
	for _, s := range h.shards {
		close(s.done)
	}
}

// Register adds a client to its shard.
func (h *Hub) Register(client *Client) {
	h.shardFor(client.ID).register <- client
}

// Unregister removes a client from its shard.
func (h *Hub) Unregister(client *Client) {
	h.shardFor(client.ID).unregister <- client
}

// SendToUser sends a message to a specific user via the user's shard.
// When a Fanout is attached and the user isn't connected to *this* pod,
// the message is also published on Redis pub/sub so a peer pod with the
// connection can deliver it.
func (h *Hub) SendToUser(userID string, message interface{}) error {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal WebSocket message",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return err
	}

	h.shardFor(userID).broadcast <- &BroadcastMessage{
		UserID:  userID,
		Message: messageBytes,
	}

	// Fanout to peer pods. We always publish (rather than gating on
	// IsUserConnected) because the local broadcast channel hasn't yet
	// been processed; checking now would race with the shard goroutine.
	// Peer pods drop quickly when the user isn't on them.
	if h.fanout != nil {
		if pubErr := h.fanout.Publish(context.Background(), userID, messageBytes); pubErr != nil {
			h.logger.Warn("ws fanout publish failed", zap.Error(pubErr), zap.String("user_id", userID))
		}
	}

	return nil
}

// IsUserConnected checks if a user is currently connected
func (h *Hub) IsUserConnected(userID string) bool {
	s := h.shardFor(userID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.clients[userID]
	return exists
}

// GetConnectedUserIDs returns a list of all connected user IDs across
// all shards. O(N); intended for diagnostics and admin only.
func (h *Hub) GetConnectedUserIDs() []string {
	out := make([]string, 0)
	for _, s := range h.shards {
		s.mu.RLock()
		for userID := range s.clients {
			out = append(out, userID)
		}
		s.mu.RUnlock()
	}
	return out
}

// close safely closes a client connection
func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		c.closed = true
		close(c.Send)
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
	}
}

// IsClosed returns whether the client connection is closed
func (c *Client) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}
