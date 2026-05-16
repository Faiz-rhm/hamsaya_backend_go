package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newTestCache spins up an in-memory miniredis + a real go-redis client
// pointed at it. miniredis keeps tests hermetic — no real network, no
// shared state between tests.
func newTestCache(t *testing.T, namespace string) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return New(rdb, namespace, zap.NewNop()), mr
}

func TestCache_NilReceiver_NoOp(t *testing.T) {
	// Calling methods on a nil *Cache must not panic and must behave as
	// a clean miss — callers rely on this for the "cache disabled" path.
	var c *Cache
	ctx := context.Background()

	var out string
	hit, err := c.Get(ctx, "k", &out)
	assert.False(t, hit)
	assert.NoError(t, err)

	assert.NoError(t, c.Set(ctx, "k", "v", time.Minute))
	c.Del(ctx, "k")
	c.DelPattern(ctx, "*")
}

func TestCache_NilClient_NoOp(t *testing.T) {
	c := New(nil, "ns", zap.NewNop())
	ctx := context.Background()

	var out string
	hit, err := c.Get(ctx, "k", &out)
	assert.False(t, hit)
	assert.NoError(t, err)
	assert.NoError(t, c.Set(ctx, "k", "v", time.Minute))
	c.Del(ctx, "k")
	c.DelPattern(ctx, "*")
}

func TestCache_SetThenGet_RoundTrip(t *testing.T) {
	c, mr := newTestCache(t, "ns")
	ctx := context.Background()

	type payload struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	in := payload{Name: "alice", N: 42}
	require.NoError(t, c.Set(ctx, "user:1", in, time.Minute))

	// Stored under the namespaced key, not the raw key.
	assert.True(t, mr.Exists("ns:user:1"))
	assert.False(t, mr.Exists("user:1"))

	var out payload
	hit, err := c.Get(ctx, "user:1", &out)
	require.NoError(t, err)
	require.True(t, hit)
	assert.Equal(t, in, out)
}

func TestCache_Get_MissReturnsFalse(t *testing.T) {
	c, _ := newTestCache(t, "ns")
	var out string
	hit, err := c.Get(context.Background(), "missing", &out)
	assert.False(t, hit)
	assert.NoError(t, err)
}

func TestCache_TTL_Expires(t *testing.T) {
	c, mr := newTestCache(t, "ns")
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "k", "v", 100*time.Millisecond))

	var out string
	hit, _ := c.Get(ctx, "k", &out)
	require.True(t, hit)

	mr.FastForward(200 * time.Millisecond)

	hit, _ = c.Get(ctx, "k", &out)
	assert.False(t, hit)
}

func TestCache_Set_ClampsZeroTTL(t *testing.T) {
	// A zero or negative TTL is clamped to 60s — no infinite cache
	// because of a typo'd loader.
	c, mr := newTestCache(t, "ns")
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "k", "v", 0))
	ttl := mr.TTL("ns:k")
	assert.GreaterOrEqual(t, ttl, 59*time.Second)
	assert.LessOrEqual(t, ttl, 60*time.Second)

	require.NoError(t, c.Set(ctx, "k2", "v", -1*time.Hour))
	ttl2 := mr.TTL("ns:k2")
	assert.GreaterOrEqual(t, ttl2, 59*time.Second)
}

func TestCache_Get_CorruptValue_TreatedAsMiss(t *testing.T) {
	c, mr := newTestCache(t, "ns")
	ctx := context.Background()
	// Inject garbage that isn't valid JSON.
	require.NoError(t, mr.Set("ns:k", "not-json-{"))

	var out map[string]any
	hit, err := c.Get(ctx, "k", &out)
	assert.False(t, hit)
	assert.NoError(t, err)
	// Corrupt entry is evicted so the next write fixes it.
	assert.False(t, mr.Exists("ns:k"))
}

func TestCache_Del_RemovesNamespacedKey(t *testing.T) {
	c, mr := newTestCache(t, "ns")
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "a", 1, time.Minute))
	require.NoError(t, c.Set(ctx, "b", 2, time.Minute))

	c.Del(ctx, "a")
	assert.False(t, mr.Exists("ns:a"))
	assert.True(t, mr.Exists("ns:b"))
}

func TestCache_DelPattern_EvictsMatching(t *testing.T) {
	c, mr := newTestCache(t, "businesses")
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "abc:anon", 1, time.Minute))
	require.NoError(t, c.Set(ctx, "abc:user-1", 2, time.Minute))
	require.NoError(t, c.Set(ctx, "xyz:anon", 3, time.Minute))

	// Mutating business "abc" → wipe every viewer's cached variant.
	c.DelPattern(ctx, "abc:*")
	assert.False(t, mr.Exists("businesses:abc:anon"))
	assert.False(t, mr.Exists("businesses:abc:user-1"))
	assert.True(t, mr.Exists("businesses:xyz:anon"))
}

func TestCache_DelPattern_EmptyNamespace(t *testing.T) {
	c, mr := newTestCache(t, "")
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "k1", 1, time.Minute))
	require.NoError(t, c.Set(ctx, "k2", 2, time.Minute))
	c.DelPattern(ctx, "*")
	assert.False(t, mr.Exists("k1"))
	assert.False(t, mr.Exists("k2"))
}

func TestCache_Set_RejectsUnmarshalable(t *testing.T) {
	c, _ := newTestCache(t, "ns")
	// channels can't be JSON-encoded — Set should return the marshal
	// error so callers can log it. Cache stays empty.
	err := c.Set(context.Background(), "k", make(chan int), time.Minute)
	assert.Error(t, err)
}
