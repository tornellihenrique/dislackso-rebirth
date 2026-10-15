package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/sqlite/sqlcgen"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

func (db *DB) CreateChannel(ctx context.Context, c domain.Channel) error {
	err := db.writer(ctx).CreateChannel(ctx, sqlcgen.CreateChannelParams{
		ID:              int64(c.ID),
		GuildID:         toNullID(c.GuildID),
		ParentID:        toNullID(c.ParentID),
		Type:            string(c.Type),
		Name:            c.Name,
		Topic:           c.Topic,
		Position:        int64(c.Position),
		UserLimit:       int64(c.UserLimit),
		DrawingAllowed:  boolInt(c.DrawingAllowed),
		MaxShareQuality: c.MaxShareQuality,
		CreatedAt:       toMS(c.CreatedAt),
		UpdatedAt:       toMS(c.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("create channel: %w", mapError(err))
	}
	return nil
}

func (db *DB) ChannelByID(ctx context.Context, id domain.ID) (domain.Channel, error) {
	row, err := db.reader(ctx).GetChannel(ctx, int64(id))
	if err != nil {
		return domain.Channel{}, fmt.Errorf("load channel %d: %w", id, mapError(err))
	}
	return toChannel(row), nil
}

func (db *DB) UpdateChannel(ctx context.Context, c domain.Channel) error {
	n, err := db.writer(ctx).UpdateChannel(ctx, sqlcgen.UpdateChannelParams{
		ParentID:        toNullID(c.ParentID),
		Name:            c.Name,
		Topic:           c.Topic,
		Position:        int64(c.Position),
		UserLimit:       int64(c.UserLimit),
		DrawingAllowed:  boolInt(c.DrawingAllowed),
		MaxShareQuality: c.MaxShareQuality,
		UpdatedAt:       toMS(c.UpdatedAt),
		ID:              int64(c.ID),
	})
	return affected(n, err, "update channel %d", c.ID)
}

func (db *DB) DeleteChannel(ctx context.Context, id domain.ID) error {
	n, err := db.writer(ctx).DeleteChannel(ctx, int64(id))
	return affected(n, err, "delete channel %d", id)
}

func (db *DB) GuildChannels(ctx context.Context, guildID domain.ID) ([]domain.Channel, error) {
	rows, err := db.reader(ctx).ListGuildChannels(ctx, sql.NullInt64{Int64: int64(guildID), Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list channels of guild %d: %w", guildID, mapError(err))
	}
	out := make([]domain.Channel, len(rows))
	for i, r := range rows {
		out[i] = toChannel(r)
	}
	return out, nil
}

func (db *DB) CountChannels(ctx context.Context, guildID domain.ID) (int, error) {
	n, err := db.reader(ctx).CountGuildChannels(ctx, sql.NullInt64{Int64: int64(guildID), Valid: true})
	if err != nil {
		return 0, fmt.Errorf("count channels of guild %d: %w", guildID, mapError(err))
	}
	return int(n), nil
}

func toChannel(r sqlcgen.Channel) domain.Channel {
	return domain.Channel{
		ID:              domain.ID(r.ID),
		GuildID:         fromNullID(r.GuildID),
		ParentID:        fromNullID(r.ParentID),
		Type:            domain.ChannelType(r.Type),
		Name:            r.Name,
		Topic:           r.Topic,
		Position:        int(r.Position),
		UserLimit:       int(r.UserLimit),
		DrawingAllowed:  r.DrawingAllowed != 0,
		MaxShareQuality: r.MaxShareQuality,
		CreatedAt:       fromMS(r.CreatedAt),
		UpdatedAt:       fromMS(r.UpdatedAt),
	}
}
