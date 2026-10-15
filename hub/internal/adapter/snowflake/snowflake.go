// Package snowflake generates time-ordered 64-bit IDs:
// 41 bits of milliseconds since 2026-01-01 UTC | 10-bit worker | 12-bit sequence.
package snowflake

import (
	"fmt"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

const (
	workerBits = 10
	seqBits    = 12
	maxWorker  = 1<<workerBits - 1
	seqMask    = 1<<seqBits - 1
)

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

type Generator struct {
	clock  ports.Clock
	worker int64

	mu     sync.Mutex
	lastMS int64
	seq    int64
}

func New(clock ports.Clock, worker int) (*Generator, error) {
	if worker < 0 || worker > maxWorker {
		return nil, fmt.Errorf("snowflake worker %d out of range 0..%d", worker, maxWorker)
	}
	return &Generator{clock: clock, worker: int64(worker)}, nil
}

func (g *Generator) NewID() domain.ID {
	g.mu.Lock()
	defer g.mu.Unlock()

	ms := g.clock.Now().Sub(epoch).Milliseconds()
	// A clock that steps backwards must not produce IDs that sort earlier.
	ms = max(ms, g.lastMS)
	if ms == g.lastMS {
		g.seq = (g.seq + 1) & seqMask
		if g.seq == 0 {
			// Sequence exhausted within this millisecond: borrow the next one
			// instead of sleeping.
			ms++
		}
	} else {
		g.seq = 0
	}
	g.lastMS = ms
	return domain.ID(ms<<(workerBits+seqBits) | g.worker<<seqBits | g.seq)
}
