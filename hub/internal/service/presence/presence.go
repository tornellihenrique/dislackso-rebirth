// Package presence tracks who is online. It lives in memory only: a user is
// online while at least one gateway connection is open.
package presence

import (
	"context"
	"log/slog"
	"sync"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/access"
)

type user struct {
	pref   domain.StatusPref
	custom string
	// conns maps each open connection to whether it reported idle.
	conns map[string]bool
	shown domain.Presence
}

type Tracker struct {
	members ports.MemberRepo
	events  ports.EventPublisher
	logger  *slog.Logger

	// mu also serializes publishing, so receivers see changes in order.
	mu    sync.Mutex
	users map[domain.ID]*user
}

func New(members ports.MemberRepo, pub ports.EventPublisher, logger *slog.Logger) *Tracker {
	return &Tracker{members: members, events: pub, logger: logger, users: map[domain.ID]*user{}}
}

// Connect registers a gateway connection of u.
func (t *Tracker) Connect(ctx context.Context, u domain.User, connID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.users[u.ID]
	if !ok {
		p = &user{conns: map[string]bool{}, shown: domain.Presence{UserID: u.ID, Status: domain.PresenceOffline}}
		t.users[u.ID] = p
	}
	p.pref, p.custom = u.StatusPref, u.CustomStatus
	p.conns[connID] = false
	t.changed(ctx, u.ID, p)
}

func (t *Tracker) Disconnect(ctx context.Context, userID domain.ID, connID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.users[userID]
	if !ok {
		return
	}
	delete(p.conns, connID)
	t.changed(ctx, userID, p)
	if len(p.conns) == 0 {
		delete(t.users, userID)
	}
}

// SetIdle records whether a connection reported no recent input.
func (t *Tracker) SetIdle(ctx context.Context, userID domain.ID, connID string, idle bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.users[userID]
	if !ok {
		return
	}
	if _, open := p.conns[connID]; !open {
		return
	}
	p.conns[connID] = idle
	t.changed(ctx, userID, p)
}

// SetPreference applies a new status preference or custom status.
func (t *Tracker) SetPreference(ctx context.Context, userID domain.ID, pref domain.StatusPref, custom string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.users[userID]
	if !ok {
		return
	}
	p.pref, p.custom = pref, custom
	t.changed(ctx, userID, p)
}

// Presences returns what others see for each user, offline when unknown.
func (t *Tracker) Presences(userIDs []domain.ID) []domain.Presence {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]domain.Presence, len(userIDs))
	for i, id := range userIDs {
		if p, ok := t.users[id]; ok {
			out[i] = p.shown
		} else {
			out[i] = domain.Presence{UserID: id, Status: domain.PresenceOffline}
		}
	}
	return out
}

// changed recomputes the presence and, when others would see a difference,
// tells everyone sharing a guild with the user.
func (t *Tracker) changed(ctx context.Context, userID domain.ID, p *user) {
	idle := 0
	for _, isIdle := range p.conns {
		if isIdle {
			idle++
		}
	}
	next := domain.EffectivePresence(userID, p.pref, p.custom, len(p.conns), idle)
	if next == p.shown {
		return
	}
	p.shown = next
	topics, err := access.GuildTopics(ctx, t.members, userID)
	if err != nil {
		t.logger.WarnContext(ctx, "publish presence", slog.String("user_id", userID.String()), slog.Any("error", err))
		return
	}
	t.events.Publish(ctx, events.Event{Topics: topics, Payload: events.PresenceUpdated{Presence: next}})
}
