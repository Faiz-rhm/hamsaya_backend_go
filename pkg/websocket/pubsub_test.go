package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func newRedisHub(t *testing.T, processID string) (*Hub, *Fanout, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	hub := NewHub(zap.NewNop())
	go hub.Run()
	t.Cleanup(hub.Shutdown)
	f := NewFanout(client, hub, processID, zap.NewNop())
	hub.AttachFanout(f)
	f.Start()
	t.Cleanup(f.Stop)
	return hub, f, client
}

// TestFanout_LoopbackDropped publishes a message with our own processID;
// the subscriber must drop it (already delivered locally before publish).
func TestFanout_LoopbackDropped(t *testing.T) {
	hub, _, _ := newRedisHub(t, "podA")
	c := newTestClient(hub, "user-1")
	hub.Register(c)
	time.Sleep(50 * time.Millisecond)

	// Direct local send → 1 message lands.
	_ = hub.SendToUser("user-1", "hi")

	// Read exactly one message; loopback from our own publish must NOT
	// double-deliver.
	select {
	case <-c.Send:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("local send didn't deliver")
	}
	select {
	case msg := <-c.Send:
		t.Fatalf("loopback delivered duplicate message: %s", msg)
	case <-time.After(300 * time.Millisecond):
		// expected — loopback dropped
	}
}

// TestFanout_PeerDelivery confirms a message published by hub A is routed
// to hub B's local client (the user-on-peer-pod scenario). Both hubs
// share one Redis broker.
func TestFanout_PeerDelivery(t *testing.T) {
	mr := miniredis.RunT(t)
	clientA := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	clientB := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	hubA := NewHub(zap.NewNop())
	hubB := NewHub(zap.NewNop())
	go hubA.Run()
	go hubB.Run()
	t.Cleanup(hubA.Shutdown)
	t.Cleanup(hubB.Shutdown)

	fA := NewFanout(clientA, hubA, "podA", zap.NewNop())
	fB := NewFanout(clientB, hubB, "podB", zap.NewNop())
	hubA.AttachFanout(fA)
	hubB.AttachFanout(fB)
	fA.Start()
	fB.Start()
	t.Cleanup(fA.Stop)
	t.Cleanup(fB.Stop)

	// User connected only to hub B.
	c := newTestClient(hubB, "user-1")
	hubB.Register(c)
	time.Sleep(100 * time.Millisecond) // pub/sub registration

	// Hub A receives a SendToUser for that user — it has no local conn,
	// so delivery must come via Redis fanout to hub B.
	if err := hubA.SendToUser("user-1", map[string]string{"hi": "via-pubsub"}); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-c.Send:
		if len(msg) == 0 {
			t.Fatal("empty payload")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("peer delivery timed out — fanout not routing to hub B")
	}
}

func TestFanout_PublishMessageEnvelope(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	hub := NewHub(zap.NewNop())
	f := NewFanout(client, hub, "pod-x", zap.NewNop())

	if err := f.Publish(context.Background(), "u-1", []byte("hello")); err != nil {
		t.Fatal(err)
	}
}
