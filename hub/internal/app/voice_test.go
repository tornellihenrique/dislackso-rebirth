package app

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/config"
)

// voiceChannel returns the id of the first voice channel in a guild state.
func voiceChannel(t *testing.T, state *dislacksov1.GuildState) string {
	t.Helper()
	for _, c := range state.GetChannels() {
		if c.GetType() == dislacksov1.ChannelType_CHANNEL_TYPE_VOICE {
			return c.GetId()
		}
	}
	t.Fatal("no voice channel")
	return ""
}

func TestVoiceWithoutMediaServer(t *testing.T) {
	// A configured binary that is missing leaves the hub running without voice.
	r := start(t, func(c *config.Config) {
		// A free port, so no media server running on this machine answers the proxy.
		c.LiveKit = config.LiveKit{Binary: filepath.Join(t.TempDir(), "livekit-server"), Port: freePort(t)}
	})
	ctx := t.Context()
	admin := r.browser(t)
	completeSetup(t, admin)
	created, err := admin.guilds.CreateGuild(ctx, &dislacksov1.CreateGuildRequest{Name: "Team"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = admin.voice.Join(ctx, &dislacksov1.JoinRequest{ChannelId: voiceChannel(t, created.GetState())})
	requireCode(t, err, connect.CodeUnavailable)
	if _, err := admin.voice.Leave(ctx, &dislacksov1.LeaveRequest{}); err != nil {
		t.Errorf("leave without a call: %v", err)
	}
	// A hub configured without voice is still ready.
	if code, _ := admin.get(t, r.base+"/readyz"); code != http.StatusOK {
		t.Errorf("readyz = %d", code)
	}
	if code, _ := admin.get(t, r.base+"/lk/rtc/v1/validate?access_token=x"); code != http.StatusBadGateway {
		t.Errorf("signaling without a media server = %d", code)
	}
	if code, _ := admin.get(t, r.base+"/lk/twirp/livekit.RoomService/ListRooms"); code != http.StatusNotFound {
		t.Errorf("room API through the proxy = %d", code)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, r.base+"/internal/livekit/webhook", strings.NewReader("{}"))
	res, err := admin.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("unsigned webhook = %d", res.StatusCode)
	}
}
