package app

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/gateway/v1"
	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
)

// gatewayConn is a client of /gateway using a browser's session cookie.
type gatewayConn struct {
	t  *testing.T
	ws *websocket.Conn
}

func (b *browser) openGateway(t *testing.T) (*gatewayConn, *gatewayv1.Ready) {
	t.Helper()
	u, _ := url.Parse(b.base)
	h := http.Header{}
	for _, c := range b.http.Jar.Cookies(u) {
		h.Add("Cookie", c.String())
	}
	h.Set("Origin", b.base)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ws, res, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(b.base, "http")+"/gateway", &websocket.DialOptions{HTTPHeader: h})
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial gateway: %v", err)
	}
	ws.SetReadLimit(1 << 20)
	t.Cleanup(func() { _ = ws.CloseNow() })
	g := &gatewayConn{t: t, ws: ws}
	if g.next().GetHello() == nil {
		t.Fatal("expected Hello")
	}
	data, _ := proto.Marshal(&gatewayv1.ClientFrame{Kind: &gatewayv1.ClientFrame_Identify{Identify: &gatewayv1.Identify{}}})
	if err := ws.Write(t.Context(), websocket.MessageBinary, data); err != nil {
		t.Fatal(err)
	}
	ready := g.next().GetReady()
	if ready == nil {
		t.Fatal("expected Ready")
	}
	return g, ready
}

func (g *gatewayConn) next() *gatewayv1.ServerFrame {
	g.t.Helper()
	ctx, cancel := context.WithTimeout(g.t.Context(), 5*time.Second)
	defer cancel()
	_, data, err := g.ws.Read(ctx)
	if err != nil {
		g.t.Fatalf("read gateway: %v", err)
	}
	f := &gatewayv1.ServerFrame{}
	if err := proto.Unmarshal(data, f); err != nil {
		g.t.Fatal(err)
	}
	return f
}

// waitEvent reads dispatches until one matches.
func (g *gatewayConn) waitEvent(match func(*gatewayv1.Event) bool) *gatewayv1.Event {
	g.t.Helper()
	for {
		if ev := g.next().GetDispatch().GetEvent(); ev != nil && match(ev) {
			return ev
		}
	}
}

func registerMember(t *testing.T, r *running, invite, username string) *browser {
	t.Helper()
	b := r.browser(t)
	_, err := b.auth.Register(t.Context(), &dislacksov1.RegisterRequest{
		Username: username, Email: username + "@example.com", Password: "correct horse battery", InviteCode: invite,
	})
	if err != nil {
		t.Fatalf("Register %s: %v", username, err)
	}
	return b
}

func TestGuildFlowOverHTTPAndGateway(t *testing.T) {
	r := start(t)
	ctx := t.Context()
	admin := r.browser(t)
	completeSetup(t, admin)

	created, err := admin.guilds.CreateGuild(ctx, &dislacksov1.CreateGuildRequest{Name: "Game Dev"})
	if err != nil {
		t.Fatal(err)
	}
	g := created.GetState().GetGuild()
	if len(created.GetState().GetChannels()) != 4 {
		t.Fatalf("default channels = %v", created.GetState().GetChannels())
	}
	inv, err := admin.invites.CreateInvite(ctx, &dislacksov1.CreateInviteRequest{GuildId: g.GetId(), MaxUses: 5})
	if err != nil {
		t.Fatal(err)
	}
	code := inv.GetInvite().GetCode()
	if inv.GetInvite().GetUrl() != "http://localhost:7300/invite/"+code {
		t.Errorf("invite URL = %s", inv.GetInvite().GetUrl())
	}

	// Signed out, the landing page can preview the invite.
	anon := r.browser(t)
	preview, err := anon.invites.PreviewInvite(ctx, &dislacksov1.PreviewInviteRequest{Code: code})
	if err != nil || preview.GetPreview().GetGuildName() != "Game Dev" || preview.GetPreview().GetMemberCount() != 1 {
		t.Fatalf("preview = %v, %v", preview, err)
	}

	adminGW, ready := admin.openGateway(t)
	if len(ready.GetGuilds()) != 1 || ready.GetInstance().GetName() != "Team" {
		t.Fatalf("admin ready = %v", ready)
	}

	// Registering with the guild invite joins the guild; the admin sees it.
	ana := registerMember(t, r, code, "ana")
	added := adminGW.waitEvent(func(e *gatewayv1.Event) bool { return e.GetMemberAdd() != nil }).GetMemberAdd()
	if added.GetUser().GetUsername() != "ana" || added.GetMember().GetRole() != dislacksov1.GuildRole_GUILD_ROLE_MEMBER {
		t.Errorf("member add = %v", added)
	}
	anaGW, anaReady := ana.openGateway(t)
	if len(anaReady.GetGuilds()) != 1 || len(anaReady.GetGuilds()[0].GetMembers()) != 2 {
		t.Fatalf("ana ready = %v", anaReady)
	}
	if p := adminGW.waitEvent(func(e *gatewayv1.Event) bool { return e.GetPresenceUpdate() != nil }).GetPresenceUpdate(); p.GetStatus() != dislacksov1.PresenceStatus_PRESENCE_STATUS_ONLINE {
		t.Errorf("presence = %v", p)
	}

	// A rename by the admin reaches ana's gateway.
	var voice *dislacksov1.Channel
	for _, c := range created.GetState().GetChannels() {
		if c.GetType() == dislacksov1.ChannelType_CHANNEL_TYPE_VOICE {
			voice = c
		}
	}
	if _, err := admin.channels.UpdateChannel(ctx, &dislacksov1.UpdateChannelRequest{ChannelId: voice.GetId(), Name: new("Standup")}); err != nil {
		t.Fatal(err)
	}
	upd := anaGW.waitEvent(func(e *gatewayv1.Event) bool { return e.GetChannelUpdate() != nil }).GetChannelUpdate()
	if upd.GetName() != "Standup" {
		t.Errorf("channel update = %v", upd)
	}

	// A member cannot manage channels, and an outsider cannot see the guild.
	_, err = ana.channels.CreateChannel(ctx, &dislacksov1.CreateChannelRequest{GuildId: g.GetId(), Type: dislacksov1.ChannelType_CHANNEL_TYPE_TEXT, Name: "x"})
	requireCode(t, err, connect.CodePermissionDenied)
	inv2, _ := admin.invites.CreateInvite(ctx, &dislacksov1.CreateInviteRequest{GuildId: g.GetId()})
	other := registerMember(t, r, inv2.GetInvite().GetCode(), "bruno")
	if _, err := admin.moderation.BanMember(ctx, &dislacksov1.BanMemberRequest{GuildId: g.GetId(), UserId: idOf(t, other)}); err != nil {
		t.Fatal(err)
	}
	_, err = other.guilds.ListMembers(ctx, &dislacksov1.ListMembersRequest{GuildId: g.GetId()})
	requireCode(t, err, connect.CodeNotFound)
	_, err = other.invites.AcceptInvite(ctx, &dislacksov1.AcceptInviteRequest{Code: code})
	requireCode(t, err, connect.CodePermissionDenied)

	// Kicking ana removes the guild from her gateway at once.
	if _, err := admin.moderation.KickMember(ctx, &dislacksov1.KickMemberRequest{GuildId: g.GetId(), UserId: idOf(t, ana)}); err != nil {
		t.Fatal(err)
	}
	gone := anaGW.waitEvent(func(e *gatewayv1.Event) bool { return e.GetGuildDelete() != nil }).GetGuildDelete()
	if gone.GetGuildId() != g.GetId() || gone.GetReason() != gatewayv1.GuildDeleteReason_GUILD_DELETE_REASON_KICKED {
		t.Errorf("guild delete = %v", gone)
	}

	log, err := admin.moderation.ListAuditLog(ctx, &dislacksov1.ListAuditLogRequest{GuildId: g.GetId()})
	if err != nil || len(log.GetEntries()) < 4 {
		t.Fatalf("audit log = %v, %v", log, err)
	}
	if log.GetEntries()[0].GetAction() != dislacksov1.AuditAction_AUDIT_ACTION_MEMBER_KICK {
		t.Errorf("newest entry = %v", log.GetEntries()[0])
	}
}

func idOf(t *testing.T, b *browser) string {
	t.Helper()
	me, err := b.account.GetMe(t.Context(), &dislacksov1.GetMeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return me.GetMe().GetUser().GetId()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// upload posts one file with the query string and returns the status and the response body.
func (b *browser) upload(t *testing.T, query, origin string, data []byte) (int, []byte) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "me.png")
	_, _ = part.Write(data)
	_ = mw.Close()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, b.base+"/api/uploads?"+query, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	res, err := b.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, raw
}

func TestAvatarUploadAndDownload(t *testing.T) {
	r := start(t)
	ctx := t.Context()
	admin := r.browser(t)
	completeSetup(t, admin)

	status, raw := admin.upload(t, "purpose=avatar", "", pngBytes(t, 400, 300))
	if status != http.StatusCreated {
		t.Fatalf("upload: %d %s", status, raw)
	}
	ref := &dislacksov1.FileRef{}
	if err := protojson.Unmarshal(raw, ref); err != nil {
		t.Fatal(err)
	}
	if ref.GetMime() != "image/jpeg" || ref.GetWidth() != 256 || ref.GetHeight() != 256 {
		t.Errorf("file ref = %v", ref)
	}

	me, err := admin.account.UpdateProfile(ctx, &dislacksov1.UpdateProfileRequest{AvatarFileId: new(ref.GetId())})
	if err != nil {
		t.Fatal(err)
	}
	avatar := me.GetMe().GetUser().GetAvatarUrl()
	if avatar != "/files/"+ref.GetId()+"/avatar" {
		t.Errorf("avatar URL = %s", avatar)
	}
	code, h := admin.get(t, r.base+avatar)
	if code != http.StatusOK || h.Get("Content-Type") != "image/jpeg" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("download: %d %v", code, h)
	}
	if code, _ := r.browser(t).get(t, r.base+avatar); code != http.StatusUnauthorized {
		t.Errorf("signed-out download: %d", code)
	}

	status, raw = admin.upload(t, "purpose=avatar", "", []byte("<svg onload=alert(1)>"))
	var body struct {
		Code   string `json:"code"`
		Fields []struct {
			Reason string `json:"reason"`
		} `json:"fields"`
	}
	_ = json.Unmarshal(raw, &body)
	if status != http.StatusBadRequest || len(body.Fields) != 1 || body.Fields[0].Reason != "unsupported_file" {
		t.Errorf("svg upload: %d %+v", status, body)
	}
	if status, _ := admin.upload(t, "purpose=guild_icon", "https://evil.example", pngBytes(t, 8, 8)); status != http.StatusForbidden {
		t.Errorf("cross-site upload: %d", status)
	}
	if status, _ := r.browser(t).upload(t, "purpose=guild_icon", "", pngBytes(t, 8, 8)); status != http.StatusUnauthorized {
		t.Errorf("signed-out upload: %d", status)
	}
	if status, raw := admin.upload(t, "purpose=guild_icon", "", pngBytes(t, 8, 8)); status != http.StatusCreated {
		t.Errorf("guild icon upload: %d %s", status, raw)
	}
}

func TestAnimatedAvatarUpload(t *testing.T) {
	r := start(t)
	admin := r.browser(t)
	completeSetup(t, admin)

	var anim bytes.Buffer
	frames := &gif.GIF{}
	for _, c := range []color.Color{color.White, color.Black} {
		m := image.NewPaletted(image.Rect(0, 0, 300, 200), color.Palette{c})
		frames.Image = append(frames.Image, m)
		frames.Delay = append(frames.Delay, 10)
	}
	if err := gif.EncodeAll(&anim, frames); err != nil {
		t.Fatal(err)
	}
	status, raw := admin.upload(t, "purpose=avatar&crop=200,100,100,100", "", anim.Bytes())
	ref := &dislacksov1.FileRef{}
	if err := protojson.Unmarshal(raw, ref); status != http.StatusCreated || err != nil {
		t.Fatalf("upload: %d %s", status, raw)
	}
	if ref.GetMime() != "image/gif" || ref.GetWidth() != 100 {
		t.Errorf("file ref = %v", ref)
	}
	if status, raw := admin.upload(t, "purpose=avatar&crop=1,2,3", "", anim.Bytes()); status != http.StatusBadRequest || !strings.Contains(string(raw), "crop") {
		t.Errorf("bad crop: %d %s", status, raw)
	}
}
