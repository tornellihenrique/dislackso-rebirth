package ratelimit

import (
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

type manualClock struct{ now time.Time }

func (c *manualClock) Now() time.Time { return c.now }

func TestAllow(t *testing.T) {
	clock := &manualClock{now: time.Unix(1_000, 0)}
	m := NewMemory(clock)
	limit := ports.Limit{Burst: 3, Every: 10 * time.Second}

	for i := range 3 {
		if ok, _ := m.Allow("k", limit); !ok {
			t.Fatalf("request %d within the burst was refused", i)
		}
	}
	ok, retry := m.Allow("k", limit)
	if ok || retry != 10*time.Second {
		t.Fatalf("over burst: ok=%v retry=%v, want refused with 10s", ok, retry)
	}
	if ok, _ := m.Allow("other", limit); !ok {
		t.Error("keys must not share buckets")
	}

	clock.now = clock.now.Add(4 * time.Second)
	if ok, retry := m.Allow("k", limit); ok || retry != 6*time.Second {
		t.Errorf("partly refilled: ok=%v retry=%v, want refused with 6s", ok, retry)
	}
	clock.now = clock.now.Add(6 * time.Second)
	if ok, _ := m.Allow("k", limit); !ok {
		t.Error("a token should be back after Every")
	}

	clock.now = clock.now.Add(time.Hour)
	for i := range 3 {
		if ok, _ := m.Allow("k", limit); !ok {
			t.Fatalf("refill must cap at the burst, request %d refused", i)
		}
	}
	if ok, _ := m.Allow("k", limit); ok {
		t.Error("refill exceeded the burst")
	}
}

func TestSweepDropsFullBuckets(t *testing.T) {
	clock := &manualClock{now: time.Unix(1_000, 0)}
	m := NewMemory(clock)
	limit := ports.Limit{Burst: 1, Every: time.Second}
	m.Allow("a", limit)
	m.Allow("b", ports.Limit{Burst: 1, Every: 24 * time.Hour})

	clock.now = clock.now.Add(sweepEvery + time.Second)
	m.Allow("c", limit)

	if _, ok := m.buckets["a"]; ok {
		t.Error("refilled bucket survived the sweep")
	}
	if _, ok := m.buckets["b"]; !ok {
		t.Error("bucket still limiting was swept")
	}
}
