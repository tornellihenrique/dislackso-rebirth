package invite

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

type stubSettings struct{}

func (stubSettings) Get(context.Context) (domain.InstanceSettings, error) {
	return domain.InstanceSettings{PublicURL: "https://hub.example.com"}, nil
}

var admin = domain.Actor{UserID: 1, InstanceAdmin: true}

func newService(t *testing.T) (*Service, *fakes.Clock) {
	t.Helper()
	store, clock := fakes.NewStore(), fakes.NewClock()
	if err := store.CreateUser(t.Context(), domain.User{ID: 1, Username: "admin", Email: "admin@example.com"}); err != nil {
		t.Fatal(err)
	}
	return New(Deps{Tokens: store, Users: store, TokenGen: &fakes.Tokens{}, IDs: &fakes.IDs{}, Clock: clock, Settings: stubSettings{}}), clock
}

func TestInstanceInviteLifecycle(t *testing.T) {
	svc, clock := newService(t)
	ctx := t.Context()

	created, err := svc.CreateInstanceInvite(ctx, admin, 0)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(created.URL)
	if err != nil || u.Host != "hub.example.com" || u.Path != "/register" || u.Query().Get("invite") != created.Code {
		t.Errorf("URL = %s, code = %s", created.URL, created.Code)
	}
	if !created.Invite.Token.ExpiresAt.Equal(clock.Now().Add(DefaultInstanceInviteTTL)) {
		t.Errorf("expires = %v", created.Invite.Token.ExpiresAt)
	}
	if created.Invite.CreatedBy == nil || created.Invite.CreatedBy.Username != "admin" {
		t.Errorf("CreatedBy = %+v", created.Invite.CreatedBy)
	}

	list, err := svc.ListInstanceInvites(ctx, admin)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, %v", list, err)
	}

	apply, err := svc.ClaimForRegistration(ctx, created.Code)
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(ctx, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ClaimForRegistration(ctx, created.Code); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("second claim err = %v, want invalid (single use)", err)
	}
}

func TestClaimRejectsExpiredAndRevoked(t *testing.T) {
	svc, clock := newService(t)
	ctx := t.Context()

	expiring, _ := svc.CreateInstanceInvite(ctx, admin, time.Hour)
	revoked, _ := svc.CreateInstanceInvite(ctx, admin, time.Hour)
	if err := svc.RevokeInstanceInvite(ctx, admin, revoked.Invite.Token.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeInstanceInvite(ctx, admin, revoked.Invite.Token.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("revoking twice: %v", err)
	}
	clock.Advance(time.Hour + time.Second)

	for _, code := range []string{expiring.Code, revoked.Code, "", "made-up"} {
		if _, err := svc.ClaimForRegistration(ctx, code); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("claim %q err = %v, want invalid", code, err)
		}
	}
}

func TestInstanceInvitesRequireAdmin(t *testing.T) {
	svc, _ := newService(t)
	ctx := t.Context()
	member := domain.Actor{UserID: 2}

	if _, err := svc.CreateInstanceInvite(ctx, member, 0); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("create: %v", err)
	}
	if _, err := svc.ListInstanceInvites(ctx, member); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("list: %v", err)
	}
	if err := svc.RevokeInstanceInvite(ctx, member, 1); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("revoke: %v", err)
	}
	for _, ttl := range []time.Duration{time.Second, MaxInstanceInviteTTL + time.Hour} {
		if _, err := svc.CreateInstanceInvite(ctx, admin, ttl); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("ttl %v: err = %v, want invalid", ttl, err)
		}
	}
}
