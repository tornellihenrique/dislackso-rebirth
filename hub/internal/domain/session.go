package domain

import (
	"crypto/sha256"
	"net/netip"
	"time"
)

type Session struct {
	ID         ID
	UserID     ID
	TokenHash  []byte
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	UserAgent  string
	IP         string
	RevokedAt  *time.Time
}

func (s Session) Active(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt)
}

// HashToken is how session tokens and emailed tokens are stored: only the
// hash, so a leaked database cannot be replayed.
func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// Actor is the authenticated caller of a service method.
type Actor struct {
	UserID        ID
	SessionID     ID
	InstanceAdmin bool
	EmailVerified bool
}

// ClientInfo describes where a request came from.
type ClientInfo struct {
	IP        netip.Addr
	UserAgent string
	// Locale is the best supported match for the client's languages, used for emails.
	Locale string
}

// IsLocal reports whether the request came from the host machine itself.
func (c ClientInfo) IsLocal() bool { return c.IP.IsValid() && c.IP.Unmap().IsLoopback() }
