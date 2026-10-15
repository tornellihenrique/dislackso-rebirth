package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// seedGuild creates two users and a guild owned by the first one.
func seedGuild(t *testing.T, db *DB) (owner, member domain.User, g domain.Guild) {
	t.Helper()
	ctx := t.Context()
	owner, member = newUser(1, "owner", "owner@example.com"), newUser(2, "member", "member@example.com")
	for _, u := range []domain.User{owner, member} {
		if err := db.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	g = domain.Guild{ID: 10, Name: "Team", OwnerID: owner.ID, CreatedAt: t0, UpdatedAt: t0}
	if err := db.CreateGuild(ctx, g); err != nil {
		t.Fatalf("CreateGuild: %v", err)
	}
	if err := db.AddMember(ctx, domain.Member{GuildID: g.ID, UserID: owner.ID, Role: domain.RoleOwner, JoinedAt: t0, UpdatedAt: t0}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	return owner, member, g
}

func TestGuildsAndMembers(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	owner, member, g := seedGuild(t, db)

	got, err := db.GuildByID(ctx, g.ID)
	if err != nil || !cmp.Equal(got, g) {
		t.Fatalf("GuildByID = %+v, %v; want %+v", got, err, g)
	}
	if _, err := db.GuildByID(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown guild: %v", err)
	}

	icon := domain.File{ID: 50, OwnerID: owner.ID, Purpose: domain.FileGuildIcon, SHA256: []byte{1}, Size: 3, Mime: "image/png", StorageKey: "k", CreatedAt: t0}
	if err := db.CreateFile(ctx, icon); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	g.Name, g.IconFileID, g.UpdatedAt = "Renamed", icon.ID, t0.Add(time.Minute)
	if err := db.UpdateGuild(ctx, g); err != nil {
		t.Fatalf("UpdateGuild: %v", err)
	}
	if got, _ := db.GuildByID(ctx, g.ID); !cmp.Equal(got, g) {
		t.Errorf("after update: %+v", got)
	}
	if err := db.UpdateGuild(ctx, domain.Guild{ID: 99, Name: "x", OwnerID: owner.ID}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("update unknown guild: %v", err)
	}

	m := domain.Member{GuildID: g.ID, UserID: member.ID, Role: domain.RoleMember, JoinedAt: t0.Add(time.Second), UpdatedAt: t0.Add(time.Second)}
	if err := db.AddMember(ctx, m); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if err := db.AddMember(ctx, m); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("adding twice: %v", err)
	}
	if n, _ := db.CountMembers(ctx, g.ID); n != 2 {
		t.Errorf("CountMembers = %d", n)
	}
	if err := db.UpdateMemberRole(ctx, g.ID, member.ID, domain.RoleAdmin, t0.Add(time.Hour)); err != nil {
		t.Fatalf("UpdateMemberRole: %v", err)
	}
	gotM, err := db.MemberByID(ctx, g.ID, member.ID)
	if err != nil || gotM.Role != domain.RoleAdmin || !gotM.UpdatedAt.Equal(t0.Add(time.Hour)) {
		t.Errorf("MemberByID = %+v, %v", gotM, err)
	}
	members, _ := db.GuildMembers(ctx, g.ID)
	if len(members) != 2 || members[0].UserID != owner.ID || members[1].UserID != member.ID {
		t.Errorf("GuildMembers order: %+v", members)
	}
	users, _ := db.GuildUsers(ctx, g.ID)
	if len(users) != 2 || users[0].ID != owner.ID {
		t.Errorf("GuildUsers: %+v", users)
	}
	ms, _ := db.UserMemberships(ctx, member.ID)
	if len(ms) != 1 || ms[0].GuildID != g.ID {
		t.Errorf("UserMemberships: %+v", ms)
	}
	gs, _ := db.UserGuilds(ctx, member.ID)
	if len(gs) != 1 || gs[0].ID != g.ID {
		t.Errorf("UserGuilds: %+v", gs)
	}
	if n, _ := db.CountGuildsOwnedBy(ctx, owner.ID); n != 1 {
		t.Errorf("CountGuildsOwnedBy = %d", n)
	}

	if err := db.RemoveMember(ctx, g.ID, member.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if err := db.RemoveMember(ctx, g.ID, member.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("removing twice: %v", err)
	}
}

func TestDeleteGuildCascades(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	owner, member, g := seedGuild(t, db)
	if err := db.CreateChannel(ctx, domain.Channel{ID: 20, GuildID: g.ID, Type: domain.ChannelText, Name: "general", CreatedAt: t0, UpdatedAt: t0}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateInvite(ctx, domain.Invite{Code: "ABCDEFGH", GuildID: g.ID, CreatedBy: owner.ID, CreatedAt: t0}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateBan(ctx, domain.Ban{GuildID: g.ID, UserID: member.ID, BannedBy: owner.ID, CreatedAt: t0}); err != nil {
		t.Fatal(err)
	}
	if err := db.AppendAudit(ctx, domain.AuditEntry{ID: 30, GuildID: g.ID, ActorID: owner.ID, Action: domain.AuditMemberBan, TargetType: domain.AuditTargetUser, CreatedAt: t0}); err != nil {
		t.Fatal(err)
	}

	if err := db.DeleteGuild(ctx, g.ID); err != nil {
		t.Fatalf("DeleteGuild: %v", err)
	}
	if _, err := db.ChannelByID(ctx, 20); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("channel survived: %v", err)
	}
	if _, err := db.InviteByCode(ctx, "ABCDEFGH"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("invite survived: %v", err)
	}
	if _, err := db.BanByID(ctx, g.ID, member.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ban survived: %v", err)
	}
	if ms, _ := db.UserMemberships(ctx, owner.ID); len(ms) != 0 {
		t.Errorf("membership survived: %+v", ms)
	}
	if entries, _ := db.AuditLog(ctx, g.ID, 0, 10); len(entries) != 0 {
		t.Errorf("audit log survived: %+v", entries)
	}
}

func TestChannels(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	_, _, g := seedGuild(t, db)

	cat := domain.Channel{ID: 20, GuildID: g.ID, Type: domain.ChannelCategory, Name: "Voice", Position: 0, CreatedAt: t0, UpdatedAt: t0}
	voice := domain.Channel{
		ID: 21, GuildID: g.ID, ParentID: cat.ID, Type: domain.ChannelVoice, Name: "Lounge", Position: 1,
		UserLimit: 25, DrawingAllowed: true, MaxShareQuality: "1080p60", CreatedAt: t0, UpdatedAt: t0,
	}
	text := domain.Channel{ID: 22, GuildID: g.ID, Type: domain.ChannelText, Name: "general", Topic: "hi", Position: 0, CreatedAt: t0, UpdatedAt: t0}
	for _, c := range []domain.Channel{cat, voice, text} {
		if err := db.CreateChannel(ctx, c); err != nil {
			t.Fatalf("CreateChannel %s: %v", c.Name, err)
		}
	}
	got, err := db.ChannelByID(ctx, voice.ID)
	if err != nil || !cmp.Equal(got, voice) {
		t.Fatalf("ChannelByID = %+v, %v", got, err)
	}
	if n, _ := db.CountChannels(ctx, g.ID); n != 3 {
		t.Errorf("CountChannels = %d", n)
	}
	all, _ := db.GuildChannels(ctx, g.ID)
	if ids := []domain.ID{all[0].ID, all[1].ID, all[2].ID}; !cmp.Equal(ids, []domain.ID{20, 22, 21}) {
		t.Errorf("GuildChannels order by position then id: %v", ids)
	}

	voice.Name, voice.UserLimit, voice.DrawingAllowed, voice.UpdatedAt = "Studio", 0, false, t0.Add(time.Minute)
	if err := db.UpdateChannel(ctx, voice); err != nil {
		t.Fatalf("UpdateChannel: %v", err)
	}
	if got, _ := db.ChannelByID(ctx, voice.ID); !cmp.Equal(got, voice) {
		t.Errorf("after update: %+v", got)
	}

	// Deleting the category leaves its channels at the top level.
	if err := db.DeleteChannel(ctx, cat.ID); err != nil {
		t.Fatalf("DeleteChannel: %v", err)
	}
	if got, _ := db.ChannelByID(ctx, voice.ID); got.ParentID != 0 {
		t.Errorf("child kept parent %d", got.ParentID)
	}
	if err := db.DeleteChannel(ctx, cat.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("deleting twice: %v", err)
	}
}

func TestInvites(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	owner, _, g := seedGuild(t, db)
	expires := t0.Add(time.Hour)
	limited := domain.Invite{Code: "LIMITED2", GuildID: g.ID, CreatedBy: owner.ID, MaxUses: 1, ExpiresAt: &expires, CreatedAt: t0}
	forever := domain.Invite{Code: "FOREVER2", GuildID: g.ID, CreatedBy: owner.ID, CreatedAt: t0.Add(time.Second)}
	for _, inv := range []domain.Invite{limited, forever} {
		if err := db.CreateInvite(ctx, inv); err != nil {
			t.Fatalf("CreateInvite: %v", err)
		}
	}
	if err := db.CreateInvite(ctx, forever); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate code: %v", err)
	}
	got, err := db.InviteByCode(ctx, limited.Code)
	if err != nil || !cmp.Equal(got, limited) {
		t.Fatalf("InviteByCode = %+v, %v", got, err)
	}

	usable, _ := db.UsableInvites(ctx, g.ID, t0)
	if len(usable) != 2 || usable[0].Code != forever.Code {
		t.Errorf("UsableInvites newest first: %+v", usable)
	}
	if err := db.UseInvite(ctx, limited.Code, t0); err != nil {
		t.Fatalf("UseInvite: %v", err)
	}
	if err := db.UseInvite(ctx, limited.Code, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("using past the limit: %v", err)
	}
	if err := db.UseInvite(ctx, forever.Code, t0.Add(48*time.Hour)); err != nil {
		t.Errorf("never-expiring invite: %v", err)
	}
	if usable, _ := db.UsableInvites(ctx, g.ID, t0); len(usable) != 1 {
		t.Errorf("used-up invite still listed: %+v", usable)
	}

	if err := db.RevokeInvite(ctx, forever.Code, t0); err != nil {
		t.Fatalf("RevokeInvite: %v", err)
	}
	if err := db.UseInvite(ctx, forever.Code, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("using a revoked invite: %v", err)
	}
	if err := db.RevokeInvite(ctx, forever.Code, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("revoking twice: %v", err)
	}

	expired := domain.Invite{Code: "EXPIRED2", GuildID: g.ID, CreatedBy: owner.ID, ExpiresAt: &t0, CreatedAt: t0}
	if err := db.CreateInvite(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if err := db.UseInvite(ctx, expired.Code, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("using at the expiry instant: %v", err)
	}
}

func TestBansAndAudit(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	owner, member, g := seedGuild(t, db)

	ban := domain.Ban{GuildID: g.ID, UserID: member.ID, BannedBy: owner.ID, Reason: "spam", CreatedAt: t0}
	if err := db.CreateBan(ctx, ban); err != nil {
		t.Fatalf("CreateBan: %v", err)
	}
	if err := db.CreateBan(ctx, ban); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("banning twice: %v", err)
	}
	if got, err := db.BanByID(ctx, g.ID, member.ID); err != nil || !cmp.Equal(got, ban) {
		t.Errorf("BanByID = %+v, %v", got, err)
	}
	if bans, _ := db.GuildBans(ctx, g.ID); len(bans) != 1 {
		t.Errorf("GuildBans: %+v", bans)
	}
	if err := db.DeleteBan(ctx, g.ID, member.ID); err != nil {
		t.Fatalf("DeleteBan: %v", err)
	}
	if err := db.DeleteBan(ctx, g.ID, member.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbanning twice: %v", err)
	}

	for i := range 5 {
		e := domain.AuditEntry{
			ID: domain.ID(100 + i), GuildID: g.ID, ActorID: owner.ID, Action: domain.AuditChannelCreate,
			TargetType: domain.AuditTargetChannel, TargetID: "7", Data: map[string]string{"name": "c"}, CreatedAt: t0,
		}
		if err := db.AppendAudit(ctx, e); err != nil {
			t.Fatalf("AppendAudit: %v", err)
		}
	}
	page, _ := db.AuditLog(ctx, g.ID, 0, 2)
	if len(page) != 2 || page[0].ID != 104 || page[1].ID != 103 || page[0].Data["name"] != "c" {
		t.Errorf("first page: %+v", page)
	}
	page, _ = db.AuditLog(ctx, g.ID, page[1].ID, 10)
	if len(page) != 3 || page[0].ID != 102 {
		t.Errorf("second page: %+v", page)
	}
}

func TestAfterCommit(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	ran := 0
	db.AfterCommit(ctx, func() { ran++ })
	if ran != 1 {
		t.Fatalf("outside a transaction the hook runs at once")
	}
	err := db.InTx(ctx, func(ctx context.Context) error {
		db.AfterCommit(ctx, func() { ran++ })
		if ran != 1 {
			t.Error("hook ran before commit")
		}
		return nil
	})
	if err != nil || ran != 2 {
		t.Fatalf("after commit: ran = %d, err = %v", ran, err)
	}
	_ = db.InTx(ctx, func(ctx context.Context) error {
		db.AfterCommit(ctx, func() { ran++ })
		return errors.New("boom")
	})
	if ran != 2 {
		t.Errorf("hook ran after rollback")
	}
}
