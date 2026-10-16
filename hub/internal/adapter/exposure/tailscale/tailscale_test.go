package tailscale

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// probe fails until it has been asked failures times.
type probe struct {
	failures int32
	calls    atomic.Int32
}

func (p *probe) Reachable(context.Context, string) error {
	if p.calls.Add(1) <= p.failures {
		return errors.New("not yet")
	}
	return nil
}

func TestAwaitPublished(t *testing.T) {
	tests := []struct {
		name      string
		failures  int32
		limit     time.Duration
		cancel    bool
		want      bool
		wantCalls int32
	}{
		{name: "answers at once", want: true, limit: time.Second, wantCalls: 1},
		{name: "answers after a few tries", failures: 3, limit: time.Second, want: true, wantCalls: 4},
		{name: "never answers in time", failures: 1 << 30, limit: 30 * time.Millisecond},
		{name: "stopped while waiting", failures: 1 << 30, limit: time.Hour, cancel: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &probe{failures: tt.failures}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.cancel {
				time.AfterFunc(30*time.Millisecond, cancel)
			}
			got := awaitPublished(ctx, p, "https://hub.example.ts.net/", 5*time.Millisecond, tt.limit)
			if got != tt.want {
				t.Errorf("awaitPublished = %v, want %v", got, tt.want)
			}
			if tt.wantCalls != 0 && p.calls.Load() != tt.wantCalls {
				t.Errorf("probed %d times, want %d", p.calls.Load(), tt.wantCalls)
			}
		})
	}
}
