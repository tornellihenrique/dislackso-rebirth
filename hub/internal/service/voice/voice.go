// Package voice keeps who is in which voice channel. The hub is the
// authority: joins go through it and mint the media token, and media server
// events only confirm or end connections. Anyone in a voice room without a
// matching voice state is removed from it.
package voice

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/access"
)

const (
	// tokenTTL only bounds the first connection: the media server refreshes
	// tokens of connected participants.
	tokenTTL = 10 * time.Minute
	iceTTL   = 12 * time.Hour
	// ConfirmTimeout ends a join the media server has not confirmed.
	ConfirmTimeout = 30 * time.Second
	sweepInterval  = 5 * time.Second
	// mediaCallTimeout bounds calls to the media server made on behalf of a
	// request that already returned or an event.
	mediaCallTimeout = 10 * time.Second
	inboxSize        = 256
)

var limitJoin = ports.Limit{Burst: 10, Every: 2 * time.Second}

type SettingsReader interface {
	Get(ctx context.Context) (domain.InstanceSettings, error)
}

type Subscriber interface {
	Subscribe(fn func(events.Event)) (cancel func())
}

type Deps struct {
	Guilds       ports.GuildRepo
	Members      ports.MemberRepo
	Channels     ports.ChannelRepo
	Users        ports.UserRepo
	Audit        ports.AuditRepo
	Tx           ports.TxRunner
	IDs          ports.IDGen
	Tokens       ports.TokenGen
	Clock        ports.Clock
	Limiter      ports.RateLimiter
	Events       ports.EventPublisher
	Bus          Subscriber
	Media        ports.MediaProvider
	ICE          ports.IceServerProvider
	Settings     SettingsReader
	Presence     access.Presences
	Verification access.Verification
	Logger       *slog.Logger
}

type entry struct {
	state domain.VoiceState
	// sid is the media server's id for the confirmed connection.
	sid string
	// pendingSince is when the unconfirmed connection started.
	pendingSince time.Time
	// moved lets the user into the channel an admin moved them to even when
	// it is full.
	moved bool
}

type Service struct {
	d       Deps
	access  access.Guilds
	auditor access.Auditor
	policy  domain.Policy

	// mu also serializes publishing, so receivers see voice changes in order.
	mu     sync.Mutex
	states map[domain.ID]*entry
	inbox  chan events.Event
}

func New(d Deps) *Service {
	return &Service{
		d: d,
		access: access.Guilds{
			Guilds: d.Guilds, Members: d.Members, Channels: d.Channels, Users: d.Users,
			Presence: d.Presence, Verification: d.Verification,
		},
		auditor: access.Auditor{Repo: d.Audit, IDs: d.IDs, Clock: d.Clock, Events: d.Events},
		states:  map[domain.ID]*entry{},
		inbox:   make(chan events.Event, inboxSize),
	}
}

// Join is what a client needs to connect to the media server.
type Join struct {
	URL               string
	Token             string
	ICE               []ports.IceServer
	State             domain.VoiceState
	MicBitrateMaxKbps int
}

// Join connects the caller to a voice channel, replacing any voice
// connection they had, on any device.
func (s *Service) Join(ctx context.Context, actor domain.Actor, channelID domain.ID, selfMute, selfDeaf bool) (Join, error) {
	if ok, retry := s.d.Limiter.Allow("voice-join:user:"+actor.UserID.String(), limitJoin); !ok {
		return Join{}, domain.RateLimited(retry)
	}
	c, m, err := s.access.Channel(ctx, actor, channelID)
	if err != nil {
		return Join{}, err
	}
	if c.Type != domain.ChannelVoice {
		return Join{}, domain.Precondition(domain.ReasonNotVoiceChannel)
	}
	if !s.d.Media.Available() {
		return Join{}, domain.Unavailable(domain.ReasonVoiceUnavailable)
	}
	settings, err := s.d.Settings.Get(ctx)
	if err != nil {
		return Join{}, err
	}
	sessionID, err := s.d.Tokens.NewToken()
	if err != nil {
		return Join{}, err
	}
	token, err := s.d.Media.JoinToken(ports.MediaJoin{
		Room:        domain.VoiceRoom(c.ID),
		Identity:    actor.UserID.String(),
		Metadata:    sessionID,
		Permissions: domain.VoicePermissionsFor(m.ServerMute, m.ServerDeaf),
		TTL:         tokenTTL,
	})
	if err != nil {
		return Join{}, fmt.Errorf("mint voice token: %w", err)
	}
	ice, err := s.d.ICE.Issue(ctx, actor.UserID, iceTTL)
	if err != nil {
		s.d.Logger.WarnContext(ctx, "issue ice servers", slog.String("provider", s.d.ICE.Name()), slog.Any("error", err))
		ice = nil
	}

	now := s.d.Clock.Now()
	state := domain.VoiceState{
		UserID: actor.UserID, GuildID: c.GuildID, ChannelID: c.ID, SessionID: sessionID,
		SelfMute: selfMute || selfDeaf, SelfDeaf: selfDeaf,
		ServerMute: m.ServerMute, ServerDeaf: m.ServerDeaf, JoinedAt: now,
	}
	s.mu.Lock()
	prev := s.states[actor.UserID]
	movedHere := prev != nil && prev.moved && prev.state.ChannelID == c.ID
	if c.UserLimit > 0 && !movedHere && !s.policy.Can(m.Role, domain.ActModerateVoice, false) &&
		s.occupants(c.ID, actor.UserID) >= c.UserLimit {
		s.mu.Unlock()
		return Join{}, domain.Precondition(domain.ReasonChannelFull)
	}
	s.states[actor.UserID] = &entry{state: state, pendingSince: now}
	if prev != nil && prev.state.GuildID != c.GuildID {
		s.publish(ctx, ended(prev.state))
	}
	s.publish(ctx, state)
	s.mu.Unlock()

	// In the same room the media server replaces the old participant when
	// the new one connects; removing it here could hit the new one instead.
	if prev != nil && prev.state.ChannelID != c.ID {
		s.removeParticipant(ctx, prev.state.ChannelID, actor.UserID)
	}
	return Join{
		URL: s.d.Media.URL(), Token: token, ICE: ice, State: state,
		MicBitrateMaxKbps: settings.Limits.MicBitrateMaxKbps,
	}, nil
}

// occupants counts the voice states in a channel, leaving out one user.
func (s *Service) occupants(channelID, except domain.ID) int {
	n := 0
	for id, e := range s.states {
		if id != except && e.state.ChannelID == channelID {
			n++
		}
	}
	return n
}

// Leave ends the caller's voice connection if sessionID (when given) is
// still theirs.
func (s *Service) Leave(ctx context.Context, actor domain.Actor, sessionID string) error {
	s.mu.Lock()
	e := s.states[actor.UserID]
	if e == nil || (sessionID != "" && e.state.SessionID != sessionID) {
		s.mu.Unlock()
		return nil
	}
	old := e.state
	s.end(ctx, old)
	s.mu.Unlock()
	// A client that left but kept its media connection would listen in
	// unseen.
	s.removeParticipant(ctx, old.ChannelID, old.UserID)
	return nil
}

// UpdateSelf records the client's own mute and deafen for its session.
func (s *Service) UpdateSelf(ctx context.Context, userID domain.ID, sessionID string, mute, deaf bool) {
	mute = mute || deaf
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.states[userID]
	if e == nil || e.state.SessionID != sessionID || (e.state.SelfMute == mute && e.state.SelfDeaf == deaf) {
		return
	}
	e.state.SelfMute, e.state.SelfDeaf = mute, deaf
	s.publish(ctx, e.state)
}

// GuildVoiceStates lists everyone in a guild's voice channels, earliest first.
func (s *Service) GuildVoiceStates(guildID domain.ID) []domain.VoiceState {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.VoiceState
	for _, e := range s.states {
		if e.state.GuildID == guildID {
			out = append(out, e.state)
		}
	}
	slices.SortFunc(out, func(a, b domain.VoiceState) int {
		return cmp.Or(a.JoinedAt.Compare(b.JoinedAt), cmp.Compare(a.UserID, b.UserID))
	})
	return out
}

// SetServerMute sets whether a member may speak in the guild's voice
// channels; it holds across reconnects until lifted.
func (s *Service) SetServerMute(ctx context.Context, actor domain.Actor, guildID, userID domain.ID, mute bool) (domain.Member, error) {
	return s.moderate(ctx, actor, guildID, userID, func(m *domain.Member) (string, bool) {
		m.ServerMute = mute
		return "server_mute", mute
	})
}

// SetServerDeafen sets whether a member hears anyone in the guild's voice channels.
func (s *Service) SetServerDeafen(ctx context.Context, actor domain.Actor, guildID, userID domain.ID, deaf bool) (domain.Member, error) {
	return s.moderate(ctx, actor, guildID, userID, func(m *domain.Member) (string, bool) {
		m.ServerDeaf = deaf
		return "server_deaf", deaf
	})
}

func (s *Service) moderate(ctx context.Context, actor domain.Actor, guildID, userID domain.ID, apply func(*domain.Member) (string, bool)) (domain.Member, error) {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return domain.Member{}, err
	}
	target, err := s.access.Target(ctx, guildID, userID)
	if err != nil {
		return domain.Member{}, err
	}
	if err := s.policy.TargetMember(m, target, domain.ActModerateVoice); err != nil {
		return domain.Member{}, err
	}
	next := target
	key, value := apply(&next)
	if next.ServerMute == target.ServerMute && next.ServerDeaf == target.ServerDeaf {
		return target, nil
	}
	next.UpdatedAt = s.d.Clock.Now()
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Members.UpdateMemberVoiceModeration(ctx, guildID, userID, next.ServerMute, next.ServerDeaf, next.UpdatedAt); err != nil {
			return err
		}
		if err := s.record(ctx, actor, guildID, userID, domain.AuditMemberVoiceUpdate, map[string]string{key: strconv.FormatBool(value)}); err != nil {
			return err
		}
		s.d.Events.Publish(ctx, events.Event{
			Topics:  []events.Topic{events.GuildTopic(guildID)},
			Payload: events.MemberUpdated{Member: next},
		})
		return nil
	})
	if err != nil {
		return domain.Member{}, err
	}

	s.mu.Lock()
	e := s.states[userID]
	live := e != nil && e.state.GuildID == guildID
	var channelID domain.ID
	if live {
		e.state.ServerMute, e.state.ServerDeaf = next.ServerMute, next.ServerDeaf
		channelID = e.state.ChannelID
		s.publish(ctx, e.state)
	}
	s.mu.Unlock()
	if live {
		s.enforce(ctx, channelID, userID, next.ServerMute, next.ServerDeaf)
	}
	return next, nil
}

// enforce has the media server apply server mute and deafen to a connected
// participant, so a modified client cannot get around them: the media server
// drops a microphone track that is no longer allowed and refuses new ones.
func (s *Service) enforce(ctx context.Context, channelID, userID domain.ID, mute, deaf bool) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mediaCallTimeout)
	defer cancel()
	room, identity := domain.VoiceRoom(channelID), userID.String()
	if err := s.d.Media.SetPermissions(ctx, room, identity, domain.VoicePermissionsFor(mute, deaf)); err != nil {
		s.d.Logger.WarnContext(ctx, "apply voice permissions", slog.String("room", room), slog.String("user_id", identity), slog.Any("error", err))
	}
}

// MoveMember sends a member who is in voice to another voice channel of the
// guild. The member's client connects there with a new join; the session
// stays, so the client knows the move is about its own connection.
func (s *Service) MoveMember(ctx context.Context, actor domain.Actor, guildID, userID, channelID domain.ID) error {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return err
	}
	target, err := s.access.Target(ctx, guildID, userID)
	if err != nil {
		return err
	}
	if err := s.policy.TargetMember(m, target, domain.ActModerateVoice); err != nil {
		return err
	}
	c, err := s.d.Channels.ChannelByID(ctx, channelID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && c.GuildID != guildID) {
		return domain.NotFound(domain.ReasonChannelNotFound)
	}
	if err != nil {
		return err
	}
	if c.Type != domain.ChannelVoice {
		return domain.Precondition(domain.ReasonNotVoiceChannel)
	}

	s.mu.Lock()
	e := s.states[userID]
	if e == nil || e.state.GuildID != guildID {
		s.mu.Unlock()
		return domain.NotFound(domain.ReasonNotInVoice)
	}
	if e.state.ChannelID == channelID {
		s.mu.Unlock()
		return nil
	}
	from := e.state.ChannelID
	e.state.ChannelID, e.state.Connected, e.state.Streaming = channelID, false, false
	e.pendingSince, e.moved = s.d.Clock.Now(), true
	s.forced(ctx, e.state, actor.UserID, events.VoiceMoved)
	s.publish(ctx, e.state)
	s.mu.Unlock()

	s.removeParticipant(ctx, from, userID)
	return s.record(ctx, actor, guildID, userID, domain.AuditMemberVoiceMove,
		map[string]string{"channel_id": c.ID.String(), "name": c.Name})
}

// DisconnectMember drops a member from voice.
func (s *Service) DisconnectMember(ctx context.Context, actor domain.Actor, guildID, userID domain.ID) error {
	m, err := s.access.Member(ctx, actor, guildID)
	if err != nil {
		return err
	}
	target, err := s.access.Target(ctx, guildID, userID)
	if err != nil {
		return err
	}
	if err := s.policy.TargetMember(m, target, domain.ActModerateVoice); err != nil {
		return err
	}
	s.mu.Lock()
	e := s.states[userID]
	if e == nil || e.state.GuildID != guildID {
		s.mu.Unlock()
		return domain.NotFound(domain.ReasonNotInVoice)
	}
	old := e.state
	s.forced(ctx, old, actor.UserID, events.VoiceDisconnected)
	s.end(ctx, old)
	s.mu.Unlock()

	s.removeParticipant(ctx, old.ChannelID, userID)
	return s.record(ctx, actor, guildID, userID, domain.AuditMemberVoiceDrop, nil)
}

// HandleMediaEvent reconciles voice state with what the media server reports.
func (s *Service) HandleMediaEvent(ctx context.Context, ev ports.MediaEvent) {
	channelID, ok := domain.ParseVoiceRoom(ev.Room)
	if !ok {
		return
	}
	userID, err := domain.ParseID(ev.Participant.Identity, "identity")
	if err != nil {
		return
	}
	switch ev.Kind {
	case ports.MediaParticipantJoined:
		s.confirm(ctx, channelID, userID, ev.Participant)
	case ports.MediaParticipantLeft:
		s.mu.Lock()
		if e := s.states[userID]; e != nil && s.matches(e, channelID, ev.Participant) {
			s.end(ctx, e.state)
		}
		s.mu.Unlock()
	case ports.MediaTrackPublished, ports.MediaTrackUnpublished:
		s.track(ctx, channelID, userID, ev)
	}
}

func (s *Service) matches(e *entry, channelID domain.ID, p ports.MediaParticipant) bool {
	return e.state.ChannelID == channelID && e.state.SessionID == p.Metadata
}

// tracks reports whether a track event's participant (known only by SID)
// is the user's confirmed connection.
func (s *Service) tracks(e *entry, channelID domain.ID, p ports.MediaParticipant) bool {
	return e.state.ChannelID == channelID && e.sid != "" && e.sid == p.SID
}

func (s *Service) track(ctx context.Context, channelID, userID domain.ID, ev ports.MediaEvent) {
	published := ev.Kind == ports.MediaTrackPublished
	s.mu.Lock()
	e := s.states[userID]
	if e == nil || !s.tracks(e, channelID, ev.Participant) {
		s.mu.Unlock()
		return
	}
	mute, deaf := e.state.ServerMute, e.state.ServerDeaf
	if ev.Source == domain.SourceScreenShare && e.state.Streaming != published {
		e.state.Streaming = published
		s.publish(ctx, e.state)
	}
	s.mu.Unlock()
	if published && ev.Source == domain.SourceMicrophone && mute {
		s.enforce(ctx, channelID, userID, mute, deaf)
	}
}

// confirm marks a join as connected. A participant whose session is not
// the user's current one is an older connection that came back and is
// removed; one the hub has no state for is adopted, which rebuilds voice
// state after a hub restart.
func (s *Service) confirm(ctx context.Context, channelID, userID domain.ID, p ports.MediaParticipant) {
	s.mu.Lock()
	e := s.states[userID]
	if e != nil && s.matches(e, channelID, p) {
		state := s.connected(ctx, e, p)
		s.mu.Unlock()
		// The token predates any server mute applied while connecting.
		if state.ServerMute || state.ServerDeaf {
			s.enforce(ctx, channelID, userID, state.ServerMute, state.ServerDeaf)
		}
		return
	}
	s.mu.Unlock()
	if e != nil {
		s.removeParticipant(ctx, channelID, userID)
		return
	}
	s.adopt(ctx, channelID, userID, p)
}

func (s *Service) connected(ctx context.Context, e *entry, p ports.MediaParticipant) domain.VoiceState {
	streaming := slices.Contains(p.Sources, domain.SourceScreenShare)
	e.sid = p.SID
	if !e.state.Connected || e.state.Streaming != streaming {
		e.state.Connected, e.state.Streaming = true, streaming
		e.pendingSince, e.moved = time.Time{}, false
		s.publish(ctx, e.state)
	}
	return e.state
}

func (s *Service) adopt(ctx context.Context, channelID, userID domain.ID, p ports.MediaParticipant) {
	c, err := s.d.Channels.ChannelByID(ctx, channelID)
	if err == nil && c.Type != domain.ChannelVoice {
		err = domain.ErrNotFound
	}
	var m domain.Member
	if err == nil {
		m, err = s.d.Members.MemberByID(ctx, c.GuildID, userID)
	}
	if errors.Is(err, domain.ErrNotFound) {
		s.removeParticipant(ctx, channelID, userID)
		return
	}
	if err != nil {
		s.d.Logger.WarnContext(ctx, "adopt voice participant", slog.String("user_id", userID.String()), slog.Any("error", err))
		return
	}
	s.mu.Lock()
	if _, taken := s.states[userID]; taken {
		s.mu.Unlock()
		return
	}
	e := &entry{state: domain.VoiceState{
		UserID: userID, GuildID: c.GuildID, ChannelID: c.ID, SessionID: p.Metadata,
		ServerMute: m.ServerMute, ServerDeaf: m.ServerDeaf, JoinedAt: s.d.Clock.Now(),
	}}
	s.states[userID] = e
	state := s.connected(ctx, e, p)
	s.mu.Unlock()
	if state.ServerMute || state.ServerDeaf {
		s.enforce(ctx, channelID, userID, state.ServerMute, state.ServerDeaf)
	}
}

// Reconcile adopts everyone already in a voice room, after the hub starts.
func (s *Service) Reconcile(ctx context.Context) error {
	rooms, err := s.d.Media.ListRooms(ctx)
	if err != nil {
		return fmt.Errorf("list voice rooms: %w", err)
	}
	for _, room := range rooms {
		channelID, ok := domain.ParseVoiceRoom(room)
		if !ok {
			continue
		}
		participants, err := s.d.Media.ListParticipants(ctx, room)
		if err != nil {
			return fmt.Errorf("list participants of %s: %w", room, err)
		}
		for _, p := range participants {
			userID, err := domain.ParseID(p.Identity, "identity")
			if err != nil || !p.Active {
				continue
			}
			s.confirm(ctx, channelID, userID, p)
		}
	}
	return nil
}

// MediaRestarted puts every connection back to unconfirmed after the media
// server lost its rooms. Clients reconnect on their own; the rest expire.
func (s *Service) MediaRestarted(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.d.Clock.Now()
	for _, e := range s.states {
		if e.state.Connected {
			e.state.Connected, e.state.Streaming = false, false
			e.pendingSince = now
			s.publish(ctx, e.state)
		}
	}
}

// Sweep ends joins that were not confirmed in time.
func (s *Service) Sweep(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.d.Clock.Now()
	for _, e := range s.states {
		if !e.state.Connected && now.Sub(e.pendingSince) >= ConfirmTimeout {
			s.end(ctx, e.state)
		}
	}
}

// Run ends voice connections that lose their guild, channel or membership,
// and sweeps unconfirmed joins, until ctx is done.
func (s *Service) Run(ctx context.Context) {
	cancel := s.d.Bus.Subscribe(func(ev events.Event) {
		switch p := ev.Payload.(type) {
		case events.MemberRemoved, events.ChannelDeleted:
		case events.GuildDeleted:
			if p.UserID != 0 {
				return
			}
		default:
			return
		}
		// The bus calls subscribers while publishing, so the work happens
		// on the Run goroutine.
		select {
		case s.inbox <- ev:
		default:
			s.d.Logger.WarnContext(ctx, "voice event queue full; dropping event")
		}
	})
	defer cancel()
	tick := time.NewTicker(sweepInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-s.inbox:
			s.cleanup(ctx, ev.Payload)
		case <-tick.C:
			s.Sweep(ctx)
		}
	}
}

func (s *Service) cleanup(ctx context.Context, p events.Payload) {
	var gone func(domain.VoiceState) bool
	switch p := p.(type) {
	case events.MemberRemoved:
		gone = func(v domain.VoiceState) bool { return v.GuildID == p.GuildID && v.UserID == p.UserID }
	case events.GuildDeleted:
		gone = func(v domain.VoiceState) bool { return v.GuildID == p.GuildID }
	case events.ChannelDeleted:
		gone = func(v domain.VoiceState) bool { return v.ChannelID == p.ChannelID }
	default:
		return
	}
	s.mu.Lock()
	var ended []domain.VoiceState
	for _, e := range s.states {
		if gone(e.state) {
			ended = append(ended, e.state)
		}
	}
	for _, v := range ended {
		s.end(ctx, v)
	}
	s.mu.Unlock()
	for _, v := range ended {
		s.removeParticipant(ctx, v.ChannelID, v.UserID)
	}
}

// end drops the user's voice state and tells the guild. Callers hold mu.
func (s *Service) end(ctx context.Context, v domain.VoiceState) {
	delete(s.states, v.UserID)
	s.publish(ctx, ended(v))
}

func ended(v domain.VoiceState) domain.VoiceState {
	return domain.VoiceState{UserID: v.UserID, GuildID: v.GuildID, SessionID: v.SessionID}
}

// publish tells the guild, and the user's own sessions (who may no longer
// be in it), about a voice state. Callers hold mu.
func (s *Service) publish(ctx context.Context, v domain.VoiceState) {
	s.d.Events.Publish(ctx, events.Event{
		Topics:  []events.Topic{events.GuildTopic(v.GuildID), events.UserTopic(v.UserID)},
		Payload: events.VoiceStateUpdated{State: v},
	})
}

// forced tells the user their connection was ended or moved. Callers hold mu.
func (s *Service) forced(ctx context.Context, v domain.VoiceState, by domain.ID, action events.VoiceForcedAction) {
	s.d.Events.Publish(ctx, events.Event{
		Topics: []events.Topic{events.UserTopic(v.UserID)},
		Payload: events.VoiceForced{
			UserID: v.UserID, GuildID: v.GuildID, ChannelID: v.ChannelID, ByUserID: by,
			SessionID: v.SessionID, Action: action,
		},
	})
}

func (s *Service) removeParticipant(ctx context.Context, channelID, userID domain.ID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mediaCallTimeout)
	defer cancel()
	room := domain.VoiceRoom(channelID)
	if err := s.d.Media.RemoveParticipant(ctx, room, userID.String()); err != nil && !errors.Is(err, domain.ErrNotFound) {
		s.d.Logger.WarnContext(ctx, "remove voice participant", slog.String("room", room), slog.String("user_id", userID.String()), slog.Any("error", err))
	}
}

func (s *Service) record(ctx context.Context, actor domain.Actor, guildID, targetID domain.ID, action domain.AuditAction, data map[string]string) error {
	return s.auditor.Record(ctx, access.Entry{
		GuildID: guildID, ActorID: actor.UserID, Action: action,
		TargetType: domain.AuditTargetUser, TargetID: targetID.String(), Data: data,
	})
}
