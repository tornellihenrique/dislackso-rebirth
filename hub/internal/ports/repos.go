package ports

import (
	"context"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// Repositories return domain.ErrNotFound when a lookup finds nothing and
// domain.ErrConflict (with the field) when a unique column is taken.

type UserRepo interface {
	CreateUser(ctx context.Context, u domain.User) error
	UserByID(ctx context.Context, id domain.ID) (domain.User, error)
	UserByEmail(ctx context.Context, email string) (domain.User, error)
	// UserByLogin matches the username or the email, case-insensitively.
	UserByLogin(ctx context.Context, login string) (domain.User, error)
	// UpdateUser writes every mutable field of u.
	UpdateUser(ctx context.Context, u domain.User) error
	ListUsers(ctx context.Context) ([]domain.User, error)
	CountUsers(ctx context.Context) (int, error)
}

type SessionRepo interface {
	CreateSession(ctx context.Context, s domain.Session) error
	SessionByTokenHash(ctx context.Context, hash []byte) (domain.Session, error)
	TouchSession(ctx context.Context, id domain.ID, lastSeen, expires time.Time) error
	// RevokeSession revokes one active session of the user.
	RevokeSession(ctx context.Context, userID, id domain.ID, at time.Time) error
	// RevokeUserSessions revokes every active session except keep (zero keeps none).
	RevokeUserSessions(ctx context.Context, userID, keep domain.ID, at time.Time) (int, error)
	ActiveSessions(ctx context.Context, userID domain.ID, now time.Time) ([]domain.Session, error)
}

type EmailTokenRepo interface {
	CreateEmailToken(ctx context.Context, t domain.EmailToken) error
	EmailTokenByHash(ctx context.Context, hash []byte) (domain.EmailToken, error)
	// UseEmailToken marks the token used; it fails with ErrNotFound when the
	// token was already used, so two concurrent uses cannot both succeed.
	UseEmailToken(ctx context.Context, id domain.ID, at time.Time) error
	// ExpireUserTokens marks every unused token of a purpose as used.
	ExpireUserTokens(ctx context.Context, userID domain.ID, purpose domain.TokenPurpose, at time.Time) error
	EmailTokensByPurpose(ctx context.Context, purpose domain.TokenPurpose) ([]domain.EmailToken, error)
	DeleteEmailToken(ctx context.Context, id domain.ID, purpose domain.TokenPurpose) error
}

type SettingsRepo interface {
	// LoadSettings returns the stored settings over the defaults.
	LoadSettings(ctx context.Context, defaults domain.InstanceSettings) (domain.InstanceSettings, error)
	SaveSettings(ctx context.Context, s domain.InstanceSettings) error
}
