package gateway

import (
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/gateway/v1"
	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/snapshot"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/wire"
)

// Field numbers of ServerFrame.dispatch, Dispatch.seq and Dispatch.event in
// gateway.proto. A dispatch is encoded by hand so the event is marshaled once
// for all receivers, each with their own seq.
const (
	fieldServerFrameDispatch = 6
	fieldDispatchSeq         = 1
	fieldDispatchEvent       = 2
)

func dispatchFrame(seq uint64, event []byte) []byte {
	d := protowire.AppendTag(nil, fieldDispatchSeq, protowire.VarintType)
	d = protowire.AppendVarint(d, seq)
	d = protowire.AppendTag(d, fieldDispatchEvent, protowire.BytesType)
	d = protowire.AppendBytes(d, event)
	f := protowire.AppendTag(make([]byte, 0, len(d)+8), fieldServerFrameDispatch, protowire.BytesType)
	return protowire.AppendBytes(f, d)
}

func marshal(f *gatewayv1.ServerFrame) []byte {
	// Marshaling a message the hub built cannot fail.
	b, _ := proto.Marshal(f)
	return b
}

func helloFrame(heartbeatMS, resumeMS int) []byte {
	return marshal(&gatewayv1.ServerFrame{Kind: &gatewayv1.ServerFrame_Hello{Hello: &gatewayv1.Hello{
		HeartbeatIntervalMs: wire.ClampInt32(heartbeatMS),
		ResumeWindowMs:      wire.ClampInt32(resumeMS),
	}}})
}

func readyFrame(sessionID string, r snapshot.Ready) []byte {
	l := r.Instance.Limits
	guilds := make([]*dislacksov1.GuildState, len(r.Guilds))
	for i, g := range r.Guilds {
		guilds[i] = wire.GuildState(g)
	}
	return marshal(&gatewayv1.ServerFrame{Kind: &gatewayv1.ServerFrame_Ready{Ready: &gatewayv1.Ready{
		SessionId: sessionID,
		Me:        wire.Me(r.Me),
		Instance: &gatewayv1.InstanceInfo{
			Name:                  r.Instance.Name,
			MembersCanInvite:      r.Instance.MembersCanInvite,
			MaxGuildsOwnedPerUser: wire.ClampInt32(l.MaxGuildsOwnedPerUser),
			MaxChannelsPerGuild:   wire.ClampInt32(l.MaxChannelsPerGuild),
			MaxGuildMembers:       wire.ClampInt32(l.MaxGuildMembers),
			DefaultVoiceUserLimit: wire.ClampInt32(l.DefaultVoiceUserLimit),
			MaxUploadMb:           wire.ClampInt32(l.MaxUploadMB),
		},
		Guilds: guilds,
	}}})
}

var (
	resumedFrame        = marshal(&gatewayv1.ServerFrame{Kind: &gatewayv1.ServerFrame_Resumed{Resumed: &gatewayv1.Resumed{}}})
	invalidSessionFrame = marshal(&gatewayv1.ServerFrame{Kind: &gatewayv1.ServerFrame_InvalidSession{InvalidSession: &gatewayv1.InvalidSession{}}})
	heartbeatAckFrame   = marshal(&gatewayv1.ServerFrame{Kind: &gatewayv1.ServerFrame_HeartbeatAck{HeartbeatAck: &gatewayv1.HeartbeatAck{}}})
)

var guildDeleteReasons = map[events.GuildDeleteReason]gatewayv1.GuildDeleteReason{
	events.ReasonGuildDeleted: gatewayv1.GuildDeleteReason_GUILD_DELETE_REASON_DELETED,
	events.ReasonKicked:       gatewayv1.GuildDeleteReason_GUILD_DELETE_REASON_KICKED,
	events.ReasonBanned:       gatewayv1.GuildDeleteReason_GUILD_DELETE_REASON_BANNED,
	events.ReasonLeft:         gatewayv1.GuildDeleteReason_GUILD_DELETE_REASON_LEFT,
}

// toEvent maps a payload to its wire event; payloads the gateway does not
// forward return nil.
func toEvent(p events.Payload) *gatewayv1.Event {
	ev := &gatewayv1.Event{}
	switch p := p.(type) {
	case events.GuildCreated:
		ev.Kind = &gatewayv1.Event_GuildCreate{GuildCreate: wire.GuildState(p.State)}
	case events.GuildUpdated:
		ev.Kind = &gatewayv1.Event_GuildUpdate{GuildUpdate: wire.Guild(p.Guild)}
	case events.GuildDeleted:
		ev.Kind = &gatewayv1.Event_GuildDelete{GuildDelete: &gatewayv1.GuildDelete{
			GuildId: p.GuildID.String(), Reason: guildDeleteReasons[p.Reason],
		}}
	case events.MemberAdded:
		ev.Kind = &gatewayv1.Event_MemberAdd{MemberAdd: &gatewayv1.MemberAdd{
			Member: wire.Member(p.Member), User: wire.PublicUser(p.User), Presence: wire.Presence(p.Presence),
		}}
	case events.MemberUpdated:
		ev.Kind = &gatewayv1.Event_MemberUpdate{MemberUpdate: wire.Member(p.Member)}
	case events.MemberRemoved:
		ev.Kind = &gatewayv1.Event_MemberRemove{MemberRemove: &gatewayv1.MemberRemove{
			GuildId: p.GuildID.String(), UserId: p.UserID.String(),
		}}
	case events.ChannelCreated:
		ev.Kind = &gatewayv1.Event_ChannelCreate{ChannelCreate: wire.Channel(p.Channel)}
	case events.ChannelUpdated:
		ev.Kind = &gatewayv1.Event_ChannelUpdate{ChannelUpdate: wire.Channel(p.Channel)}
	case events.ChannelDeleted:
		ev.Kind = &gatewayv1.Event_ChannelDelete{ChannelDelete: &gatewayv1.ChannelDelete{
			GuildId: p.GuildID.String(), ChannelId: p.ChannelID.String(),
		}}
	case events.ChannelsReordered:
		ev.Kind = &gatewayv1.Event_ChannelsReordered{ChannelsReordered: &gatewayv1.ChannelsReordered{
			GuildId: p.GuildID.String(), Channels: wire.Channels(p.Channels),
		}}
	case events.UserUpdated:
		ev.Kind = &gatewayv1.Event_UserUpdate{UserUpdate: wire.PublicUser(p.User)}
	case events.PresenceUpdated:
		ev.Kind = &gatewayv1.Event_PresenceUpdate{PresenceUpdate: wire.Presence(p.Presence)}
	case events.InviteCreated:
		ev.Kind = &gatewayv1.Event_InviteCreate{InviteCreate: wire.Invite(p.Invite)}
	case events.InviteDeleted:
		ev.Kind = &gatewayv1.Event_InviteDelete{InviteDelete: &gatewayv1.InviteDelete{
			GuildId: p.GuildID.String(), Code: p.Code,
		}}
	case events.BanAdded:
		ev.Kind = &gatewayv1.Event_BanAdd{BanAdd: wire.Ban(p.Ban)}
	case events.BanRemoved:
		ev.Kind = &gatewayv1.Event_BanRemove{BanRemove: &gatewayv1.BanRemove{
			GuildId: p.GuildID.String(), UserId: p.UserID.String(),
		}}
	case events.AuditAppended:
		ev.Kind = &gatewayv1.Event_AuditEntry{AuditEntry: wire.AuditEntry(p.Entry)}
	case events.MeUpdated:
		ev.Kind = &gatewayv1.Event_MeUpdate{MeUpdate: wire.Me(p.User)}
	default:
		return nil
	}
	return ev
}

// adminRole reports whether the role receives the guild's admin-only events.
func adminRole(r domain.GuildRole) bool { return r == domain.RoleOwner || r == domain.RoleAdmin }
