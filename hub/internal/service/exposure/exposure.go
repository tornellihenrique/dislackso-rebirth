// Package exposure keeps the hub reachable from outside this machine: it runs
// the configured exposure provider, restarts it when the settings change or
// it fails, and keeps the public URL (invite links, emails) in step with it.
package exposure

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

const (
	minBackoff = 5 * time.Second
	maxBackoff = time.Minute
	// A provider that ran this long failed for a new reason; retry soon.
	stableAfter = 5 * time.Minute
)

type Settings interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
	Update(ctx context.Context, p settings.Patch) (domain.InstanceSettings, error)
	Secret(ctx context.Context, key string) (string, error)
	SecretSet(ctx context.Context, key string) (bool, error)
}

// Factory builds the provider for the settings; nil keeps the hub local.
type Factory func(s domain.ExposureSettings, tunnelToken string) ports.ExposureProvider

type Deps struct {
	Settings Settings
	Factory  Factory
	Events   ports.EventPublisher
	Clock    ports.Clock
	Logger   *slog.Logger
}

type Service struct {
	d      Deps
	policy domain.Policy

	mu      sync.Mutex
	status  domain.ExposureStatus
	restart chan struct{}
}

func New(d Deps) *Service {
	return &Service{
		d:       d,
		status:  domain.ExposureStatus{Kind: domain.ExposureLocal, State: domain.ExposureOff, Since: d.Clock.Now()},
		restart: make(chan struct{}, 1),
	}
}

// Status is the provider's current state.
func (s *Service) Status() domain.ExposureStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Patch changes the exposure settings; an empty TunnelToken keeps the stored one.
type Patch struct {
	Settings    domain.ExposureSettings
	TunnelToken string
}

// Apply saves new exposure settings and restarts the provider with them.
func (s *Service) Apply(ctx context.Context, actor domain.Actor, p Patch) (domain.InstanceSettings, error) {
	if err := s.policy.ManageInstance(actor); err != nil {
		return domain.InstanceSettings{}, err
	}
	st, err := s.d.Settings.Update(ctx, settings.Patch{Exposure: &p.Settings, TunnelToken: p.TunnelToken})
	if err != nil {
		return domain.InstanceSettings{}, err
	}
	select {
	case s.restart <- struct{}{}:
	default:
	}
	return st, nil
}

// Run keeps the configured provider serving handler until ctx ends.
func (s *Service) Run(ctx context.Context, handler http.Handler) {
	for {
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			s.runConfigured(runCtx, handler)
		}()
		select {
		case <-ctx.Done():
			cancel()
			<-done
			return
		case <-s.restart:
			cancel()
			<-done
		}
	}
}

func (s *Service) runConfigured(ctx context.Context, handler http.Handler) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		s.report(ctx, failed(domain.ExposureLocal, err, s.d.Clock.Now()))
		return
	}
	token, err := s.d.Settings.Secret(ctx, settings.TunnelTokenSecret)
	if err != nil {
		s.report(ctx, failed(st.Exposure.Kind, err, s.d.Clock.Now()))
		return
	}
	provider := s.d.Factory(st.Exposure, token)
	if provider == nil {
		s.report(ctx, domain.ExposureStatus{Kind: domain.ExposureLocal, State: domain.ExposureOff, Since: s.d.Clock.Now()})
		return
	}
	backoff := minBackoff
	for {
		started := s.d.Clock.Now()
		err := provider.Run(ctx, handler, func(st domain.ExposureStatus) { s.report(ctx, st) })
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			err = errors.New("stopped")
		}
		s.d.Logger.WarnContext(ctx, "exposure provider stopped; retrying",
			slog.String("provider", string(provider.Kind())), slog.Any("error", err), slog.Duration("in", backoff))
		s.report(ctx, failed(provider.Kind(), err, s.d.Clock.Now()))
		if s.d.Clock.Now().Sub(started) >= stableAfter {
			backoff = minBackoff
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func failed(kind domain.ExposureKind, err error, now time.Time) domain.ExposureStatus {
	st := domain.ExposureStatus{
		Kind: kind, State: domain.ExposureFailed, Since: now,
		Problem: domain.ProblemExposureFailed, Detail: err.Error(),
	}
	if ee, ok := errors.AsType[*ports.ExposureError](err); ok {
		st.Problem, st.Detail = ee.Problem, ee.Err.Error()
	}
	return st
}

// report records a provider's state, tells instance admins, and makes a new
// public address the instance's public URL.
func (s *Service) report(ctx context.Context, st domain.ExposureStatus) {
	s.mu.Lock()
	prev := s.status
	if prev.Kind == st.Kind && prev.State == st.State && prev.URL == st.URL &&
		prev.LoginURL == st.LoginURL && prev.Problem == st.Problem && prev.Detail == st.Detail {
		s.mu.Unlock()
		return
	}
	s.status = st
	s.mu.Unlock()

	attrs := []any{slog.String("provider", string(st.Kind)), slog.String("state", string(st.State))}
	switch st.State {
	case domain.ExposureRunning:
		s.d.Logger.InfoContext(ctx, "hub reachable", append(attrs, slog.String("url", st.URL))...)
	case domain.ExposureNeedsLogin:
		s.d.Logger.WarnContext(ctx, "sign the hub in to Tailscale", append(attrs, slog.String("url", st.LoginURL))...)
	case domain.ExposureFailed:
		s.d.Logger.WarnContext(ctx, "hub not reachable", append(attrs, slog.String("problem", st.Problem))...)
	case domain.ExposureOff, domain.ExposureStarting:
	}
	if st.State == domain.ExposureRunning && st.URL != "" {
		s.syncPublicURL(ctx, st)
	}
	s.d.Events.Publish(ctx, events.Event{
		Topics:  []events.Topic{events.InstanceAdminsTopic},
		Payload: events.ExposureChanged{Status: st},
	})
}

func (s *Service) syncPublicURL(ctx context.Context, st domain.ExposureStatus) {
	cur, err := s.d.Settings.Get(ctx)
	if err != nil || cur.PublicURL == st.URL {
		return
	}
	url := st.URL
	if _, err := s.d.Settings.Update(ctx, settings.Patch{PublicURL: &url}); err != nil {
		s.d.Logger.WarnContext(ctx, "save public url", slog.Any("error", err))
		return
	}
	if cur.PublicURL != "" && st.Kind == domain.ExposureCloudflareQuick {
		s.d.Logger.WarnContext(ctx, "the public URL changed: links sent before no longer work",
			slog.String("old", cur.PublicURL), slog.String("new", st.URL))
	}
}
