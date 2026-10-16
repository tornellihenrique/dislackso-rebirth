package livekit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/livekit/protocol/auth"
	lkproto "github.com/livekit/protocol/livekit"
	"github.com/twitchtv/twirp"
	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

const (
	key    = "APItest"
	secret = "a-secret-that-is-long-enough-for-livekit-0123"
)

func grants(t *testing.T, token string) *auth.ClaimGrants {
	t.Helper()
	v, err := auth.ParseAPIToken(token)
	if err != nil {
		t.Fatal(err)
	}
	_, g, err := v.Verify(secret)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestJoinToken(t *testing.T) {
	p := New(Config{PublicURL: "/lk", APIKey: key, APISecret: secret})
	token, err := p.JoinToken(ports.MediaJoin{
		Room: "vc_10", Identity: "4", Metadata: "session-1", TTL: 10 * time.Minute,
		Permissions: domain.VoicePermissionsFor(true, false),
	})
	if err != nil {
		t.Fatal(err)
	}
	g := grants(t, token)
	v := g.Video
	if g.Identity != "4" || g.Metadata != "session-1" || !v.RoomJoin || v.Room != "vc_10" {
		t.Errorf("grants = %+v, video %+v", g, v)
	}
	if !v.GetCanPublish() || !v.GetCanSubscribe() || !v.GetCanPublishData() || v.GetCanUpdateOwnMetadata() {
		t.Errorf("video = %+v", v)
	}
	if !cmp.Equal(v.CanPublishSources, []string{"screen_share", "screen_share_audio"}) {
		t.Errorf("sources = %v, want no microphone", v.CanPublishSources)
	}

	deaf, err := p.JoinToken(ports.MediaJoin{Room: "vc_10", Identity: "4", TTL: time.Minute, Permissions: domain.VoicePermissions{}})
	if err != nil {
		t.Fatal(err)
	}
	if v := grants(t, deaf).Video; v.GetCanPublish() || v.GetCanSubscribe() {
		t.Errorf("no permissions = %+v", v)
	}
}

// rooms is a LiveKit room API that records requests and their admin grants.
type rooms struct {
	lkproto.RoomService
	mu       sync.Mutex
	requests []proto.Message
	grants   []*auth.VideoGrant
}

func (r *rooms) record(ctx context.Context, req proto.Message) error {
	header, _ := ctx.Value(authKey{}).(string)
	token := strings.TrimPrefix(header, "Bearer ")
	v, err := auth.ParseAPIToken(token)
	if err != nil {
		return twirp.Unauthenticated.Error("bad token")
	}
	_, g, err := v.Verify(secret)
	if err != nil {
		return twirp.Unauthenticated.Error("bad signature")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
	r.grants = append(r.grants, g.Video)
	return nil
}

func (r *rooms) ListRooms(ctx context.Context, req *lkproto.ListRoomsRequest) (*lkproto.ListRoomsResponse, error) {
	if err := r.record(ctx, req); err != nil {
		return nil, err
	}
	return &lkproto.ListRoomsResponse{Rooms: []*lkproto.Room{{Name: "vc_10"}, {Name: "other"}}}, nil
}

func (r *rooms) ListParticipants(ctx context.Context, req *lkproto.ListParticipantsRequest) (*lkproto.ListParticipantsResponse, error) {
	if err := r.record(ctx, req); err != nil {
		return nil, err
	}
	return &lkproto.ListParticipantsResponse{Participants: []*lkproto.ParticipantInfo{{
		Sid: "PA_1", Identity: "4", Metadata: "s", State: lkproto.ParticipantInfo_ACTIVE,
		Tracks: []*lkproto.TrackInfo{{Source: lkproto.TrackSource_MICROPHONE}, {Source: lkproto.TrackSource_SCREEN_SHARE}},
	}}}, nil
}

func (r *rooms) UpdateParticipant(ctx context.Context, req *lkproto.UpdateParticipantRequest) (*lkproto.ParticipantInfo, error) {
	if err := r.record(ctx, req); err != nil {
		return nil, err
	}
	return &lkproto.ParticipantInfo{}, nil
}

func (r *rooms) RemoveParticipant(ctx context.Context, req *lkproto.RoomParticipantIdentity) (*lkproto.RemoveParticipantResponse, error) {
	if err := r.record(ctx, req); err != nil {
		return nil, err
	}
	if req.GetIdentity() == "gone" {
		return nil, twirp.NotFoundError("participant not found")
	}
	return &lkproto.RemoveParticipantResponse{}, nil
}

type authKey struct{}

// withAuth hands the Authorization header to the fake, as Twirp servers do
// not expose request headers.
func withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authKey{}, r.Header.Get("Authorization"))))
	})
}

func TestRoomAPI(t *testing.T) {
	ctx := t.Context()
	fake := &rooms{}
	srv := httptest.NewServer(withAuth(lkproto.NewRoomServiceServer(fake)))
	defer srv.Close()
	p := New(Config{APIURL: srv.URL, APIKey: key, APISecret: secret, Ready: func() bool { return false }})
	if p.Available() {
		t.Error("available while the server is not ready")
	}

	names, err := p.ListRooms(ctx)
	if err != nil || !cmp.Equal(names, []string{"vc_10", "other"}) {
		t.Fatalf("rooms = %v, %v", names, err)
	}
	ps, err := p.ListParticipants(ctx, "vc_10")
	want := []ports.MediaParticipant{{
		SID: "PA_1", Identity: "4", Metadata: "s", Active: true,
		Sources: []domain.MediaSource{domain.SourceMicrophone, domain.SourceScreenShare},
	}}
	if err != nil || !cmp.Equal(ps, want) {
		t.Fatalf("participants = %+v, %v", ps, err)
	}
	if err := p.SetPermissions(ctx, "vc_10", "4", domain.VoicePermissionsFor(true, true)); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveParticipant(ctx, "vc_10", "4"); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveParticipant(ctx, "vc_10", "gone"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("removing someone absent: %v", err)
	}

	update, ok := fake.requests[2].(*lkproto.UpdateParticipantRequest)
	wantPerm := &lkproto.ParticipantPermission{
		CanSubscribe: false, CanPublish: true, CanPublishData: true,
		CanPublishSources: []lkproto.TrackSource{lkproto.TrackSource_SCREEN_SHARE, lkproto.TrackSource_SCREEN_SHARE_AUDIO},
	}
	if !ok || update.GetRoom() != "vc_10" || update.GetIdentity() != "4" {
		t.Fatalf("update = %v", fake.requests[2])
	}
	if diff := cmp.Diff(wantPerm, update.GetPermission(), protocmp.Transform()); diff != "" {
		t.Errorf("permission (-want +got):\n%s", diff)
	}
	if g := fake.grants[0]; !g.RoomList || g.RoomAdmin {
		t.Errorf("list rooms grant = %+v", g)
	}
	for _, g := range fake.grants[1:] {
		if !g.RoomAdmin || g.Room != "vc_10" {
			t.Errorf("room grant = %+v", g)
		}
	}
}

func signedWebhook(t *testing.T, ev *lkproto.WebhookEvent, signWith string) *http.Request {
	t.Helper()
	body, err := protojson.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	token, err := auth.NewAccessToken(key, signWith).SetValidFor(5 * time.Minute).
		SetSha256(base64.StdEncoding.EncodeToString(sum[:])).ToJWT()
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/internal/livekit/webhook", bytes.NewReader(body))
	r.Header.Set("Authorization", token)
	r.Header.Set("Content-Type", "application/webhook+json")
	return r
}

func TestParseWebhook(t *testing.T) {
	p := New(Config{APIKey: key, APISecret: secret})
	joined := &lkproto.WebhookEvent{
		Event: "participant_joined", Room: &lkproto.Room{Name: "vc_10"},
		Participant: &lkproto.ParticipantInfo{Sid: "PA_1", Identity: "4", Metadata: "s", State: lkproto.ParticipantInfo_ACTIVE},
	}
	ev, ok, err := p.ParseWebhook(httptest.NewRecorder(), signedWebhook(t, joined, secret))
	want := ports.MediaEvent{
		Kind: ports.MediaParticipantJoined, Room: "vc_10",
		Participant: ports.MediaParticipant{SID: "PA_1", Identity: "4", Metadata: "s", Active: true},
	}
	if err != nil || !ok || !cmp.Equal(ev, want) {
		t.Fatalf("joined = %+v %v %v", ev, ok, err)
	}

	track := &lkproto.WebhookEvent{
		Event: "track_published", Room: &lkproto.Room{Name: "vc_10"},
		Participant: &lkproto.ParticipantInfo{Sid: "PA_1", Identity: "4"},
		Track:       &lkproto.TrackInfo{Source: lkproto.TrackSource_SCREEN_SHARE},
	}
	if ev, ok, err := p.ParseWebhook(httptest.NewRecorder(), signedWebhook(t, track, secret)); err != nil || !ok ||
		ev.Kind != ports.MediaTrackPublished || ev.Source != domain.SourceScreenShare {
		t.Errorf("track = %+v %v %v", ev, ok, err)
	}
	aborted := &lkproto.WebhookEvent{Event: "participant_connection_aborted", Room: &lkproto.Room{Name: "vc_10"}, Participant: &lkproto.ParticipantInfo{Identity: "4"}}
	if ev, _, _ := p.ParseWebhook(httptest.NewRecorder(), signedWebhook(t, aborted, secret)); ev.Kind != ports.MediaParticipantLeft {
		t.Errorf("aborted = %+v", ev)
	}
	if _, ok, err := p.ParseWebhook(httptest.NewRecorder(), signedWebhook(t, &lkproto.WebhookEvent{Event: "room_started"}, secret)); ok || err != nil {
		t.Errorf("room_started: ok %v, err %v", ok, err)
	}
	if _, _, err := p.ParseWebhook(httptest.NewRecorder(), signedWebhook(t, joined, "another-secret-that-is-long-enough-000000")); err == nil {
		t.Error("accepted a webhook signed with another secret")
	}
	unsigned := signedWebhook(t, joined, secret)
	unsigned.Header.Del("Authorization")
	if _, _, err := p.ParseWebhook(httptest.NewRecorder(), unsigned); err == nil {
		t.Error("accepted an unsigned webhook")
	}
}

func TestServerConfig(t *testing.T) {
	data, err := ServerConfig(ServerOptions{
		Port: 7880, UDPPort: 7882, TCPPort: 7881, APIKey: key,
		WebhookURL: "http://127.0.0.1:7300/internal/livekit/webhook", ExternalIP: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(secret)) {
		t.Fatal("the config holds the API secret")
	}
	var got map[string]any
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"port":           7880,
		"bind_addresses": []any{"127.0.0.1"},
		"rtc": map[string]any{
			"udp_port": 7882, "tcp_port": 7881, "use_external_ip": true,
			"skip_external_ip_validation": true, "advertise_internal_ip": true,
			"stun_servers": []any{"stun.cloudflare.com:3478", "stun.l.google.com:19302"},
		},
		"room":    map[string]any{"auto_create": true, "empty_timeout": 30, "departure_timeout": 20},
		"webhook": map[string]any{"api_key": key, "urls": []any{"http://127.0.0.1:7300/internal/livekit/webhook"}},
		"logging": map[string]any{"level": "info", "json": true},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("config (-want +got):\n%s", diff)
	}
	if KeysEnv(key, secret) != "LIVEKIT_KEYS=APItest: "+secret {
		t.Errorf("keys env = %s", KeysEnv(key, secret))
	}
}

func TestServerConfigWithKnownNodeIP(t *testing.T) {
	data, err := ServerConfig(ServerOptions{Port: 7880, UDPPort: 7882, ExternalIP: true, NodeIP: "203.0.113.9"})
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ RTC map[string]any }
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	// An explicit node IP with the lookup off is what LiveKit advertises as is.
	for k, want := range map[string]any{
		"node_ip": "203.0.113.9", "use_external_ip": false,
		"skip_external_ip_validation": false, "advertise_internal_ip": true,
	} {
		if got.RTC[k] != want {
			t.Errorf("rtc.%s = %v, want %v", k, got.RTC[k], want)
		}
	}
}
