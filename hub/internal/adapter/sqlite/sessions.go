package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/sqlite/sqlcgen"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

func (db *DB) CreateSession(ctx context.Context, s domain.Session) error {
	err := db.writer(ctx).CreateSession(ctx, sqlcgen.CreateSessionParams{
		ID:         int64(s.ID),
		UserID:     int64(s.UserID),
		TokenHash:  s.TokenHash,
		CreatedAt:  toMS(s.CreatedAt),
		LastSeenAt: toMS(s.LastSeenAt),
		ExpiresAt:  toMS(s.ExpiresAt),
		UserAgent:  s.UserAgent,
		Ip:         s.IP,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", mapError(err))
	}
	return nil
}

func (db *DB) SessionByTokenHash(ctx context.Context, hash []byte) (domain.Session, error) {
	row, err := db.reader(ctx).GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return domain.Session{}, fmt.Errorf("load session: %w", mapError(err))
	}
	return toSession(row), nil
}

func (db *DB) TouchSession(ctx context.Context, id domain.ID, lastSeen, expires time.Time) error {
	err := db.writer(ctx).TouchSession(ctx, sqlcgen.TouchSessionParams{
		LastSeenAt: toMS(lastSeen),
		ExpiresAt:  toMS(expires),
		ID:         int64(id),
	})
	if err != nil {
		return fmt.Errorf("touch session %d: %w", id, mapError(err))
	}
	return nil
}

func (db *DB) RevokeSession(ctx context.Context, userID, id domain.ID, at time.Time) error {
	n, err := db.writer(ctx).RevokeSession(ctx, sqlcgen.RevokeSessionParams{
		RevokedAt: toNullMS(&at),
		ID:        int64(id),
		UserID:    int64(userID),
	})
	if err != nil {
		return fmt.Errorf("revoke session %d: %w", id, mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("revoke session %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (db *DB) RevokeUserSessions(ctx context.Context, userID, keep domain.ID, at time.Time) (int, error) {
	n, err := db.writer(ctx).RevokeUserSessions(ctx, sqlcgen.RevokeUserSessionsParams{
		RevokedAt: toNullMS(&at),
		UserID:    int64(userID),
		KeepID:    int64(keep),
	})
	if err != nil {
		return 0, fmt.Errorf("revoke sessions of user %d: %w", userID, mapError(err))
	}
	return int(n), nil
}

func (db *DB) ActiveSessions(ctx context.Context, userID domain.ID, now time.Time) ([]domain.Session, error) {
	rows, err := db.reader(ctx).ListActiveSessions(ctx, sqlcgen.ListActiveSessionsParams{
		UserID: int64(userID),
		Now:    toMS(now),
	})
	if err != nil {
		return nil, fmt.Errorf("list sessions of user %d: %w", userID, mapError(err))
	}
	out := make([]domain.Session, len(rows))
	for i, r := range rows {
		out[i] = toSession(r)
	}
	return out, nil
}

func toSession(r sqlcgen.Session) domain.Session {
	return domain.Session{
		ID:         domain.ID(r.ID),
		UserID:     domain.ID(r.UserID),
		TokenHash:  r.TokenHash,
		CreatedAt:  fromMS(r.CreatedAt),
		LastSeenAt: fromMS(r.LastSeenAt),
		ExpiresAt:  fromMS(r.ExpiresAt),
		UserAgent:  r.UserAgent,
		IP:         r.Ip,
		RevokedAt:  fromNullMS(r.RevokedAt),
	}
}
