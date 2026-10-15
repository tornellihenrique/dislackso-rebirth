package domain

import "time"

type AuditAction string

const (
	AuditGuildUpdate       AuditAction = "guild_update"
	AuditChannelCreate     AuditAction = "channel_create"
	AuditChannelUpdate     AuditAction = "channel_update"
	AuditChannelDelete     AuditAction = "channel_delete"
	AuditChannelsReorder   AuditAction = "channels_reorder"
	AuditInviteCreate      AuditAction = "invite_create"
	AuditInviteRevoke      AuditAction = "invite_revoke"
	AuditMemberKick        AuditAction = "member_kick"
	AuditMemberBan         AuditAction = "member_ban"
	AuditMemberUnban       AuditAction = "member_unban"
	AuditMemberRoleUpdate  AuditAction = "member_role_update"
	AuditOwnershipTransfer AuditAction = "ownership_transfer"
)

type AuditTarget string

const (
	AuditTargetGuild   AuditTarget = "guild"
	AuditTargetChannel AuditTarget = "channel"
	AuditTargetUser    AuditTarget = "user"
	AuditTargetInvite  AuditTarget = "invite"
)

// AuditEntry records one moderation or settings change in a guild.
type AuditEntry struct {
	ID         ID
	GuildID    ID
	ActorID    ID
	Action     AuditAction
	TargetType AuditTarget
	// TargetID is an ID in decimal, or an invite code.
	TargetID  string
	Data      map[string]string
	CreatedAt time.Time
}

const (
	AuditPageDefault = 50
	AuditPageMax     = 100
)
