package moderation

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

const guildID domain.ID = 100

var (
	owner  = domain.Actor{UserID: 1}
	admin  = domain.Actor{UserID: 2}
	admin2 = domain.Actor{UserID: 3}
	member = domain.Actor{UserID: 4}
)

type harness struct {
	svc    *Service
	store  *fakes.Store
	events *fakes.Events
}

// Guild 100: owner 1, admins 2 and 3, members 4 and 5; user 6 is outside.
func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := t.Context()
	h := &harness{store: fakes.NewStore()}
	h.events = fakes.NewEvents(h.store)
	h.svc = New(Deps{
		Guilds: h.store, Members: h.store, Channels: h.store, Users: h.store, Bans: h.store, Audit: h.store,
		Tx: h.store, IDs: &fakes.IDs{}, Clock: fakes.NewClock(), Events: h.events,
		Presence: &fakes.Presence{}, Verification: fakes.Verification(false),
	})
	_ = h.store.CreateGuild(ctx, domain.Guild{ID: guildID, Name: "Team", OwnerID: 1})
	roles := map[domain.ID]domain.GuildRole{1: domain.RoleOwner, 2: domain.RoleAdmin, 3: domain.RoleAdmin, 4: domain.RoleMember, 5: domain.RoleMember}
	for id := domain.ID(1); id <= 6; id++ {
		if err := h.store.CreateUser(ctx, domain.User{ID: id, Username: "u" + id.String(), Email: id.String() + "@x.io"}); err != nil {
			t.Fatal(err)
		}
		if role, ok := roles[id]; ok {
			_ = h.store.AddMember(ctx, domain.Member{GuildID: guildID, UserID: id, Role: role})
		}
	}
	return h
}

func requireReason(t *testing.T, err, kind error, reason string) {
	t.Helper()
	de, ok := domain.AsError(err)
	if !errors.Is(err, kind) || !ok || (de.Reason != reason && (len(de.Fields) == 0 || de.Fields[0].Reason != reason)) {
		t.Fatalf("err = %v, want %v (%s)", err, kind, reason)
	}
}

func TestKick(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()

	if err := h.svc.Kick(ctx, admin, guildID, 4, " spam "); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.MemberByID(ctx, guildID, 4); !errors.Is(err, domain.ErrNotFound) {
		t.Error("still a member")
	}
	evs := h.events.Take()
	var kinds []string
	for _, e := range evs {
		switch p := e.Payload.(type) {
		case events.AuditAppended:
			if p.Entry.Action != domain.AuditMemberKick || p.Entry.Data["reason"] != "spam" || p.Entry.TargetID != "4" {
				t.Errorf("audit = %+v", p.Entry)
			}
			kinds = append(kinds, "audit")
		case events.GuildDeleted:
			if p.Reason != events.ReasonKicked || p.UserID != 4 || !cmp.Equal(e.Topics, []events.Topic{"user:4"}) {
				t.Errorf("guild delete = %+v to %v", p, e.Topics)
			}
			kinds = append(kinds, "guild_delete")
		case events.MemberRemoved:
			kinds = append(kinds, "member_remove")
		}
	}
	if want := []string{"audit", "guild_delete", "member_remove"}; !cmp.Equal(kinds, want) {
		t.Errorf("events = %v", kinds)
	}

	requireReason(t, h.svc.Kick(ctx, admin, guildID, 3, ""), domain.ErrForbidden, domain.ReasonCannotTargetAdmin)
	requireReason(t, h.svc.Kick(ctx, admin, guildID, 1, ""), domain.ErrForbidden, domain.ReasonCannotTargetOwner)
	requireReason(t, h.svc.Kick(ctx, domain.Actor{UserID: 5}, guildID, 3, ""), domain.ErrForbidden, domain.ReasonMissingPermission)
	requireReason(t, h.svc.Kick(ctx, admin, guildID, 6, ""), domain.ErrNotFound, domain.ReasonMemberNotFound)
	requireReason(t, h.svc.Kick(ctx, owner, guildID, 5, strings.Repeat("x", 600)), domain.ErrInvalid, domain.ReasonFieldTooLong)
	if err := h.svc.Kick(ctx, owner, guildID, 3, ""); err != nil {
		t.Errorf("owner kicks an admin: %v", err)
	}
}

func TestBanAndUnban(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()

	if err := h.svc.Ban(ctx, admin, guildID, 5, "raid"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.MemberByID(ctx, guildID, 5); !errors.Is(err, domain.ErrNotFound) {
		t.Error("banned user is still a member")
	}
	evs := h.events.Take()
	var sawDelete, sawBan bool
	for _, e := range evs {
		switch p := e.Payload.(type) {
		case events.GuildDeleted:
			sawDelete = p.Reason == events.ReasonBanned && p.UserID == 5
		case events.BanAdded:
			sawBan = p.Ban.User.ID == 5 && p.Ban.BannedBy.ID == 2 && p.Ban.Ban.Reason == "raid" &&
				cmp.Equal(e.Topics, []events.Topic{events.GuildAdminsTopic(guildID)})
		}
	}
	if !sawDelete || !sawBan {
		t.Errorf("events = %+v", evs)
	}

	// Banning someone outside the guild keeps them out.
	if err := h.svc.Ban(ctx, admin, guildID, 6, ""); err != nil {
		t.Fatalf("ban non-member: %v", err)
	}
	for _, e := range h.events.Take() {
		if _, ok := e.Payload.(events.GuildDeleted); ok {
			t.Error("a non-member got a guild delete")
		}
	}
	requireReason(t, h.svc.Ban(ctx, admin, guildID, 6, ""), domain.ErrConflict, domain.ReasonFieldTaken)
	requireReason(t, h.svc.Ban(ctx, admin, guildID, 99, ""), domain.ErrNotFound, domain.ReasonUserNotFound)
	requireReason(t, h.svc.Ban(ctx, admin, guildID, 3, ""), domain.ErrForbidden, domain.ReasonCannotTargetAdmin)

	bans, err := h.svc.Bans(ctx, admin2, guildID)
	if err != nil || len(bans) != 2 {
		t.Fatalf("bans = %+v, %v", bans, err)
	}
	_, err = h.svc.Bans(ctx, member, guildID)
	requireReason(t, err, domain.ErrForbidden, domain.ReasonMissingPermission)

	if err := h.svc.Unban(ctx, admin, guildID, 5); err != nil {
		t.Fatal(err)
	}
	if br, ok := h.events.Take()[1].Payload.(events.BanRemoved); !ok || br.UserID != 5 {
		t.Errorf("missing BanRemoved")
	}
	requireReason(t, h.svc.Unban(ctx, admin, guildID, 5), domain.ErrNotFound, domain.ReasonBanNotFound)
	requireReason(t, h.svc.Unban(ctx, member, guildID, 6), domain.ErrForbidden, domain.ReasonMissingPermission)
}

func TestAuditLog(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	if err := h.svc.Kick(ctx, admin, guildID, 5, ""); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.Ban(ctx, admin, guildID, 6, ""); err != nil {
		t.Fatal(err)
	}
	page, err := h.svc.AuditLog(ctx, owner, guildID, 0, 1)
	if err != nil || len(page.Entries) != 1 || page.Entries[0].TargetID != "6" {
		t.Fatalf("first page = %+v, %v", page, err)
	}
	ids := map[domain.ID]bool{}
	for _, u := range page.Users {
		ids[u.ID] = true
	}
	if !ids[2] || !ids[6] || len(ids) != 2 {
		t.Errorf("users = %+v", page.Users)
	}
	next, _ := h.svc.AuditLog(ctx, owner, guildID, page.Entries[0].ID, 0)
	if len(next.Entries) != 1 || next.Entries[0].TargetID != "5" {
		t.Errorf("second page = %+v", next.Entries)
	}
	_, err = h.svc.AuditLog(ctx, member, guildID, 0, 0)
	requireReason(t, err, domain.ErrForbidden, domain.ReasonMissingPermission)
	_, err = h.svc.AuditLog(ctx, domain.Actor{UserID: 6}, guildID, 0, 0)
	requireReason(t, err, domain.ErrNotFound, domain.ReasonGuildNotFound)
}
