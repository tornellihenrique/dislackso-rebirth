package instance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

type stubResetLinks struct{}

func (stubResetLinks) IssuePasswordResetLink(_ context.Context, id domain.ID) (string, time.Time, error) {
	return "https://hub.example.com/reset-password?token=for-" + id.String(), time.Unix(10, 0), nil
}

var (
	admin  = domain.Actor{UserID: 1, SessionID: 100, InstanceAdmin: true}
	member = domain.Actor{UserID: 2, SessionID: 200}
)

func newService(t *testing.T) (*Service, *fakes.Store, *fakes.Emails) {
	t.Helper()
	store, clock, emails := fakes.NewStore(), fakes.NewClock(), &fakes.Emails{}
	for _, u := range []domain.User{
		{ID: 1, Username: "admin", Email: "admin@example.com", IsInstanceAdmin: true},
		{ID: 2, Username: "ana", Email: "ana@example.com"},
	} {
		if err := store.CreateUser(t.Context(), u); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.CreateSession(t.Context(), domain.Session{ID: 200, UserID: 2, TokenHash: []byte("s"), ExpiresAt: clock.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	svc := New(Deps{
		Users: store, Sessions: store, Tx: store, Clock: clock, Events: fakes.NewEvents(store),
		Settings:   settings.New(store, &fakes.Secrets{}, clock),
		ResetLinks: stubResetLinks{},
		Emails:     emails,
	})
	return svc, store, emails
}

func TestEverythingRequiresAnInstanceAdmin(t *testing.T) {
	svc, _, _ := newService(t)
	ctx := t.Context()
	calls := map[string]func() error{
		"Settings":       func() error { _, err := svc.Settings(ctx, member); return err },
		"UpdateSettings": func() error { _, err := svc.UpdateSettings(ctx, member, settings.Patch{}); return err },
		"SendTestEmail":  func() error { return svc.SendTestEmail(ctx, member, domain.ClientInfo{}, "a@example.com") },
		"Users":          func() error { _, err := svc.Users(ctx, member); return err },
		"SetUserDisabled": func() error {
			_, err := svc.SetUserDisabled(ctx, member, 1, true)
			return err
		},
		"SetUserInstanceAdmin": func() error {
			_, err := svc.SetUserInstanceAdmin(ctx, member, 2, true)
			return err
		},
		"CreatePasswordResetLink": func() error { _, _, err := svc.CreatePasswordResetLink(ctx, member, 1); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("%s: err = %v, want forbidden", name, err)
		}
	}
}

func TestSetUserDisabledSignsOut(t *testing.T) {
	svc, store, _ := newService(t)
	ctx := t.Context()

	u, err := svc.SetUserDisabled(ctx, admin, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if !u.Disabled() {
		t.Error("user not disabled")
	}
	if active, _ := store.ActiveSessions(ctx, 2, time.Unix(0, 0)); len(active) != 0 {
		t.Errorf("sessions survived: %v", active)
	}
	if u, _ = svc.SetUserDisabled(ctx, admin, 2, false); u.Disabled() {
		t.Error("user still disabled")
	}

	if _, err := svc.SetUserDisabled(ctx, admin, 1, true); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("disabling yourself: err = %v, want forbidden", err)
	}
	if _, err := svc.SetUserInstanceAdmin(ctx, admin, 1, false); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("demoting yourself: err = %v, want forbidden", err)
	}
	if _, err := svc.SetUserDisabled(ctx, admin, 99, true); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown user: err = %v, want not found", err)
	}
}

func TestPromoteAndList(t *testing.T) {
	svc, _, _ := newService(t)
	ctx := t.Context()
	u, err := svc.SetUserInstanceAdmin(ctx, admin, 2, true)
	if err != nil || !u.IsInstanceAdmin {
		t.Fatalf("promote: %+v, %v", u, err)
	}
	users, err := svc.Users(ctx, admin)
	if err != nil || len(users) != 2 {
		t.Errorf("Users = %v, %v", users, err)
	}
	link, _, err := svc.CreatePasswordResetLink(ctx, admin, 2)
	if err != nil || link != "https://hub.example.com/reset-password?token=for-2" {
		t.Errorf("link = %q, %v", link, err)
	}
}

func TestSettingsAndTestEmail(t *testing.T) {
	svc, _, emails := newService(t)
	ctx := t.Context()

	name := "Team"
	view, err := svc.UpdateSettings(ctx, admin, settings.Patch{Name: &name, MarkSetupComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if view.Settings.Name != "Team" || view.Settings.SetupComplete() {
		t.Errorf("settings = %+v (admins must not be able to mark setup complete)", view.Settings)
	}

	if err := svc.SendTestEmail(ctx, admin, domain.ClientInfo{Locale: "pt-BR"}, "ops@example.com"); err != nil {
		t.Fatal(err)
	}
	if e, ok := emails.Last("test"); !ok || e.To != "ops@example.com" || e.Locale != "pt-BR" {
		t.Errorf("test email = %+v", e)
	}
	emails.Err = errors.New("535 authentication failed")
	err = svc.SendTestEmail(ctx, admin, domain.ClientInfo{}, "ops@example.com")
	de, ok := domain.AsError(err)
	if !ok || de.Reason != domain.ReasonEmailDeliveryFailed || de.Message == "" {
		t.Errorf("err = %v, want delivery failure with the server reply", err)
	}
}
