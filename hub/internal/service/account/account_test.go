package account

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

type verification bool

func (v verification) VerificationRequired(context.Context) bool { return bool(v) }

type statusCall struct {
	userID domain.ID
	pref   domain.StatusPref
	custom string
}

type statusSink struct{ calls []statusCall }

func (s *statusSink) SetPreference(_ context.Context, userID domain.ID, pref domain.StatusPref, custom string) {
	s.calls = append(s.calls, statusCall{userID, pref, custom})
}

type harness struct {
	svc      *Service
	store    *fakes.Store
	clock    *fakes.Clock
	limiter  *fakes.Limiter
	events   *fakes.Events
	presence *statusSink
	actor    domain.Actor
}

func newHarness(t *testing.T, required, verified bool) *harness {
	t.Helper()
	h := &harness{store: fakes.NewStore(), clock: fakes.NewClock(), limiter: &fakes.Limiter{}, presence: &statusSink{}}
	h.events = fakes.NewEvents(h.store)
	h.svc = New(Deps{
		Users: h.store, Sessions: h.store, Members: h.store, Files: h.store, Tx: h.store, Hasher: fakes.Hasher{},
		Clock: h.clock, Limiter: h.limiter, Events: h.events, Presence: h.presence, Verification: verification(required),
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
	u, err := h.svc.UpdateProfile(t.Context(), h.actor, ProfilePatch{DisplayName: new("  Ana Beatriz  ")})
	if err != nil {
		t.Fatal(err)
	}
	if u.DisplayName != "Ana Beatriz" {
		t.Errorf("DisplayName = %q", u.DisplayName)
	}
	for _, name := range []string{"", "   ", "x\x00y", "123456789012345678901234567890123"} {
		_, err := h.svc.UpdateProfile(t.Context(), h.actor, ProfilePatch{DisplayName: new(name)})
		if !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("UpdateProfile(%q) err = %v, want invalid", name, err)
		}
	}
}

func TestUnverifiedAccountsAreGated(t *testing.T) {
	h := newHarness(t, true, false)
	ctx := t.Context()
	_, err := h.svc.UpdateProfile(ctx, h.actor, ProfilePatch{DisplayName: new("Ana")})
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailNotVerified)
	_, err = h.svc.SetStatus(ctx, h.actor, domain.StatusDND, "")
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailNotVerified)
	err = h.svc.ChangePassword(ctx, h.actor, "correct horse", "brand new secret")
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailNotVerified)
	if _, err := h.svc.Sessions(ctx, h.actor); err != nil {
		t.Errorf("unverified users must still manage their sessions: %v", err)
	}

	open := newHarness(t, false, false)
	if _, err := open.svc.UpdateProfile(ctx, open.actor, ProfilePatch{DisplayName: new("Ana")}); err != nil {
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

func TestUpdateProfileAvatarAndEvents(t *testing.T) {
	h := newHarness(t, true, true)
	ctx := t.Context()
	now := h.clock.Now()
	for _, f := range []domain.File{
		{ID: 50, OwnerID: 1, Purpose: domain.FileAvatar, CreatedAt: now},
		{ID: 51, OwnerID: 2, Purpose: domain.FileAvatar, CreatedAt: now},
		{ID: 52, OwnerID: 1, Purpose: domain.FileGuildIcon, CreatedAt: now},
	} {
		if err := h.store.CreateFile(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	for _, g := range []domain.ID{7, 8} {
		if err := h.store.AddMember(ctx, domain.Member{GuildID: g, UserID: 1, Role: domain.RoleMember, JoinedAt: now}); err != nil {
			t.Fatal(err)
		}
	}

	u, err := h.svc.UpdateProfile(ctx, h.actor, ProfilePatch{AvatarFileID: new(domain.ID(50))})
	if err != nil || u.AvatarFileID != 50 || u.DisplayName != "ana" {
		t.Fatalf("set avatar: %+v, %v", u, err)
	}
	evs := h.events.Take()
	if len(evs) != 2 {
		t.Fatalf("events = %+v", evs)
	}
	if me, ok := evs[0].Payload.(events.MeUpdated); !ok || me.User.AvatarFileID != 50 || !cmp.Equal(evs[0].Topics, []events.Topic{"user:1"}) {
		t.Errorf("first event: %+v", evs[0])
	}
	if _, ok := evs[1].Payload.(events.UserUpdated); !ok || !cmp.Equal(evs[1].Topics, []events.Topic{"user:1", "guild:7", "guild:8"}) {
		t.Errorf("second event: %+v", evs[1])
	}

	for _, id := range []domain.ID{51, 52, 99} {
		_, err := h.svc.UpdateProfile(ctx, h.actor, ProfilePatch{AvatarFileID: new(id)})
		requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalid)
	}
	if u, err := h.svc.UpdateProfile(ctx, h.actor, ProfilePatch{AvatarFileID: new(domain.ID(0))}); err != nil || u.AvatarFileID != 0 {
		t.Errorf("remove avatar: %+v, %v", u, err)
	}
}

func TestSetStatusUpdatesPresence(t *testing.T) {
	h := newHarness(t, true, true)
	if _, err := h.svc.SetStatus(t.Context(), h.actor, domain.StatusDND, " focusing "); err != nil {
		t.Fatal(err)
	}
	want := []statusCall{{1, domain.StatusDND, "focusing"}}
	if !cmp.Equal(h.presence.calls, want, cmp.AllowUnexported(statusCall{})) {
		t.Errorf("presence calls = %+v", h.presence.calls)
	}
	evs := h.events.Take()
	if len(evs) != 1 {
		t.Fatalf("status changes stay private: %+v", evs)
	}
	if _, ok := evs[0].Payload.(events.MeUpdated); !ok {
		t.Errorf("event: %+v", evs[0])
	}
	if _, err := h.svc.SetStatus(t.Context(), h.actor, "busy", ""); err == nil || len(h.presence.calls) != 1 {
		t.Errorf("an invalid status reached presence: %v", err)
	}
}
