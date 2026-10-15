package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func reasonOfErr(err error) string {
	if err == nil {
		return ""
	}
	if de, ok := AsError(err); ok {
		return de.Reason
	}
	return err.Error()
}

// TestPolicyMatrix is ARCHITECTURE §13.4: owner, admin and member against
// every action, with members inviting both with and without the instance
// setting.
func TestPolicyMatrix(t *testing.T) {
	type want struct{ owner, admin, member, memberNoInvites bool }
	matrix := map[GuildAction]want{
		ActUpdateGuild:       {true, true, false, false},
		ActDeleteGuild:       {true, false, false, false},
		ActTransferOwnership: {true, false, false, false},
		ActSetMemberRole:     {true, false, false, false},
		ActManageChannels:    {true, true, false, false},
		ActCreateInvite:      {true, true, true, false},
		ActManageInvites:     {true, true, false, false},
		ActKickMembers:       {true, true, false, false},
		ActBanMembers:        {true, true, false, false},
		ActViewAuditLog:      {true, true, false, false},
		ActModerateVoice:     {true, true, false, false},
		ActDeleteAnyMessage:  {true, true, false, false},
	}
	if len(matrix) != len(GuildActions) {
		t.Fatalf("matrix covers %d actions, policy has %d", len(matrix), len(GuildActions))
	}
	var p Policy
	for _, action := range GuildActions {
		w, ok := matrix[action]
		if !ok {
			t.Fatalf("action %s missing from the matrix", action)
		}
		cases := []struct {
			role             GuildRole
			membersCanInvite bool
			want             bool
		}{
			{RoleOwner, true, w.owner},
			{RoleOwner, false, w.owner},
			{RoleAdmin, true, w.admin},
			{RoleAdmin, false, w.admin},
			{RoleMember, true, w.member},
			{RoleMember, false, w.memberNoInvites},
			{GuildRole(""), true, false},
		}
		for _, c := range cases {
			if got := p.Can(c.role, action, c.membersCanInvite); got != c.want {
				t.Errorf("Can(%q, %s, invites=%v) = %v, want %v", c.role, action, c.membersCanInvite, got, c.want)
			}
		}
	}
}

func TestPolicyGuildErrors(t *testing.T) {
	var p Policy
	member := Member{UserID: 1, Role: RoleMember}
	if got := reasonOfErr(p.Guild(member, ActManageChannels)); got != ReasonMissingPermission {
		t.Errorf("member managing channels: %q", got)
	}
	if err := p.Guild(member, ActManageChannels); !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden, got %v", err)
	}
	if got := reasonOfErr(p.CreateInvite(member, false)); got != ReasonInvitesDisabled {
		t.Errorf("member invite with setting off: %q", got)
	}
	if err := p.CreateInvite(member, true); err != nil {
		t.Errorf("member invite with setting on: %v", err)
	}
}

func TestPolicyTargets(t *testing.T) {
	var p Policy
	owner := Member{UserID: 1, Role: RoleOwner}
	admin := Member{UserID: 2, Role: RoleAdmin}
	admin2 := Member{UserID: 3, Role: RoleAdmin}
	member := Member{UserID: 4, Role: RoleMember}
	member2 := Member{UserID: 5, Role: RoleMember}

	targetActions := []GuildAction{ActKickMembers, ActBanMembers, ActModerateVoice}
	tests := []struct {
		name          string
		actor, target Member
		want          string
	}{
		{"owner on admin", owner, admin, ""},
		{"owner on member", owner, member, ""},
		{"owner on self", owner, owner, ReasonCannotTargetSelf},
		{"admin on member", admin, member, ""},
		{"admin on admin", admin, admin2, ReasonCannotTargetAdmin},
		{"admin on owner", admin, owner, ReasonCannotTargetOwner},
		{"admin on self", admin, admin, ReasonCannotTargetSelf},
		{"member on member", member, member2, ReasonMissingPermission},
		{"member on owner", member, owner, ReasonMissingPermission},
	}
	for _, action := range targetActions {
		for _, tt := range tests {
			if got := reasonOfErr(p.TargetMember(tt.actor, tt.target, action)); got != tt.want {
				t.Errorf("%s %s: got %q, want %q", action, tt.name, got, tt.want)
			}
		}
	}

	roleTests := []struct {
		name          string
		actor, target Member
		role          GuildRole
		want          string
	}{
		{"owner promotes", owner, member, RoleAdmin, ""},
		{"owner demotes", owner, admin, RoleMember, ""},
		{"owner to owner", owner, member, RoleOwner, ReasonFieldInvalid},
		{"owner on self", owner, owner, RoleMember, ReasonCannotTargetSelf},
		{"admin promotes", admin, member, RoleAdmin, ReasonMissingPermission},
		{"member promotes", member, member2, RoleAdmin, ReasonMissingPermission},
	}
	for _, tt := range roleTests {
		err := p.SetMemberRole(tt.actor, tt.target, tt.role)
		got := reasonOfErr(err)
		if de, ok := AsError(err); ok && len(de.Fields) > 0 {
			got = de.Fields[0].Reason
		}
		if got != tt.want {
			t.Errorf("SetMemberRole %s: got %q, want %q", tt.name, got, tt.want)
		}
	}

	if got := reasonOfErr(p.TransferOwnership(owner, admin)); got != "" {
		t.Errorf("owner transfers: %q", got)
	}
	if got := reasonOfErr(p.TransferOwnership(owner, owner)); got != ReasonCannotTargetSelf {
		t.Errorf("owner transfers to self: %q", got)
	}
	if got := reasonOfErr(p.TransferOwnership(admin, member)); got != ReasonMissingPermission {
		t.Errorf("admin transfers: %q", got)
	}

	for _, tt := range []struct {
		m    Member
		want string
	}{{owner, ReasonOwnerCannotLeave}, {admin, ""}, {member, ""}} {
		if got := reasonOfErr(p.LeaveGuild(tt.m)); got != tt.want {
			t.Errorf("LeaveGuild(%s): %q, want %q", tt.m.Role, got, tt.want)
		}
	}

	ownInvite := Invite{CreatedBy: member.UserID}
	otherInvite := Invite{CreatedBy: admin.UserID}
	if err := p.RevokeInvite(member, ownInvite); err != nil {
		t.Errorf("member revokes own invite: %v", err)
	}
	if got := reasonOfErr(p.RevokeInvite(member, otherInvite)); got != ReasonMissingPermission {
		t.Errorf("member revokes another invite: %q", got)
	}
	if err := p.RevokeInvite(admin2, otherInvite); err != nil {
		t.Errorf("admin revokes another invite: %v", err)
	}

	if err := p.DeleteMessage(member, member.UserID); err != nil {
		t.Errorf("member deletes own message: %v", err)
	}
	if got := reasonOfErr(p.DeleteMessage(member, member2.UserID)); got != ReasonMissingPermission {
		t.Errorf("member deletes another message: %q", got)
	}
	if err := p.DeleteMessage(admin, member.UserID); err != nil {
		t.Errorf("admin deletes a message: %v", err)
	}
}

func TestGuildValidators(t *testing.T) {
	tests := []struct {
		name string
		got  *FieldViolation
		want string
	}{
		{"guild name ok", ValidateGuildName("n", "Game Dev ✨"), ""},
		{"guild name empty", ValidateGuildName("n", ""), ReasonFieldRequired},
		{"guild name long", ValidateGuildName("n", strings.Repeat("é", 101)), ReasonFieldTooLong},
		{"guild name control", ValidateGuildName("n", "a\tb"), ReasonFieldInvalidFormat},
		{"reason multiline", ValidateReason("r", "spam\nagain"), ""},
		{"reason control", ValidateReason("r", "a\x00"), ReasonFieldInvalidFormat},
		{"reason long", ValidateReason("r", strings.Repeat("x", 513)), ReasonFieldTooLong},
		{"channel name ok", ValidateChannelName("n", "general"), ""},
		{"channel name empty", ValidateChannelName("n", ""), ReasonFieldRequired},
		{"channel name long", ValidateChannelName("n", strings.Repeat("a", 101)), ReasonFieldTooLong},
		{"topic long", ValidateChannelTopic("t", strings.Repeat("a", 1025)), ReasonFieldTooLong},
		{"user limit ok", ValidateUserLimit("u", 99), ""},
		{"user limit high", ValidateUserLimit("u", 100), ReasonFieldInvalid},
		{"user limit negative", ValidateUserLimit("u", -1), ReasonFieldInvalid},
		{"preset ok", ValidateSharePreset("q", "1080p60"), ""},
		{"preset empty", ValidateSharePreset("q", ""), ""},
		{"preset unknown", ValidateSharePreset("q", "4k"), ReasonFieldInvalid},
	}
	for _, tt := range tests {
		if got := reasonOf(tt.got); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestNormalizeChannelName(t *testing.T) {
	tests := []struct {
		typ  ChannelType
		in   string
		want string
	}{
		{ChannelText, "  Release Notes ", "release-notes"},
		{ChannelText, "a -- b", "a-b"},
		{ChannelText, "Ação", "ação"},
		{ChannelVoice, "  Daily Standup ", "Daily Standup"},
		{ChannelCategory, "Voice Channels", "Voice Channels"},
	}
	for _, tt := range tests {
		if got := NormalizeChannelName(tt.typ, tt.in); got != tt.want {
			t.Errorf("NormalizeChannelName(%s, %q) = %q, want %q", tt.typ, tt.in, got, tt.want)
		}
	}
}

func TestInviteCodes(t *testing.T) {
	tests := []struct{ in, want string }{
		{"abcd2345", "ABCD2345"},
		{" https://hub.example.com/invite/ABCD2345 ", "ABCD2345"},
		{"https://hub.example.com/invite/ABCD2345/?x=1", "ABCD2345"},
	}
	for _, tt := range tests {
		if got := NormalizeInviteCode(tt.in); got != tt.want {
			t.Errorf("NormalizeInviteCode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if !IsGuildInviteCode("ABCD2345") {
		t.Error("ABCD2345 should be a guild invite code")
	}
	for _, code := range []string{"ABCD234", "ABCD2340", "abcd2345", "kFh3l0-long-instance-token"} {
		if IsGuildInviteCode(code) {
			t.Errorf("%q should not be a guild invite code", code)
		}
	}
	if err := ValidateInviteOptions(30*time.Minute, 5); err != nil {
		t.Errorf("valid options: %v", err)
	}
	err := ValidateInviteOptions(2*time.Hour, 3)
	de, ok := AsError(err)
	if !ok || len(de.Fields) != 2 {
		t.Errorf("invalid options: %v", err)
	}
}

func TestInviteUsable(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	past, future := now.Add(-time.Minute), now.Add(time.Minute)
	tests := []struct {
		name string
		inv  Invite
		want bool
	}{
		{"fresh", Invite{}, true},
		{"expired", Invite{ExpiresAt: &past}, false},
		{"not yet expired", Invite{ExpiresAt: &future}, true},
		{"used up", Invite{MaxUses: 2, Uses: 2}, false},
		{"uses left", Invite{MaxUses: 2, Uses: 1}, true},
		{"revoked", Invite{RevokedAt: &past}, false},
	}
	for _, tt := range tests {
		if got := tt.inv.Usable(now); got != tt.want {
			t.Errorf("%s: Usable = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestEffectivePresence(t *testing.T) {
	tests := []struct {
		name              string
		pref              StatusPref
		connections, idle int
		want              PresenceStatus
		custom            string
	}{
		{"no connection", StatusOnline, 0, 0, PresenceOffline, ""},
		{"online", StatusOnline, 2, 1, PresenceOnline, "busy"},
		{"all idle", StatusOnline, 2, 2, PresenceIdle, "busy"},
		{"idle preference", StatusIdle, 1, 0, PresenceIdle, "busy"},
		{"dnd beats idle", StatusDND, 1, 1, PresenceDND, "busy"},
		{"invisible", StatusInvisible, 3, 0, PresenceOffline, ""},
	}
	for _, tt := range tests {
		got := EffectivePresence(7, tt.pref, "busy", tt.connections, tt.idle)
		if got.Status != tt.want || got.CustomStatus != tt.custom || got.UserID != 7 {
			t.Errorf("%s: got %+v, want status %s custom %q", tt.name, got, tt.want, tt.custom)
		}
	}
}
