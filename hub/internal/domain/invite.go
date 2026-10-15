package domain

import (
	"slices"
	"strings"
	"time"
)

// Invite is a guild invite, shared as /invite/<code>.
type Invite struct {
	Code      string
	GuildID   ID
	CreatedBy ID
	// MaxUses zero means unlimited.
	MaxUses   int
	Uses      int
	ExpiresAt *time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// Usable reports whether the invite can still be accepted.
func (i Invite) Usable(now time.Time) bool {
	return i.RevokedAt == nil &&
		(i.ExpiresAt == nil || now.Before(*i.ExpiresAt)) &&
		(i.MaxUses == 0 || i.Uses < i.MaxUses)
}

const (
	// InviteCodeAlphabet leaves out look-alikes (0/O, 1/I).
	InviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	InviteCodeLen      = 8
)

// InviteMaxAges are the expiry choices offered when creating an invite; zero never expires.
var InviteMaxAges = []time.Duration{
	0, 30 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour, 7 * 24 * time.Hour,
}

// InviteMaxUses are the use limits offered; zero is unlimited.
var InviteMaxUses = []int{0, 1, 5, 10, 25}

// NormalizeInviteCode accepts codes typed in lowercase or pasted as a full link.
func NormalizeInviteCode(code string) string {
	code = strings.TrimSpace(code)
	if i := strings.LastIndex(code, "/invite/"); i >= 0 {
		code = code[i+len("/invite/"):]
	}
	code, _, _ = strings.Cut(code, "?")
	return strings.ToUpper(strings.TrimRight(code, "/"))
}

// IsGuildInviteCode tells a guild invite code from an instance invite token.
func IsGuildInviteCode(code string) bool {
	if len(code) != InviteCodeLen {
		return false
	}
	for _, r := range code {
		if !strings.ContainsRune(InviteCodeAlphabet, r) {
			return false
		}
	}
	return true
}

func ValidateInviteOptions(maxAge time.Duration, maxUses int) error {
	var out []*FieldViolation
	if !slices.Contains(InviteMaxAges, maxAge) {
		out = append(out, &FieldViolation{"max_age_ms", ReasonFieldInvalid})
	}
	if !slices.Contains(InviteMaxUses, maxUses) {
		out = append(out, &FieldViolation{"max_uses", ReasonFieldInvalid})
	}
	return Violations(out...)
}

// InviteView is an invite as listed to members, with its creator (nil when
// that account no longer exists) and its absolute link.
type InviteView struct {
	Invite  Invite
	Creator *User
	URL     string
}
