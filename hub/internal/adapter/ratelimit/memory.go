// Package ratelimit implements ports.RateLimiter with in-memory token buckets.
// The hub is a single process, so nothing needs to be shared.
package ratelimit

import (
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

const sweepEvery = 10 * time.Minute

type bucket struct {
	tokens  float64
	updated time.Time
	limit   ports.Limit
}

type Memory struct {
	clock ports.Clock

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

func NewMemory(clock ports.Clock) *Memory {
	return &Memory{clock: clock, buckets: map[string]*bucket{}, lastSweep: clock.Now()}
}

func (m *Memory) Allow(key string, limit ports.Limit) (bool, time.Duration) {
	now := m.clock.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	if now.Sub(m.lastSweep) > sweepEvery {
		m.sweep(now)
	}

	b, ok := m.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(limit.Burst), updated: now, limit: limit}
		m.buckets[key] = b
	}
	b.refill(now)
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) * float64(limit.Every))
}

func (b *bucket) refill(now time.Time) {
	if elapsed := now.Sub(b.updated); elapsed > 0 && b.limit.Every > 0 {
		b.tokens = min(float64(b.limit.Burst), b.tokens+float64(elapsed)/float64(b.limit.Every))
	}
	b.updated = now
}

// sweep drops buckets that refilled completely; they behave exactly like
// new ones.
func (m *Memory) sweep(now time.Time) {
	for key, b := range m.buckets {
		b.refill(now)
		if b.tokens >= float64(b.limit.Burst) {
			delete(m.buckets, key)
		}
	}
	m.lastSweep = now
}
