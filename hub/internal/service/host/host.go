// Package host offers conveniences for the machine running the hub: keep it
// awake while hosting, start the hub at login, let the media server through
// the firewall. Only an instance admin sitting at that machine changes them.
package host

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

type OS interface {
	SetKeepAwake(ctx context.Context, on bool) error
	RunAtLoginSupported() bool
	RunAtLogin() (bool, error)
	SetRunAtLogin(on bool) error
	FirewallSupported() bool
	AddFirewallRule() error
}

type Settings interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
	Update(ctx context.Context, p settings.Patch) (domain.InstanceSettings, error)
}

type Deps struct {
	Settings Settings
	OS       OS
	Logger   *slog.Logger
}

type Service struct {
	d      Deps
	policy domain.Policy
}

func New(d Deps) *Service { return &Service{d: d} }

// Start applies the saved keep-awake choice.
func (s *Service) Start(ctx context.Context) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return
	}
	if err := s.d.OS.SetKeepAwake(ctx, st.Host.KeepAwake); err != nil {
		s.d.Logger.InfoContext(ctx, "keep awake unavailable", slog.Any("error", err))
	}
}

// Stop lets the machine sleep again.
func (s *Service) Stop(ctx context.Context) { _ = s.d.OS.SetKeepAwake(ctx, false) }

func (s *Service) Options(ctx context.Context) (domain.HostOptions, error) {
	st, err := s.d.Settings.Get(ctx)
	if err != nil {
		return domain.HostOptions{}, err
	}
	o := domain.HostOptions{
		KeepAwake:           st.Host.KeepAwake,
		RunAtLoginSupported: s.d.OS.RunAtLoginSupported(),
		FirewallSupported:   s.d.OS.FirewallSupported(),
	}
	if o.RunAtLoginSupported {
		if o.RunAtLogin, err = s.d.OS.RunAtLogin(); err != nil {
			s.d.Logger.WarnContext(ctx, "read run at login", slog.Any("error", err))
		}
	}
	return o, nil
}

func (s *Service) allowed(actor domain.Actor, client domain.ClientInfo) error {
	if err := s.policy.ManageInstance(actor); err != nil {
		return err
	}
	if !client.IsLocal() {
		return domain.Forbidden(domain.ReasonNotLocal)
	}
	return nil
}

// Set changes the options given; nil leaves one as it is.
func (s *Service) Set(ctx context.Context, actor domain.Actor, client domain.ClientInfo, keepAwake, runAtLogin *bool) (domain.HostOptions, error) {
	if err := s.allowed(actor, client); err != nil {
		return domain.HostOptions{}, err
	}
	if keepAwake != nil {
		if _, err := s.d.Settings.Update(ctx, settings.Patch{Host: &domain.HostSettings{KeepAwake: *keepAwake}}); err != nil {
			return domain.HostOptions{}, err
		}
		if err := s.d.OS.SetKeepAwake(ctx, *keepAwake); err != nil {
			s.d.Logger.WarnContext(ctx, "keep awake", slog.Any("error", err))
		}
	}
	if runAtLogin != nil {
		if !s.d.OS.RunAtLoginSupported() {
			return domain.HostOptions{}, domain.Precondition(domain.ReasonUnsupported)
		}
		if err := s.d.OS.SetRunAtLogin(*runAtLogin); err != nil {
			return domain.HostOptions{}, fmt.Errorf("run at login: %w", err)
		}
	}
	return s.Options(ctx)
}

// AddFirewallRule lets the media server receive calls through the host's
// firewall; the system asks the user to confirm.
func (s *Service) AddFirewallRule(_ context.Context, actor domain.Actor, client domain.ClientInfo) error {
	if err := s.allowed(actor, client); err != nil {
		return err
	}
	if !s.d.OS.FirewallSupported() {
		return domain.Precondition(domain.ReasonUnsupported)
	}
	if err := s.d.OS.AddFirewallRule(); err != nil {
		return fmt.Errorf("add firewall rule: %w", err)
	}
	return nil
}
