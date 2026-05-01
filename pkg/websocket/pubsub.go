// Cross-instance fanout via Redis pub/sub.
//
// Single-instance: SendToUser → shard → client.Send. Works as long as the
// userID is connected to *this* pod.
//
// Multi-instance: when N pods serve WebSocket traffic, a userID is connected
// to exactly one of them. SendToUser called from any pod must reach the
// owning pod.
//
// Strategy:
//   1. SendToUser locally tries the owning shard. If found, send and return.
//   2. If not found locally, also publish to Redis channel
//      "ws:user:<userID>". Other pods receive via pattern subscribe.
//   3. On pub message receipt, peer pods locate the shard locally and
//      attempt delivery. No-op if the user isn't connected there either.
//
// Fanout traffic is bounded: only "miss locally" messages publish, and
// each peer drops the message immediately if the user isn't on it.

package websocket

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const pubsubChannelPrefix = "ws:user:"

// Fanout wraps a redis.Client to publish + subscribe. Optional — when
// nil, [Hub] runs single-instance only.
type Fanout struct {
	client    *redis.Client
	logger    *zap.Logger
	hub       *Hub
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	processID string
}

// NewFanout creates a fanout wired to the given hub.
//
// processID is included in published messages so a pod that publishes to
// itself can ignore the loopback (every pattern subscriber receives every
// message). Use os.Hostname() or k8s pod name.
func NewFanout(client *redis.Client, hub *Hub, processID string, logger *zap.Logger) *Fanout {
	return &Fanout{
		client:    client,
		logger:    logger,
		hub:       hub,
		processID: processID,
	}
}

type fanoutMessage struct {
	From    string `json:"from"`
	UserID  string `json:"user_id"`
	Payload []byte `json:"payload"`
}

// Start subscribes to the pubsub pattern and routes incoming messages to
// the local hub. Idempotent on multiple calls.
func (f *Fanout) Start() {
	if f.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	pubsub := f.client.PSubscribe(ctx, pubsubChannelPrefix+"*")
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		defer func() { _ = pubsub.Close() }()
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				f.handle(msg)
			}
		}
	}()
}

// Stop ends the subscriber goroutine.
func (f *Fanout) Stop() {
	if f.cancel == nil {
		return
	}
	f.cancel()
	f.wg.Wait()
	f.cancel = nil
}

func (f *Fanout) handle(msg *redis.Message) {
	var m fanoutMessage
	if err := json.Unmarshal([]byte(msg.Payload), &m); err != nil {
		f.logger.Warn("ws fanout: bad payload", zap.Error(err))
		return
	}
	if m.From == f.processID {
		// Loopback — already delivered locally before publishing.
		return
	}
	// Locate locally and attempt delivery. The shard's broadcast chan
	// is non-blocking against the client.Send buffer; if the user isn't
	// here, the shard's "exists" check returns false and the message
	// is dropped (expected behavior for fanout).
	s := f.hub.shardFor(m.UserID)
	s.broadcast <- &BroadcastMessage{UserID: m.UserID, Message: m.Payload}
}

// Publish writes a message to the cross-instance fanout channel.
func (f *Fanout) Publish(ctx context.Context, userID string, payload []byte) error {
	body, err := json.Marshal(fanoutMessage{
		From:    f.processID,
		UserID:  userID,
		Payload: payload,
	})
	if err != nil {
		return err
	}
	return f.client.Publish(ctx, pubsubChannelPrefix+userID, body).Err()
}
