package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/sqlite/sqlcgen"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

// AddTURNUsage adds relayed bytes to a month ("2026-10").
func (db *DB) AddTURNUsage(ctx context.Context, month string, bytes int64) error {
	if err := db.writer(ctx).AddTURNUsage(ctx, sqlcgen.AddTURNUsageParams{Month: month, Bytes: bytes}); err != nil {
		return fmt.Errorf("add turn usage: %w", mapError(err))
	}
	return nil
}

// TURNUsage is the relayed bytes of a month; zero before any.
func (db *DB) TURNUsage(ctx context.Context, month string) (int64, error) {
	n, err := db.reader(ctx).GetTURNUsage(ctx, month)
	if err != nil {
		if err := mapError(err); errors.Is(err, domain.ErrNotFound) {
			return 0, nil
		}
		return 0, fmt.Errorf("turn usage: %w", mapError(err))
	}
	return n, nil
}
