package domain

// Policy holds every authorization rule, so services never compare roles
// themselves.
type Policy struct{}

func (Policy) ManageInstance(a Actor) error {
	if !a.InstanceAdmin {
		return Forbidden(ReasonNotInstanceAdmin)
	}
	return nil
}

// GuildAction is something a member may be allowed to do in a guild.
type GuildAction string

const (
	ActUpdateGuild       GuildAction = "update_guild"
	ActDeleteGuild       GuildAction = "delete_guild"
	ActTransferOwnership GuildAction = "transfer_ownership"
	ActSetMemberRole     GuildAction = "set_member_role"
	ActManageChannels    GuildAction = "manage_channels"
	ActCreateInvite      GuildAction = "create_invite"
	// ActManageInvites lists and revokes everyone's invites.
	ActManageInvites    GuildAction = "manage_invites"
	ActKickMembers      GuildAction = "kick_members"
	ActBanMembers       GuildAction = "ban_members"
	ActViewAuditLog     GuildAction = "view_audit_log"
	ActModerateVoice    GuildAction = "moderate_voice"
	ActDeleteAnyMessage GuildAction = "delete_any_message"
)

// GuildActions lists every action, for exhaustive tests.
var GuildActions = []GuildAction{
	ActUpdateGuild, ActDeleteGuild, ActTransferOwnership, ActSetMemberRole, ActManageChannels,
	ActCreateInvite, ActManageInvites, ActKickMembers, ActBanMembers, ActViewAuditLog,
	ActModerateVoice, ActDeleteAnyMessage,
}

var ownerOnly = map[GuildAction]bool{
	ActDeleteGuild:       true,
	ActTransferOwnership: true,
	ActSetMemberRole:     true,
}

// Can is the permission matrix of ARCHITECTURE §13.4. Members may only create
// invites, and only while the instance allows it.
func (Policy) Can(role GuildRole, action GuildAction, membersCanInvite bool) bool {
	switch role {
	case RoleOwner:
		return true
	case RoleAdmin:
		return !ownerOnly[action]
	case RoleMember:
		return action == ActCreateInvite && membersCanInvite
	default:
		return false
	}
}

// Guild checks an action that has no target member.
func (p Policy) Guild(m Member, action GuildAction) error {
	return p.guild(m, action, false)
}

func (p Policy) guild(m Member, action GuildAction, membersCanInvite bool) error {
	if !p.Can(m.Role, action, membersCanInvite) {
		if action == ActCreateInvite && m.Role == RoleMember {
			return Forbidden(ReasonInvitesDisabled)
		}
		return Forbidden(ReasonMissingPermission)
	}
	return nil
}

func (p Policy) CreateInvite(m Member, membersCanInvite bool) error {
	return p.guild(m, ActCreateInvite, membersCanInvite)
}

// RevokeInvite lets anyone revoke their own invites, and managers any invite.
func (p Policy) RevokeInvite(m Member, inv Invite) error {
	if inv.CreatedBy == m.UserID {
		return nil
	}
	return p.Guild(m, ActManageInvites)
}

// TargetMember checks an action on another member (kick, ban, voice
// moderation): nobody targets the owner or themselves, and admins cannot
// target other admins.
func (p Policy) TargetMember(actor, target Member, action GuildAction) error {
	if err := p.Guild(actor, action); err != nil {
		return err
	}
	switch {
	case actor.UserID == target.UserID:
		return Forbidden(ReasonCannotTargetSelf)
	case target.Role == RoleOwner:
		return Forbidden(ReasonCannotTargetOwner)
	case target.Role == RoleAdmin && actor.Role != RoleOwner:
		return Forbidden(ReasonCannotTargetAdmin)
	}
	return nil
}

// SetMemberRole lets the owner make members admins and back.
func (p Policy) SetMemberRole(actor, target Member, role GuildRole) error {
	if err := p.Guild(actor, ActSetMemberRole); err != nil {
		return err
	}
	if actor.UserID == target.UserID {
		return Forbidden(ReasonCannotTargetSelf)
	}
	if role != RoleAdmin && role != RoleMember {
		return Invalid(FieldViolation{Field: "role", Reason: ReasonFieldInvalid})
	}
	return nil
}

func (p Policy) TransferOwnership(actor, target Member) error {
	if err := p.Guild(actor, ActTransferOwnership); err != nil {
		return err
	}
	if actor.UserID == target.UserID {
		return Forbidden(ReasonCannotTargetSelf)
	}
	return nil
}

// LeaveGuild refuses the owner, who must transfer ownership or delete the guild.
func (Policy) LeaveGuild(m Member) error {
	if m.Role == RoleOwner {
		return Precondition(ReasonOwnerCannotLeave)
	}
	return nil
}

// DeleteMessage lets authors delete their own messages, and managers any message.
func (p Policy) DeleteMessage(m Member, authorID ID) error {
	if authorID == m.UserID {
		return nil
	}
	return p.Guild(m, ActDeleteAnyMessage)
}
