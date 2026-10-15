package guild

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

type settings struct{ st domain.InstanceSettings }

func (s *settings) Get(context.Context) (domain.InstanceSettings, error) { return s.st, nil }

type harness struct {
	svc      *Service
	store    *fakes.Store
	events   *fakes.Events
	settings *settings
	owner    domain.Actor
	admin    domain.Actor
	member   domain.Actor
	outsider domain.Actor
}

func actor(id domain.ID) domain.Actor { return domain.Actor{UserID: id, EmailVerified: true} }

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{store: fakes.NewStore(), settings: &settings{st: domain.DefaultInstanceSettings()}}
	h.events = fakes.NewEvents(h.store)
	clock := fakes.NewClock()
	h.svc = New(Deps{
		Guilds: h.store, Members: h.store, Channels: h.store, Users: h.store, Files: h.store, Audit: h.store,
		Tx: h.store, IDs: &fakes.IDs{}, Clock: clock, Events: h.events, Settings: h.settings,
		Presence: &fakes.Presence{}, Verification: fakes.Verification(true),
	})
	for _, id := range []domain.ID{1, 2, 3, 4} {
		u := domain.User{ID: id, Username: "user" + id.String(), Email: id.String() + "@x.io", CreatedAt: clock.Now()}
		if err := h.store.CreateUser(t.Context(), u); err != nil {
			t.Fatal(err)
		}
	}
	h.owner, h.admin, h.member, h.outsider = actor(1), actor(2), actor(3), actor(4)
	return h
}

// newGuild creates a guild owned by user 1 with user 2 as admin and user 3 as member.
func (h *harness) newGuild(t *testing.T) domain.GuildState {
	t.Helper()
	ctx := t.Context()
	state, err := h.svc.Create(ctx, h.owner, "en", "Team", 0)
	if err != nil {
		t.Fatal(err)
	}
	for id, role := range map[domain.ID]domain.GuildRole{2: domain.RoleAdmin, 3: domain.RoleMember} {
		if err := h.store.AddMember(ctx, domain.Member{GuildID: state.Guild.ID, UserID: id, Role: role}); err != nil {
			t.Fatal(err)
		}
	}
	h.events.Take()
	return state
}

func requireReason(t *testing.T, err, kind error, reason string) {
	t.Helper()
	de, ok := domain.AsError(err)
	if !errors.Is(err, kind) || !ok || (de.Reason != reason && (len(de.Fields) == 0 || de.Fields[0].Reason != reason)) {
		t.Fatalf("err = %v, want %v (%s)", err, kind, reason)
	}
}

func payloads(evs []events.Event) []events.Payload {
	out := make([]events.Payload, len(evs))
	for i, e := range evs {
		out[i] = e.Payload
	}
	return out
}

func TestCreate(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()

	state, err := h.svc.Create(ctx, h.owner, "pt-BR", "  Game Dev  ", 0)
	if err != nil {
		t.Fatal(err)
	}
	if state.Guild.Name != "Game Dev" || state.Guild.OwnerID != 1 {
		t.Errorf("guild = %+v", state.Guild)
	}
	if len(state.Members) != 1 || state.Members[0].Role != domain.RoleOwner || len(state.Users) != 1 {
		t.Errorf("members = %+v, users = %+v", state.Members, state.Users)
	}
	type ch struct {
		Type   domain.ChannelType
		Name   string
		Parent bool
		Pos    int
		Limit  int
	}
	var got []ch
	for _, c := range state.Channels {
		got = append(got, ch{c.Type, c.Name, c.ParentID != 0, c.Position, c.UserLimit})
	}
	want := []ch{
		{domain.ChannelCategory, "Canais de texto", false, 0, 0},
		{domain.ChannelText, "geral", true, 0, 0},
		{domain.ChannelVoice, "Geral", true, 0, 25},
		{domain.ChannelCategory, "Canais de voz", false, 1, 0},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("channels: %s", cmp.Diff(want, got))
	}
	evs := h.events.Take()
	if len(evs) != 1 || !cmp.Equal(evs[0].Topics, []events.Topic{"user:1"}) {
		t.Fatalf("events = %+v", evs)
	}
	if gc, ok := evs[0].Payload.(events.GuildCreated); !ok || gc.UserID != 1 || gc.State.Guild.ID != state.Guild.ID {
		t.Errorf("payload = %+v", evs[0].Payload)
	}

	_, err = h.svc.Create(ctx, h.owner, "en", " ", 0)
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldRequired)
	_, err = h.svc.Create(ctx, h.owner, "en", "Team", 999)
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalid)
	_, err = h.svc.Create(ctx, domain.Actor{UserID: 1}, "en", "Team", 0)
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonEmailNotVerified)
}

func TestCreateWithIconAndLimit(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	_ = h.store.CreateFile(ctx, domain.File{ID: 50, OwnerID: 1, Purpose: domain.FileGuildIcon})
	_ = h.store.CreateFile(ctx, domain.File{ID: 51, OwnerID: 1, Purpose: domain.FileAvatar})

	state, err := h.svc.Create(ctx, h.owner, "en", "With icon", 50)
	if err != nil || state.Guild.IconFileID != 50 {
		t.Fatalf("create with icon: %+v, %v", state.Guild, err)
	}
	_, err = h.svc.Create(ctx, h.owner, "en", "Avatar as icon", 51)
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalid)

	h.settings.st.Limits.MaxGuildsOwnedPerUser = 2
	if _, err := h.svc.Create(ctx, h.owner, "en", "Second", 0); err != nil {
		t.Fatal(err)
	}
	_, err = h.svc.Create(ctx, h.owner, "en", "Third", 0)
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonGuildLimitReached)
	if _, err := h.svc.Create(ctx, h.admin, "en", "Someone else's", 0); err != nil {
		t.Errorf("the limit is per owner: %v", err)
	}
}

func TestUpdate(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	g := h.newGuild(t).Guild

	updated, err := h.svc.Update(ctx, h.admin, g.ID, Patch{Name: new(" Renamed ")})
	if err != nil || updated.Name != "Renamed" {
		t.Fatalf("admin rename: %+v, %v", updated, err)
	}
	evs := h.events.Take()
	if len(evs) != 2 {
		t.Fatalf("events = %+v", payloads(evs))
	}
	audit, ok := evs[0].Payload.(events.AuditAppended)
	if !ok || !cmp.Equal(evs[0].Topics, []events.Topic{events.GuildAdminsTopic(g.ID)}) {
		t.Fatalf("first event: %+v", evs[0])
	}
	if e := audit.Entry; e.Action != domain.AuditGuildUpdate || e.ActorID != 2 || e.Data["old_name"] != "Team" || e.Data["name"] != "Renamed" {
		t.Errorf("audit entry = %+v", e)
	}
	if gu, ok := evs[1].Payload.(events.GuildUpdated); !ok || gu.Guild.Name != "Renamed" || !cmp.Equal(evs[1].Topics, []events.Topic{events.GuildTopic(g.ID)}) {
		t.Errorf("second event: %+v", evs[1])
	}

	if _, err := h.svc.Update(ctx, h.admin, g.ID, Patch{Name: new("Renamed")}); err != nil {
		t.Fatal(err)
	}
	if evs := h.events.Take(); len(evs) != 0 || len(h.store.Audit(g.ID)) != 1 {
		t.Errorf("a no-op update was recorded: %+v", payloads(evs))
	}

	_, err = h.svc.Update(ctx, h.member, g.ID, Patch{Name: new("Nope")})
	requireReason(t, err, domain.ErrForbidden, domain.ReasonMissingPermission)
	_, err = h.svc.Update(ctx, h.outsider, g.ID, Patch{Name: new("Nope")})
	requireReason(t, err, domain.ErrNotFound, domain.ReasonGuildNotFound)
	_, err = h.svc.Update(ctx, h.owner, 999, Patch{Name: new("Nope")})
	requireReason(t, err, domain.ErrNotFound, domain.ReasonGuildNotFound)
}

func TestDelete(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	g := h.newGuild(t).Guild

	requireReason(t, h.svc.Delete(ctx, h.admin, g.ID, "Team"), domain.ErrForbidden, domain.ReasonMissingPermission)
	requireReason(t, h.svc.Delete(ctx, h.owner, g.ID, "team"), domain.ErrInvalid, domain.ReasonFieldIncorrect)
	if err := h.svc.Delete(ctx, h.owner, g.ID, " Team "); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.GuildByID(ctx, g.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Error("guild still exists")
	}
	want := []events.Payload{events.GuildDeleted{GuildID: g.ID, Reason: events.ReasonGuildDeleted}}
	if got := payloads(h.events.Take()); !cmp.Equal(got, want) {
		t.Errorf("events: %s", cmp.Diff(want, got))
	}
}

func TestLeave(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	g := h.newGuild(t).Guild

	requireReason(t, h.svc.Leave(ctx, h.owner, g.ID), domain.ErrFailedPrecondition, domain.ReasonOwnerCannotLeave)
	if err := h.svc.Leave(ctx, h.member, g.ID); err != nil {
		t.Fatal(err)
	}
	evs := h.events.Take()
	want := []events.Payload{
		events.GuildDeleted{GuildID: g.ID, UserID: 3, Reason: events.ReasonLeft},
		events.MemberRemoved{GuildID: g.ID, UserID: 3},
	}
	if !cmp.Equal(payloads(evs), want) || !cmp.Equal(evs[0].Topics, []events.Topic{"user:3"}) {
		t.Errorf("events: %+v", evs)
	}
	requireReason(t, h.svc.Leave(ctx, h.member, g.ID), domain.ErrNotFound, domain.ReasonGuildNotFound)
}

func TestTransferOwnership(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	g := h.newGuild(t).Guild

	_, err := h.svc.TransferOwnership(ctx, h.admin, g.ID, 3)
	requireReason(t, err, domain.ErrForbidden, domain.ReasonMissingPermission)
	_, err = h.svc.TransferOwnership(ctx, h.owner, g.ID, 4)
	requireReason(t, err, domain.ErrNotFound, domain.ReasonMemberNotFound)

	updated, err := h.svc.TransferOwnership(ctx, h.owner, g.ID, 3)
	if err != nil || updated.OwnerID != 3 {
		t.Fatalf("transfer: %+v, %v", updated, err)
	}
	newOwner, _ := h.store.MemberByID(ctx, g.ID, 3)
	oldOwner, _ := h.store.MemberByID(ctx, g.ID, 1)
	if newOwner.Role != domain.RoleOwner || oldOwner.Role != domain.RoleAdmin {
		t.Errorf("roles: new %s, old %s", newOwner.Role, oldOwner.Role)
	}
	if a := h.store.Audit(g.ID); len(a) != 1 || a[0].Action != domain.AuditOwnershipTransfer || a[0].TargetID != "3" {
		t.Errorf("audit = %+v", a)
	}
	evs := payloads(h.events.Take())
	if len(evs) != 4 {
		t.Fatalf("events = %+v", evs)
	}
	if mu, ok := evs[2].(events.MemberUpdated); !ok || mu.Member.UserID != 3 || mu.Member.Role != domain.RoleOwner {
		t.Errorf("new owner event: %+v", evs[2])
	}

	// The receiver must stay within the owned-guilds limit.
	h.settings.st.Limits.MaxGuildsOwnedPerUser = 1
	if _, err := h.svc.Create(ctx, h.owner, "en", "Own", 0); err != nil {
		t.Fatal(err)
	}
	_, err = h.svc.TransferOwnership(ctx, actor(3), g.ID, 1)
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonGuildLimitReached)
}

func TestSetMemberRole(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	g := h.newGuild(t).Guild

	m, err := h.svc.SetMemberRole(ctx, h.owner, g.ID, 3, domain.RoleAdmin)
	if err != nil || m.Role != domain.RoleAdmin {
		t.Fatalf("promote: %+v, %v", m, err)
	}
	evs := h.events.Take()
	if len(evs) != 2 {
		t.Fatalf("events = %+v", payloads(evs))
	}
	if mu, ok := evs[1].Payload.(events.MemberUpdated); !ok || mu.Member.Role != domain.RoleAdmin {
		t.Errorf("event = %+v", evs[1])
	}
	if a := h.store.Audit(g.ID); len(a) != 1 || a[0].Data["role"] != "admin" {
		t.Errorf("audit = %+v", a)
	}
	if _, err := h.svc.SetMemberRole(ctx, h.owner, g.ID, 3, domain.RoleAdmin); err != nil || len(h.events.Take()) != 0 {
		t.Errorf("same role again: %v", err)
	}
	_, err = h.svc.SetMemberRole(ctx, h.admin, g.ID, 3, domain.RoleMember)
	requireReason(t, err, domain.ErrForbidden, domain.ReasonMissingPermission)
	_, err = h.svc.SetMemberRole(ctx, h.owner, g.ID, 3, domain.RoleOwner)
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalid)
}

func TestMembers(t *testing.T) {
	h := newHarness(t)
	g := h.newGuild(t).Guild
	list, err := h.svc.Members(t.Context(), h.member, g.ID)
	if err != nil || len(list.Members) != 3 || len(list.Users) != 3 || len(list.Presences) != 3 {
		t.Fatalf("members: %+v, %v", list, err)
	}
	_, err = h.svc.Members(t.Context(), h.outsider, g.ID)
	requireReason(t, err, domain.ErrNotFound, domain.ReasonGuildNotFound)
}
