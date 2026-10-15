package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/sqlite/sqlcgen"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

func (db *DB) CreateGuild(ctx context.Context, g domain.Guild) error {
	err := db.writer(ctx).CreateGuild(ctx, sqlcgen.CreateGuildParams{
		ID:         int64(g.ID),
		Name:       g.Name,
		IconFileID: toNullID(g.IconFileID),
		OwnerID:    int64(g.OwnerID),
		CreatedAt:  toMS(g.CreatedAt),
		UpdatedAt:  toMS(g.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("create guild: %w", mapError(err))
	}
	return nil
}

func (db *DB) GuildByID(ctx context.Context, id domain.ID) (domain.Guild, error) {
	row, err := db.reader(ctx).GetGuild(ctx, int64(id))
	if err != nil {
		return domain.Guild{}, fmt.Errorf("load guild %d: %w", id, mapError(err))
	}
	return toGuild(row), nil
}

func (db *DB) UpdateGuild(ctx context.Context, g domain.Guild) error {
	n, err := db.writer(ctx).UpdateGuild(ctx, sqlcgen.UpdateGuildParams{
		Name:       g.Name,
		IconFileID: toNullID(g.IconFileID),
		OwnerID:    int64(g.OwnerID),
		UpdatedAt:  toMS(g.UpdatedAt),
		ID:         int64(g.ID),
	})
	return affected(n, err, "update guild %d", g.ID)
}

func (db *DB) DeleteGuild(ctx context.Context, id domain.ID) error {
	n, err := db.writer(ctx).DeleteGuild(ctx, int64(id))
	return affected(n, err, "delete guild %d", id)
}

func (db *DB) CountGuildsOwnedBy(ctx context.Context, userID domain.ID) (int, error) {
	n, err := db.reader(ctx).CountGuildsOwnedBy(ctx, int64(userID))
	if err != nil {
		return 0, fmt.Errorf("count guilds of user %d: %w", userID, mapError(err))
	}
	return int(n), nil
}

func (db *DB) UserGuilds(ctx context.Context, userID domain.ID) ([]domain.Guild, error) {
	rows, err := db.reader(ctx).ListUserGuilds(ctx, int64(userID))
	if err != nil {
		return nil, fmt.Errorf("list guilds of user %d: %w", userID, mapError(err))
	}
	out := make([]domain.Guild, len(rows))
	for i, r := range rows {
		out[i] = toGuild(r)
	}
	return out, nil
}

func toGuild(r sqlcgen.Guild) domain.Guild {
	return domain.Guild{
		ID:         domain.ID(r.ID),
		Name:       r.Name,
		IconFileID: fromNullID(r.IconFileID),
		OwnerID:    domain.ID(r.OwnerID),
		CreatedAt:  fromMS(r.CreatedAt),
		UpdatedAt:  fromMS(r.UpdatedAt),
	}
}

func (db *DB) AddMember(ctx context.Context, m domain.Member) error {
	err := db.writer(ctx).AddMember(ctx, sqlcgen.AddMemberParams{
		GuildID:   int64(m.GuildID),
		UserID:    int64(m.UserID),
		Role:      string(m.Role),
		JoinedAt:  toMS(m.JoinedAt),
		UpdatedAt: toMS(m.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("add user %d to guild %d: %w", m.UserID, m.GuildID, mapError(err))
	}
	return nil
}

func (db *DB) MemberByID(ctx context.Context, guildID, userID domain.ID) (domain.Member, error) {
	row, err := db.reader(ctx).GetMember(ctx, sqlcgen.GetMemberParams{GuildID: int64(guildID), UserID: int64(userID)})
	if err != nil {
		return domain.Member{}, fmt.Errorf("load member %d of guild %d: %w", userID, guildID, mapError(err))
	}
	return toMember(row), nil
}

func (db *DB) UpdateMemberRole(ctx context.Context, guildID, userID domain.ID, role domain.GuildRole, at time.Time) error {
	n, err := db.writer(ctx).UpdateMemberRole(ctx, sqlcgen.UpdateMemberRoleParams{
		Role:      string(role),
		UpdatedAt: toMS(at),
		GuildID:   int64(guildID),
		UserID:    int64(userID),
	})
	return affected(n, err, "update role of member %d", userID)
}

func (db *DB) UpdateMemberVoiceModeration(ctx context.Context, guildID, userID domain.ID, serverMute, serverDeaf bool, at time.Time) error {
	n, err := db.writer(ctx).UpdateMemberVoiceModeration(ctx, sqlcgen.UpdateMemberVoiceModerationParams{
		ServerMute: boolInt(serverMute),
		ServerDeaf: boolInt(serverDeaf),
		UpdatedAt:  toMS(at),
		GuildID:    int64(guildID),
		UserID:     int64(userID),
	})
	return affected(n, err, "update voice moderation of member %d", userID)
}

func (db *DB) RemoveMember(ctx context.Context, guildID, userID domain.ID) error {
	n, err := db.writer(ctx).RemoveMember(ctx, sqlcgen.RemoveMemberParams{GuildID: int64(guildID), UserID: int64(userID)})
	return affected(n, err, "remove member %d", userID)
}

func (db *DB) GuildMembers(ctx context.Context, guildID domain.ID) ([]domain.Member, error) {
	rows, err := db.reader(ctx).ListGuildMembers(ctx, int64(guildID))
	if err != nil {
		return nil, fmt.Errorf("list members of guild %d: %w", guildID, mapError(err))
	}
	return toMembers(rows), nil
}

func (db *DB) UserMemberships(ctx context.Context, userID domain.ID) ([]domain.Member, error) {
	rows, err := db.reader(ctx).ListUserMemberships(ctx, int64(userID))
	if err != nil {
		return nil, fmt.Errorf("list memberships of user %d: %w", userID, mapError(err))
	}
	return toMembers(rows), nil
}

func (db *DB) CountMembers(ctx context.Context, guildID domain.ID) (int, error) {
	n, err := db.reader(ctx).CountGuildMembers(ctx, int64(guildID))
	if err != nil {
		return 0, fmt.Errorf("count members of guild %d: %w", guildID, mapError(err))
	}
	return int(n), nil
}

func toMembers(rows []sqlcgen.GuildMember) []domain.Member {
	out := make([]domain.Member, len(rows))
	for i, r := range rows {
		out[i] = toMember(r)
	}
	return out
}

func toMember(r sqlcgen.GuildMember) domain.Member {
	return domain.Member{
		GuildID:    domain.ID(r.GuildID),
		UserID:     domain.ID(r.UserID),
		Role:       domain.GuildRole(r.Role),
		ServerMute: r.ServerMute != 0,
		ServerDeaf: r.ServerDeaf != 0,
		JoinedAt:   fromMS(r.JoinedAt),
		UpdatedAt:  fromMS(r.UpdatedAt),
	}
}

// affected turns an UPDATE or DELETE result into ErrNotFound when no row matched.
func affected(n int64, err error, format string, args ...any) error {
	what := fmt.Sprintf(format, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", what, mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", what, domain.ErrNotFound)
	}
	return nil
}
