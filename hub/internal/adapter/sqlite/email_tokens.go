package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/sqlite/sqlcgen"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

func (db *DB) CreateEmailToken(ctx context.Context, t domain.EmailToken) error {
	err := db.writer(ctx).CreateEmailToken(ctx, sqlcgen.CreateEmailTokenParams{
		ID:        int64(t.ID),
		UserID:    nullID(t.UserID),
		Purpose:   string(t.Purpose),
		TokenHash: t.TokenHash,
		Payload:   t.Payload,
		ExpiresAt: toMS(t.ExpiresAt),
		CreatedAt: toMS(t.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("create %s token: %w", t.Purpose, mapError(err))
	}
	return nil
}

func (db *DB) EmailTokenByHash(ctx context.Context, hash []byte) (domain.EmailToken, error) {
	row, err := db.reader(ctx).GetEmailTokenByHash(ctx, hash)
	if err != nil {
		return domain.EmailToken{}, fmt.Errorf("load email token: %w", mapError(err))
	}
	return toEmailToken(row), nil
}

func (db *DB) UseEmailToken(ctx context.Context, id domain.ID, at time.Time) error {
	n, err := db.writer(ctx).UseEmailToken(ctx, sqlcgen.UseEmailTokenParams{UsedAt: toNullMS(&at), ID: int64(id)})
	if err != nil {
		return fmt.Errorf("use email token %d: %w", id, mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("use email token %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (db *DB) ExpireUserTokens(ctx context.Context, userID domain.ID, purpose domain.TokenPurpose, at time.Time) error {
	err := db.writer(ctx).ExpireUserTokens(ctx, sqlcgen.ExpireUserTokensParams{
		UsedAt:  toNullMS(&at),
		UserID:  nullID(userID),
		Purpose: string(purpose),
	})
	if err != nil {
		return fmt.Errorf("expire %s tokens of user %d: %w", purpose, userID, mapError(err))
	}
	return nil
}

func (db *DB) EmailTokensByPurpose(ctx context.Context, purpose domain.TokenPurpose) ([]domain.EmailToken, error) {
	rows, err := db.reader(ctx).ListEmailTokensByPurpose(ctx, string(purpose))
	if err != nil {
		return nil, fmt.Errorf("list %s tokens: %w", purpose, mapError(err))
	}
	out := make([]domain.EmailToken, len(rows))
	for i, r := range rows {
		out[i] = toEmailToken(r)
	}
	return out, nil
}

func (db *DB) DeleteEmailToken(ctx context.Context, id domain.ID, purpose domain.TokenPurpose) error {
	n, err := db.writer(ctx).DeleteEmailToken(ctx, sqlcgen.DeleteEmailTokenParams{ID: int64(id), Purpose: string(purpose)})
	if err != nil {
		return fmt.Errorf("delete %s token %d: %w", purpose, id, mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("delete %s token %d: %w", purpose, id, domain.ErrNotFound)
	}
	return nil
}

func toEmailToken(r sqlcgen.EmailToken) domain.EmailToken {
	return domain.EmailToken{
		ID:        domain.ID(r.ID),
		UserID:    domain.ID(r.UserID.Int64),
		Purpose:   domain.TokenPurpose(r.Purpose),
		TokenHash: r.TokenHash,
		Payload:   r.Payload,
		ExpiresAt: fromMS(r.ExpiresAt),
		UsedAt:    fromNullMS(r.UsedAt),
		CreatedAt: fromMS(r.CreatedAt),
	}
}

func nullID(id domain.ID) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(id), Valid: id != 0}
}
