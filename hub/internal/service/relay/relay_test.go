package relay

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/ice"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

type usageRepo struct {
	mu     sync.Mutex
	months map[string]int64
}

func (u *usageRepo) AddTURNUsage(_ context.Context, month string, bytes int64) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.months[month] += bytes
	return nil
}

func (u *usageRepo) TURNUsage(_ context.Context, month string) (int64, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.months[month], nil
}

var admin = domain.Actor{UserID: 1, InstanceAdmin: true}

type harness struct {
	svc   *Service
	clock *fakes.Clock
	usage *usageRepo
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	clock := fakes.NewClock()
	usage := &usageRepo{months: map[string]int64{}}
	st := settings.New(fakes.NewStore(), &fakes.Secrets{}, clock)
	svc := New(Deps{
		Settings: st,
		Factory: func(s domain.TURNSettings, secret string) ports.IceServerProvider {
			if s.Kind != domain.TURNStatic {
				return nil
			}
			return ice.Static{URLs: s.URLs, Username: s.Username, Credential: secret}
		},
		Usage:  usage,
		Clock:  clock,
		Logger: slog.New(slog.DiscardHandler),
	})
	return &harness{svc: svc, clock: clock, usage: usage}
}

func (h *harness) configure(t *testing.T, budgetGB int, override bool) {
	t.Helper()
	_, err := h.svc.Apply(t.Context(), admin, Patch{
		Settings: domain.TURNSettings{
			Kind: domain.TURNStatic, URLs: []string{"turn:turn.example:3478"}, Username: "team", BudgetOverride: override,
		},
		Secret:   "s3cret",
		BudgetGB: budgetGB,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIssue(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	if got, err := h.svc.Issue(ctx, 7, time.Hour); err != nil || got != nil {
		t.Fatalf("without a relay = %v, %v", got, err)
	}
	h.configure(t, 1, false)
	want := []ports.IceServer{{URLs: []string{"turn:turn.example:3478"}, Username: "team", Credential: "s3cret"}}
	got, err := h.svc.Issue(ctx, 7, time.Hour)
	if err != nil || !cmp.Equal(got, want) {
		t.Fatalf("issue = %v, %v", got, err)
	}

	// A spent budget stops new relays, unless the admin overrides it.
	h.usage.months[month(h.clock.Now())] = gb
	if got, _ := h.svc.Issue(ctx, 7, time.Hour); got != nil {
		t.Errorf("over budget = %v, want none", got)
	}
	h.configure(t, 1, true)
	if got, _ := h.svc.Issue(ctx, 7, time.Hour); !cmp.Equal(got, want) {
		t.Errorf("overridden = %v", got)
	}
	h.configure(t, 0, false)
	if got, _ := h.svc.Issue(ctx, 7, time.Hour); !cmp.Equal(got, want) {
		t.Errorf("no budget = %v", got)
	}
}

func TestReportCountsWhatGoesThroughTheRelay(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	report := func(user domain.ID, session string, relay bool, bytes uint64) {
		h.svc.Report(ctx, user, Telemetry{SessionID: session, Relay: relay, Bytes: bytes})
	}
	report(1, "a", true, 1000)
	report(1, "a", true, 1500)
	report(2, "b", false, 9000)
	// Another user cannot inflate someone else's session.
	report(2, "a", true, 100)
	// A counter that went backwards adds nothing.
	report(1, "a", true, 1200)
	report(1, "a", true, 1300)
	// Implausible growth counts as the cap at most.
	report(3, "c", true, 0)
	report(3, "c", true, 10*maxDelta)

	got := h.usage.months[month(h.clock.Now())]
	if want := int64(1500 + 100 + 100 + maxDelta); got != want {
		t.Errorf("relayed bytes = %d, want %d", got, want)
	}
	u, err := h.svc.Usage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if u.VoiceSessions != 4 || u.RelayedSessions != 3 {
		t.Errorf("sessions = %d relayed of %d, want 3 of 4", u.RelayedSessions, u.VoiceSessions)
	}
	h.clock.Advance(staleAfter + time.Second)
	if u, _ := h.svc.Usage(ctx); u.VoiceSessions != 0 {
		t.Errorf("silent sessions still counted: %d", u.VoiceSessions)
	}
}

func TestUsageProjectsTheMonth(t *testing.T) {
	if got := project(10*gb, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)); got != 20*gb {
		t.Errorf("halfway through = %d, want double", got)
	}
	if got := project(5, time.Date(2026, 10, 1, 0, 30, 0, 0, time.UTC)); got != 5 {
		t.Errorf("first hour = %d, want the bytes so far", got)
	}
}

func TestApplyNeedsAnInstanceAdmin(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Apply(t.Context(), domain.Actor{UserID: 2}, Patch{Settings: domain.TURNSettings{Kind: domain.TURNNone}})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("err = %v, want forbidden", err)
	}
}
