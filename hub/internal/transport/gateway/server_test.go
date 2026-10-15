package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/gateway/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/clock"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/auth"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/snapshot"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/httpauth"
)

// fakeAuth accepts "token-<userID>" unless the token is revoked.
type fakeAuth struct {
	mu      sync.Mutex
	revoked map[string]bool
}

func (a *fakeAuth) Authenticate(_ context.Context, raw string) (auth.Principal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id, ok := strings.CutPrefix(raw, "token-")
	if !ok || a.revoked[raw] {
		return auth.Principal{}, domain.Unauthenticated(domain.ReasonSessionExpired)
	}
	userID, err := domain.ParseID(id, "token")
	if err != nil {
		return auth.Principal{}, domain.Unauthenticated(domain.ReasonSessionExpired)
	}
	return auth.Principal{
		Actor:   domain.Actor{UserID: userID, SessionID: userID * 10, EmailVerified: true},
		Session: domain.Session{ID: userID * 10, UserID: userID},
	}, nil
}

func (a *fakeAuth) revoke(raw string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revoked[raw] = true
}

// fakeSnapshots puts every user in guild 100 as a member, except user 1 who
// is its owner. beforeReady runs while the snapshot is being read.
type fakeSnapshots struct {
	beforeReady func()
}

func (f *fakeSnapshots) Memberships(_ context.Context, userID domain.ID) ([]domain.Member, error) {
	role := domain.RoleMember
	if userID == 1 {
		role = domain.RoleOwner
	}
	return []domain.Member{{GuildID: 100, UserID: userID, Role: role}}, nil
}

func (f *fakeSnapshots) Ready(_ context.Context, actor domain.Actor) (snapshot.Ready, error) {
	if f.beforeReady != nil {
		f.beforeReady()
	}
	return snapshot.Ready{Me: domain.User{ID: actor.UserID, Username: "u" + actor.UserID.String()}}, nil
}

type presenceCall struct {
	kind   string
	userID domain.ID
	idle   bool
}

type fakePresence struct {
	mu    sync.Mutex
	calls []presenceCall
}

func (p *fakePresence) record(c presenceCall) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, c)
}

func (p *fakePresence) Connect(_ context.Context, u domain.User, _ string) {
	p.record(presenceCall{kind: "connect", userID: u.ID})
}

func (p *fakePresence) Disconnect(_ context.Context, userID domain.ID, _ string) {
	p.record(presenceCall{kind: "disconnect", userID: userID})
}

func (p *fakePresence) SetIdle(_ context.Context, userID domain.ID, _ string, idle bool) {
	p.record(presenceCall{kind: "idle", userID: userID, idle: idle})
}

func (p *fakePresence) count(kind string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, c := range p.calls {
		if c.kind == kind {
			n++
		}
	}
	return n
}

type env struct {
	t         *testing.T
	bus       *events.Bus
	auth      *fakeAuth
	snapshots *fakeSnapshots
	presence  *fakePresence
	gw        *Server
	srv       *httptest.Server
}

func newEnv(t *testing.T, tune func(*Options)) *env {
	t.Helper()
	e := &env{t: t, bus: events.NewBus(), auth: &fakeAuth{revoked: map[string]bool{}}, snapshots: &fakeSnapshots{}, presence: &fakePresence{}}
	o := Options{
		Events: e.bus, Auth: e.auth, Snapshots: e.snapshots, Presence: e.presence,
		Tokens: &fakes.Tokens{}, Clock: clock.System{}, Logger: slog.New(slog.DiscardHandler),
	}
	if tune != nil {
		tune(&o)
	}
	e.gw = New(o)
	e.srv = httptest.NewServer(e.gw)
	t.Cleanup(func() {
		e.gw.Shutdown(context.Background())
		e.srv.Close()
	})
	return e
}

type client struct {
	t  *testing.T
	ws *websocket.Conn
}

func (e *env) dial(token string) *client {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(e.t.Context(), 5*time.Second)
	defer cancel()
	h := http.Header{}
	if token != "" {
		h.Set("Cookie", httpauth.SessionCookie+"="+token)
	}
	ws, res, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.srv.URL, "http"), &websocket.DialOptions{HTTPHeader: h})
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
	if err != nil {
		e.t.Fatalf("dial: %v", err)
	}
	ws.SetReadLimit(1 << 20)
	// Runs before the server shuts down, so no close handshake waits on an idle client.
	e.t.Cleanup(func() { _ = ws.CloseNow() })
	c := &client{t: e.t, ws: ws}
	if hello := c.next().GetHello(); hello == nil {
		e.t.Fatal("first frame is not Hello")
	}
	return c
}

func (c *client) send(f *gatewayv1.ClientFrame) {
	c.t.Helper()
	data, _ := proto.Marshal(f)
	if err := c.ws.Write(c.t.Context(), websocket.MessageBinary, data); err != nil {
		c.t.Fatalf("write: %v", err)
	}
}

func (c *client) next() *gatewayv1.ServerFrame {
	c.t.Helper()
	f, err := c.read()
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	return f
}

func (c *client) read() (*gatewayv1.ServerFrame, error) {
	ctx, cancel := context.WithTimeout(c.t.Context(), 5*time.Second)
	defer cancel()
	_, data, err := c.ws.Read(ctx)
	if err != nil {
		return nil, err
	}
	f := &gatewayv1.ServerFrame{}
	if err := proto.Unmarshal(data, f); err != nil {
		c.t.Fatalf("decode: %v", err)
	}
	return f, nil
}

// closedWith reads until the server closes and returns the close code.
func (c *client) closedWith() websocket.StatusCode {
	c.t.Helper()
	for {
		if _, err := c.read(); err != nil {
			return websocket.CloseStatus(err)
		}
	}
}

func (c *client) identify() string {
	c.t.Helper()
	c.send(&gatewayv1.ClientFrame{Kind: &gatewayv1.ClientFrame_Identify{Identify: &gatewayv1.Identify{}}})
	ready := c.next().GetReady()
	if ready == nil {
		c.t.Fatal("expected Ready")
	}
	return ready.GetSessionId()
}

func (c *client) resume(sessionID string, lastSeq uint64) {
	c.send(&gatewayv1.ClientFrame{Kind: &gatewayv1.ClientFrame_Resume{Resume: &gatewayv1.Resume{SessionId: sessionID, LastSeq: lastSeq}}})
}

func (c *client) heartbeat(lastSeq uint64) {
	c.send(&gatewayv1.ClientFrame{Kind: &gatewayv1.ClientFrame_Heartbeat{Heartbeat: &gatewayv1.Heartbeat{LastSeq: lastSeq}}})
}

// dispatch reads the next frame, which must be a dispatch.
func (c *client) dispatch() *gatewayv1.Dispatch {
	c.t.Helper()
	f := c.next()
	d := f.GetDispatch()
	if d == nil {
		c.t.Fatalf("expected a dispatch, got %v", f)
	}
	return d
}

// rename publishes a guild update; the name tells dispatches apart.
func (e *env) rename(guildID domain.ID, name string) {
	e.bus.Publish(events.Event{
		Topics:  []events.Topic{events.GuildTopic(guildID)},
		Payload: events.GuildUpdated{Guild: domain.Guild{ID: guildID, Name: name}},
	})
}

func nameOf(d *gatewayv1.Dispatch) string { return d.GetEvent().GetGuildUpdate().GetName() }

func TestIdentifyAndDispatch(t *testing.T) {
	e := newEnv(t, nil)
	c := e.dial("token-2")
	if id := c.identify(); id == "" {
		t.Fatal("Ready without a session ID")
	}
	e.rename(100, "first")
	e.rename(999, "other guild")
	e.rename(100, "second")
	for i, want := range []string{"first", "second"} {
		d := c.dispatch()
		if d.GetSeq() != uint64(i+1) || nameOf(d) != want {
			t.Errorf("dispatch %d = seq %d %q", i, d.GetSeq(), nameOf(d))
		}
	}
	if e.presence.count("connect") != 1 {
		t.Errorf("presence = %+v", e.presence.calls)
	}

	c.heartbeat(2)
	if c.next().GetHeartbeatAck() == nil {
		t.Error("no heartbeat ack")
	}
	c.send(&gatewayv1.ClientFrame{Kind: &gatewayv1.ClientFrame_UpdatePresence{UpdatePresence: &gatewayv1.UpdatePresence{Idle: true}}})
	c.heartbeat(2)
	c.next()
	if e.presence.count("idle") != 1 {
		t.Errorf("idle not reported: %+v", e.presence.calls)
	}

	_ = c.ws.Close(websocket.StatusNormalClosure, "")
	waitFor(t, func() bool { return e.presence.count("disconnect") == 1 })
}

func TestEventsDuringSnapshotArriveAfterReady(t *testing.T) {
	e := newEnv(t, nil)
	e.snapshots.beforeReady = func() { e.rename(100, "while loading") }
	c := e.dial("token-2")
	c.identify()
	if d := c.dispatch(); d.GetSeq() != 1 || nameOf(d) != "while loading" {
		t.Errorf("dispatch = %v", d)
	}
}

func TestResumeReplaysMissedEvents(t *testing.T) {
	e := newEnv(t, nil)
	c := e.dial("token-2")
	id := c.identify()
	e.rename(100, "seen")
	c.dispatch()

	// The network drops: no close frame.
	_ = c.ws.CloseNow()
	waitFor(t, func() bool {
		e.gw.mu.Lock()
		defer e.gw.mu.Unlock()
		return e.gw.sessions[id] != nil && e.gw.sessions[id].conn == nil
	})
	e.rename(100, "missed 1")
	e.rename(100, "missed 2")

	c2 := e.dial("token-2")
	c2.resume(id, 1)
	if c2.next().GetResumed() == nil {
		t.Fatal("expected Resumed")
	}
	for i, want := range []string{"missed 1", "missed 2"} {
		d := c2.dispatch()
		if d.GetSeq() != uint64(i+2) || nameOf(d) != want {
			t.Errorf("replayed seq %d %q", d.GetSeq(), nameOf(d))
		}
	}
	e.rename(100, "live")
	if d := c2.dispatch(); d.GetSeq() != 4 || nameOf(d) != "live" {
		t.Errorf("after resume: seq %d %q", d.GetSeq(), nameOf(d))
	}
	if e.presence.count("disconnect") != 0 || e.presence.count("connect") != 1 {
		t.Errorf("a resumed session must not flap presence: %+v", e.presence.calls)
	}
}

func TestResumeRefused(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.ResumeWindow = 150 * time.Millisecond; o.ReplayLimit = 2 })
	c := e.dial("token-2")
	id := c.identify()
	_ = c.ws.CloseNow()
	waitFor(t, func() bool { return e.presence.count("disconnect") == 1 })

	// Past the window the session is gone; the client identifies again on
	// the same connection.
	c2 := e.dial("token-2")
	c2.resume(id, 0)
	if c2.next().GetInvalidSession() == nil {
		t.Fatal("expected InvalidSession")
	}
	id2 := c2.identify()

	// More events than the replay buffer holds.
	_ = c2.ws.CloseNow()
	waitFor(t, func() bool {
		e.gw.mu.Lock()
		defer e.gw.mu.Unlock()
		return e.gw.sessions[id2] != nil && e.gw.sessions[id2].conn == nil
	})
	for _, n := range []string{"a", "b", "c"} {
		e.rename(100, n)
	}
	c3 := e.dial("token-2")
	c3.resume(id2, 0)
	if c3.next().GetInvalidSession() == nil {
		t.Error("a gap in the replay buffer must refuse the resume")
	}

	// Someone else's session cannot be resumed.
	c4 := e.dial("token-3")
	c4.resume(id2, 3)
	if c4.next().GetInvalidSession() == nil {
		t.Error("resumed another user's session")
	}
}

func TestCloseCodes(t *testing.T) {
	e := newEnv(t, nil)

	if got := e.dial("").closedWith(); got != closeNotAuthenticated {
		t.Errorf("no cookie: %d", got)
	}
	c := e.dial("token-2")
	c.heartbeat(0)
	c.next()
	c.send(&gatewayv1.ClientFrame{Kind: &gatewayv1.ClientFrame_UpdatePresence{UpdatePresence: &gatewayv1.UpdatePresence{}}})
	if got := c.closedWith(); got != closeNotAuthenticated {
		t.Errorf("frame before identify: %d", got)
	}

	c = e.dial("token-2")
	c.identify()
	c.send(&gatewayv1.ClientFrame{Kind: &gatewayv1.ClientFrame_Identify{Identify: &gatewayv1.Identify{}}})
	if got := c.closedWith(); got != closeAlreadyIdentify {
		t.Errorf("identify twice: %d", got)
	}

	c = e.dial("token-2")
	c.identify()
	_ = c.ws.Write(t.Context(), websocket.MessageText, []byte("{}"))
	if got := c.closedWith(); got != closeDecodeError {
		t.Errorf("text frame: %d", got)
	}

	c = e.dial("token-2")
	c.identify()
	c.send(&gatewayv1.ClientFrame{})
	if got := c.closedWith(); got != closeBadOpcode {
		t.Errorf("empty frame: %d", got)
	}

	c = e.dial("token-4")
	c.identify()
	e.auth.revoke("token-4")
	c.heartbeat(0)
	if got := c.closedWith(); got != closeSessionRevoked {
		t.Errorf("revoked session: %d", got)
	}

	c = e.dial("token-5")
	c.identify()
	for range 200 {
		if err := c.ws.Write(t.Context(), websocket.MessageBinary, []byte{}); err != nil {
			break
		}
	}
	if got := c.closedWith(); got != closeRateLimited && got != closeBadOpcode {
		t.Errorf("flood: %d", got)
	}
}

func TestHeartbeatTimeout(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.Heartbeat = 80 * time.Millisecond })
	c := e.dial("token-2")
	id := c.identify()
	if got := c.closedWith(); got != closeSessionExpired {
		t.Errorf("silent client: %d", got)
	}
	e.gw.mu.Lock()
	kept := e.gw.sessions[id] != nil
	e.gw.mu.Unlock()
	if !kept {
		t.Error("a heartbeat timeout must leave the session resumable")
	}
}

func TestShutdownAndOrigin(t *testing.T) {
	e := newEnv(t, nil)
	c := e.dial("token-2")
	c.identify()
	go e.gw.Shutdown(context.Background())
	if got := c.closedWith(); got != closeHubRestarting {
		t.Errorf("shutdown: %d", got)
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, e.srv.URL, http.NoBody)
	req.Header.Set("Origin", "https://evil.example")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin handshake: %d", res.StatusCode)
	}
}

func TestTopicSubscriptions(t *testing.T) {
	e := newEnv(t, nil)
	owner := e.dial("token-1")
	owner.identify()
	member := e.dial("token-2")
	member.identify()

	admins := func(name string) {
		e.bus.Publish(events.Event{
			Topics:  []events.Topic{events.GuildAdminsTopic(100)},
			Payload: events.GuildUpdated{Guild: domain.Guild{ID: 100, Name: name}},
		})
	}
	admins("owner only")
	if nameOf(owner.dispatch()) != "owner only" {
		t.Error("owner missed an admin event")
	}

	// Promotion subscribes the member to admin events of that guild.
	e.bus.Publish(events.Event{
		Topics:  []events.Topic{events.GuildTopic(100)},
		Payload: events.MemberUpdated{Member: domain.Member{GuildID: 100, UserID: 2, Role: domain.RoleAdmin}},
	})
	owner.dispatch()
	member.dispatch()
	admins("both")
	if nameOf(member.dispatch()) != "both" {
		t.Error("promoted member missed an admin event")
	}
	owner.dispatch()

	// Joining another guild subscribes to it.
	e.bus.Publish(events.Event{
		Topics: []events.Topic{events.UserTopic(2)},
		Payload: events.GuildCreated{UserID: 2, State: domain.GuildState{
			Guild:   domain.Guild{ID: 200},
			Members: []domain.Member{{GuildID: 200, UserID: 2, Role: domain.RoleMember}},
		}},
	})
	if member.dispatch().GetEvent().GetGuildCreate() == nil {
		t.Fatal("expected GuildCreate")
	}
	e.rename(200, "new guild")
	if nameOf(member.dispatch()) != "new guild" {
		t.Error("missed an event of the joined guild")
	}

	// Being kicked delivers the delete, then nothing more from that guild.
	e.bus.Publish(events.Event{
		Topics:  []events.Topic{events.UserTopic(2)},
		Payload: events.GuildDeleted{GuildID: 100, UserID: 2, Reason: events.ReasonKicked},
	})
	if d := member.dispatch(); d.GetEvent().GetGuildDelete().GetReason() != gatewayv1.GuildDeleteReason_GUILD_DELETE_REASON_KICKED {
		t.Errorf("delete = %v", d)
	}
	e.rename(100, "after kick")
	e.rename(200, "still here")
	if nameOf(owner.dispatch()) != "after kick" {
		t.Error("owner missed an event")
	}
	if name := nameOf(member.dispatch()); name != "still here" {
		t.Errorf("kicked member received %q", name)
	}
}

func TestDispatchFrameEncoding(t *testing.T) {
	ev, _ := proto.Marshal(toEvent(events.GuildUpdated{Guild: domain.Guild{ID: 5, Name: "x"}}))
	f := &gatewayv1.ServerFrame{}
	if err := proto.Unmarshal(dispatchFrame(42, ev), f); err != nil {
		t.Fatal(err)
	}
	if f.GetDispatch().GetSeq() != 42 || f.GetDispatch().GetEvent().GetGuildUpdate().GetId() != "5" {
		t.Errorf("decoded %v", f)
	}
}

func TestReplayBuffer(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	s := &session{ready: true}
	for i := range 5 {
		s.push([]byte{byte(i)}, now.Add(time.Duration(i)*time.Second), 3, time.Minute)
	}
	if len(s.replay) != 3 || s.replay[0].seq != 3 {
		t.Fatalf("limit: %+v", s.replay)
	}
	if _, ok := s.missed(1); ok {
		t.Error("seq 2 is gone, resuming from 1 must fail")
	}
	if frames, ok := s.missed(2); !ok || len(frames) != 3 {
		t.Errorf("missed(2) = %d frames, %v", len(frames), ok)
	}
	if frames, ok := s.missed(5); !ok || len(frames) != 0 {
		t.Error("an up-to-date client resumes with nothing to replay")
	}
	if _, ok := s.missed(6); ok {
		t.Error("a client ahead of the server must not resume")
	}
	s.ack(4)
	if len(s.replay) != 1 || s.replay[0].seq != 5 {
		t.Errorf("ack: %+v", s.replay)
	}
	s.push([]byte{9}, now.Add(2*time.Minute), 10, time.Minute)
	if len(s.replay) != 1 || s.replay[0].seq != 6 {
		t.Errorf("window: %+v", s.replay)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
