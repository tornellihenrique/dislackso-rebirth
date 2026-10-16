package host

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

type machine struct {
	awake, atLogin, firewall bool
}

func (m *machine) SetKeepAwake(_ context.Context, on bool) error {
	m.awake = on
	return nil
}
func (m *machine) RunAtLoginSupported() bool   { return true }
func (m *machine) RunAtLogin() (bool, error)   { return m.atLogin, nil }
func (m *machine) SetRunAtLogin(on bool) error { m.atLogin = on; return nil }
func (m *machine) FirewallSupported() bool     { return true }
func (m *machine) AddFirewallRule() error      { m.firewall = true; return nil }

var (
	admin  = domain.Actor{UserID: 1, InstanceAdmin: true}
	here   = domain.ClientInfo{IP: netip.MustParseAddr("127.0.0.1")}
	remote = domain.ClientInfo{IP: netip.MustParseAddr("198.51.100.7")}
)

func newService(t *testing.T) (*Service, *machine) {
	t.Helper()
	m := &machine{}
	clock := fakes.NewClock()
	svc := New(Deps{
		Settings: settings.New(fakes.NewStore(), &fakes.Secrets{}, clock),
		OS:       m,
		Logger:   slog.New(slog.DiscardHandler),
	})
	return svc, m
}

func TestStartAppliesTheSavedChoice(t *testing.T) {
	svc, m := newService(t)
	svc.Start(t.Context())
	if !m.awake {
		t.Error("keep awake is on by default")
	}
	svc.Stop(t.Context())
	if m.awake {
		t.Error("still awake after Stop")
	}
}

func TestOnlyAnAdminAtTheHostChangesOptions(t *testing.T) {
	svc, m := newService(t)
	ctx := t.Context()
	off, on := false, true

	if _, err := svc.Set(ctx, admin, remote, &off, nil); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("from elsewhere err = %v, want forbidden", err)
	}
	if _, err := svc.Set(ctx, domain.Actor{UserID: 2}, here, &off, nil); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("member err = %v, want forbidden", err)
	}
	if err := svc.AddFirewallRule(ctx, admin, remote); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("firewall from elsewhere err = %v, want forbidden", err)
	}

	got, err := svc.Set(ctx, admin, here, &off, &on)
	if err != nil {
		t.Fatal(err)
	}
	if got.KeepAwake || !got.RunAtLogin || m.awake || !m.atLogin {
		t.Errorf("options = %+v, machine = %+v", got, m)
	}
	if err := svc.AddFirewallRule(ctx, admin, here); err != nil || !m.firewall {
		t.Errorf("firewall: %v, added %v", err, m.firewall)
	}
}
