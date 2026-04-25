package websocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	hub := NewHub(zap.NewNop())
	go hub.Run()
	t.Cleanup(hub.Shutdown)
	return hub
}

func newTestClient(hub *Hub, id string) *Client {
	c := &Client{
		ID:   id,
		Hub:  hub,
		Send: make(chan []byte, 16),
	}
	return c
}

func TestHub_RegisterAndIsConnected(t *testing.T) {
	hub := newTestHub(t)
	c := newTestClient(hub, "user-1")

	hub.Register(c)
	time.Sleep(20 * time.Millisecond)

	assert.True(t, hub.IsUserConnected("user-1"))
}

func TestHub_UnregisterRemovesClient(t *testing.T) {
	hub := newTestHub(t)
	c := newTestClient(hub, "user-2")

	hub.Register(c)
	time.Sleep(20 * time.Millisecond)
	assert.True(t, hub.IsUserConnected("user-2"))

	hub.Unregister(c)
	time.Sleep(20 * time.Millisecond)
	assert.False(t, hub.IsUserConnected("user-2"))
}

func TestHub_SendToUser_Delivers(t *testing.T) {
	hub := newTestHub(t)
	c := newTestClient(hub, "user-3")

	hub.Register(c)
	time.Sleep(20 * time.Millisecond)

	msg := map[string]string{"type": "ping"}
	err := hub.SendToUser("user-3", msg)
	assert.NoError(t, err)

	select {
	case data := <-c.Send:
		assert.Contains(t, string(data), "ping")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("message not received")
	}
}

func TestHub_SendToUser_UserNotConnected(t *testing.T) {
	hub := newTestHub(t)

	// No error expected — just silently dropped
	err := hub.SendToUser("ghost-user", map[string]string{"x": "y"})
	assert.NoError(t, err)
}

func TestHub_RegisterReplacesDuplicate(t *testing.T) {
	hub := newTestHub(t)

	c1 := newTestClient(hub, "user-4")
	c2 := newTestClient(hub, "user-4")

	hub.Register(c1)
	time.Sleep(20 * time.Millisecond)
	hub.Register(c2)
	time.Sleep(20 * time.Millisecond)

	assert.True(t, hub.IsUserConnected("user-4"))
	// Original client should be closed
	assert.True(t, c1.IsClosed())
}

func TestHub_GetConnectedUserIDs(t *testing.T) {
	hub := newTestHub(t)

	c1 := newTestClient(hub, "user-5")
	c2 := newTestClient(hub, "user-6")
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(20 * time.Millisecond)

	ids := hub.GetConnectedUserIDs()
	assert.Len(t, ids, 2)
	assert.ElementsMatch(t, []string{"user-5", "user-6"}, ids)
}

func TestClient_IsClosed_InitiallyFalse(t *testing.T) {
	c := &Client{
		ID:   "test",
		Send: make(chan []byte, 4),
	}
	assert.False(t, c.IsClosed())
}
