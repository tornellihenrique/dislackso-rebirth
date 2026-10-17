package exposure

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

var admin = domain.Actor{UserID: 1, InstanceAdmin: true}

// provider reports running at url and serves until stopped, or fails with err.
type provider struct {
	kind domain.ExposureKind
	url  string
	err  error
}

func (p provider) Kind() domain.ExposureKind { return p.kind }

func (p provider) Run(ctx context.Context, _ http.Handler, report func(domain.ExposureStatus)) error {
	if p.err != nil {
		return p.err
	}
	report(domain.ExposureStatus{Kind: p.kind, State: domain.ExposureRunning, URL: p.url})
	<-ctx.Done()
	return nil
}

type harness struct {
	svc      *Service
	settings *settings.Service
	events   *fakes.Events
}

func newHarness(t *testing.T, failWith error) *harness {
	t.Helper()
	store := fakes.NewStore()
	clock := fakes.NewClock()
	st := settings.New(store, &fakes.Secrets{}, clock)
	evs := fakes.NewEvents(store)
	svc := New(Deps{
		Settings: st,
		Factory: func(s domain.ExposureSettings, _ Secrets) ports.ExposureProvider {
			if s.Kind != domain.ExposureCloudflareQuick {
				return nil
			}
			return provider{kind: s.Kind, url: "https://three-words-here.trycloudflare.com", err: failWith}
		},
		Events: evs,
		Clock:  clock,
		Logger: slog.New(slog.DiscardHandler),
	})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.Run(ctx, http.NotFoundHandler())
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return &harness{svc: svc, settings: st, events: evs}
}

func (h *harness) await(t *testing.T, state domain.ExposureState) domain.ExposureStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := h.svc.Status()
		if st.State == state {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("status = %+v, want %s", st, state)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestApplyStartsTheProviderAndPublishesItsAddress(t *testing.T) {
	h := newHarness(t, nil)
	ctx := t.Context()
	h.await(t, domain.ExposureOff)

	s := domain.DefaultExposureSettings()
	s.Kind = domain.ExposureCloudflareQuick
	if _, err := h.svc.Apply(ctx, admin, Patch{Settings: s}); err != nil {
		t.Fatal(err)
	}
	st := h.await(t, domain.ExposureRunning)
	if st.URL != "https://three-words-here.trycloudflare.com" {
		t.Errorf("url = %q", st.URL)
	}
	saved, _ := h.settings.Get(ctx)
	if saved.PublicURL != st.URL || saved.Exposure.Kind != domain.ExposureCloudflareQuick {
		t.Errorf("settings = %s %s, want the tunnel as public URL", saved.Exposure.Kind, saved.PublicURL)
	}
	var told bool
	for _, ev := range h.events.Take() {
		if p, ok := ev.Payload.(events.ExposureChanged); ok && p.Status.State == domain.ExposureRunning &&
			ev.Topics[0] == events.InstanceAdminsTopic {
			told = true
		}
	}
	if !told {
		t.Error("instance admins not told the hub is reachable")
	}

	s.Kind = domain.ExposureLocal
	if _, err := h.svc.Apply(ctx, admin, Patch{Settings: s}); err != nil {
		t.Fatal(err)
	}
	h.await(t, domain.ExposureOff)
}

func TestAProviderProblemShowsInTheStatus(t *testing.T) {
	h := newHarness(t, &ports.ExposureError{Problem: domain.ProblemCloudflaredFetch, Err: errors.New("github unreachable")})
	s := domain.DefaultExposureSettings()
	s.Kind = domain.ExposureCloudflareQuick
	if _, err := h.svc.Apply(t.Context(), admin, Patch{Settings: s}); err != nil {
		t.Fatal(err)
	}
	st := h.await(t, domain.ExposureFailed)
	if st.Problem != domain.ProblemCloudflaredFetch || st.Detail != "github unreachable" {
		t.Errorf("status = %+v", st)
	}
}

func TestApplyChecksTheCallerAndTheSettings(t *testing.T) {
	h := newHarness(t, nil)
	s := domain.DefaultExposureSettings()
	if _, err := h.svc.Apply(t.Context(), domain.Actor{UserID: 2}, Patch{Settings: s}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("member err = %v, want forbidden", err)
	}
	s.Kind = domain.ExposureCloudflareTunnel
	s.TunnelURL = "https://chat.example.com"
	if _, err := h.svc.Apply(t.Context(), admin, Patch{Settings: s}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("tunnel without a token err = %v, want invalid", err)
	}
}
