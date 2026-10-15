package ports

import (
	"context"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

type IDGen interface {
	NewID() domain.ID
}

// TokenGen creates unguessable URL-safe secrets for sessions and links.
type TokenGen interface {
	NewToken() (string, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	// Verify reports whether password matches encoded, a string from Hash.
	Verify(encoded, password string) (bool, error)
}

// Limit is a token bucket: Burst requests at once, then one every Every.
type Limit struct {
	Burst int
	Every time.Duration
}

type RateLimiter interface {
	// Allow spends one token for key and, when none is left, reports how
	// long until the next one.
	Allow(key string, limit Limit) (allowed bool, retryAfter time.Duration)
}

// TxRunner runs fn in a database transaction. Repositories called with the
// ctx passed to fn take part in it.
type TxRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// SecretStore keeps credentials outside the database, so backups and
// replicas of the database never carry them.
type SecretStore interface {
	Secret(ctx context.Context, key string) (string, bool, error)
	SetSecret(ctx context.Context, key, value string) error
}

type Mail struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Mailer interface {
	// Available reports whether mail can be sent at all (SMTP configured, or
	// the development log mailer).
	Available(ctx context.Context) bool
	Send(ctx context.Context, m Mail) error
}

// MailTester sends one message through SMTP settings that are not saved yet.
type MailTester interface {
	SendWith(ctx context.Context, settings domain.SMTPSettings, password string, m Mail) error
}
