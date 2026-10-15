package channel

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

const guildID domain.ID = 100

type harness struct {
	svc      *Service
	store    *fakes.Store
	events   *fakes.Events
	settings *settings
	admin    domain.Actor
	member   domain.Actor
}

// The guild has: category Text (200) with #general (201), category Voice
// (202) with Lounge (203), and #top (204) at the top level.
func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := t.Context()
	h := &harness{store: fakes.NewStore(), settings: &settings{st: domain.DefaultInstanceSettings()}}
	h.events = fakes.NewEvents(h.store)
	h.svc = New(Deps{
		Guilds: h.store, Members: h.store, Channels: h.store, Users: h.store, Audit: h.store, Tx: h.store,
		IDs: &fakes.IDs{}, Clock: fakes.NewClock(), Events: h.events, Settings: h.settings,
		Presence: &fakes.Presence{}, Verification: fakes.Verification(false),
	})
	_ = h.store.CreateGuild(ctx, domain.Guild{ID: guildID, Name: "Team", OwnerID: 1})
	_ = h.store.AddMember(ctx, domain.Member{GuildID: guildID, UserID: 1, Role: domain.RoleAdmin})
	_ = h.store.AddMember(ctx, domain.Member{GuildID: guildID, UserID: 2, Role: domain.RoleMember})
	for _, c := range []domain.Channel{
		{ID: 200, Type: domain.ChannelCategory, Name: "Text", Position: 0},
		{ID: 201, ParentID: 200, Type: domain.ChannelText, Name: "general", Position: 0},
		{ID: 202, Type: domain.ChannelCategory, Name: "Voice", Position: 1},
		{ID: 203, ParentID: 202, Type: domain.ChannelVoice, Name: "Lounge", Position: 0, UserLimit: 25, DrawingAllowed: true},
		{ID: 204, Type: domain.ChannelText, Name: "top", Position: 0},
	} {
		c.GuildID = guildID
		if err := h.store.CreateChannel(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	h.admin, h.member = domain.Actor{UserID: 1}, domain.Actor{UserID: 2}
	return h
}

func requireReason(t *testing.T, err, kind error, reason string) {
	t.Helper()
	de, ok := domain.AsError(err)
	if !errors.Is(err, kind) || !ok || (de.Reason != reason && (len(de.Fields) == 0 || de.Fields[0].Reason != reason)) {
		t.Fatalf("err = %v, want %v (%s)", err, kind, reason)
	}
}

func lastPayload(t *testing.T, evs []events.Event) events.Payload {
	t.Helper()
	if len(evs) == 0 {
		t.Fatal("no events")
	}
	return evs[len(evs)-1].Payload
}

func TestCreate(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()

	c, err := h.svc.Create(ctx, h.admin, guildID, domain.ChannelText, " Release Notes ", 200)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "release-notes" || c.ParentID != 200 || c.Position != 1 || c.UserLimit != 0 {
		t.Errorf("text channel = %+v", c)
	}
	evs := h.events.Take()
	if cc, ok := lastPayload(t, evs).(events.ChannelCreated); !ok || cc.Channel.ID != c.ID {
		t.Errorf("events = %+v", evs)
	}
	if a := h.store.Audit(guildID); len(a) != 1 || a[0].Action != domain.AuditChannelCreate || a[0].Data["name"] != "release-notes" {
		t.Errorf("audit = %+v", a)
	}

	v, err := h.svc.Create(ctx, h.admin, guildID, domain.ChannelVoice, "Daily Standup", 0)
	if err != nil || v.Name != "Daily Standup" || v.Position != 1 || v.UserLimit != 25 || !v.DrawingAllowed {
		t.Errorf("voice channel = %+v, %v", v, err)
	}
	cat, err := h.svc.Create(ctx, h.admin, guildID, domain.ChannelCategory, "Archive", 0)
	if err != nil || cat.Position != 2 {
		t.Errorf("category = %+v, %v", cat, err)
	}

	_, err = h.svc.Create(ctx, h.member, guildID, domain.ChannelText, "nope", 0)
	requireReason(t, err, domain.ErrForbidden, domain.ReasonMissingPermission)
	_, err = h.svc.Create(ctx, domain.Actor{UserID: 9}, guildID, domain.ChannelText, "nope", 0)
	requireReason(t, err, domain.ErrNotFound, domain.ReasonGuildNotFound)
	for name, parent := range map[string]domain.ID{"text as parent": 201, "missing parent": 999} {
		_, err = h.svc.Create(ctx, h.admin, guildID, domain.ChannelText, name, parent)
		requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalid)
	}
	_, err = h.svc.Create(ctx, h.admin, guildID, domain.ChannelCategory, "nested", 200)
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalid)
	_, err = h.svc.Create(ctx, h.admin, guildID, "thread", "x", 0)
	requireReason(t, err, domain.ErrInvalid, domain.ReasonFieldInvalid)

	h.settings.st.Limits.MaxChannelsPerGuild = 8
	_, err = h.svc.Create(ctx, h.admin, guildID, domain.ChannelText, "one-too-many", 0)
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonChannelLimitReached)
}

func TestUpdate(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()

	c, err := h.svc.Update(ctx, h.admin, 203, Patch{Name: new("Studio"), UserLimit: new(5), DrawingAllowed: new(false), MaxShareQuality: new("1080p60")})
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "Studio" || c.UserLimit != 5 || c.DrawingAllowed || c.MaxShareQuality != "1080p60" {
		t.Errorf("updated = %+v", c)
	}
	if cu, ok := lastPayload(t, h.events.Take()).(events.ChannelUpdated); !ok || cu.Channel.Name != "Studio" {
		t.Errorf("missing ChannelUpdated")
	}
	if a := h.store.Audit(guildID); len(a) != 1 || a[0].Data["old_name"] != "Lounge" || a[0].Data["user_limit"] != "5" {
		t.Errorf("audit = %+v", a)
	}

	if _, err := h.svc.Update(ctx, h.admin, 203, Patch{Name: new("Studio")}); err != nil || len(h.events.Take()) != 0 {
		t.Errorf("no-op update: %v", err)
	}
	renamed, _ := h.svc.Update(ctx, h.admin, 201, Patch{Name: new("Off Topic"), Topic: new("anything goes")})
	if renamed.Name != "off-topic" || renamed.Topic != "anything goes" {
		t.Errorf("text rename = %+v", renamed)
	}

	tests := []struct {
		name string
		id   domain.ID
		p    Patch
		want string
	}{
		{"voice field on text", 201, Patch{UserLimit: new(3)}, domain.ReasonFieldInvalid},
		{"topic on category", 200, Patch{Topic: new("x")}, domain.ReasonFieldInvalid},
		{"limit too high", 203, Patch{UserLimit: new(100)}, domain.ReasonFieldInvalid},
		{"unknown preset", 203, Patch{MaxShareQuality: new("8k")}, domain.ReasonFieldInvalid},
		{"empty name", 203, Patch{Name: new(" ")}, domain.ReasonFieldRequired},
	}
	for _, tt := range tests {
		_, err := h.svc.Update(ctx, h.admin, tt.id, tt.p)
		if de, ok := domain.AsError(err); !ok || !errors.Is(err, domain.ErrInvalid) || de.Fields[0].Reason != tt.want {
			t.Errorf("%s: %v", tt.name, err)
		}
	}
	_, err = h.svc.Update(ctx, h.member, 203, Patch{Name: new("x")})
	requireReason(t, err, domain.ErrForbidden, domain.ReasonMissingPermission)
	_, err = h.svc.Update(ctx, domain.Actor{UserID: 9}, 203, Patch{Name: new("x")})
	requireReason(t, err, domain.ErrNotFound, domain.ReasonChannelNotFound)
	_, err = h.svc.Update(ctx, h.admin, 999, Patch{Name: new("x")})
	requireReason(t, err, domain.ErrNotFound, domain.ReasonChannelNotFound)
}

func TestDeleteCategoryMovesChildren(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()

	if err := h.svc.Delete(ctx, h.admin, 200); err != nil {
		t.Fatal(err)
	}
	moved, _ := h.store.ChannelByID(ctx, 201)
	if moved.ParentID != 0 || moved.Position != 1 {
		t.Errorf("#general after its category was deleted: %+v", moved)
	}
	evs := h.events.Take()
	var got []string
	for _, e := range evs {
		switch p := e.Payload.(type) {
		case events.ChannelDeleted:
			got = append(got, "deleted "+p.ChannelID.String())
		case events.ChannelsReordered:
			got = append(got, "reordered "+p.Channels[0].ID.String())
		case events.AuditAppended:
			got = append(got, "audit")
		}
	}
	if want := []string{"audit", "deleted 200", "reordered 201"}; !cmp.Equal(got, want) {
		t.Errorf("events = %v, want %v", got, want)
	}
	requireReason(t, h.svc.Delete(ctx, h.member, 203), domain.ErrForbidden, domain.ReasonMissingPermission)
}

func TestReorder(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()

	// Voice first, #top moves into Voice, #general stays.
	order := []Placement{
		{ChannelID: 202},
		{ChannelID: 204, ParentID: 202},
		{ChannelID: 203, ParentID: 202},
		{ChannelID: 200},
		{ChannelID: 201, ParentID: 200},
	}
	all, err := h.svc.Reorder(ctx, h.admin, guildID, order)
	if err != nil {
		t.Fatal(err)
	}
	type pos struct {
		ID, Parent domain.ID
		Pos        int
	}
	got := map[domain.ID]pos{}
	for _, c := range all {
		got[c.ID] = pos{c.ID, c.ParentID, c.Position}
	}
	want := map[domain.ID]pos{
		200: {200, 0, 1}, 201: {201, 200, 0}, 202: {202, 0, 0}, 203: {203, 202, 1}, 204: {204, 202, 0},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("positions: %s", cmp.Diff(want, got))
	}
	stored, _ := h.store.ChannelByID(ctx, 204)
	if stored.ParentID != 202 {
		t.Errorf("stored parent = %d", stored.ParentID)
	}
	re, ok := lastPayload(t, h.events.Take()).(events.ChannelsReordered)
	if !ok || len(re.Channels) != 4 {
		t.Errorf("reordered event carries only changed channels: %+v", re)
	}

	if _, err := h.svc.Reorder(ctx, h.admin, guildID, order); err != nil || len(h.events.Take()) != 0 {
		t.Errorf("same order again: %v", err)
	}

	bad := map[string][]Placement{
		"missing channel":     order[:4],
		"duplicate":           {order[0], order[0], order[2], order[3], order[4]},
		"category in a cat":   {{ChannelID: 202, ParentID: 200}, order[1], order[2], order[3], order[4]},
		"text as parent":      {order[0], {ChannelID: 204, ParentID: 201}, order[2], order[3], order[4]},
		"foreign channel":     {order[0], order[1], order[2], order[3], {ChannelID: 999}},
		"parent not in guild": {order[0], {ChannelID: 204, ParentID: 999}, order[2], order[3], order[4]},
	}
	for name, p := range bad {
		_, err := h.svc.Reorder(ctx, h.admin, guildID, p)
		if !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	_, err = h.svc.Reorder(ctx, h.member, guildID, order)
	requireReason(t, err, domain.ErrForbidden, domain.ReasonMissingPermission)
}
