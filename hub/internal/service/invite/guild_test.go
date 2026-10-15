package invite

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

type guildSettings struct{ st domain.InstanceSettings }

func (s *guildSettings) Get(context.Context) (domain.InstanceSettings, error) { return s.st, nil }

const guildID domain.ID = 100

type guildHarness struct {
	svc      *Service
	store    *fakes.Store
	clock    *fakes.Clock
	events   *fakes.Events
	codes    *fakes.Codes
	limiter  *fakes.Limiter
	presence *fakes.Presence
	settings *guildSettings
}

var (
	owner    = domain.Actor{UserID: 1, EmailVerified: true}
	member   = domain.Actor{UserID: 2, EmailVerified: true}
	outsider = domain.Actor{UserID: 3, EmailVerified: true}
	client   = domain.ClientInfo{IP: netip.MustParseAddr("203.0.113.9")}
)

// Users 1 (owner) and 2 (member) are in guild 100; user 3 is not.
func newGuildHarness(t *testing.T) *guildHarness {
	t.Helper()
	ctx := t.Context()
	st := domain.DefaultInstanceSettings()
	st.PublicURL = "https://hub.example.com"
	h := &guildHarness{
		store: fakes.NewStore(), clock: fakes.NewClock(), codes: &fakes.Codes{}, limiter: &fakes.Limiter{},
		presence: &fakes.Presence{}, settings: &guildSettings{st: st},
	}
	h.events = fakes.NewEvents(h.store)
	h.svc = New(Deps{
		Tokens: h.store, Users: h.store, Guilds: h.store, Members: h.store, Channels: h.store,
		Invites: h.store, Bans: h.store, Audit: h.store, Tx: h.store, TokenGen: &fakes.Tokens{}, Codes: h.codes,
		IDs: &fakes.IDs{}, Clock: h.clock, Limiter: h.limiter, Events: h.events, Settings: h.settings,
		Presence: h.presence, Verification: fakes.Verification(true),
	})
	for _, id := range []domain.ID{1, 2, 3} {
		if err := h.store.CreateUser(ctx, domain.User{ID: id, Username: "u" + id.String(), Email: id.String() + "@x.io"}); err != nil {
			t.Fatal(err)
		}
	}
	_ = h.store.CreateGuild(ctx, domain.Guild{ID: guildID, Name: "Team", OwnerID: 1})
	_ = h.store.AddMember(ctx, domain.Member{GuildID: guildID, UserID: 1, Role: domain.RoleOwner})
	_ = h.store.AddMember(ctx, domain.Member{GuildID: guildID, UserID: 2, Role: domain.RoleMember})
	return h
}

func requireReason(t *testing.T, err, kind error, reason string) {
	t.Helper()
	de, ok := domain.AsError(err)
	if !errors.Is(err, kind) || !ok || (de.Reason != reason && (len(de.Fields) == 0 || de.Fields[0].Reason != reason)) {
		t.Fatalf("err = %v, want %v (%s)", err, kind, reason)
	}
}

func TestCreateGuildInvite(t *testing.T) {
	h := newGuildHarness(t)
	ctx := t.Context()

	v, err := h.svc.CreateGuildInvite(ctx, owner, guildID, time.Hour, 5)
	if err != nil {
		t.Fatal(err)
	}
	if v.Invite.Code != "TESTAAAB" || v.URL != "https://hub.example.com/invite/TESTAAAB" || v.Invite.MaxUses != 5 {
		t.Errorf("invite = %+v", v)
	}
	if !v.Invite.ExpiresAt.Equal(h.clock.Now().Add(time.Hour)) || v.Creator == nil || v.Creator.ID != 1 {
		t.Errorf("expiry/creator = %v / %+v", v.Invite.ExpiresAt, v.Creator)
	}
	evs := h.events.Take()
	if ic, ok := evs[len(evs)-1].Payload.(events.InviteCreated); !ok || ic.Invite.Invite.Code != "TESTAAAB" ||
		!cmp.Equal(evs[len(evs)-1].Topics, []events.Topic{events.GuildAdminsTopic(guildID), "user:1"}) {
		t.Errorf("events = %+v", evs)
	}
	if a := h.store.Audit(guildID); len(a) != 1 || a[0].Action != domain.AuditInviteCreate || a[0].TargetID != "TESTAAAB" {
		t.Errorf("audit = %+v", a)
	}

	// A taken code is retried with a fresh one.
	h.codes.Repeat = 1
	again, err := h.svc.CreateGuildInvite(ctx, member, guildID, 0, 0)
	if err != nil || again.Invite.Code != "TESTAAAC" || again.Invite.ExpiresAt != nil {
		t.Errorf("retry: %+v, %v", again.Invite, err)
	}

	h.settings.st.MembersCanInvite = false
	_, err = h.svc.CreateGuildInvite(ctx, member, guildID, 0, 0)
	requireReason(t, err, domain.ErrForbidden, domain.ReasonInvitesDisabled)
	_, err = h.svc.CreateGuildInvite(ctx, outsider, guildID, 0, 0)
	requireReason(t, err, domain.ErrNotFound, domain.ReasonGuildNotFound)
	_, err = h.svc.CreateGuildInvite(ctx, owner, guildID, 2*time.Hour, 0)
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalid)
}

func TestListAndRevokeGuildInvites(t *testing.T) {
	h := newGuildHarness(t)
	ctx := t.Context()
	own, _ := h.svc.CreateGuildInvite(ctx, owner, guildID, 0, 0)
	mine, _ := h.svc.CreateGuildInvite(ctx, member, guildID, 0, 0)

	all, _ := h.svc.ListGuildInvites(ctx, owner, guildID)
	if len(all) != 2 {
		t.Errorf("owner sees %d invites", len(all))
	}
	only, _ := h.svc.ListGuildInvites(ctx, member, guildID)
	if len(only) != 1 || only[0].Invite.Code != mine.Invite.Code {
		t.Errorf("member sees %+v", only)
	}

	requireReason(t, h.svc.RevokeGuildInvite(ctx, member, own.Invite.Code), domain.ErrForbidden, domain.ReasonMissingPermission)
	requireReason(t, h.svc.RevokeGuildInvite(ctx, outsider, own.Invite.Code), domain.ErrNotFound, domain.ReasonInviteNotFound)
	h.events.Take()
	if err := h.svc.RevokeGuildInvite(ctx, member, mine.Invite.Code); err != nil {
		t.Fatal(err)
	}
	evs := h.events.Take()
	if d, ok := evs[len(evs)-1].Payload.(events.InviteDeleted); !ok || d.Code != mine.Invite.Code {
		t.Errorf("events = %+v", evs)
	}
	if err := h.svc.RevokeGuildInvite(ctx, owner, "testaaab"); err != nil {
		t.Errorf("revoke by lowercase code: %v", err)
	}
	requireReason(t, h.svc.RevokeGuildInvite(ctx, owner, own.Invite.Code), domain.ErrNotFound, domain.ReasonInviteNotFound)
	if left, _ := h.svc.ListGuildInvites(ctx, owner, guildID); len(left) != 0 {
		t.Errorf("revoked invites listed: %+v", left)
	}
}

func TestPreviewGuildInvite(t *testing.T) {
	h := newGuildHarness(t)
	ctx := t.Context()
	v, _ := h.svc.CreateGuildInvite(ctx, owner, guildID, 0, 0)
	h.presence.Status = map[domain.ID]domain.PresenceStatus{1: domain.PresenceOnline}

	p, err := h.svc.PreviewGuildInvite(ctx, client, nil, "https://hub.example.com/invite/"+v.Invite.Code)
	if err != nil {
		t.Fatal(err)
	}
	if p.Guild.Name != "Team" || p.MemberCount != 2 || p.OnlineCount != 1 || p.Inviter.ID != 1 || p.Member {
		t.Errorf("anonymous preview = %+v", p)
	}
	if p, _ := h.svc.PreviewGuildInvite(ctx, client, &member, v.Invite.Code); !p.Member {
		t.Error("member flag not set")
	}

	_ = h.store.CreateBan(ctx, domain.Ban{GuildID: guildID, UserID: 3})
	_, err = h.svc.PreviewGuildInvite(ctx, client, &outsider, v.Invite.Code)
	requireReason(t, err, domain.ErrForbidden, domain.ReasonBanned)

	for _, code := range []string{"NOPE2345", "not-a-code", ""} {
		_, err := h.svc.PreviewGuildInvite(ctx, client, nil, code)
		requireReason(t, err, domain.ErrNotFound, domain.ReasonInviteNotFound)
	}
	h.limiter.Deny = map[string]time.Duration{"invite:preview:ip:": time.Second}
	_, err = h.svc.PreviewGuildInvite(ctx, client, nil, v.Invite.Code)
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("rate limit: %v", err)
	}
}

func TestAcceptGuildInvite(t *testing.T) {
	h := newGuildHarness(t)
	ctx := t.Context()
	v, _ := h.svc.CreateGuildInvite(ctx, owner, guildID, time.Hour, 1)
	h.events.Take()

	state, err := h.svc.AcceptGuildInvite(ctx, outsider, v.Invite.Code)
	if err != nil {
		t.Fatal(err)
	}
	if state.Guild.ID != guildID || len(state.Members) != 3 {
		t.Errorf("state = %+v", state)
	}
	evs := h.events.Take()
	if len(evs) != 2 {
		t.Fatalf("events = %+v", evs)
	}
	if gc, ok := evs[0].Payload.(events.GuildCreated); !ok || gc.UserID != 3 || !cmp.Equal(evs[0].Topics, []events.Topic{"user:3"}) {
		t.Errorf("first event = %+v", evs[0])
	}
	if ma, ok := evs[1].Payload.(events.MemberAdded); !ok || ma.Member.UserID != 3 || ma.User.ID != 3 ||
		!cmp.Equal(evs[1].Topics, []events.Topic{events.GuildTopic(guildID)}) {
		t.Errorf("second event = %+v", evs[1])
	}

	// The single use is spent, so the invite now refuses everyone.
	_, err = h.svc.AcceptGuildInvite(ctx, member, v.Invite.Code)
	requireReason(t, err, domain.ErrNotFound, domain.ReasonInviteNotFound)
}

func TestAcceptGuildInviteRules(t *testing.T) {
	h := newGuildHarness(t)
	ctx := t.Context()
	v, _ := h.svc.CreateGuildInvite(ctx, owner, guildID, time.Hour, 0)

	if _, err := h.svc.AcceptGuildInvite(ctx, member, v.Invite.Code); err != nil {
		t.Errorf("member accepting again: %v", err)
	}
	if inv, _ := h.store.InviteByCode(ctx, v.Invite.Code); inv.Uses != 0 {
		t.Errorf("uses = %d after a member re-accepted", inv.Uses)
	}

	_ = h.store.CreateBan(ctx, domain.Ban{GuildID: guildID, UserID: 3})
	_, err := h.svc.AcceptGuildInvite(ctx, outsider, v.Invite.Code)
	requireReason(t, err, domain.ErrForbidden, domain.ReasonBanned)
	_ = h.store.DeleteBan(ctx, guildID, 3)

	h.settings.st.Limits.MaxGuildMembers = 2
	_, err = h.svc.AcceptGuildInvite(ctx, outsider, v.Invite.Code)
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonGuildFull)
	h.settings.st.Limits.MaxGuildMembers = 200

	h.clock.Advance(2 * time.Hour)
	_, err = h.svc.AcceptGuildInvite(ctx, outsider, v.Invite.Code)
	requireReason(t, err, domain.ErrNotFound, domain.ReasonInviteNotFound)

	_, err = h.svc.AcceptGuildInvite(ctx, domain.Actor{UserID: 3}, v.Invite.Code)
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailNotVerified)
}

func TestRegisterWithGuildInvite(t *testing.T) {
	h := newGuildHarness(t)
	ctx := t.Context()
	v, _ := h.svc.CreateGuildInvite(ctx, owner, guildID, 0, 1)
	h.events.Take()

	_, err := h.svc.ClaimForRegistration(ctx, "UNKNOWN2")
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalid)

	var apply func(context.Context, domain.ID) error
	err = h.store.InTx(ctx, func(ctx context.Context) error {
		var err error
		if apply, err = h.svc.ClaimForRegistration(ctx, " "+v.Invite.Code+" "); err != nil {
			return err
		}
		if err := h.store.CreateUser(ctx, domain.User{ID: 9, Username: "newbie", Email: "n@x.io"}); err != nil {
			return err
		}
		return apply(ctx, 9)
	})
	if err != nil {
		t.Fatal(err)
	}
	if m, err := h.store.MemberByID(ctx, guildID, 9); err != nil || m.Role != domain.RoleMember {
		t.Errorf("newcomer membership: %+v, %v", m, err)
	}
	evs := h.events.Take()
	if len(evs) != 1 {
		t.Fatalf("events = %+v", evs)
	}
	if ma, ok := evs[0].Payload.(events.MemberAdded); !ok || ma.User.Username != "newbie" {
		t.Errorf("event = %+v", evs[0])
	}
	_, err = h.svc.ClaimForRegistration(ctx, v.Invite.Code)
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalid)
}
