package account

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

type verification bool

func (v verification) VerificationRequired(context.Context) bool { return bool(v) }

type harness struct {
	svc     *Service
	store   *fakes.Store
	clock   *fakes.Clock
	limiter *fakes.Limiter
	actor   domain.Actor
}

func newHarness(t *testing.T, required, verified bool) *harness {
	t.Helper()
	h := &harness{store: fakes.NewStore(), clock: fakes.NewClock(), limiter: &fakes.Limiter{}}
	h.svc = New(Deps{
		Users: h.store, Sessions: h.store, Tx: h.store, Hasher: fakes.Hasher{},
		Clock: h.clock, Limiter: h.limiter, Verification: verification(required),
	})
	now := h.clock.Now()
	user := domain.User{
		ID: 1, Username: "ana", Email: "ana@example.com", PasswordHash: "hashed:correct horse",
		DisplayName: "ana", StatusPref: domain.StatusOnline, CreatedAt: now, UpdatedAt: now,
	}
	if verified {
		user.EmailVerifiedAt = &now
	}
	if err := h.store.CreateUser(t.Context(), user); err != nil {
		t.Fatal(err)
	}
	for _, id := range []domain.ID{10, 11, 12} {
		s := domain.Session{ID: id, UserID: 1, TokenHash: []byte{byte(id)}, CreatedAt: now, LastSeenAt: now.Add(time.Duration(id) * time.Second), ExpiresAt: now.Add(time.Hour)}
		if err := h.store.CreateSession(t.Context(), s); err != nil {
			t.Fatal(err)
		}
	}
	h.actor = domain.Actor{UserID: 1, SessionID: 10, EmailVerified: verified}
	return h
}

func requireReason(t *testing.T, err, kind error, reason string) {
	t.Helper()
	de, ok := domain.AsError(err)
	if !errors.Is(err, kind) || !ok || (de.Reason != reason && (len(de.Fields) == 0 || de.Fields[0].Reason != reason)) {
		t.Fatalf("err = %v, want %v (%s)", err, kind, reason)
	}
}

func TestUpdateProfile(t *testing.T) {
	h := newHarness(t, true, true)
	u, err := h.svc.UpdateProfile(t.Context(), h.actor, "  Ana Beatriz  ")
	if err != nil {
		t.Fatal(err)
	}
	if u.DisplayName != "Ana Beatriz" {
		t.Errorf("DisplayName = %q", u.DisplayName)
	}
	for _, name := range []string{"", "   ", "x\x00y", "123456789012345678901234567890123"} {
		_, err := h.svc.UpdateProfile(t.Context(), h.actor, name)
		if !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("UpdateProfile(%q) err = %v, want invalid", name, err)
		}
	}
}

func TestUnverifiedAccountsAreGated(t *testing.T) {
	h := newHarness(t, true, false)
	ctx := t.Context()
	_, err := h.svc.UpdateProfile(ctx, h.actor, "Ana")
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailNotVerified)
	_, err = h.svc.SetStatus(ctx, h.actor, domain.StatusDND, "")
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailNotVerified)
	err = h.svc.ChangePassword(ctx, h.actor, "correct horse", "brand new secret")
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailNotVerified)
	if _, err := h.svc.Sessions(ctx, h.actor); err != nil {
		t.Errorf("unverified users must still manage their sessions: %v", err)
	}

	open := newHarness(t, false, false)
	if _, err := open.svc.UpdateProfile(ctx, open.actor, "Ana"); err != nil {
		t.Errorf("without email, unverified accounts work normally: %v", err)
	}
}

func TestSetStatus(t *testing.T) {
	h := newHarness(t, true, true)
	u, err := h.svc.SetStatus(t.Context(), h.actor, domain.StatusDND, "  deep work ")
	if err != nil {
		t.Fatal(err)
	}
	if u.StatusPref != domain.StatusDND || u.CustomStatus != "deep work" {
		t.Errorf("user = %+v", u)
	}
	_, err = h.svc.SetStatus(t.Context(), h.actor, "busy", "")
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalidFormat)
}

func TestChangePassword(t *testing.T) {
	h := newHarness(t, true, true)
	ctx := t.Context()

	err := h.svc.ChangePassword(ctx, h.actor, "wrong", "brand new secret")
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldIncorrect)
	err = h.svc.ChangePassword(ctx, h.actor, "correct horse", "password1")
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldTooCommon)

	if err := h.svc.ChangePassword(ctx, h.actor, "correct horse", "brand new secret"); err != nil {
		t.Fatal(err)
	}
	u, _ := h.store.UserByID(ctx, 1)
	if u.PasswordHash != "hashed:brand new secret" {
		t.Errorf("hash = %q", u.PasswordHash)
	}
	sessions, _ := h.svc.Sessions(ctx, h.actor)
	if len(sessions) != 1 || sessions[0].ID != h.actor.SessionID {
		t.Errorf("sessions after change = %v, want only the current one", sessions)
	}

	h.limiter.Deny = map[string]time.Duration{"password:user:1": time.Minute}
	if err := h.svc.ChangePassword(ctx, h.actor, "brand new secret", "another secret"); !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("err = %v, want rate limited", err)
	}
}

func TestSessions(t *testing.T) {
	h := newHarness(t, true, true)
	ctx := t.Context()

	sessions, err := h.svc.Sessions(ctx, h.actor)
	if err != nil || len(sessions) != 3 {
		t.Fatalf("Sessions = %v, %v", sessions, err)
	}
	if err := h.svc.RevokeSession(ctx, h.actor, 11); err != nil {
		t.Fatal(err)
	}
	err = h.svc.RevokeSession(ctx, h.actor, 11)
	requireReason(t, err, domain.ErrNotFound, domain.ReasonSessionNotFound)
	err = h.svc.RevokeSession(ctx, domain.Actor{UserID: 2}, 12)
	requireReason(t, err, domain.ErrNotFound, domain.ReasonSessionNotFound)

	n, err := h.svc.RevokeOtherSessions(ctx, h.actor)
	if err != nil || n != 1 {
		t.Errorf("RevokeOtherSessions = %d, %v; want 1", n, err)
	}
	if sessions, _ := h.svc.Sessions(ctx, h.actor); len(sessions) != 1 {
		t.Errorf("left %v", sessions)
	}
}
