// Package events is the in-process pub/sub between services and the
// realtime gateway. Services publish typed payloads addressed to topics; the
// gateway delivers them to every session subscribed to one of the topics.
package events

import (
	"context"
	"sync"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// Topic names a group of receivers.
type Topic string

// UserTopic reaches every session of one user.
func UserTopic(id domain.ID) Topic { return Topic("user:" + id.String()) }

// GuildTopic reaches every member of a guild.
func GuildTopic(id domain.ID) Topic { return Topic("guild:" + id.String()) }

// GuildAdminsTopic reaches the owner and admins of a guild.
func GuildAdminsTopic(id domain.ID) Topic { return Topic("admins:" + id.String()) }

// InstanceAdminsTopic reaches the instance admins.
const InstanceAdminsTopic Topic = "instance-admins"

type Event struct {
	Topics  []Topic
	Payload Payload
}

// Payload is one of the event types below.
type Payload interface{ isPayload() }

// GuildCreated tells UserID they joined or created a guild.
type GuildCreated struct {
	UserID domain.ID
	State  domain.GuildState
}

type GuildUpdated struct{ Guild domain.Guild }

type GuildDeleteReason string

const (
	ReasonGuildDeleted GuildDeleteReason = "deleted"
	ReasonKicked       GuildDeleteReason = "kicked"
	ReasonBanned       GuildDeleteReason = "banned"
	ReasonLeft         GuildDeleteReason = "left"
)

// GuildDeleted tells receivers they can no longer see a guild: everyone in it
// when UserID is zero, otherwise that user only.
type GuildDeleted struct {
	GuildID domain.ID
	UserID  domain.ID
	Reason  GuildDeleteReason
}

type MemberAdded struct {
	Member   domain.Member
	User     domain.User
	Presence domain.Presence
}

type MemberUpdated struct{ Member domain.Member }

type MemberRemoved struct{ GuildID, UserID domain.ID }

type ChannelCreated struct{ Channel domain.Channel }

type ChannelUpdated struct{ Channel domain.Channel }

type ChannelDeleted struct{ GuildID, ChannelID domain.ID }

// ChannelsReordered carries every channel whose parent or position changed.
type ChannelsReordered struct {
	GuildID  domain.ID
	Channels []domain.Channel
}

// UserUpdated carries a changed public profile.
type UserUpdated struct{ User domain.User }

type PresenceUpdated struct{ Presence domain.Presence }

type InviteCreated struct{ Invite domain.InviteView }

type InviteDeleted struct {
	GuildID domain.ID
	Code    string
}

type BanAdded struct{ Ban domain.BanView }

type BanRemoved struct{ GuildID, UserID domain.ID }

type AuditAppended struct{ Entry domain.AuditEntry }

// MeUpdated tells a user's sessions that their own account changed.
type MeUpdated struct{ User domain.User }

// VoiceStateUpdated carries a joined, changed or ended voice state; an ended
// one has a zero ChannelID.
type VoiceStateUpdated struct{ State domain.VoiceState }

type VoiceForcedAction string

const (
	VoiceDisconnected VoiceForcedAction = "disconnected"
	VoiceMoved        VoiceForcedAction = "moved"
)

// VoiceForced tells a user that someone else ended or moved their voice
// connection.
type VoiceForced struct {
	UserID    domain.ID
	GuildID   domain.ID
	ChannelID domain.ID
	ByUserID  domain.ID
	SessionID string
	Action    VoiceForcedAction
}

// ExposureChanged tells instance admins the exposure provider's state or
// public URL changed.
type ExposureChanged struct{ Status domain.ExposureStatus }

func (ExposureChanged) isPayload()   {}
func (GuildCreated) isPayload()      {}
func (GuildUpdated) isPayload()      {}
func (GuildDeleted) isPayload()      {}
func (MemberAdded) isPayload()       {}
func (MemberUpdated) isPayload()     {}
func (MemberRemoved) isPayload()     {}
func (ChannelCreated) isPayload()    {}
func (ChannelUpdated) isPayload()    {}
func (ChannelDeleted) isPayload()    {}
func (ChannelsReordered) isPayload() {}
func (UserUpdated) isPayload()       {}
func (PresenceUpdated) isPayload()   {}
func (InviteCreated) isPayload()     {}
func (InviteDeleted) isPayload()     {}
func (BanAdded) isPayload()          {}
func (BanRemoved) isPayload()        {}
func (AuditAppended) isPayload()     {}
func (MeUpdated) isPayload()         {}
func (VoiceStateUpdated) isPayload() {}
func (VoiceForced) isPayload()       {}

// Bus delivers events to subscribers synchronously and in publish order;
// subscribers must not block.
type Bus struct {
	mu   sync.Mutex
	subs map[int]func(Event)
	next int
}

func NewBus() *Bus { return &Bus{subs: map[int]func(Event){}} }

func (b *Bus) Publish(evs ...Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ev := range evs {
		for _, fn := range b.subs {
			fn(ev)
		}
	}
}

// Subscribe registers fn and returns a function that removes it.
func (b *Bus) Subscribe(fn func(Event)) (cancel func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	b.subs[id] = fn
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs, id)
	}
}

// CommitHooks defers work until the transaction in ctx commits.
type CommitHooks interface {
	AfterCommit(ctx context.Context, fn func())
}

// Publisher publishes once the caller's transaction commits, so receivers
// never see a change that was rolled back, and a snapshot read after an
// event always includes it.
type Publisher struct {
	Bus   *Bus
	Hooks CommitHooks
}

func (p Publisher) Publish(ctx context.Context, evs ...Event) {
	if len(evs) == 0 {
		return
	}
	p.Hooks.AfterCommit(ctx, func() { p.Bus.Publish(evs...) })
}
