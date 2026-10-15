// Package wire maps domain types to their protobuf form for every transport
// (RPC responses and gateway events), so each entity has one wire shape.
package wire

import (
	"math"
	"strconv"
	"time"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// PublicUser is the only place a user becomes visible to other users; any
// field added here becomes public.
func PublicUser(u domain.User) *dislacksov1.User {
	return &dislacksov1.User{
		Id:          u.ID.String(),
		Username:    u.Username,
		DisplayName: u.DisplayName,
		AvatarUrl:   domain.FileURL(u.AvatarFileID, "avatar"),
		UpdatedAt:   u.UpdatedAt.UnixMilli(),
	}
}

func Users(us []domain.User) []*dislacksov1.User {
	out := make([]*dislacksov1.User, len(us))
	for i, u := range us {
		out[i] = PublicUser(u)
	}
	return out
}

func Me(u domain.User) *dislacksov1.Me {
	return &dislacksov1.Me{
		User:          PublicUser(u),
		Email:         u.Email,
		EmailVerified: u.EmailVerified(),
		Status:        StatusToProto(u.StatusPref),
		CustomStatus:  u.CustomStatus,
		InstanceAdmin: u.IsInstanceAdmin,
		CreatedAt:     u.CreatedAt.UnixMilli(),
	}
}

var statuses = map[domain.StatusPref]dislacksov1.StatusPreference{
	domain.StatusOnline:    dislacksov1.StatusPreference_STATUS_PREFERENCE_ONLINE,
	domain.StatusIdle:      dislacksov1.StatusPreference_STATUS_PREFERENCE_IDLE,
	domain.StatusDND:       dislacksov1.StatusPreference_STATUS_PREFERENCE_DND,
	domain.StatusInvisible: dislacksov1.StatusPreference_STATUS_PREFERENCE_INVISIBLE,
}

func StatusToProto(s domain.StatusPref) dislacksov1.StatusPreference { return statuses[s] }

// StatusFromProto maps unknown values to an invalid preference the service rejects.
func StatusFromProto(s dislacksov1.StatusPreference) domain.StatusPref {
	return reverse(statuses, s)
}

var presences = map[domain.PresenceStatus]dislacksov1.PresenceStatus{
	domain.PresenceOffline: dislacksov1.PresenceStatus_PRESENCE_STATUS_OFFLINE,
	domain.PresenceOnline:  dislacksov1.PresenceStatus_PRESENCE_STATUS_ONLINE,
	domain.PresenceIdle:    dislacksov1.PresenceStatus_PRESENCE_STATUS_IDLE,
	domain.PresenceDND:     dislacksov1.PresenceStatus_PRESENCE_STATUS_DND,
}

func Presence(p domain.Presence) *dislacksov1.Presence {
	return &dislacksov1.Presence{UserId: p.UserID.String(), Status: presences[p.Status], CustomStatus: p.CustomStatus}
}

func Presences(ps []domain.Presence) []*dislacksov1.Presence {
	out := make([]*dislacksov1.Presence, len(ps))
	for i, p := range ps {
		out[i] = Presence(p)
	}
	return out
}

var roles = map[domain.GuildRole]dislacksov1.GuildRole{
	domain.RoleOwner:  dislacksov1.GuildRole_GUILD_ROLE_OWNER,
	domain.RoleAdmin:  dislacksov1.GuildRole_GUILD_ROLE_ADMIN,
	domain.RoleMember: dislacksov1.GuildRole_GUILD_ROLE_MEMBER,
}

// RoleFromProto maps unknown values to an invalid role the service rejects.
func RoleFromProto(r dislacksov1.GuildRole) domain.GuildRole { return reverse(roles, r) }

func Guild(g domain.Guild) *dislacksov1.Guild {
	return &dislacksov1.Guild{
		Id:        g.ID.String(),
		Name:      g.Name,
		IconUrl:   domain.FileURL(g.IconFileID, "icon"),
		OwnerId:   g.OwnerID.String(),
		CreatedAt: g.CreatedAt.UnixMilli(),
		UpdatedAt: g.UpdatedAt.UnixMilli(),
	}
}

func Member(m domain.Member) *dislacksov1.Member {
	return &dislacksov1.Member{
		GuildId:    m.GuildID.String(),
		UserId:     m.UserID.String(),
		Role:       roles[m.Role],
		JoinedAt:   m.JoinedAt.UnixMilli(),
		UpdatedAt:  m.UpdatedAt.UnixMilli(),
		ServerMute: m.ServerMute,
		ServerDeaf: m.ServerDeaf,
	}
}

func Members(ms []domain.Member) []*dislacksov1.Member {
	out := make([]*dislacksov1.Member, len(ms))
	for i, m := range ms {
		out[i] = Member(m)
	}
	return out
}

var channelTypes = map[domain.ChannelType]dislacksov1.ChannelType{
	domain.ChannelCategory: dislacksov1.ChannelType_CHANNEL_TYPE_CATEGORY,
	domain.ChannelText:     dislacksov1.ChannelType_CHANNEL_TYPE_TEXT,
	domain.ChannelVoice:    dislacksov1.ChannelType_CHANNEL_TYPE_VOICE,
}

// ChannelTypeFromProto maps unknown values to an invalid type the service rejects.
func ChannelTypeFromProto(t dislacksov1.ChannelType) domain.ChannelType {
	return reverse(channelTypes, t)
}

func Channel(c domain.Channel) *dislacksov1.Channel {
	return &dislacksov1.Channel{
		Id:              c.ID.String(),
		GuildId:         c.GuildID.String(),
		ParentId:        optionalID(c.ParentID),
		Type:            channelTypes[c.Type],
		Name:            c.Name,
		Topic:           c.Topic,
		Position:        ClampInt32(c.Position),
		UserLimit:       ClampInt32(c.UserLimit),
		DrawingAllowed:  c.DrawingAllowed,
		MaxShareQuality: c.MaxShareQuality,
		CreatedAt:       c.CreatedAt.UnixMilli(),
		UpdatedAt:       c.UpdatedAt.UnixMilli(),
	}
}

func Channels(cs []domain.Channel) []*dislacksov1.Channel {
	out := make([]*dislacksov1.Channel, len(cs))
	for i, c := range cs {
		out[i] = Channel(c)
	}
	return out
}

func GuildState(s domain.GuildState) *dislacksov1.GuildState {
	return &dislacksov1.GuildState{
		Guild:       Guild(s.Guild),
		Channels:    Channels(s.Channels),
		Members:     Members(s.Members),
		Users:       Users(s.Users),
		Presences:   Presences(s.Presences),
		VoiceStates: VoiceStates(s.VoiceStates),
	}
}

// VoiceState maps a voice state; a zero ChannelID becomes the empty
// channel_id that tells clients the user left.
func VoiceState(v domain.VoiceState) *dislacksov1.VoiceState {
	out := &dislacksov1.VoiceState{
		UserId:         v.UserID.String(),
		GuildId:        v.GuildID.String(),
		VoiceSessionId: v.SessionID,
		SelfMute:       v.SelfMute,
		SelfDeaf:       v.SelfDeaf,
		ServerMute:     v.ServerMute,
		ServerDeaf:     v.ServerDeaf,
		Streaming:      v.Streaming,
		Connected:      v.Connected,
		JoinedAt:       v.JoinedAt.UnixMilli(),
	}
	if v.ChannelID != 0 {
		out.ChannelId = v.ChannelID.String()
	}
	return out
}

func VoiceStates(vs []domain.VoiceState) []*dislacksov1.VoiceState {
	out := make([]*dislacksov1.VoiceState, len(vs))
	for i, v := range vs {
		out[i] = VoiceState(v)
	}
	return out
}

func Invite(v domain.InviteView) *dislacksov1.Invite {
	out := &dislacksov1.Invite{
		Code:      v.Invite.Code,
		GuildId:   v.Invite.GuildID.String(),
		MaxUses:   ClampInt32(v.Invite.MaxUses),
		Uses:      ClampInt32(v.Invite.Uses),
		ExpiresAt: unixMilliOrZero(v.Invite.ExpiresAt),
		CreatedAt: v.Invite.CreatedAt.UnixMilli(),
		Url:       v.URL,
	}
	if v.Creator != nil {
		out.CreatedBy = PublicUser(*v.Creator)
	}
	return out
}

func Ban(v domain.BanView) *dislacksov1.Ban {
	out := &dislacksov1.Ban{
		GuildId:   v.Ban.GuildID.String(),
		User:      PublicUser(v.User),
		Reason:    v.Ban.Reason,
		CreatedAt: v.Ban.CreatedAt.UnixMilli(),
	}
	if v.BannedBy != nil {
		out.BannedBy = PublicUser(*v.BannedBy)
	}
	return out
}

var auditActions = map[domain.AuditAction]dislacksov1.AuditAction{
	domain.AuditGuildUpdate:       dislacksov1.AuditAction_AUDIT_ACTION_GUILD_UPDATE,
	domain.AuditChannelCreate:     dislacksov1.AuditAction_AUDIT_ACTION_CHANNEL_CREATE,
	domain.AuditChannelUpdate:     dislacksov1.AuditAction_AUDIT_ACTION_CHANNEL_UPDATE,
	domain.AuditChannelDelete:     dislacksov1.AuditAction_AUDIT_ACTION_CHANNEL_DELETE,
	domain.AuditChannelsReorder:   dislacksov1.AuditAction_AUDIT_ACTION_CHANNELS_REORDER,
	domain.AuditInviteCreate:      dislacksov1.AuditAction_AUDIT_ACTION_INVITE_CREATE,
	domain.AuditInviteRevoke:      dislacksov1.AuditAction_AUDIT_ACTION_INVITE_REVOKE,
	domain.AuditMemberKick:        dislacksov1.AuditAction_AUDIT_ACTION_MEMBER_KICK,
	domain.AuditMemberBan:         dislacksov1.AuditAction_AUDIT_ACTION_MEMBER_BAN,
	domain.AuditMemberUnban:       dislacksov1.AuditAction_AUDIT_ACTION_MEMBER_UNBAN,
	domain.AuditMemberRoleUpdate:  dislacksov1.AuditAction_AUDIT_ACTION_MEMBER_ROLE_UPDATE,
	domain.AuditOwnershipTransfer: dislacksov1.AuditAction_AUDIT_ACTION_OWNERSHIP_TRANSFER,
	domain.AuditMemberVoiceUpdate: dislacksov1.AuditAction_AUDIT_ACTION_MEMBER_VOICE_UPDATE,
	domain.AuditMemberVoiceMove:   dislacksov1.AuditAction_AUDIT_ACTION_MEMBER_VOICE_MOVE,
	domain.AuditMemberVoiceDrop:   dislacksov1.AuditAction_AUDIT_ACTION_MEMBER_VOICE_DISCONNECT,
}

var auditTargets = map[domain.AuditTarget]dislacksov1.AuditTargetType{
	domain.AuditTargetGuild:   dislacksov1.AuditTargetType_AUDIT_TARGET_TYPE_GUILD,
	domain.AuditTargetChannel: dislacksov1.AuditTargetType_AUDIT_TARGET_TYPE_CHANNEL,
	domain.AuditTargetUser:    dislacksov1.AuditTargetType_AUDIT_TARGET_TYPE_USER,
	domain.AuditTargetInvite:  dislacksov1.AuditTargetType_AUDIT_TARGET_TYPE_INVITE,
}

func AuditEntry(e domain.AuditEntry) *dislacksov1.AuditEntry {
	return &dislacksov1.AuditEntry{
		Id:         e.ID.String(),
		GuildId:    e.GuildID.String(),
		ActorId:    optionalID(e.ActorID),
		Action:     auditActions[e.Action],
		TargetType: auditTargets[e.TargetType],
		TargetId:   e.TargetID,
		Data:       e.Data,
		CreatedAt:  e.CreatedAt.UnixMilli(),
	}
}

func FileRef(f domain.File) *dislacksov1.FileRef {
	return &dislacksov1.FileRef{
		Id:     f.ID.String(),
		Url:    domain.FileURL(f.ID, string(f.Purpose)),
		Mime:   f.Mime,
		Size:   f.Size,
		Width:  ClampInt32(f.Width),
		Height: ClampInt32(f.Height),
	}
}

func optionalID(id domain.ID) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(int64(id), 10)
}

func unixMilliOrZero(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixMilli()
}

func reverse[K, V comparable](m map[K]V, v V) K {
	for k, mv := range m {
		if mv == v {
			return k
		}
	}
	var zero K
	return zero
}

func ClampInt32(v int) int32 {
	switch {
	case v > math.MaxInt32:
		return math.MaxInt32
	case v < math.MinInt32:
		return math.MinInt32
	default:
		return int32(v)
	}
}
