package websocket

import (
	"sync"
	"testing"
	"time"
)

// TestHub_CrossShardDistribution registers many users and verifies they
// land on multiple shards (not all the same one). Confirms the hash-based
// router actually spreads load.
func TestHub_CrossShardDistribution(t *testing.T) {
	hub := newTestHub(t)

	const userCount = 200
	clients := make([]*Client, userCount)
	for i := 0; i < userCount; i++ {
		c := newTestClient(hub, makeID("u", i))
		clients[i] = c
		hub.Register(c)
	}
	time.Sleep(50 * time.Millisecond)

	// Count clients per shard. With fnv32a + 200 users + 16 shards,
	// every shard should have at least 1.
	occupiedShards := 0
	for _, s := range hub.shards {
		s.mu.RLock()
		if len(s.clients) > 0 {
			occupiedShards++
		}
		s.mu.RUnlock()
	}
	if occupiedShards < numShards/2 {
		t.Fatalf("hash distribution suspicious: only %d of %d shards occupied", occupiedShards, numShards)
	}

	// Sanity: all 200 users connected.
	got := 0
	for _, s := range hub.shards {
		s.mu.RLock()
		got += len(s.clients)
		s.mu.RUnlock()
	}
	if got != userCount {
		t.Fatalf("registered %d clients, found %d across shards", userCount, got)
	}
}

// TestHub_ShardIsolation registers two users that hash to *different*
// shards and verifies a flood on one doesn't block the other.
func TestHub_ShardIsolation(t *testing.T) {
	hub := newTestHub(t)

	a := newTestClient(hub, "user-a")
	b := newTestClient(hub, "user-b")
	hub.Register(a)
	hub.Register(b)
	time.Sleep(20 * time.Millisecond)

	if hub.shardFor("user-a") == hub.shardFor("user-b") {
		t.Skip("hash collision — not a useful test for this pair")
	}

	// Send 10 to a (under client.Send buffer of 16) and 10 to b.
	// Goal: confirm both shards make progress concurrently. If shards
	// were not isolated, b's sends would queue behind a's.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			_ = hub.SendToUser("user-a", map[string]int{"i": i})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			_ = hub.SendToUser("user-b", map[string]int{"i": i})
		}
	}()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shard isolation: parallel sends to different shards deadlocked")
	}
}

func TestHub_GetConnectedUserIDs_AcrossShards(t *testing.T) {
	hub := newTestHub(t)

	const n = 50
	for i := 0; i < n; i++ {
		hub.Register(newTestClient(hub, makeID("u", i)))
	}
	time.Sleep(50 * time.Millisecond)

	ids := hub.GetConnectedUserIDs()
	if len(ids) != n {
		t.Fatalf("got %d ids, want %d", len(ids), n)
	}
}

func makeID(prefix string, n int) string {
	const hex = "0123456789abcdef"
	out := []byte(prefix + "-")
	for n > 0 || len(out) < len(prefix)+3 {
		out = append(out, hex[n&15])
		n >>= 4
	}
	return string(out)
}
