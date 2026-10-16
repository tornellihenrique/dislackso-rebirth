package voice

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

const (
	guildID   domain.ID = 100
	otherID   domain.ID = 200
	lobby     domain.ID = 10
	focus     domain.ID = 11
	text      domain.ID = 12
	elsewhere domain.ID = 20
)

var (
	owner  = domain.Actor{UserID: 1}
	admin  = domain.Actor{UserID: 2}
	member = domain.Actor{UserID: 4}
	other  = domain.Actor{UserID: 5}
)

type settings struct{ st domain.InstanceSettings }

func (s *settings) Get(context.Context) (domain.InstanceSettings, error) { return s.st, nil }

type harness struct {
	svc      *Service
	store    *fakes.Store
	events   *fakes.Events
	media    *fakes.Media
	clock    *fakes.Clock
	limiter  *fakes.Limiter
	settings *settings
}

// Guild 100: owner 1, admin 2, members 4 and 5, voice channels 10 and 11 and
// text channel 12. Guild 200 (voice channel 20) has members 4 and 6.
func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := t.Context()
	h := &harness{
		store: fakes.NewStore(), media: &fakes.Media{}, clock: fakes.NewClock(), limiter: &fakes.Limiter{},
		settings: &settings{st: domain.DefaultInstanceSettings()},
	}
	h.events = fakes.NewEvents(h.store)
	h.svc = New(Deps{
		Guilds: h.store, Members: h.store, Channels: h.store, Users: h.store, Audit: h.store, Tx: h.store,
		IDs: &fakes.IDs{}, Tokens: &fakes.Tokens{}, Clock: h.clock, Limiter: h.limiter, Events: h.events,
		Bus: events.NewBus(), Media: h.media,
		ICE:      fakes.ICE{Servers: []ports.IceServer{{URLs: []string{"turn:turn.example:3478"}, Username: "u", Credential: "c"}}},
		Settings: h.settings, Presence: &fakes.Presence{},
		Verification: fakes.Verification(false), Logger: slog.New(slog.DiscardHandler),
	})
	for _, g := range []domain.Guild{{ID: guildID, Name: "Team", OwnerID: 1}, {ID: otherID, Name: "Other", OwnerID: 6}} {
		if err := h.store.CreateGuild(ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	for id := domain.ID(1); id <= 6; id++ {
		if err := h.store.CreateUser(ctx, domain.User{ID: id, Username: "u" + id.String(), Email: id.String() + "@x.io"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range []domain.Member{
		{GuildID: guildID, UserID: 1, Role: domain.RoleOwner},
		{GuildID: guildID, UserID: 2, Role: domain.RoleAdmin},
		{GuildID: guildID, UserID: 4, Role: domain.RoleMember},
		{GuildID: guildID, UserID: 5, Role: domain.RoleMember},
		{GuildID: otherID, UserID: 6, Role: domain.RoleOwner},
		{GuildID: otherID, UserID: 4, Role: domain.RoleMember},
	} {
		if err := h.store.AddMember(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []domain.Channel{
		{ID: lobby, GuildID: guildID, Type: domain.ChannelVoice, Name: "Lobby"},
		{ID: focus, GuildID: guildID, Type: domain.ChannelVoice, Name: "Focus", UserLimit: 1},
		{ID: text, GuildID: guildID, Type: domain.ChannelText, Name: "general"},
		{ID: elsewhere, GuildID: otherID, Type: domain.ChannelVoice, Name: "Lounge"},
	} {
		if err := h.store.CreateChannel(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

func requireReason(t *testing.T, err, kind error, reason string) {
	t.Helper()
	de, ok := domain.AsError(err)
	if !errors.Is(err, kind) || !ok || de.Reason != reason {
		t.Fatalf("err = %v, want %v (%s)", err, kind, reason)
	}
}

// voiceEvents returns the voice payloads published since the last call.
func (h *harness) voiceEvents() []events.Payload {
	var out []events.Payload
	for _, e := range h.events.Take() {
		switch e.Payload.(type) {
		case events.VoiceStateUpdated, events.VoiceForced:
			out = append(out, e.Payload)
		}
	}
	return out
}

func (h *harness) join(t *testing.T, actor domain.Actor, channelID domain.ID) Join {
	t.Helper()
	j, err := h.svc.Join(t.Context(), actor, channelID, false, false)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func (h *harness) joined(actor domain.Actor, channelID domain.ID, session string, sources ...domain.MediaSource) {
	h.svc.HandleMediaEvent(context.Background(), ports.MediaEvent{
		Kind: ports.MediaParticipantJoined, Room: domain.VoiceRoom(channelID),
		Participant: ports.MediaParticipant{
			SID: "sid-" + session, Identity: actor.UserID.String(), Metadata: session, Active: true, Sources: sources,
		},
	})
}

func (h *harness) state(userID domain.ID) (domain.VoiceState, bool) {
	h.svc.mu.Lock()
	defer h.svc.mu.Unlock()
	e, ok := h.svc.states[userID]
	if !ok {
		return domain.VoiceState{}, false
	}
	return e.state, true
}

func TestJoin(t *testing.T) {
	h := newHarness(t)
	j, err := h.svc.Join(t.Context(), member, lobby, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "vc_10|4|token-1|microphone,screen_share,screen_share_audio|true|10m0s"; j.Token != want {
		t.Errorf("token = %q, want %q", j.Token, want)
	}
	if j.URL != "/lk" || len(j.ICE) != 1 || j.MicBitrateMaxKbps != 96 {
		t.Errorf("join = %+v", j)
	}
	want := domain.VoiceState{
		UserID: 4, GuildID: guildID, ChannelID: lobby, SessionID: "token-1",
		SelfMute: true, SelfDeaf: true, JoinedAt: h.clock.Now(),
	}
	if diff := cmp.Diff(want, j.State); diff != "" {
		t.Errorf("state (-want +got):\n%s", diff)
	}
	evs := h.events.Take()
	if len(evs) != 1 || !cmp.Equal(evs[0].Topics, []events.Topic{"guild:100", "user:4"}) ||
		!cmp.Equal(evs[0].Payload, events.VoiceStateUpdated{State: want}) {
		t.Errorf("events = %+v", evs)
	}
	if got := h.svc.GuildVoiceStates(guildID); !cmp.Equal(got, []domain.VoiceState{want}) {
		t.Errorf("guild voice states = %+v", got)
	}

	h.joined(member, lobby, "token-1")
	if st, _ := h.state(4); !st.Connected {
		t.Error("participant_joined did not confirm the join")
	}
}

func TestJoinRefusals(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	if _, err := h.svc.Join(ctx, member, text, false, false); err == nil {
		t.Fatal("joined a text channel")
	} else {
		requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonNotVoiceChannel)
	}
	if _, err := h.svc.Join(ctx, domain.Actor{UserID: 6}, lobby, false, false); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("non-member join: %v", err)
	}
	h.media.Down = true
	_, err := h.svc.Join(ctx, member, lobby, false, false)
	requireReason(t, err, domain.ErrUnavailable, domain.ReasonVoiceUnavailable)
	h.media.Down = false
	h.limiter.Deny = map[string]time.Duration{"voice-join:user:4": time.Second}
	if _, err := h.svc.Join(ctx, member, lobby, false, false); !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("rate limit: %v", err)
	}
	if len(h.voiceEvents()) != 0 {
		t.Error("a refused join published events")
	}
}

func TestJoinUserLimit(t *testing.T) {
	h := newHarness(t)
	h.join(t, member, focus)
	_, err := h.svc.Join(t.Context(), other, focus, false, false)
	requireReason(t, err, domain.ErrFailedPrecondition, domain.ReasonChannelFull)
	// Rejoining does not count yourself, and moderators may always join.
	h.join(t, member, focus)
	h.join(t, admin, focus)
}

func TestSwitchChannels(t *testing.T) {
	h := newHarness(t)
	h.join(t, member, lobby)
	h.media.TakeCalls()
	h.events.Take()

	// Another channel of the same guild: the old room is left.
	h.join(t, member, focus)
	if calls := h.media.TakeCalls(); !cmp.Equal(calls, []fakes.MediaCall{{Op: "remove", Room: "vc_10", Identity: "4"}}) {
		t.Errorf("calls = %+v", calls)
	}
	h.events.Take()
	// Another guild: the first guild hears that the user left.
	h.join(t, member, elsewhere)
	evs := h.voiceEvents()
	if len(evs) != 2 {
		t.Fatalf("events = %+v", evs)
	}
	if left := evs[0].(events.VoiceStateUpdated).State; left.GuildID != guildID || left.ChannelID != 0 {
		t.Errorf("first event = %+v, want the user leaving guild 100", left)
	}
	// The same channel again (a second device): the media server replaces
	// the participant itself, so nothing is removed.
	h.media.TakeCalls()
	j := h.join(t, member, elsewhere)
	if calls := h.media.TakeCalls(); len(calls) != 0 {
		t.Errorf("calls = %+v", calls)
	}
	if j.State.SessionID != "token-4" {
		t.Errorf("session = %s, want a new one", j.State.SessionID)
	}
}

func TestConfirmAdoptAndLeave(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	h.join(t, member, lobby)

	// An older session coming back is removed; the state is untouched.
	h.joined(member, lobby, "stale")
	if calls := h.media.TakeCalls(); !cmp.Equal(calls, []fakes.MediaCall{{Op: "remove", Room: "vc_10", Identity: "4"}}) {
		t.Errorf("stale participant: calls = %+v", calls)
	}
	if st, _ := h.state(4); st.Connected || st.SessionID != "token-1" {
		t.Errorf("state = %+v", st)
	}

	// Someone the hub does not know: a member is adopted, others removed.
	h.joined(other, lobby, "from-before-restart", domain.SourceScreenShare)
	if st, ok := h.state(5); !ok || !st.Connected || !st.Streaming || st.SessionID != "from-before-restart" {
		t.Errorf("adopted = %+v %v", st, ok)
	}
	h.joined(domain.Actor{UserID: 6}, lobby, "x")
	if _, ok := h.state(6); ok {
		t.Error("adopted a non-member")
	}
	if calls := h.media.TakeCalls(); !cmp.Equal(calls, []fakes.MediaCall{{Op: "remove", Room: "vc_10", Identity: "6"}}) {
		t.Errorf("non-member: calls = %+v", calls)
	}

	// participant_left for another session changes nothing; for this one it ends the state.
	left := func(session string) {
		h.svc.HandleMediaEvent(ctx, ports.MediaEvent{
			Kind: ports.MediaParticipantLeft, Room: "vc_10",
			Participant: ports.MediaParticipant{Identity: "5", Metadata: session},
		})
	}
	left("other")
	if _, ok := h.state(5); !ok {
		t.Fatal("left with another session ended the state")
	}
	left("from-before-restart")
	if _, ok := h.state(5); ok {
		t.Fatal("still in voice after participant_left")
	}

	// Leave from a closed tab whose call moved elsewhere is ignored.
	if err := h.svc.Leave(ctx, member, "old-tab"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.state(4); !ok {
		t.Fatal("a stale leave ended the call")
	}
	h.media.TakeCalls()
	if err := h.svc.Leave(ctx, member, "token-1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.state(4); ok {
		t.Fatal("still in voice after leaving")
	}
	if calls := h.media.TakeCalls(); !cmp.Equal(calls, []fakes.MediaCall{{Op: "remove", Room: "vc_10", Identity: "4"}}) {
		t.Errorf("leave: calls = %+v", calls)
	}
}

func TestUpdateSelf(t *testing.T) {
	h := newHarness(t)
	h.join(t, member, lobby)
	h.events.Take()
	h.svc.UpdateSelf(t.Context(), 4, "token-1", false, true)
	if st, _ := h.state(4); !st.SelfMute || !st.SelfDeaf {
		t.Errorf("deafen without mute: %+v", st)
	}
	h.svc.UpdateSelf(t.Context(), 4, "token-1", true, true)
	h.svc.UpdateSelf(t.Context(), 4, "someone-else", false, false)
	if evs := h.voiceEvents(); len(evs) != 1 {
		t.Errorf("events = %+v, want one change", evs)
	}
}

func TestServerMuteAndDeafen(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	h.join(t, member, lobby)
	h.joined(member, lobby, "token-1")
	h.events.Take()
	h.media.TakeCalls()

	m, err := h.svc.SetServerMute(ctx, admin, guildID, 4, true)
	if err != nil || !m.ServerMute {
		t.Fatalf("mute: %+v %v", m, err)
	}
	if stored, _ := h.store.MemberByID(ctx, guildID, 4); !stored.ServerMute {
		t.Error("server mute not stored")
	}
	if st, _ := h.state(4); !st.ServerMute {
		t.Error("voice state not updated")
	}
	want := []fakes.MediaCall{
		{Op: "permissions", Room: "vc_10", Identity: "4", Permissions: domain.VoicePermissions{
			Publish: []domain.MediaSource{domain.SourceScreenShare, domain.SourceScreenShareAudio}, Subscribe: true,
		}},
	}
	if diff := cmp.Diff(want, h.media.TakeCalls()); diff != "" {
		t.Errorf("calls (-want +got):\n%s", diff)
	}
	var kinds []string
	for _, e := range h.events.Take() {
		switch p := e.Payload.(type) {
		case events.MemberUpdated:
			kinds = append(kinds, "member")
		case events.AuditAppended:
			if p.Entry.Action != domain.AuditMemberVoiceUpdate || p.Entry.Data["server_mute"] != "true" {
				t.Errorf("audit = %+v", p.Entry)
			}
			kinds = append(kinds, "audit")
		case events.VoiceStateUpdated:
			kinds = append(kinds, "voice")
		}
	}
	if !cmp.Equal(kinds, []string{"audit", "member", "voice"}) {
		t.Errorf("events = %v", kinds)
	}

	// Leaving and joining again does not lift it: the new token has no microphone.
	j := h.join(t, member, lobby)
	if !strings.Contains(j.Token, "|screen_share,screen_share_audio|") || !j.State.ServerMute {
		t.Errorf("token after rejoin = %s, state %+v", j.Token, j.State)
	}
	// A connection that comes up after the mute is enforced again.
	h.media.TakeCalls()
	h.joined(member, lobby, j.State.SessionID)
	if calls := h.media.TakeCalls(); len(calls) != 1 || calls[0].Op != "permissions" {
		t.Errorf("confirm: calls = %+v", calls)
	}

	m, err = h.svc.SetServerDeafen(ctx, owner, guildID, 4, true)
	if err != nil || !m.ServerDeaf || !m.ServerMute {
		t.Fatalf("deafen: %+v %v", m, err)
	}
	if calls := h.media.TakeCalls(); len(calls) != 1 || calls[0].Permissions.Subscribe {
		t.Errorf("deafen: calls = %+v", calls)
	}
	// No change, no audit entry.
	h.events.Take()
	if _, err := h.svc.SetServerDeafen(ctx, owner, guildID, 4, true); err != nil || len(h.events.Take()) != 0 {
		t.Errorf("repeat deafen: %v", err)
	}

	if _, err := h.svc.SetServerMute(ctx, member, guildID, 5, true); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("member muting: %v", err)
	}
	if _, err := h.svc.SetServerMute(ctx, admin, guildID, 1, true); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("muting the owner: %v", err)
	}
}

func TestMuteEnforcedOnMicPublish(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	if _, err := h.svc.SetServerMute(ctx, admin, guildID, 4, true); err != nil {
		t.Fatal(err)
	}
	h.join(t, member, lobby)
	h.joined(member, lobby, "token-1")
	h.media.TakeCalls()
	h.svc.HandleMediaEvent(ctx, ports.MediaEvent{
		Kind: ports.MediaTrackPublished, Room: "vc_10", Source: domain.SourceMicrophone,
		Participant: ports.MediaParticipant{SID: "sid-token-1", Identity: "4"},
	})
	if calls := h.media.TakeCalls(); len(calls) != 1 || calls[0].Op != "permissions" {
		t.Errorf("calls = %+v", calls)
	}
}

func TestStreaming(t *testing.T) {
	h := newHarness(t)
	h.join(t, member, lobby)
	h.joined(member, lobby, "token-1")
	track := func(kind ports.MediaEventKind, sid string) {
		h.svc.HandleMediaEvent(t.Context(), ports.MediaEvent{
			Kind: kind, Room: "vc_10", Source: domain.SourceScreenShare,
			Participant: ports.MediaParticipant{SID: sid, Identity: "4"},
		})
	}
	track(ports.MediaTrackPublished, "sid-other")
	if st, _ := h.state(4); st.Streaming {
		t.Error("a track of another connection counted")
	}
	track(ports.MediaTrackPublished, "sid-token-1")
	if st, _ := h.state(4); !st.Streaming {
		t.Error("not streaming after the share was published")
	}
	track(ports.MediaTrackUnpublished, "sid-token-1")
	if st, _ := h.state(4); st.Streaming {
		t.Error("still streaming after the share was unpublished")
	}
}

func TestShareLimit(t *testing.T) {
	h := newHarness(t)
	h.settings.st.Limits.MaxSharesPerChannel = 2
	share := func(actor domain.Actor, session string, kind ports.MediaEventKind) {
		h.svc.HandleMediaEvent(t.Context(), ports.MediaEvent{
			Kind: kind, Room: "vc_10", Source: domain.SourceScreenShare,
			Participant: ports.MediaParticipant{SID: "sid-" + session, Identity: actor.UserID.String()},
		})
	}
	sessions := map[domain.ID]string{}
	for _, a := range []domain.Actor{owner, admin, member, other} {
		sessions[a.UserID] = h.join(t, a, lobby).State.SessionID
		h.joined(a, lobby, sessions[a.UserID])
	}
	if _, err := h.svc.SetServerMute(t.Context(), admin, guildID, 5, true); err != nil {
		t.Fatal(err)
	}
	share(owner, sessions[1], ports.MediaTrackPublished)
	share(admin, sessions[2], ports.MediaTrackPublished)
	h.media.TakeCalls()

	share(member, sessions[4], ports.MediaTrackPublished)
	if st, _ := h.state(4); st.Streaming {
		t.Error("a third share counted in a channel that allows two")
	}
	allowed := domain.VoicePermissionsFor(false, false)
	want := []fakes.MediaCall{
		{Op: "permissions", Room: "vc_10", Identity: "4", Permissions: domain.VoicePermissions{
			Publish: []domain.MediaSource{domain.SourceMicrophone}, Subscribe: true,
		}},
		{Op: "permissions", Room: "vc_10", Identity: "4", Permissions: allowed},
	}
	if diff := cmp.Diff(want, h.media.TakeCalls()); diff != "" {
		t.Errorf("calls (-want +got):\n%s", diff)
	}

	// Stopping the share leaves the server mute of the one stopped in place.
	share(other, sessions[5], ports.MediaTrackPublished)
	calls := h.media.TakeCalls()
	if len(calls) != 2 || slices.Contains(calls[1].Permissions.Publish, domain.SourceMicrophone) {
		t.Errorf("calls for a server-muted sharer = %+v", calls)
	}

	share(owner, sessions[1], ports.MediaTrackUnpublished)
	share(member, sessions[4], ports.MediaTrackPublished)
	if st, _ := h.state(4); !st.Streaming {
		t.Error("a share was refused although another one ended")
	}
	if calls := h.media.TakeCalls(); len(calls) != 0 {
		t.Errorf("a share within the limit was stopped: %+v", calls)
	}
}

func TestMoveMember(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	if err := h.svc.MoveMember(ctx, admin, guildID, 4, focus); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("moving someone not in voice: %v", err)
	}
	h.join(t, other, focus)
	h.join(t, member, lobby)
	h.joined(member, lobby, "token-2")
	h.events.Take()
	h.media.TakeCalls()

	if err := h.svc.MoveMember(ctx, admin, guildID, 4, focus); err != nil {
		t.Fatal(err)
	}
	evs := h.voiceEvents()
	if len(evs) != 2 {
		t.Fatalf("events = %+v", evs)
	}
	forced, ok := evs[0].(events.VoiceForced)
	if !ok || forced.Action != events.VoiceMoved || forced.ChannelID != focus || forced.ByUserID != 2 || forced.SessionID != "token-2" {
		t.Errorf("forced = %+v", evs[0])
	}
	if st := evs[1].(events.VoiceStateUpdated).State; st.ChannelID != focus || st.Connected || st.SessionID != "token-2" {
		t.Errorf("state = %+v", st)
	}
	if calls := h.media.TakeCalls(); !cmp.Equal(calls, []fakes.MediaCall{{Op: "remove", Room: "vc_10", Identity: "4"}}) {
		t.Errorf("calls = %+v", calls)
	}
	// The channel is full, but the moved member's own join goes through.
	j := h.join(t, member, focus)
	h.joined(member, focus, j.State.SessionID)
	if st, _ := h.state(4); !st.Connected || st.ChannelID != focus {
		t.Errorf("after the move = %+v", st)
	}

	if err := h.svc.MoveMember(ctx, admin, guildID, 4, text); err == nil {
		t.Error("moved into a text channel")
	}
	if err := h.svc.MoveMember(ctx, admin, guildID, 4, elsewhere); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("moved into another guild: %v", err)
	}
	if err := h.svc.MoveMember(ctx, member, guildID, 5, lobby); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("member moving: %v", err)
	}
}

func TestDisconnectMember(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	h.join(t, member, lobby)
	h.events.Take()
	h.media.TakeCalls()
	if err := h.svc.DisconnectMember(ctx, owner, guildID, 4); err != nil {
		t.Fatal(err)
	}
	evs := h.voiceEvents()
	if len(evs) != 2 || evs[0].(events.VoiceForced).Action != events.VoiceDisconnected ||
		evs[1].(events.VoiceStateUpdated).State.ChannelID != 0 {
		t.Errorf("events = %+v", evs)
	}
	if calls := h.media.TakeCalls(); len(calls) != 1 || calls[0].Op != "remove" {
		t.Errorf("calls = %+v", calls)
	}
	if err := h.svc.DisconnectMember(ctx, owner, guildID, 4); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("disconnecting again: %v", err)
	}
}

func TestSweepAndMediaRestart(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	h.join(t, member, lobby)
	h.join(t, other, lobby)
	h.joined(other, lobby, "token-2")

	h.clock.Advance(ConfirmTimeout)
	h.svc.Sweep(ctx)
	if _, ok := h.state(4); ok {
		t.Error("an unconfirmed join survived the timeout")
	}
	if _, ok := h.state(5); !ok {
		t.Fatal("a confirmed connection was swept")
	}

	h.svc.MediaRestarted(ctx)
	if st, _ := h.state(5); st.Connected {
		t.Error("still connected after the media server restarted")
	}
	h.clock.Advance(ConfirmTimeout / 2)
	h.joined(other, lobby, "token-2")
	h.clock.Advance(ConfirmTimeout)
	h.svc.Sweep(ctx)
	if st, ok := h.state(5); !ok || !st.Connected {
		t.Errorf("reconnected participant = %+v %v", st, ok)
	}
}

func TestReconcile(t *testing.T) {
	h := newHarness(t)
	h.media.Rooms = map[string][]ports.MediaParticipant{
		"vc_10":     {{Identity: "4", Metadata: "s4", Active: true}, {Identity: "5", Metadata: "s5"}},
		"vc_20":     {{Identity: "6", Metadata: "s6", Active: true}},
		"not-voice": {{Identity: "1", Metadata: "s1", Active: true}},
	}
	if err := h.svc.Reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := len(h.svc.GuildVoiceStates(guildID)); got != 1 {
		t.Errorf("guild 100 has %d voice states, want 1 (the connecting participant waits for its webhook)", got)
	}
	if got := h.svc.GuildVoiceStates(otherID); len(got) != 1 || got[0].SessionID != "s6" || !got[0].Connected {
		t.Errorf("guild 200 = %+v", got)
	}
}

func TestCleanup(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	h.join(t, member, lobby)
	h.join(t, other, focus)
	h.join(t, admin, lobby)
	h.join(t, domain.Actor{UserID: 6}, elsewhere)
	h.media.TakeCalls()

	h.svc.cleanup(ctx, events.MemberRemoved{GuildID: guildID, UserID: 4})
	h.svc.cleanup(ctx, events.ChannelDeleted{GuildID: guildID, ChannelID: focus})
	if _, ok := h.state(4); ok {
		t.Error("a removed member is still in voice")
	}
	if _, ok := h.state(5); ok {
		t.Error("still in a deleted channel")
	}
	h.svc.cleanup(ctx, events.GuildDeleted{GuildID: guildID})
	if got := h.svc.GuildVoiceStates(guildID); len(got) != 0 {
		t.Errorf("deleted guild still has %+v", got)
	}
	if _, ok := h.state(6); !ok {
		t.Error("another guild's voice was ended")
	}
	if calls := h.media.TakeCalls(); len(calls) != 3 {
		t.Errorf("calls = %+v", calls)
	}
}

func TestRunCleansUpFromEvents(t *testing.T) {
	h := newHarness(t)
	bus := events.NewBus()
	h.svc.d.Bus = bus
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		h.svc.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()
	h.join(t, member, lobby)
	// Run subscribes asynchronously; publish until the event is handled.
	deadline := time.Now().Add(2 * time.Second)
	for {
		bus.Publish(events.Event{Payload: events.MemberRemoved{GuildID: guildID, UserID: 4}})
		if _, ok := h.state(4); !ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("member removal did not end the voice state")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
