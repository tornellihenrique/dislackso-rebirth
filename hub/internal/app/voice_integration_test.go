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
	eventually(t, 30*time.Second, "the media server", r.app.media.ready)
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
func (v *voiceRig) connect(ctx context.Context, t *testing.T, channelID string, cb *lksdk.RoomCallback) (*dislacksov1.JoinResponse, *lksdk.Room) {
	t.Helper()
	join, err := v.member.voice.Join(ctx, &dislacksov1.JoinRequest{ChannelId: channelID})
	if err != nil {
		t.Fatal(err)
	}
	if cb == nil {
		cb = &lksdk.RoomCallback{}
	}
	room, err := lksdk.ConnectToRoomWithToken(v.signalURL(join), join.GetToken(), cb, lksdk.WithAutoSubscribe(false))
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

func TestVoiceWithLiveKit(t *testing.T) {
	v := newVoiceRig(t)
	ctx := t.Context()

	join, room := v.connect(ctx, t, v.lobby, nil)
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
		join2, _ := v.connect(ctx, t, v.lobby, nil)
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
		join3, _ := v.connect(ctx, t, v.focus, nil)
		eventually(t, 10*time.Second, "the switch", v.connected(join3.GetState().GetVoiceSessionId()))
		t.Logf("switched channels in %s", time.Since(start))
		if time.Since(start) > 3*time.Second {
			t.Errorf("switch took %s", time.Since(start))
		}
		eventually(t, 5*time.Second, "leaving the old room", func() bool { return v.sources(ctx, t, v.lobby) == nil })
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
