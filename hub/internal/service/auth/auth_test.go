package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

type stubSettings struct{ s domain.InstanceSettings }

func (s *stubSettings) Get(context.Context) (domain.InstanceSettings, error) { return s.s, nil }

type stubInvites struct {
	claimed []domain.ID
}

func (i *stubInvites) ClaimForRegistration(_ context.Context, code string) (func(context.Context, domain.ID) error, error) {
	if code != "good-invite" {
		return nil, domain.Invalid(domain.FieldViolation{Field: "invite_code", Reason: domain.ReasonFieldInvalid})
	}
	return func(_ context.Context, id domain.ID) error {
		i.claimed = append(i.claimed, id)
		return nil
	}, nil
}

type harness struct {
	svc      *Service
	store    *fakes.Store
	clock    *fakes.Clock
	mailer   *fakes.Mailer
	emails   *fakes.Emails
	limiter  *fakes.Limiter
	settings *stubSettings
	invites  *stubInvites
	events   *fakes.Events
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	completed := time.Unix(1, 0)
	st := domain.DefaultInstanceSettings()
	st.Name = "Team"
	st.PublicURL = "https://hub.example.com"
	st.SetupCompletedAt = &completed
	st.RegistrationMode = domain.RegistrationOpen

	h := &harness{
		store:    fakes.NewStore(),
		clock:    fakes.NewClock(),
		mailer:   &fakes.Mailer{},
		emails:   &fakes.Emails{},
		limiter:  &fakes.Limiter{},
		settings: &stubSettings{s: st},
		invites:  &stubInvites{},
	}
	h.events = fakes.NewEvents(h.store)
	h.svc = New(Deps{
		Users:    h.store,
		Sessions: h.store,
		Tokens:   h.store,
		Tx:       h.store,
		Hasher:   fakes.Hasher{},
		TokenGen: &fakes.Tokens{},
		IDs:      &fakes.IDs{},
		Clock:    h.clock,
		Limiter:  h.limiter,
		Mail:     h.mailer,
		Settings: h.settings,
		Emails:   h.emails,
		Invites:  h.invites,
		Events:   h.events,
		Logger:   slog.New(slog.DiscardHandler),
	})
	return h
}

var client = domain.ClientInfo{IP: netip.MustParseAddr("203.0.113.7"), UserAgent: "test", Locale: "pt-BR"}

// registerAna registers ana@example.com with the password "correct horse".
func (h *harness) registerAna(t *testing.T) SignedIn {
	t.Helper()
	signed, err := h.svc.Register(t.Context(), client, RegisterInput{Username: "ana", Email: "ana@example.com", Password: "correct horse"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	return signed
}

func requireDomainErr(t *testing.T, err, kind error, reason string) {
	t.Helper()
	de, ok := domain.AsError(err)
	if !errors.Is(err, kind) || !ok || de.Reason != reason {
		t.Fatalf("err = %v, want %v with reason %q", err, kind, reason)
	}
}

func requireField(t *testing.T, err, kind error, field, reason string) {
	t.Helper()
	de, ok := domain.AsError(err)
	if !errors.Is(err, kind) || !ok {
		t.Fatalf("err = %v, want %v", err, kind)
	}
	for _, f := range de.Fields {
		if f.Field == field && f.Reason == reason {
			return
		}
	}
	t.Fatalf("err = %v, want field %s: %s", err, field, reason)
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("token")
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name   string
		mode   domain.RegistrationMode
		setup  bool
		in     RegisterInput
		denyIP bool
		check  func(t *testing.T, h *harness, signed SignedIn, err error)
	}{
		{
			name: "open registration signs the user in",
			mode: domain.RegistrationOpen, setup: true,
			in: RegisterInput{Username: " ana_b ", Email: " ana@example.com ", Password: "correct horse"},
			check: func(t *testing.T, h *harness, signed SignedIn, err error) {
				if err != nil {
					t.Fatal(err)
				}
				if signed.User.Username != "ana_b" || signed.User.Email != "ana@example.com" || signed.User.DisplayName != "ana_b" {
					t.Errorf("user = %+v", signed.User)
				}
				if signed.Token == "" || signed.Session.UserID != signed.User.ID {
					t.Errorf("no session for the new user: %+v", signed.Session)
				}
				if signed.User.EmailVerified() {
					t.Error("a new account must not start verified while email works")
				}
				if _, err := h.svc.Authenticate(t.Context(), signed.Token); err != nil {
					t.Errorf("session token does not authenticate: %v", err)
				}
			},
		},
		{
			name: "invite-only without a code",
			mode: domain.RegistrationInviteOnly, setup: true,
			in: RegisterInput{Username: "ana", Email: "ana@example.com", Password: "correct horse"},
			check: func(t *testing.T, _ *harness, _ SignedIn, err error) {
				requireField(t, err, domain.ErrInvalid, "invite_code", domain.ReasonFieldRequired)
			},
		},
		{
			name: "invite-only with a bad code",
			mode: domain.RegistrationInviteOnly, setup: true,
			in: RegisterInput{Username: "ana", Email: "ana@example.com", Password: "correct horse", InviteCode: "nope"},
			check: func(t *testing.T, h *harness, _ SignedIn, err error) {
				requireField(t, err, domain.ErrInvalid, "invite_code", domain.ReasonFieldInvalid)
				if n, _ := h.store.CountUsers(t.Context()); n != 0 {
					t.Errorf("user created despite the bad invite")
				}
			},
		},
		{
			name: "invite-only with a valid code claims it for the user",
			mode: domain.RegistrationInviteOnly, setup: true,
			in: RegisterInput{Username: "ana", Email: "ana@example.com", Password: "correct horse", InviteCode: "good-invite"},
			check: func(t *testing.T, h *harness, signed SignedIn, err error) {
				if err != nil {
					t.Fatal(err)
				}
				if len(h.invites.claimed) != 1 || h.invites.claimed[0] != signed.User.ID {
					t.Errorf("claimed = %v, want the new user %d", h.invites.claimed, signed.User.ID)
				}
			},
		},
		{
			name: "closed registration",
			mode: domain.RegistrationClosed, setup: true,
			in: RegisterInput{Username: "ana", Email: "ana@example.com", Password: "correct horse", InviteCode: "good-invite"},
			check: func(t *testing.T, _ *harness, _ SignedIn, err error) {
				requireDomainErr(t, err, domain.ErrForbidden, domain.ReasonRegistrationClosed)
			},
		},
		{
			name: "before setup",
			mode: domain.RegistrationOpen,
			in:   RegisterInput{Username: "ana", Email: "ana@example.com", Password: "correct horse"},
			check: func(t *testing.T, _ *harness, _ SignedIn, err error) {
				requireDomainErr(t, err, domain.ErrFailedPrecondition, domain.ReasonSetupRequired)
			},
		},
		{
			name: "invalid fields are reported together",
			mode: domain.RegistrationOpen, setup: true,
			in: RegisterInput{Username: "a!", Email: "not-an-email", Password: "password123"},
			check: func(t *testing.T, _ *harness, _ SignedIn, err error) {
				requireField(t, err, domain.ErrInvalid, "username", domain.ReasonFieldTooShort)
				requireField(t, err, domain.ErrInvalid, "email", domain.ReasonFieldInvalidFormat)
				requireField(t, err, domain.ErrInvalid, "password", domain.ReasonFieldTooCommon)
			},
		},
		{
			name: "rate limited by IP",
			mode: domain.RegistrationOpen, setup: true, denyIP: true,
			in: RegisterInput{Username: "ana", Email: "ana@example.com", Password: "correct horse"},
			check: func(t *testing.T, _ *harness, _ SignedIn, err error) {
				de, ok := domain.AsError(err)
				if !errors.Is(err, domain.ErrRateLimited) || !ok || de.RetryAfter != time.Minute {
					t.Fatalf("err = %v, want rate limited with retry after 1m", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.settings.s.RegistrationMode = tt.mode
			if !tt.setup {
				h.settings.s.SetupCompletedAt = nil
			}
			if tt.denyIP {
				h.limiter.Deny = map[string]time.Duration{"register:ip:203.0.113.7": time.Minute}
			}
			signed, err := h.svc.Register(t.Context(), client, tt.in)
			tt.check(t, h, signed, err)
		})
	}
}

func TestRegisterConflicts(t *testing.T) {
	h := newHarness(t)
	h.registerAna(t)

	_, err := h.svc.Register(t.Context(), client, RegisterInput{Username: "ANA", Email: "other@example.com", Password: "correct horse"})
	requireField(t, err, domain.ErrConflict, "username", domain.ReasonFieldTaken)
	_, err = h.svc.Register(t.Context(), client, RegisterInput{Username: "bia", Email: "ANA@example.com", Password: "correct horse"})
	requireField(t, err, domain.ErrConflict, "email", domain.ReasonFieldTaken)

	if n, _ := h.store.CountUsers(t.Context()); n != 1 {
		t.Errorf("users = %d, want 1", n)
	}
}

func TestRegisterSendsVerificationOnlyWhenMailWorks(t *testing.T) {
	h := newHarness(t)
	h.registerAna(t)
	email, ok := h.emails.Last("verify")
	if !ok {
		t.Fatal("no verification email")
	}
	if email.To != "ana@example.com" || email.Locale != "pt-BR" || !strings.HasPrefix(email.Link, "https://hub.example.com/verify-email?token=") {
		t.Errorf("email = %+v", email)
	}

	h = newHarness(t)
	h.mailer.Off = true
	h.registerAna(t)
	if len(h.emails.Sent) != 0 {
		t.Errorf("sent %v without email configured", h.emails.Sent)
	}
}

func TestRegisterSurvivesEmailFailure(t *testing.T) {
	h := newHarness(t)
	h.emails.Err = errors.New("smtp down")
	if _, err := h.svc.Register(t.Context(), client, RegisterInput{Username: "ana", Email: "ana@example.com", Password: "correct horse"}); err != nil {
		t.Fatalf("Register failed because the email did: %v", err)
	}
}

func TestLogin(t *testing.T) {
	h := newHarness(t)
	h.registerAna(t)
	ctx := t.Context()

	unknown := func() error { _, err := h.svc.Login(ctx, client, "nobody", "correct horse"); return err }()
	wrong := func() error { _, err := h.svc.Login(ctx, client, "ana", "wrong password"); return err }()
	if unknown == nil || wrong == nil || unknown.Error() != wrong.Error() {
		t.Fatalf("unknown user (%v) and wrong password (%v) must fail identically", unknown, wrong)
	}
	requireDomainErr(t, wrong, domain.ErrUnauthenticated, domain.ReasonInvalidCredentials)

	for _, login := range []string{"ana", "ANA", "ana@example.com", " Ana@Example.com "} {
		if _, err := h.svc.Login(ctx, client, login, "correct horse"); err != nil {
			t.Errorf("Login(%q): %v", login, err)
		}
	}

	user, _ := h.store.UserByLogin(ctx, "ana")
	now := h.clock.Now()
	user.DisabledAt = &now
	_ = h.store.UpdateUser(ctx, user)
	_, err := h.svc.Login(ctx, client, "ana", "correct horse")
	requireDomainErr(t, err, domain.ErrForbidden, domain.ReasonAccountDisabled)
	_, err = h.svc.Login(ctx, client, "ana", "wrong password")
	requireDomainErr(t, err, domain.ErrUnauthenticated, domain.ReasonInvalidCredentials)
}

func TestLoginRateLimits(t *testing.T) {
	for _, prefix := range []string{"login:ip:203.0.113.7", "login:account:ana"} {
		h := newHarness(t)
		h.registerAna(t)
		h.limiter.Deny = map[string]time.Duration{prefix: 30 * time.Second}
		_, err := h.svc.Login(t.Context(), client, "ANA", "correct horse")
		if !errors.Is(err, domain.ErrRateLimited) {
			t.Errorf("deny %s: err = %v, want rate limited", prefix, err)
		}
	}
}

func TestAuthenticate(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	signed := h.registerAna(t)

	p, err := h.svc.Authenticate(ctx, signed.Token)
	if err != nil {
		t.Fatal(err)
	}
	if p.Actor.UserID != signed.User.ID || p.Actor.SessionID != signed.Session.ID || p.Refreshed {
		t.Errorf("principal = %+v", p)
	}

	h.clock.Advance(sessionRefreshEvery)
	p, err = h.svc.Authenticate(ctx, signed.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Refreshed || !p.Session.ExpiresAt.Equal(h.clock.Now().Add(SessionLifetime)) {
		t.Errorf("session not extended: %+v", p.Session)
	}

	for _, raw := range []string{"", "token-unknown"} {
		_, err := h.svc.Authenticate(ctx, raw)
		requireDomainErr(t, err, domain.ErrUnauthenticated, domain.ReasonSessionExpired)
	}

	h.clock.Advance(SessionLifetime + time.Second)
	_, err = h.svc.Authenticate(ctx, signed.Token)
	requireDomainErr(t, err, domain.ErrUnauthenticated, domain.ReasonSessionExpired)
}

func TestAuthenticateRejectsRevokedAndDisabled(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	first := h.registerAna(t)
	second, err := h.svc.Login(ctx, client, "ana", "correct horse")
	if err != nil {
		t.Fatal(err)
	}

	p, _ := h.svc.Authenticate(ctx, first.Token)
	if err := h.svc.Logout(ctx, p.Actor); err != nil {
		t.Fatal(err)
	}
	_, err = h.svc.Authenticate(ctx, first.Token)
	requireDomainErr(t, err, domain.ErrUnauthenticated, domain.ReasonSessionExpired)
	if err := h.svc.Logout(ctx, p.Actor); err != nil {
		t.Errorf("logging out twice: %v", err)
	}

	user, _ := h.store.UserByID(ctx, first.User.ID)
	now := h.clock.Now()
	user.DisabledAt = &now
	_ = h.store.UpdateUser(ctx, user)
	_, err = h.svc.Authenticate(ctx, second.Token)
	requireDomainErr(t, err, domain.ErrUnauthenticated, domain.ReasonAccountDisabled)
}

func TestVerifyEmail(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	signed := h.registerAna(t)
	email, _ := h.emails.Last("verify")
	token := tokenFromLink(t, email.Link)

	if err := h.svc.VerifyEmail(ctx, client, token); err != nil {
		t.Fatal(err)
	}
	if u, _ := h.store.UserByID(ctx, signed.User.ID); !u.EmailVerified() {
		t.Error("email not marked verified")
	}
	err := h.svc.VerifyEmail(ctx, client, token)
	requireDomainErr(t, err, domain.ErrNotFound, domain.ReasonTokenInvalid)
}

func TestVerifyEmailRejectsExpiredAndStaleTokens(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	signed := h.registerAna(t)
	email, _ := h.emails.Last("verify")

	h.clock.Advance(verifyTokenTTL + time.Second)
	err := h.svc.VerifyEmail(ctx, client, tokenFromLink(t, email.Link))
	requireDomainErr(t, err, domain.ErrNotFound, domain.ReasonTokenInvalid)

	p, _ := h.svc.Login(ctx, client, "ana", "correct horse")
	if err := h.svc.ResendVerification(ctx, client, ActorFor(p.User, p.Session)); err != nil {
		t.Fatal(err)
	}
	fresh, _ := h.emails.Last("verify")
	user, _ := h.store.UserByID(ctx, signed.User.ID)
	user.Email = "new@example.com"
	_ = h.store.UpdateUser(ctx, user)
	err = h.svc.VerifyEmail(ctx, client, tokenFromLink(t, fresh.Link))
	requireDomainErr(t, err, domain.ErrNotFound, domain.ReasonTokenInvalid)
}

func TestResendVerification(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	signed := h.registerAna(t)
	actor := ActorFor(signed.User, signed.Session)
	first, _ := h.emails.Last("verify")

	if err := h.svc.ResendVerification(ctx, client, actor); err != nil {
		t.Fatal(err)
	}
	second, _ := h.emails.Last("verify")
	if second.Link == first.Link {
		t.Fatal("resend reused the old link")
	}
	err := h.svc.VerifyEmail(ctx, client, tokenFromLink(t, first.Link))
	requireDomainErr(t, err, domain.ErrNotFound, domain.ReasonTokenInvalid)
	if err := h.svc.VerifyEmail(ctx, client, tokenFromLink(t, second.Link)); err != nil {
		t.Fatal(err)
	}

	err = h.svc.ResendVerification(ctx, client, actor)
	requireDomainErr(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailAlreadyVerified)

	h.mailer.Off = true
	err = h.svc.ResendVerification(ctx, client, actor)
	requireDomainErr(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailUnavailable)
}

func TestPasswordReset(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	signed := h.registerAna(t)
	other, _ := h.svc.Login(ctx, client, "ana", "correct horse")

	if err := h.svc.RequestPasswordReset(ctx, client, "nobody@example.com"); err != nil {
		t.Errorf("unknown email must look like success: %v", err)
	}
	if _, ok := h.emails.Last("reset"); ok {
		t.Fatal("sent a reset email for an unknown address")
	}

	if err := h.svc.RequestPasswordReset(ctx, client, "ANA@example.com"); err != nil {
		t.Fatal(err)
	}
	email, ok := h.emails.Last("reset")
	if !ok || email.To != "ana@example.com" || !strings.HasPrefix(email.Link, "https://hub.example.com/reset-password?token=") {
		t.Fatalf("reset email = %+v", email)
	}
	token := tokenFromLink(t, email.Link)

	err := h.svc.ResetPassword(ctx, client, token, "short")
	requireField(t, err, domain.ErrInvalid, "new_password", domain.ReasonFieldTooShort)

	if err := h.svc.ResetPassword(ctx, client, token, "brand new secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Login(ctx, client, "ana", "brand new secret"); err != nil {
		t.Errorf("new password does not work: %v", err)
	}
	for _, s := range []SignedIn{signed, other} {
		if _, err := h.svc.Authenticate(ctx, s.Token); err == nil {
			t.Error("a session survived the password reset")
		}
	}
	err = h.svc.ResetPassword(ctx, client, token, "another secret")
	requireDomainErr(t, err, domain.ErrNotFound, domain.ReasonTokenInvalid)
}

func TestPasswordResetTokenExpires(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	h.registerAna(t)
	if err := h.svc.RequestPasswordReset(ctx, client, "ana@example.com"); err != nil {
		t.Fatal(err)
	}
	email, _ := h.emails.Last("reset")
	h.clock.Advance(resetTokenTTL + time.Second)
	err := h.svc.ResetPassword(ctx, client, tokenFromLink(t, email.Link), "brand new secret")
	requireDomainErr(t, err, domain.ErrNotFound, domain.ReasonTokenInvalid)
}

func TestRequestPasswordResetWithoutEmail(t *testing.T) {
	h := newHarness(t)
	h.mailer.Off = true
	err := h.svc.RequestPasswordReset(t.Context(), client, "ana@example.com")
	requireDomainErr(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailUnavailable)

	err = h.svc.RequestPasswordReset(t.Context(), client, "not an email")
	requireField(t, err, domain.ErrInvalid, "email", domain.ReasonFieldInvalidFormat)
}

func TestIssuePasswordResetLink(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	h.mailer.Off = true
	signed := h.registerAna(t)

	link, expires, err := h.svc.IssuePasswordResetLink(ctx, signed.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, "https://hub.example.com/reset-password?token=") || !expires.Equal(h.clock.Now().Add(adminResetLinkTTL)) {
		t.Errorf("link = %s, expires = %v", link, expires)
	}
	if err := h.svc.ResetPassword(ctx, client, tokenFromLink(t, link), "brand new secret"); err != nil {
		t.Errorf("admin link does not reset: %v", err)
	}

	_, _, err = h.svc.IssuePasswordResetLink(ctx, 999)
	requireDomainErr(t, err, domain.ErrNotFound, domain.ReasonUserNotFound)
}

func TestSetupState(t *testing.T) {
	h := newHarness(t)
	local := domain.ClientInfo{IP: netip.MustParseAddr("::1")}

	st, err := h.svc.SetupState(t.Context(), local)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Complete || st.SetupAvailableHere || st.InstanceName != "Team" || !st.EmailVerification {
		t.Errorf("after setup: %+v", st)
	}

	h.settings.s.SetupCompletedAt = nil
	if st, _ := h.svc.SetupState(t.Context(), local); !st.SetupAvailableHere {
		t.Error("pending setup should be available from the host")
	}
	if st, _ := h.svc.SetupState(t.Context(), client); st.SetupAvailableHere {
		t.Error("pending setup must not be offered to remote clients")
	}
}
