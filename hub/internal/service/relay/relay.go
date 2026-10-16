// Package relay hands voice clients relay servers (TURN) from the configured
// provider, counts what they relay from their telemetry, and stops issuing
// relays past the monthly budget (ARCHITECTURE §4.4).
package relay

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

const (
	// Clients report every 30 s; a connection silent for longer has ended.
	staleAfter = 90 * time.Second
	// A report's growth past this is not believed: a client cannot relay
	// that much in one interval.
	maxDelta = 2 << 30
	// Above this share of the budget, admins are warned.
	warnShare = 0.9
	gb        = 1_000_000_000
)

type Settings interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
	Update(ctx context.Context, p settings.Patch) (domain.InstanceSettings, error)
	Secret(ctx context.Context, key string) (string, error)
}

// Factory builds the provider for the settings; nil issues nothing.
type Factory func(s domain.TURNSettings, secret string) ports.IceServerProvider

type Deps struct {
	Settings Settings
	Factory  Factory
	Usage    ports.TURNUsageRepo
	Clock    ports.Clock
	Logger   *slog.Logger
}

// Telemetry is one client report about its voice connection.
type Telemetry struct {
	SessionID string
	// Relay: the selected candidate pair goes through a relay.
	Relay bool
	// Bytes sent and received since the connection started.
	Bytes uint64
}

type sessionKey struct {
	user    domain.ID
	session string
}

type sample struct {
	relay bool
	bytes uint64
	at    time.Time
}

type Service struct {
	d      Deps
	policy domain.Policy

	mu       sync.Mutex
	sessions map[sessionKey]sample
	warned   string
}

func New(d Deps) *Service {
	return &Service{d: d, sessions: map[sessionKey]sample{}}
}

func (s *Service) Name() string { return "relay" }

// Issue gives a voice client relay servers, unless none is configured or the
// month's budget is spent.
func (s *Service) Issue(ctx context.Context, userID domain.ID, ttl time.Duration) ([]ports.IceServer, error) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil || st.TURN.Kind == domain.TURNNone {
		return nil, err
	}
	if !st.TURN.BudgetOverride {
		usage, err := s.Usage(ctx)
		if err != nil {
			return nil, err
		}
		if usage.OverBudget {
			s.warnOnce(ctx, "over", "relay budget spent: relays are off until next month or an admin overrides")
			return nil, nil
		}
	}
	secret, err := s.d.Settings.Secret(ctx, settings.TURNSecret)
	if err != nil {
		return nil, err
	}
	provider := s.d.Factory(st.TURN, secret)
	if provider == nil {
		return nil, nil
	}
	return provider.Issue(ctx, userID, ttl)
}

// Report records a client's telemetry; what went through a relay counts
// against the month.
func (s *Service) Report(ctx context.Context, userID domain.ID, t Telemetry) {
	if t.SessionID == "" {
		return
	}
	now := s.d.Clock.Now()
	key := sessionKey{userID, t.SessionID}
	s.mu.Lock()
	prev, seen := s.sessions[key]
	s.sessions[key] = sample{relay: t.Relay, bytes: t.Bytes, at: now}
	for k, v := range s.sessions {
		if now.Sub(v.at) > staleAfter {
			delete(s.sessions, k)
		}
	}
	s.mu.Unlock()
	if !t.Relay {
		return
	}
	var base uint64
	if seen {
		base = prev.bytes
	}
	if t.Bytes <= base {
		return
	}
	delta := min(t.Bytes-base, maxDelta)
	if err := s.d.Usage.AddTURNUsage(ctx, month(now), int64(delta)); err != nil {
		s.d.Logger.WarnContext(ctx, "record relay usage", slog.Any("error", err))
		return
	}
	if usage, err := s.Usage(ctx); err == nil && usage.BudgetGB > 0 &&
		float64(usage.MonthBytes) >= warnShare*float64(usage.BudgetGB)*gb && !usage.OverBudget {
		s.warnOnce(ctx, "near", "relay usage is above 90 % of the monthly budget")
	}
}

// Usage is this month's relayed bytes, where the month is heading, and how
// many voice connections go through a relay now.
func (s *Service) Usage(ctx context.Context) (domain.TURNUsage, error) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return domain.TURNUsage{}, err
	}
	now := s.d.Clock.Now()
	bytes, err := s.d.Usage.TURNUsage(ctx, month(now))
	if err != nil {
		return domain.TURNUsage{}, err
	}
	budget := st.Limits.TURNMonthlyBudgetGB
	u := domain.TURNUsage{
		MonthBytes:     bytes,
		ProjectedBytes: project(bytes, now),
		BudgetGB:       budget,
		OverBudget:     budget > 0 && bytes >= int64(budget)*gb,
	}
	s.mu.Lock()
	for _, v := range s.sessions {
		if now.Sub(v.at) <= staleAfter {
			u.VoiceSessions++
			if v.relay {
				u.RelayedSessions++
			}
		}
	}
	s.mu.Unlock()
	return u, nil
}

// Patch changes the relay settings; an empty Secret keeps the stored one.
type Patch struct {
	Settings domain.TURNSettings
	Secret   string
	BudgetGB int
}

func (s *Service) Apply(ctx context.Context, actor domain.Actor, p Patch) (domain.InstanceSettings, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return domain.InstanceSettings{}, err
	}
	s.mu.Lock()
	s.warned = ""
	s.mu.Unlock()
	return s.d.Settings.Update(ctx, settings.Patch{TURN: &p.Settings, TURNSecret: p.Secret, TURNBudgetGB: &p.BudgetGB})
}

func (s *Service) warnOnce(ctx context.Context, what, msg string) {
	tag := month(s.d.Clock.Now()) + what
	s.mu.Lock()
	first := s.warned != tag
	s.warned = tag
	s.mu.Unlock()
	if first {
		s.d.Logger.WarnContext(ctx, msg)
	}
}

func month(t time.Time) string { return t.UTC().Format("2006-01") }

// project extends the month's pace to the whole month.
func project(bytes int64, now time.Time) int64 {
	now = now.UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	elapsed := now.Sub(start)
	if elapsed < time.Hour {
		return bytes
	}
	total := start.AddDate(0, 1, 0).Sub(start)
	return int64(float64(bytes) * float64(total) / float64(elapsed))
}
