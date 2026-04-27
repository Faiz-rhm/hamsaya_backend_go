package secrets

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSource struct {
	calls atomic.Int64
	value string
	err   error
}

func (f *fakeSource) Get(_ context.Context, _ string) (string, error) {
	f.calls.Add(1)
	return f.value, f.err
}

func TestEnvSource(t *testing.T) {
	t.Setenv("HAMSAYA_TEST_KEY", "abc")
	t.Setenv("HAMSAYA_MISSING_KEY", "")
	v, err := EnvSource{}.Get(context.Background(), "HAMSAYA_TEST_KEY")
	if err != nil || v != "abc" {
		t.Fatalf("env: got %q,%v want abc,nil", v, err)
	}
	v, err = EnvSource{}.Get(context.Background(), "HAMSAYA_MISSING_KEY")
	if err != nil || v != "" {
		t.Fatalf("env empty: got %q,%v want \"\",nil", v, err)
	}
}

func TestCachingSource_HitsUpstreamOnce(t *testing.T) {
	upstream := &fakeSource{value: "v"}
	c := NewCaching(upstream, 100*time.Millisecond)

	for i := 0; i < 5; i++ {
		v, err := c.Get(context.Background(), "k")
		if err != nil || v != "v" {
			t.Fatalf("get: %q,%v", v, err)
		}
	}
	if got := upstream.calls.Load(); got != 1 {
		t.Fatalf("upstream called %d times, want 1 (cache miss only)", got)
	}
}

func TestCachingSource_RefreshAfterTTL(t *testing.T) {
	upstream := &fakeSource{value: "v"}
	c := NewCaching(upstream, 20*time.Millisecond)

	_, _ = c.Get(context.Background(), "k")
	time.Sleep(40 * time.Millisecond)
	_, _ = c.Get(context.Background(), "k")

	if got := upstream.calls.Load(); got != 2 {
		t.Fatalf("upstream called %d times, want 2 after TTL expiry", got)
	}
}

func TestCachingSource_DistinctKeys(t *testing.T) {
	upstream := &fakeSource{value: "v"}
	c := NewCaching(upstream, time.Hour)

	_, _ = c.Get(context.Background(), "a")
	_, _ = c.Get(context.Background(), "b")
	_, _ = c.Get(context.Background(), "a")
	_, _ = c.Get(context.Background(), "b")

	if got := upstream.calls.Load(); got != 2 {
		t.Fatalf("distinct keys: upstream called %d, want 2", got)
	}
}

func TestCachingSource_UpstreamErrorNotCached(t *testing.T) {
	upstream := &fakeSource{err: errors.New("boom")}
	c := NewCaching(upstream, time.Hour)

	for i := 0; i < 3; i++ {
		if _, err := c.Get(context.Background(), "k"); err == nil {
			t.Fatal("expected error from upstream")
		}
	}
	if got := upstream.calls.Load(); got != 3 {
		t.Fatalf("error path: upstream called %d, want 3 (no caching of failures)", got)
	}
}

func TestFromEnvOrBackend_Selector(t *testing.T) {
	tests := []struct {
		name      string
		setEnv    string
		wantLabel string
		wantErr   bool
	}{
		{name: "unset", setEnv: "", wantLabel: "env"},
		{name: "env explicit", setEnv: "env", wantLabel: "env"},
		{name: "ssm errors as stub", setEnv: "ssm", wantErr: true},
		{name: "unknown errors", setEnv: "vault", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SECRETS_BACKEND", tc.setEnv)
			src, label, err := FromEnvOrBackend(context.Background())
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got src=%v label=%q", src, label)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if label != tc.wantLabel {
				t.Fatalf("label = %q, want %q", label, tc.wantLabel)
			}
		})
	}
}

func TestSSMSource_StubReturnsErrSourceNotConfigured(t *testing.T) {
	_, err := NewSSMSource(context.Background())
	if !errors.Is(err, ErrSourceNotConfigured) {
		t.Fatalf("expected ErrSourceNotConfigured, got %v", err)
	}
}

func TestMustGet_PanicsOnError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	src := &fakeSource{err: errors.New("boom")}
	_ = MustGet(context.Background(), src, "k")
}
