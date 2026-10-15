package ports

import (
	"context"
	"io"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
)

type IDGen interface {
	NewID() domain.ID
}

// CodeGen creates short invite codes from domain.InviteCodeAlphabet.
type CodeGen interface {
	NewInviteCode() (string, error)
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

// EventPublisher sends events to the realtime gateway once the transaction
// in ctx commits.
type EventPublisher interface {
	Publish(ctx context.Context, evs ...events.Event)
}

// BlobStore keeps file contents by key. Keys are derived from content, so a
// key is written at most once.
type BlobStore interface {
	// Put stores r under key unless the key already exists.
	Put(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (Blob, error)
}

type Blob interface {
	io.ReadSeekCloser
	Size() int64
}

// Image is a decoded and re-encoded picture; metadata such as EXIF is gone.
type Image struct {
	Data   []byte
	Mime   string
	Width  int
	Height int
}

// ImageProcessor decodes an untrusted upload and re-encodes it.
type ImageProcessor interface {
	// Square center-crops the image to a square of at most size pixels.
	// Unsupported or undecodable input fails with domain.ErrInvalid.
	Square(r io.Reader, size int) (Image, error)
}
