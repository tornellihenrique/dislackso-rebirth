package fakes

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// Codes returns TESTAAAB, TESTAAAC, … counting in the invite alphabet.
type Codes struct {
	mu sync.Mutex
	n  int
	// Repeat makes the next n calls return the previous code again.
	Repeat int
}

func (c *Codes) NewInviteCode() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Repeat > 0 && c.n > 0 {
		c.Repeat--
	} else {
		c.n++
	}
	suffix := make([]byte, 4)
	for i, n := len(suffix)-1, c.n; i >= 0; i, n = i-1, n/len(domain.InviteCodeAlphabet) {
		suffix[i] = domain.InviteCodeAlphabet[n%len(domain.InviteCodeAlphabet)]
	}
	return "TEST" + string(suffix), nil
}

// Events records what services publish, after the fake store commits.
type Events struct {
	mu    sync.Mutex
	hooks events.CommitHooks
	all   []events.Event
}

func NewEvents(hooks events.CommitHooks) *Events { return &Events{hooks: hooks} }

func (e *Events) Publish(ctx context.Context, evs ...events.Event) {
	e.hooks.AfterCommit(ctx, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.all = append(e.all, evs...)
	})
}

// Take returns the events recorded so far and forgets them.
func (e *Events) Take() []events.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := e.all
	e.all = nil
	return out
}

// Blobs is an in-memory blob store.
type Blobs struct {
	mu   sync.Mutex
	data map[string][]byte
	Puts int
}

func (b *Blobs) Put(_ context.Context, key string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.data == nil {
		b.data = map[string][]byte{}
	}
	if _, ok := b.data[key]; !ok {
		b.data[key] = data
		b.Puts++
	}
	return nil
}

func (b *Blobs) Open(_ context.Context, key string) (ports.Blob, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.data[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return blob{bytes.NewReader(data)}, nil
}

type blob struct{ *bytes.Reader }

func (blob) Close() error { return nil }

// Images accepts input starting with "img:" and returns it as a PNG of the
// requested size.
type Images struct{}

func (Images) Square(r io.Reader, size int) (ports.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return ports.Image{}, err
	}
	if !strings.HasPrefix(string(data), "img:") {
		return ports.Image{}, domain.Invalid(domain.FieldViolation{Field: "file", Reason: domain.ReasonUnsupportedFile})
	}
	return ports.Image{Data: append([]byte("png:"), data...), Mime: "image/png", Width: size, Height: size}, nil
}

// Presence reports users in Status as having that status, others as offline.
type Presence struct {
	mu     sync.Mutex
	Status map[domain.ID]domain.PresenceStatus
}

func (p *Presence) Presences(ids []domain.ID) []domain.Presence {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]domain.Presence, len(ids))
	for i, id := range ids {
		st, ok := p.Status[id]
		if !ok {
			st = domain.PresenceOffline
		}
		out[i] = domain.Presence{UserID: id, Status: st}
	}
	return out
}

// Verification answers VerificationRequired with its value.
type Verification bool

func (v Verification) VerificationRequired(context.Context) bool { return bool(v) }
