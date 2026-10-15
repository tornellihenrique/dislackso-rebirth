package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

type Guild struct {
	ID         ID
	Name       string
	IconFileID ID
	OwnerID    ID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type GuildRole string

const (
	RoleOwner  GuildRole = "owner"
	RoleAdmin  GuildRole = "admin"
	RoleMember GuildRole = "member"
)

func (r GuildRole) Valid() bool { return r == RoleOwner || r == RoleAdmin || r == RoleMember }

type Member struct {
	GuildID ID
	UserID  ID
	Role    GuildRole
	// ServerMute and ServerDeaf are set by moderators and apply whenever the
	// member is in one of the guild's voice channels.
	ServerMute bool
	ServerDeaf bool
	JoinedAt   time.Time
	UpdatedAt  time.Time
}

type Ban struct {
	GuildID   ID
	UserID    ID
	BannedBy  ID
	Reason    string
	CreatedAt time.Time
}

const (
	GuildNameMaxLen = 100
	ReasonMaxLen    = 512
)

// NormalizeGuildName trims surrounding space.
func NormalizeGuildName(name string) string { return strings.TrimSpace(name) }

func ValidateGuildName(field, name string) *FieldViolation {
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return &FieldViolation{field, ReasonFieldRequired}
	case n > GuildNameMaxLen:
		return &FieldViolation{field, ReasonFieldTooLong}
	}
	if hasControl(name) {
		return &FieldViolation{field, ReasonFieldInvalidFormat}
	}
	return nil
}

// ValidateReason checks a moderation reason, which may be empty.
func ValidateReason(field, reason string) *FieldViolation {
	if utf8.RuneCountInString(reason) > ReasonMaxLen {
		return &FieldViolation{field, ReasonFieldTooLong}
	}
	if strings.ContainsFunc(reason, func(r rune) bool { return r != '\n' && isControl(r) }) {
		return &FieldViolation{field, ReasonFieldInvalidFormat}
	}
	return nil
}

// GuildState is everything a member needs to show a guild.
type GuildState struct {
	Guild       Guild
	Channels    []Channel
	Members     []Member
	Users       []User
	Presences   []Presence
	VoiceStates []VoiceState
}

// BanView is a ban with the people involved; BannedBy is nil when that
// account no longer exists.
type BanView struct {
	Ban      Ban
	User     User
	BannedBy *User
}
