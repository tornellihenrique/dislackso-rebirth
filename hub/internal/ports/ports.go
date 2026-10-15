package ports

import (
	"context"
	"image"
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
	// Square crops the largest square centered in crop (in source pixels;
	// the whole image when empty) and scales it down to at most size pixels.
	// Animated images stay animated. Unsupported, undecodable or oversized
	// input fails with domain.ErrInvalid.
	Square(r io.Reader, size int, crop image.Rectangle) (Image, error)
}

// MediaJoin describes a participant token for a voice room.
type MediaJoin struct {
	Room     string
	Identity string
	// Metadata travels with the participant and comes back in media events.
	Metadata    string
	Permissions domain.VoicePermissions
	TTL         time.Duration
}

// MediaParticipant is someone in a media server room.
type MediaParticipant struct {
	// SID identifies one connection; track events carry only it and Identity.
	SID      string
	Identity string
	Metadata string
	// Active is false while the participant is still connecting.
	Active  bool
	Sources []domain.MediaSource
}

type MediaEventKind string

const (
	MediaParticipantJoined MediaEventKind = "participant_joined"
	MediaParticipantLeft   MediaEventKind = "participant_left"
	MediaTrackPublished    MediaEventKind = "track_published"
	MediaTrackUnpublished  MediaEventKind = "track_unpublished"
)

// MediaEvent is a media server notification the hub reconciles voice state with.
type MediaEvent struct {
	Kind        MediaEventKind
	Room        string
	Participant MediaParticipant
	// Source is the track's source for track events.
	Source domain.MediaSource
}

// MediaProvider is the media server (an SFU) that carries voice and video.
// The hub mints its tokens and enforces moderation through it.
type MediaProvider interface {
	// URL is where clients connect; it may be relative to the instance.
	URL() string
	// Available reports whether the media server is up.
	Available() bool
	JoinToken(j MediaJoin) (string, error)
	RemoveParticipant(ctx context.Context, room, identity string) error
	// SetPermissions changes what a connected participant may publish and
	// whether it may subscribe. The media server enforces it, unpublishing
	// tracks of sources no longer allowed.
	SetPermissions(ctx context.Context, room, identity string, p domain.VoicePermissions) error
	ListRooms(ctx context.Context) ([]string, error)
	ListParticipants(ctx context.Context, room string) ([]MediaParticipant, error)
}

// IceServer is a STUN or TURN server for clients behind restrictive networks.
type IceServer struct {
	URLs       []string
	Username   string
	Credential string
}

// IceServerProvider issues ICE servers, with credentials valid for ttl.
type IceServerProvider interface {
	Name() string
	Issue(ctx context.Context, userID domain.ID, ttl time.Duration) ([]IceServer, error)
}
