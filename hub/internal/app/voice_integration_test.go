//go:build integration

package app

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/clock"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/ice"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/config"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/voice"
)

// A participant process for the crash test: the test binary connects with
// the token in HELPER_LK_TOKEN and stays until killed.
func TestMain(m *testing.M) {
	if token := os.Getenv("HELPER_LK_TOKEN"); token != "" {
		if _, err := lksdk.ConnectToRoomWithToken(os.Getenv("HELPER_LK_URL"), token, &lksdk.RoomCallback{}); err != nil {
			os.Exit(2)
		}
		select {}
	}
	os.Exit(m.Run())
}

func liveKitBinary(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("DSX_LIVEKIT_BINARY"); p != "" {
		return p
	}
	name := "livekit-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	p, err := filepath.Abs(filepath.Join("..", "..", "..", ".tools", "bin", name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("livekit-server not found at %s (run task tools, or set DSX_LIVEKIT_BINARY)", p)
	}
	return p
}

// freePort finds a port free for both TCP and UDP.
func freePort(t *testing.T) int {
	t.Helper()
	for range 20 {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		pc, err := net.ListenPacket("udp", net.JoinHostPort("", strconv.Itoa(port)))
		if err != nil {
			continue
		}
		_ = pc.Close()
		return port
	}
	t.Fatal("no free port")
	return 0
}

func eventually(t *testing.T, within time.Duration, what string, cond func() bool) time.Duration {
	t.Helper()
	start := time.Now()
	for !cond() {
		if time.Since(start) > within {
			t.Fatalf("timed out after %s waiting for %s", within, what)
		}
		time.Sleep(25 * time.Millisecond)
	}
	return time.Since(start)
}

type voiceRig struct {
	r       *running
	guildID domain.ID
	lobby   string
	focus   string
	admin   *browser
	member  *browser
	userID  domain.ID
	// rooms stay connected across subtests and close when the test ends.
	rooms []*lksdk.Room
}

func newVoiceRig(t *testing.T) *voiceRig {
	t.Helper()
	bin := liveKitBinary(t)
	ports := []int{freePort(t), freePort(t), freePort(t)}
	r := start(t, func(c *config.Config) {
		c.LiveKit = config.LiveKit{Binary: bin, Port: ports[0], UDPPort: ports[1], TCPPort: ports[2]}
	})
	eventually(t, 30*time.Second, "the media server", r.app.media.Ready)
	ctx := t.Context()
	admin := r.browser(t)
	completeSetup(t, admin)
	created, err := admin.guilds.CreateGuild(ctx, &dislacksov1.CreateGuildRequest{Name: "Team"})
	if err != nil {
		t.Fatal(err)
	}
	guildID := created.GetState().GetGuild().GetId()
	focus, err := admin.channels.CreateChannel(ctx, &dislacksov1.CreateChannelRequest{
		GuildId: guildID, Type: dislacksov1.ChannelType_CHANNEL_TYPE_VOICE, Name: "Focus",
	})
	if err != nil {
		t.Fatal(err)
	}
	inv, err := admin.invites.CreateInvite(ctx, &dislacksov1.CreateInviteRequest{GuildId: guildID})
	if err != nil {
		t.Fatal(err)
	}
	member := registerMember(t, r, inv.GetInvite().GetCode(), "bruno")
	gid, _ := domain.ParseID(guildID, "guild")
	uid, _ := domain.ParseID(idOf(t, member), "user")
	v := &voiceRig{
		r: r, guildID: gid, lobby: voiceChannel(t, created.GetState()), focus: focus.GetChannel().GetId(),
		admin: admin, member: member, userID: uid,
	}
	t.Cleanup(func() {
		for _, room := range v.rooms {
			room.Disconnect()
		}
	})
	return v
}

func (v *voiceRig) state() (domain.VoiceState, bool) {
	for _, s := range v.r.app.voice.GuildVoiceStates(v.guildID) {
		if s.UserID == v.userID {
			return s, true
		}
	}
	return domain.VoiceState{}, false
}

func (v *voiceRig) signalURL(join *dislacksov1.JoinResponse) string {
	return "ws" + strings.TrimPrefix(v.r.base, "http") + join.GetUrl()
}

// connect joins channelID as the member and connects through the hub's proxy.
func (v *voiceRig) connect(ctx context.Context, t *testing.T, channelID string) (*dislacksov1.JoinResponse, *lksdk.Room) {
	t.Helper()
	join, err := v.member.voice.Join(ctx, &dislacksov1.JoinRequest{ChannelId: channelID})
	if err != nil {
		t.Fatal(err)
	}
	room, err := lksdk.ConnectToRoomWithToken(v.signalURL(join), join.GetToken(), &lksdk.RoomCallback{}, lksdk.WithAutoSubscribe(false))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	v.rooms = append(v.rooms, room)
	return join, room
}

func (v *voiceRig) connected(session string) func() bool {
	return func() bool {
		s, ok := v.state()
		return ok && s.Connected && s.SessionID == session
	}
}

func (v *voiceRig) sources(ctx context.Context, t *testing.T, channelID string) []domain.MediaSource {
	t.Helper()
	cid, _ := domain.ParseID(channelID, "channel")
	ps, err := v.r.app.media.provider.ListParticipants(ctx, domain.VoiceRoom(cid))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		if p.Identity == v.userID.String() {
			return p.Sources
		}
	}
	return nil
}

// publishMic publishes an Opus track and keeps it fed with silence.
func publishMic(ctx context.Context, room *lksdk.Room) error {
	track, err := lksdk.NewLocalSampleTrack(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2})
	if err != nil {
		return err
	}
	go func() {
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				_ = track.WriteSample(media.Sample{Data: []byte{0xf8, 0xff, 0xfe}, Duration: 20 * time.Millisecond}, nil)
			}
		}
	}()
	_, err = room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{Name: "mic", Source: livekit.TrackSource_MICROPHONE})
	return err
}

// publishScreen publishes a screen share track kept fed with frames; the
// media server forwards them without decoding.
func publishScreen(ctx context.Context, room *lksdk.Room) (*lksdk.LocalTrackPublication, error) {
	track, err := lksdk.NewLocalSampleTrack(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000})
	if err != nil {
		return nil, err
	}
	go func() {
		tick := time.NewTicker(33 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				_ = track.WriteSample(media.Sample{Data: []byte{0x10, 0x02, 0x00, 0x9d, 0x01, 0x2a}, Duration: 33 * time.Millisecond}, nil)
			}
		}
	}()
	return room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{
		Name: "screen", Source: livekit.TrackSource_SCREEN_SHARE, VideoWidth: 1920, VideoHeight: 1080,
	})
}

func (v *voiceRig) limitShares(ctx context.Context, t *testing.T, n int) {
	t.Helper()
	st, err := v.r.app.settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	st.Limits.MaxSharesPerChannel = n
	if err := v.r.app.db.SaveSettings(ctx, st); err != nil {
		t.Fatal(err)
	}
	v.r.app.settings.Invalidate()
}

func (v *voiceRig) streaming(userID domain.ID) func() bool {
	return func() bool {
		for _, s := range v.r.app.voice.GuildVoiceStates(v.guildID) {
			if s.UserID == userID {
				return s.Streaming
			}
		}
		return false
	}
}

func TestVoiceWithLiveKit(t *testing.T) {
	v := newVoiceRig(t)
	ctx := t.Context()

	join, room := v.connect(ctx, t, v.lobby)
	took := eventually(t, 10*time.Second, "the join to be confirmed", v.connected(join.GetState().GetVoiceSessionId()))
	t.Logf("join confirmed by webhook after %s", took)

	t.Run("server mute cannot be bypassed", func(t *testing.T) {
		if err := publishMic(ctx, room); err != nil {
			t.Fatal(err)
		}
		eventually(t, 10*time.Second, "the microphone track", func() bool {
			return slices.Contains(v.sources(ctx, t, v.lobby), domain.SourceMicrophone)
		})
		gid := v.guildID.String()
		if _, err := v.admin.voice.SetServerMute(ctx, &dislacksov1.SetServerMuteRequest{GuildId: gid, UserId: v.userID.String(), Mute: true}); err != nil {
			t.Fatal(err)
		}
		eventually(t, 5*time.Second, "the microphone to be removed", func() bool {
			return !slices.Contains(v.sources(ctx, t, v.lobby), domain.SourceMicrophone)
		})
		// Publishing again is refused by the media server.
		pubCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		if err := publishMic(pubCtx, room); err == nil {
			t.Error("published a microphone while server muted")
		}
		if slices.Contains(v.sources(ctx, t, v.lobby), domain.SourceMicrophone) {
			t.Error("a microphone track got through")
		}
		if perms := room.LocalParticipant.Permissions(); slices.Contains(perms.GetCanPublishSources(), livekit.TrackSource_MICROPHONE) {
			t.Errorf("client permissions = %v", perms)
		}
		if _, err := v.admin.voice.SetServerMute(ctx, &dislacksov1.SetServerMuteRequest{GuildId: gid, UserId: v.userID.String(), Mute: false}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("second device takes over", func(t *testing.T) {
		join2, _ := v.connect(ctx, t, v.lobby)
		eventually(t, 10*time.Second, "the second device", v.connected(join2.GetState().GetVoiceSessionId()))
		eventually(t, 10*time.Second, "the first device to be dropped", func() bool {
			return room.ConnectionState() == lksdk.ConnectionStateDisconnected
		})
		if r := room.DisconnectReason(); r != livekit.DisconnectReason_DUPLICATE_IDENTITY {
			t.Errorf("first device disconnect reason = %v", r)
		}
		// The first device's departure does not end the second one's state.
		time.Sleep(time.Second)
		if s, ok := v.state(); !ok || s.SessionID != join2.GetState().GetVoiceSessionId() {
			t.Errorf("state after takeover = %+v %v", s, ok)
		}
	})

	t.Run("switching channels", func(t *testing.T) {
		start := time.Now()
		join3, _ := v.connect(ctx, t, v.focus)
		eventually(t, 10*time.Second, "the switch", v.connected(join3.GetState().GetVoiceSessionId()))
		t.Logf("switched channels in %s", time.Since(start))
		if time.Since(start) > 3*time.Second {
			t.Errorf("switch took %s", time.Since(start))
		}
		eventually(t, 5*time.Second, "leaving the old room", func() bool { return v.sources(ctx, t, v.lobby) == nil })
	})

	t.Run("screen shares count against the channel's limit", func(t *testing.T) {
		v.limitShares(ctx, t, 1)
		adminJoin, err := v.admin.voice.Join(ctx, &dislacksov1.JoinRequest{ChannelId: v.focus})
		if err != nil {
			t.Fatal(err)
		}
		adminID, _ := domain.ParseID(adminJoin.GetState().GetUserId(), "user")
		adminRoom, err := lksdk.ConnectToRoomWithToken(v.signalURL(adminJoin), adminJoin.GetToken(), &lksdk.RoomCallback{}, lksdk.WithAutoSubscribe(false))
		if err != nil {
			t.Fatal(err)
		}
		v.rooms = append(v.rooms, adminRoom)
		shareCtx, stopFeeding := context.WithCancel(ctx)
		defer stopFeeding()
		adminShare, err := publishScreen(shareCtx, adminRoom)
		if err != nil {
			t.Fatal(err)
		}
		eventually(t, 10*time.Second, "the first share to set the streaming flag", v.streaming(adminID))

		join, room := v.connect(ctx, t, v.focus)
		eventually(t, 10*time.Second, "the member to connect", v.connected(join.GetState().GetVoiceSessionId()))
		if _, err := publishScreen(shareCtx, room); err != nil {
			t.Fatal(err)
		}
		eventually(t, 5*time.Second, "the share over the limit to be removed", func() bool {
			return !slices.Contains(v.sources(ctx, t, v.focus), domain.SourceScreenShare)
		})
		if v.streaming(v.userID)() {
			t.Error("the share over the limit set the streaming flag")
		}
		eventually(t, 5*time.Second, "the permission to share to come back", func() bool {
			return slices.Contains(room.LocalParticipant.Permissions().GetCanPublishSources(), livekit.TrackSource_SCREEN_SHARE)
		})

		if err := adminRoom.LocalParticipant.UnpublishTrack(adminShare.SID()); err != nil {
			t.Fatal(err)
		}
		eventually(t, 10*time.Second, "the first share to end", func() bool { return !v.streaming(adminID)() })
		if _, err := publishScreen(shareCtx, room); err != nil {
			t.Fatal(err)
		}
		eventually(t, 10*time.Second, "a share within the limit", v.streaming(v.userID))
		adminRoom.Disconnect()
		eventually(t, 10*time.Second, "the admin to leave", func() bool {
			return len(v.r.app.voice.GuildVoiceStates(v.guildID)) == 1
		})
	})

	t.Run("a fresh hub rebuilds voice state from the media server", func(t *testing.T) {
		db := v.r.app.db
		fresh := voice.New(voice.Deps{
			Guilds: db, Members: db, Channels: db, Users: db, Audit: db, Tx: db,
			IDs: &fakes.IDs{}, Tokens: &fakes.Tokens{}, Clock: clock.System{}, Limiter: &fakes.Limiter{},
			Events: events.Publisher{Bus: events.NewBus(), Hooks: db}, Bus: events.NewBus(),
			Media: v.r.app.media.provider, ICE: ice.None{}, Settings: v.r.app.settings,
			Presence: &fakes.Presence{}, Verification: fakes.Verification(false), Logger: slog.New(slog.DiscardHandler),
		})
		if err := fresh.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
		got := fresh.GuildVoiceStates(v.guildID)
		cur, _ := v.state()
		if len(got) != 1 || got[0].UserID != v.userID || got[0].SessionID != cur.SessionID || !got[0].Connected {
			t.Errorf("rebuilt = %+v, want %+v", got, cur)
		}
	})

	t.Run("a killed client is cleared", func(t *testing.T) {
		join4, err := v.member.voice.Join(ctx, &dislacksov1.JoinRequest{ChannelId: v.lobby})
		if err != nil {
			t.Fatal(err)
		}
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, exe)
		cmd.Env = append(os.Environ(), "HELPER_LK_URL="+v.signalURL(join4), "HELPER_LK_TOKEN="+join4.GetToken())
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		eventually(t, 15*time.Second, "the helper to connect", v.connected(join4.GetState().GetVoiceSessionId()))
		if err := cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		_ = cmd.Wait()
		took := eventually(t, 45*time.Second, "the state to clear", func() bool { _, ok := v.state(); return !ok })
		t.Logf("killed client cleared after %s", took)
	})
}
