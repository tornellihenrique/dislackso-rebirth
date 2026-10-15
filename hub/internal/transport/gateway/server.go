// Package gateway is the realtime WebSocket gateway (ARCHITECTURE §8): it
// identifies or resumes sessions, sends the Ready snapshot, and delivers
// service events to every session subscribed to their topics.
package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/gateway/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/auth"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/snapshot"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/httpauth"
)

// Path is where the gateway is served.
const Path = "/gateway"

const (
	DefaultHeartbeat    = 25 * time.Second
	DefaultResumeWindow = 120 * time.Second
	DefaultReplayLimit  = 2000

	maxFrameBytes   = 64 << 10
	outboxSize      = 1024
	writeTimeout    = 10 * time.Second
	identifyTimeout = 30 * time.Second
)

// frameLimit allows a burst of client frames, then two per second.
var frameLimit = ports.Limit{Burst: 60, Every: 500 * time.Millisecond}

func code(c gatewayv1.CloseCode) websocket.StatusCode { return websocket.StatusCode(c) }

var (
	closeUnknown          = code(gatewayv1.CloseCode_CLOSE_CODE_UNKNOWN)
	closeBadOpcode        = code(gatewayv1.CloseCode_CLOSE_CODE_BAD_OPCODE)
	closeDecodeError      = code(gatewayv1.CloseCode_CLOSE_CODE_DECODE_ERROR)
	closeNotAuthenticated = code(gatewayv1.CloseCode_CLOSE_CODE_NOT_AUTHENTICATED)
	closeSessionRevoked   = code(gatewayv1.CloseCode_CLOSE_CODE_SESSION_REVOKED)
	closeAlreadyIdentify  = code(gatewayv1.CloseCode_CLOSE_CODE_ALREADY_IDENTIFIED)
	closeRateLimited      = code(gatewayv1.CloseCode_CLOSE_CODE_RATE_LIMITED)
	closeSessionExpired   = code(gatewayv1.CloseCode_CLOSE_CODE_SESSION_EXPIRED)
	closeHubRestarting    = code(gatewayv1.CloseCode_CLOSE_CODE_HUB_RESTARTING)
)

// notResumable lists the closes that end the session: sign-outs, protocol
// errors, floods and restarts.
var notResumable = map[websocket.StatusCode]bool{
	closeNotAuthenticated: true, closeSessionRevoked: true, closeHubRestarting: true, closeRateLimited: true,
	closeAlreadyIdentify: true, closeBadOpcode: true, closeDecodeError: true,
}

// resumable reports whether a session survives its connection closing with
// c: dropped networks, slow readers, missed heartbeats and clients closing
// with a 4xxx code to reconnect. A normal close means the client left.
func resumable(c websocket.StatusCode) bool {
	if c == websocket.StatusAbnormalClosure || c == websocket.StatusNoStatusRcvd || c == -1 {
		return true
	}
	return c >= 4000 && c < 5000 && !notResumable[c]
}

type Authenticator interface {
	Authenticate(ctx context.Context, raw string) (auth.Principal, error)
}

type Snapshots interface {
	Memberships(ctx context.Context, userID domain.ID) ([]domain.Member, error)
	Ready(ctx context.Context, actor domain.Actor) (snapshot.Ready, error)
}

type Presence interface {
	Connect(ctx context.Context, u domain.User, connID string)
	Disconnect(ctx context.Context, userID domain.ID, connID string)
	SetIdle(ctx context.Context, userID domain.ID, connID string, idle bool)
}

type Subscriber interface {
	Subscribe(fn func(events.Event)) (cancel func())
}

type Options struct {
	Events    Subscriber
	Auth      Authenticator
	Snapshots Snapshots
	Presence  Presence
	// Tokens generates session IDs.
	Tokens       ports.TokenGen
	Clock        ports.Clock
	Logger       *slog.Logger
	Heartbeat    time.Duration
	ResumeWindow time.Duration
	ReplayLimit  int
}

type Server struct {
	o Options

	mu       sync.Mutex
	sessions map[string]*session
	byUser   map[domain.ID]map[*session]bool
	byTopic  map[events.Topic]map[*session]bool
	conns    map[*connection]bool
	closing  bool

	wg          sync.WaitGroup
	unsubscribe func()
}

func New(o Options) *Server {
	if o.Heartbeat == 0 {
		o.Heartbeat = DefaultHeartbeat
	}
	if o.ResumeWindow == 0 {
		o.ResumeWindow = DefaultResumeWindow
	}
	if o.ReplayLimit == 0 {
		o.ReplayLimit = DefaultReplayLimit
	}
	s := &Server{
		o:        o,
		sessions: map[string]*session{},
		byUser:   map[domain.ID]map[*session]bool{},
		byTopic:  map[events.Topic]map[*session]bool{},
		conns:    map[*connection]bool{},
	}
	s.unsubscribe = o.Events.Subscribe(s.route)
	return s
}

// Shutdown closes every connection with "hub restarting" and waits for them
// to finish until ctx ends.
func (s *Server) Shutdown(ctx context.Context) {
	s.mu.Lock()
	s.closing = true
	conns := make([]*connection, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	for _, sess := range s.sessions {
		if sess.expiry != nil {
			sess.expiry.Stop()
		}
	}
	s.mu.Unlock()
	s.unsubscribe()
	for _, c := range conns {
		c.close(closeHubRestarting, "hub restarting")
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

type connection struct {
	ws     *websocket.Conn
	out    chan []byte
	once   sync.Once
	closed chan struct{}
	code   websocket.StatusCode
	reason string
}

func newConnection(ws *websocket.Conn) *connection {
	return &connection{ws: ws, out: make(chan []byte, outboxSize), closed: make(chan struct{})}
}

// send queues a frame. A client too slow to drain its queue is disconnected
// and resumes once it catches up.
func (c *connection) send(frame []byte) {
	select {
	case <-c.closed:
		return
	default:
	}
	select {
	case c.out <- frame:
	default:
		c.close(closeUnknown, "too slow")
	}
}

func (c *connection) close(code websocket.StatusCode, reason string) {
	c.once.Do(func() {
		c.code, c.reason = code, reason
		close(c.closed)
	})
}

func (c *connection) closeCode() websocket.StatusCode {
	select {
	case <-c.closed:
		return c.code
	default:
		return 0
	}
}

func (c *connection) writeLoop(ctx context.Context) {
	write := func(f []byte) bool {
		wctx, cancel := context.WithTimeout(ctx, writeTimeout)
		defer cancel()
		return c.ws.Write(wctx, websocket.MessageBinary, f) == nil
	}
	for {
		select {
		case f := <-c.out:
			if !write(f) {
				c.close(websocket.StatusAbnormalClosure, "")
				_ = c.ws.CloseNow()
				return
			}
		case <-c.closed:
			// Frames queued before closing (such as InvalidSession) still go out.
			for {
				select {
				case f := <-c.out:
					if !write(f) {
						_ = c.ws.CloseNow()
						return
					}
				default:
					_ = c.ws.Close(c.code, c.reason)
					return
				}
			}
		}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The WebSocket handshake carries the session cookie, so a cross-site
	// page must not be able to open it (cross-site WebSocket hijacking).
	if !httpauth.SameOrigin(r) {
		http.Error(w, "cross-origin request refused", http.StatusForbidden)
		return
	}
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	ws.SetReadLimit(maxFrameBytes)
	c := newConnection(ws)

	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		_ = ws.Close(closeHubRestarting, "hub restarting")
		return
	}
	s.conns[c] = true
	s.wg.Add(1)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
		s.wg.Done()
	}()

	ctx := r.Context()
	written := make(chan struct{})
	go func() {
		c.writeLoop(ctx)
		close(written)
	}()
	c.send(helloFrame(int(s.o.Heartbeat.Milliseconds()), int(s.o.ResumeWindow.Milliseconds())))
	s.run(ctx, c, httpauth.SessionToken(r))
	c.close(websocket.StatusNormalClosure, "")
	<-written
}

func (s *Server) run(ctx context.Context, c *connection, token string) {
	p, err := s.o.Auth.Authenticate(ctx, token)
	if err != nil {
		if !errors.Is(err, domain.ErrUnauthenticated) {
			s.o.Logger.ErrorContext(ctx, "gateway authenticate", slog.Any("error", err))
		}
		c.close(closeNotAuthenticated, "sign in required")
		return
	}
	sess, err := s.handshake(ctx, c, p)
	if err != nil {
		return
	}
	readErr := s.serve(ctx, c, sess, token)
	s.release(ctx, sess, c, readErr)
}

var errClosed = errors.New("connection closed")

// read returns the next frame, closing the connection on garbage.
func (s *Server) read(ctx context.Context, c *connection) (*gatewayv1.ClientFrame, error) {
	typ, data, err := c.ws.Read(ctx)
	if err != nil {
		return nil, err
	}
	f := &gatewayv1.ClientFrame{}
	if typ != websocket.MessageBinary || proto.Unmarshal(data, f) != nil {
		c.close(closeDecodeError, "frames are binary protobuf")
		return nil, errClosed
	}
	return f, nil
}

// handshake waits for Identify or Resume; a failed resume answers
// InvalidSession and the client may identify on the same connection.
func (s *Server) handshake(ctx context.Context, c *connection, p auth.Principal) (*session, error) {
	timeout := time.AfterFunc(identifyTimeout, func() { c.close(closeNotAuthenticated, "identify timed out") })
	defer timeout.Stop()
	for {
		f, err := s.read(ctx, c)
		if err != nil {
			return nil, err
		}
		switch k := f.GetKind().(type) {
		case *gatewayv1.ClientFrame_Identify:
			return s.identify(ctx, c, p)
		case *gatewayv1.ClientFrame_Resume:
			if sess, ok := s.resume(c, p, k.Resume); ok {
				return sess, nil
			}
			c.send(invalidSessionFrame)
		case *gatewayv1.ClientFrame_Heartbeat:
			c.send(heartbeatAckFrame)
		default:
			c.close(closeNotAuthenticated, "identify first")
			return nil, errClosed
		}
	}
}

func (s *Server) identify(ctx context.Context, c *connection, p auth.Principal) (*session, error) {
	id, err := s.o.Tokens.NewToken()
	if err != nil {
		c.close(websocket.StatusInternalError, "")
		return nil, err
	}
	sess := &session{id: id, userID: p.Actor.UserID, authSessionID: p.Session.ID, topics: map[events.Topic]bool{}}

	// Subscribe before reading the snapshot: events in between wait in
	// pending, and anything older is already in the snapshot because events
	// are published after commit.
	s.mu.Lock()
	s.sessions[id] = sess
	if s.byUser[sess.userID] == nil {
		s.byUser[sess.userID] = map[*session]bool{}
	}
	s.byUser[sess.userID][sess] = true
	s.subscribe(sess, events.UserTopic(sess.userID))
	s.mu.Unlock()

	fail := func(err error) (*session, error) {
		s.mu.Lock()
		s.remove(sess)
		s.mu.Unlock()
		if errors.Is(err, domain.ErrFailedPrecondition) || errors.Is(err, domain.ErrUnauthenticated) {
			c.close(closeNotAuthenticated, "sign in required")
		} else {
			s.o.Logger.ErrorContext(ctx, "gateway identify", slog.Any("error", err))
			c.close(websocket.StatusInternalError, "")
		}
		return nil, err
	}
	members, err := s.o.Snapshots.Memberships(ctx, sess.userID)
	if err != nil {
		return fail(err)
	}
	s.mu.Lock()
	for _, m := range members {
		s.subscribe(sess, events.GuildTopic(m.GuildID))
		if adminRole(m.Role) {
			s.subscribe(sess, events.GuildAdminsTopic(m.GuildID))
		}
	}
	s.mu.Unlock()
	ready, err := s.o.Snapshots.Ready(ctx, p.Actor)
	if err != nil {
		return fail(err)
	}

	frame := readyFrame(id, ready)
	s.mu.Lock()
	sess.conn = c
	c.send(frame)
	sess.ready = true
	now := s.o.Clock.Now()
	for _, ev := range sess.pending {
		sess.push(ev, now, s.o.ReplayLimit, s.o.ResumeWindow)
	}
	sess.pending = nil
	s.mu.Unlock()
	s.o.Presence.Connect(ctx, ready.Me, id)
	return sess, nil
}

func (s *Server) resume(c *connection, p auth.Principal, req *gatewayv1.Resume) (*session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.sessions[req.GetSessionId()]
	if sess == nil || sess.userID != p.Actor.UserID || !sess.ready {
		return nil, false
	}
	frames, ok := sess.missed(req.GetLastSeq())
	if !ok {
		return nil, false
	}
	if old := sess.conn; old != nil && old != c {
		old.close(closeUnknown, "resumed on another connection")
	}
	if sess.expiry != nil {
		sess.expiry.Stop()
		sess.expiry = nil
	}
	sess.conn, sess.authSessionID = c, p.Session.ID
	c.send(resumedFrame)
	for _, f := range frames {
		c.send(f)
	}
	return sess, true
}

// bucket is a token bucket for one connection's frames.
type bucket struct {
	tokens float64
	last   time.Time
}

func (b *bucket) allow(now time.Time, l ports.Limit) bool {
	if b.last.IsZero() {
		b.tokens, b.last = float64(l.Burst), now
	}
	b.tokens = min(float64(l.Burst), b.tokens+now.Sub(b.last).Seconds()/l.Every.Seconds())
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (s *Server) serve(ctx context.Context, c *connection, sess *session, token string) error {
	var lastBeat atomic.Int64
	lastBeat.Store(time.Now().UnixNano())
	// Two missed heartbeats, plus some slack for a slow network.
	deadline := 2*s.o.Heartbeat + s.o.Heartbeat/4
	watchdog := time.NewTicker(s.o.Heartbeat / 2)
	defer watchdog.Stop()
	go func() {
		for {
			select {
			case <-c.closed:
				return
			case <-watchdog.C:
				if time.Since(time.Unix(0, lastBeat.Load())) > deadline {
					c.close(closeSessionExpired, "heartbeat timed out")
					return
				}
			}
		}
	}()

	var b bucket
	for {
		f, err := s.read(ctx, c)
		if err != nil {
			return err
		}
		if !b.allow(s.o.Clock.Now(), frameLimit) {
			c.close(closeRateLimited, "too many frames")
			return errClosed
		}
		switch k := f.GetKind().(type) {
		case *gatewayv1.ClientFrame_Heartbeat:
			lastBeat.Store(time.Now().UnixNano())
			s.mu.Lock()
			sess.ack(k.Heartbeat.GetLastSeq())
			s.mu.Unlock()
			// Each heartbeat re-checks the session, so signing out elsewhere,
			// a password change or a disabled account ends the connection.
			if _, err := s.o.Auth.Authenticate(ctx, token); err != nil {
				if errors.Is(err, domain.ErrUnauthenticated) {
					c.close(closeSessionRevoked, "session ended")
					return errClosed
				}
				s.o.Logger.WarnContext(ctx, "gateway session check", slog.Any("error", err))
			}
			c.send(heartbeatAckFrame)
		case *gatewayv1.ClientFrame_UpdatePresence:
			s.o.Presence.SetIdle(ctx, sess.userID, sess.id, k.UpdatePresence.GetIdle())
		case *gatewayv1.ClientFrame_Identify, *gatewayv1.ClientFrame_Resume:
			c.close(closeAlreadyIdentify, "already identified")
			return errClosed
		default:
			c.close(closeBadOpcode, "unknown frame")
			return errClosed
		}
	}
}

// release detaches the session from a finished connection. Resumable
// endings keep it for the resume window; the rest end it now.
func (s *Server) release(ctx context.Context, sess *session, c *connection, readErr error) {
	// The request is over; presence updates still need to go out.
	ctx = context.WithoutCancel(ctx)
	closeCode := c.closeCode()
	if closeCode == 0 {
		closeCode = websocket.CloseStatus(readErr)
	}
	s.mu.Lock()
	if sess.conn != c {
		s.mu.Unlock()
		return
	}
	sess.conn = nil
	if s.closing || !resumable(closeCode) {
		s.remove(sess)
		s.mu.Unlock()
		s.o.Presence.Disconnect(ctx, sess.userID, sess.id)
		return
	}
	sess.expiry = time.AfterFunc(s.o.ResumeWindow, func() { s.expire(ctx, sess) })
	s.mu.Unlock()
}

func (s *Server) expire(ctx context.Context, sess *session) {
	s.mu.Lock()
	if sess.conn != nil || s.sessions[sess.id] != sess {
		s.mu.Unlock()
		return
	}
	s.remove(sess)
	s.mu.Unlock()
	s.o.Presence.Disconnect(ctx, sess.userID, sess.id)
}

// remove forgets a session. Callers hold s.mu.
func (s *Server) remove(sess *session) {
	delete(s.sessions, sess.id)
	if set := s.byUser[sess.userID]; set != nil {
		delete(set, sess)
		if len(set) == 0 {
			delete(s.byUser, sess.userID)
		}
	}
	for t := range sess.topics {
		s.unsubscribeTopic(sess, t)
	}
	if sess.expiry != nil {
		sess.expiry.Stop()
	}
}

func (s *Server) subscribe(sess *session, t events.Topic) {
	sess.topics[t] = true
	if s.byTopic[t] == nil {
		s.byTopic[t] = map[*session]bool{}
	}
	s.byTopic[t][sess] = true
}

func (s *Server) unsubscribeTopic(sess *session, t events.Topic) {
	delete(sess.topics, t)
	if set := s.byTopic[t]; set != nil {
		delete(set, sess)
		if len(set) == 0 {
			delete(s.byTopic, t)
		}
	}
}

// route delivers one event. Joining a guild or becoming an admin subscribes
// before delivery; losing a guild unsubscribes after it, so the receiver
// still learns why.
func (s *Server) route(ev events.Event) {
	pb := toEvent(ev.Payload)
	if pb == nil {
		return
	}
	data, err := proto.Marshal(pb)
	if err != nil {
		s.o.Logger.Error("encode gateway event", slog.Any("error", err))
		return
	}
	now := s.o.Clock.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	switch p := ev.Payload.(type) {
	case events.GuildCreated:
		var role domain.GuildRole
		for _, m := range p.State.Members {
			if m.UserID == p.UserID {
				role = m.Role
			}
		}
		for sess := range s.byUser[p.UserID] {
			s.subscribe(sess, events.GuildTopic(p.State.Guild.ID))
			if adminRole(role) {
				s.subscribe(sess, events.GuildAdminsTopic(p.State.Guild.ID))
			}
		}
	case events.MemberUpdated:
		admins := events.GuildAdminsTopic(p.Member.GuildID)
		for sess := range s.byUser[p.Member.UserID] {
			if adminRole(p.Member.Role) {
				s.subscribe(sess, admins)
			} else {
				s.unsubscribeTopic(sess, admins)
			}
		}
	}

	recipients := map[*session]bool{}
	for _, t := range ev.Topics {
		for sess := range s.byTopic[t] {
			recipients[sess] = true
		}
	}
	for sess := range recipients {
		sess.push(data, now, s.o.ReplayLimit, s.o.ResumeWindow)
	}

	if p, ok := ev.Payload.(events.GuildDeleted); ok {
		affected := s.byUser[p.UserID]
		if p.UserID == 0 {
			affected = s.byTopic[events.GuildTopic(p.GuildID)]
		}
		for _, sess := range keys(affected) {
			s.unsubscribeTopic(sess, events.GuildTopic(p.GuildID))
			s.unsubscribeTopic(sess, events.GuildAdminsTopic(p.GuildID))
		}
	}
}

func keys(m map[*session]bool) []*session {
	out := make([]*session, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
