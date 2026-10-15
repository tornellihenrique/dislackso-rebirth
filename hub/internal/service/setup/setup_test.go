package setup

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/auth"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
)

type stubSessions struct{}

func (stubSessions) StartSession(_ context.Context, _ domain.ClientInfo, u domain.User) (auth.SignedIn, error) {
	return auth.SignedIn{User: u, Token: "session-token"}, nil
}

var (
	local  = domain.ClientInfo{IP: netip.MustParseAddr("127.0.0.1"), Locale: "en"}
	remote = domain.ClientInfo{IP: netip.MustParseAddr("198.51.100.4")}
)

type harness struct {
	svc      *Service
	store    *fakes.Store
	settings *settings.Service
	limiter  *fakes.Limiter
	mail     *fakes.MailTester
}

func newHarness() *harness {
	store, clock := fakes.NewStore(), fakes.NewClock()
	h := &harness{store: store, limiter: &fakes.Limiter{}, mail: &fakes.MailTester{}}
	h.settings = settings.New(store, &fakes.Secrets{}, clock)
	h.svc = New(Deps{
		Users: store, Tx: store, Hasher: fakes.Hasher{}, IDs: &fakes.IDs{}, Clock: clock,
		Limiter: h.limiter, Settings: h.settings, Sessions: stubSessions{}, Mail: h.mail,
		Token: "setup-secret",
	})
	return h
}

func validInput() CompleteInput {
	return CompleteInput{
		Token:         "setup-secret",
		InstanceName:  " Team ",
		PublicURL:     "http://localhost:7300/",
		AdminUsername: "admin",
		AdminEmail:    "admin@example.com",
		AdminPassword: "correct horse",
	}
}

func requireReason(t *testing.T, err, kind error, reason string) {
	t.Helper()
	de, ok := domain.AsError(err)
	if !errors.Is(err, kind) || !ok || de.Reason != reason {
		t.Fatalf("err = %v, want %v (%s)", err, kind, reason)
	}
}

func TestCheckToken(t *testing.T) {
	h := newHarness()
	ctx := t.Context()

	if err := h.svc.CheckToken(ctx, local, "setup-secret"); err != nil {
		t.Errorf("valid local check: %v", err)
	}
	requireReason(t, h.svc.CheckToken(ctx, remote, "setup-secret"), domain.ErrForbidden, domain.ReasonNotLocal)
	requireReason(t, h.svc.CheckToken(ctx, local, "wrong"), domain.ErrForbidden, domain.ReasonSetupTokenInvalid)
	requireReason(t, h.svc.CheckToken(ctx, local, ""), domain.ErrForbidden, domain.ReasonSetupTokenInvalid)

	h.limiter.Deny = map[string]time.Duration{"setup:ip:": time.Second}
	if err := h.svc.CheckToken(ctx, local, "setup-secret"); !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("err = %v, want rate limited", err)
	}
}

func TestComplete(t *testing.T) {
	h := newHarness()
	ctx := t.Context()

	smtp := domain.SMTPSettings{Enabled: true, Host: "smtp.example.com", Port: 465, Security: domain.SMTPTLS, FromAddress: "hub@example.com"}
	in := validInput()
	in.SMTP = &smtp
	in.SMTPPassword = "app-password"
	signed, err := h.svc.Complete(ctx, local, in)
	if err != nil {
		t.Fatal(err)
	}
	if !signed.User.IsInstanceAdmin || !signed.User.EmailVerified() || signed.Token == "" {
		t.Errorf("admin = %+v", signed.User)
	}
	st, _ := h.settings.Get(ctx)
	if !st.SetupComplete() || st.Name != "Team" || st.PublicURL != "http://localhost:7300" || !st.SMTP.Enabled {
		t.Errorf("settings = %+v", st)
	}
	if _, pw, _ := h.settings.SMTPConfig(ctx); pw != "app-password" {
		t.Errorf("smtp password = %q", pw)
	}

	requireReason(t, h.svc.CheckToken(ctx, local, "setup-secret"), domain.ErrFailedPrecondition, domain.ReasonSetupComplete)
	_, err = h.svc.Complete(ctx, local, validInput())
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonSetupComplete)
	if n, _ := h.store.CountUsers(ctx); n != 1 {
		t.Errorf("users = %d, want 1", n)
	}
}

func TestCompleteValidates(t *testing.T) {
	h := newHarness()
	in := validInput()
	in.InstanceName = ""
	in.PublicURL = "localhost"
	in.AdminUsername = "x"
	in.AdminPassword = "12345678"
	_, err := h.svc.Complete(t.Context(), local, in)
	de, ok := domain.AsError(err)
	if !ok || !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("err = %v, want invalid", err)
	}
	fields := map[string]string{}
	for _, f := range de.Fields {
		fields[f.Field] = f.Reason
	}
	want := map[string]string{
		"name":           domain.ReasonFieldRequired,
		"public_url":     domain.ReasonFieldInvalidFormat,
		"admin_username": domain.ReasonFieldTooShort,
		"admin_password": domain.ReasonFieldTooCommon,
	}
	for f, r := range want {
		if fields[f] != r {
			t.Errorf("field %s = %q, want %q (all: %v)", f, fields[f], r, fields)
		}
	}
	if pending, _ := h.svc.Pending(t.Context()); !pending {
		t.Error("a failed attempt must leave setup pending")
	}
}

func TestCompleteFromRemoteIsRefused(t *testing.T) {
	h := newHarness()
	_, err := h.svc.Complete(t.Context(), remote, validInput())
	requireReason(t, err, domain.ErrForbidden, domain.ReasonNotLocal)
}

func TestSendTestEmail(t *testing.T) {
	h := newHarness()
	ctx := t.Context()
	smtp := domain.SMTPSettings{Host: "smtp.example.com", Port: 587, Security: domain.SMTPStartTLS, FromAddress: "hub@example.com"}

	if err := h.svc.SendTestEmail(ctx, local, "setup-secret", smtp, "pw", "me@example.com"); err != nil {
		t.Fatal(err)
	}
	if len(h.mail.Sent) != 1 || h.mail.Sent[0].To != "me@example.com" || !h.mail.Settings[0].Enabled {
		t.Errorf("sent = %+v with %+v", h.mail.Sent, h.mail.Settings)
	}

	h.mail.Err = errors.New("dial tcp: connection refused")
	err := h.svc.SendTestEmail(ctx, local, "setup-secret", smtp, "pw", "me@example.com")
	de, ok := domain.AsError(err)
	if !ok || de.Reason != domain.ReasonEmailDeliveryFailed || de.Message != "dial tcp: connection refused" {
		t.Errorf("err = %v, want the SMTP error passed through", err)
	}

	if err := h.svc.SendTestEmail(ctx, local, "wrong", smtp, "pw", "me@example.com"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("bad token: %v", err)
	}
}
