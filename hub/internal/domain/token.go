package domain

import "time"

type TokenPurpose string

const (
	TokenVerifyEmail    TokenPurpose = "verify"
	TokenPasswordReset  TokenPurpose = "reset"
	TokenChangeEmail    TokenPurpose = "change_email"
	TokenInstanceInvite TokenPurpose = "instance_invite"
)

// EmailToken is a single-use secret sent by email or handed out as a link.
// For instance invites, UserID is the admin who created it.
type EmailToken struct {
	ID        ID
	UserID    ID
	Purpose   TokenPurpose
	TokenHash []byte
	Payload   string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (t EmailToken) Usable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}
