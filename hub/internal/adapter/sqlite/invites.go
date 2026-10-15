package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/sqlite/sqlcgen"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

func (db *DB) CreateInvite(ctx context.Context, inv domain.Invite) error {
	err := db.writer(ctx).CreateInvite(ctx, sqlcgen.CreateInviteParams{
		Code:      inv.Code,
		GuildID:   int64(inv.GuildID),
		CreatedBy: int64(inv.CreatedBy),
		MaxUses:   int64(inv.MaxUses),
		Uses:      int64(inv.Uses),
		ExpiresAt: toNullMS(inv.ExpiresAt),
		RevokedAt: toNullMS(inv.RevokedAt),
		CreatedAt: toMS(inv.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("create invite: %w", mapError(err))
	}
	return nil
}

func (db *DB) InviteByCode(ctx context.Context, code string) (domain.Invite, error) {
	row, err := db.reader(ctx).GetInvite(ctx, code)
	if err != nil {
		return domain.Invite{}, fmt.Errorf("load invite: %w", mapError(err))
	}
	return toInvite(row), nil
}

func (db *DB) UseInvite(ctx context.Context, code string, now time.Time) error {
	n, err := db.writer(ctx).UseInvite(ctx, sqlcgen.UseInviteParams{Code: code, Now: toNullMS(&now)})
	return affected(n, err, "use invite")
}

func (db *DB) RevokeInvite(ctx context.Context, code string, at time.Time) error {
	n, err := db.writer(ctx).RevokeInvite(ctx, sqlcgen.RevokeInviteParams{RevokedAt: toNullMS(&at), Code: code})
	return affected(n, err, "revoke invite")
}

func (db *DB) UsableInvites(ctx context.Context, guildID domain.ID, now time.Time) ([]domain.Invite, error) {
	rows, err := db.reader(ctx).ListUsableGuildInvites(ctx, sqlcgen.ListUsableGuildInvitesParams{
		GuildID: int64(guildID), Now: toNullMS(&now),
	})
	if err != nil {
		return nil, fmt.Errorf("list invites of guild %d: %w", guildID, mapError(err))
	}
	out := make([]domain.Invite, len(rows))
	for i, r := range rows {
		out[i] = toInvite(r)
	}
	return out, nil
}

func toInvite(r sqlcgen.Invite) domain.Invite {
	return domain.Invite{
		Code:      r.Code,
		GuildID:   domain.ID(r.GuildID),
		CreatedBy: domain.ID(r.CreatedBy),
		MaxUses:   int(r.MaxUses),
		Uses:      int(r.Uses),
		ExpiresAt: fromNullMS(r.ExpiresAt),
		RevokedAt: fromNullMS(r.RevokedAt),
		CreatedAt: fromMS(r.CreatedAt),
	}
}

func (db *DB) CreateBan(ctx context.Context, b domain.Ban) error {
	err := db.writer(ctx).CreateBan(ctx, sqlcgen.CreateBanParams{
		GuildID:   int64(b.GuildID),
		UserID:    int64(b.UserID),
		BannedBy:  toNullID(b.BannedBy),
		Reason:    b.Reason,
		CreatedAt: toMS(b.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("ban user %d from guild %d: %w", b.UserID, b.GuildID, mapError(err))
	}
	return nil
}

func (db *DB) BanByID(ctx context.Context, guildID, userID domain.ID) (domain.Ban, error) {
	row, err := db.reader(ctx).GetBan(ctx, sqlcgen.GetBanParams{GuildID: int64(guildID), UserID: int64(userID)})
	if err != nil {
		return domain.Ban{}, fmt.Errorf("load ban of user %d in guild %d: %w", userID, guildID, mapError(err))
	}
	return toBan(row), nil
}

func (db *DB) DeleteBan(ctx context.Context, guildID, userID domain.ID) error {
	n, err := db.writer(ctx).DeleteBan(ctx, sqlcgen.DeleteBanParams{GuildID: int64(guildID), UserID: int64(userID)})
	return affected(n, err, "unban user %d", userID)
}

func (db *DB) GuildBans(ctx context.Context, guildID domain.ID) ([]domain.Ban, error) {
	rows, err := db.reader(ctx).ListGuildBans(ctx, int64(guildID))
	if err != nil {
		return nil, fmt.Errorf("list bans of guild %d: %w", guildID, mapError(err))
	}
	out := make([]domain.Ban, len(rows))
	for i, r := range rows {
		out[i] = toBan(r)
	}
	return out, nil
}

func toBan(r sqlcgen.GuildBan) domain.Ban {
	return domain.Ban{
		GuildID:   domain.ID(r.GuildID),
		UserID:    domain.ID(r.UserID),
		BannedBy:  fromNullID(r.BannedBy),
		Reason:    r.Reason,
		CreatedAt: fromMS(r.CreatedAt),
	}
}

func (db *DB) AppendAudit(ctx context.Context, e domain.AuditEntry) error {
	data := e.Data
	if data == nil {
		data = map[string]string{}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode audit data: %w", err)
	}
	err = db.writer(ctx).AppendAudit(ctx, sqlcgen.AppendAuditParams{
		ID:         int64(e.ID),
		GuildID:    int64(e.GuildID),
		ActorID:    toNullID(e.ActorID),
		Action:     string(e.Action),
		TargetType: string(e.TargetType),
		TargetID:   e.TargetID,
		DataJson:   string(raw),
		CreatedAt:  toMS(e.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("append audit entry: %w", mapError(err))
	}
	return nil
}

func (db *DB) AuditLog(ctx context.Context, guildID, before domain.ID, limit int) ([]domain.AuditEntry, error) {
	b := int64(before)
	if b == 0 {
		b = math.MaxInt64
	}
	rows, err := db.reader(ctx).ListAudit(ctx, sqlcgen.ListAuditParams{GuildID: int64(guildID), Before: b, PageSize: int64(limit)})
	if err != nil {
		return nil, fmt.Errorf("list audit log of guild %d: %w", guildID, mapError(err))
	}
	out := make([]domain.AuditEntry, len(rows))
	for i, r := range rows {
		data := map[string]string{}
		if err := json.Unmarshal([]byte(r.DataJson), &data); err != nil {
			return nil, fmt.Errorf("decode audit entry %d: %w", r.ID, err)
		}
		out[i] = domain.AuditEntry{
			ID:         domain.ID(r.ID),
			GuildID:    domain.ID(r.GuildID),
			ActorID:    fromNullID(r.ActorID),
			Action:     domain.AuditAction(r.Action),
			TargetType: domain.AuditTarget(r.TargetType),
			TargetID:   r.TargetID,
			Data:       data,
			CreatedAt:  fromMS(r.CreatedAt),
		}
	}
	return out, nil
}

func (db *DB) CreateFile(ctx context.Context, f domain.File) error {
	err := db.writer(ctx).CreateFile(ctx, sqlcgen.CreateFileParams{
		ID:           int64(f.ID),
		OwnerID:      toNullID(f.OwnerID),
		Purpose:      string(f.Purpose),
		Sha256:       f.SHA256,
		Size:         f.Size,
		Mime:         f.Mime,
		OriginalName: f.OriginalName,
		Width:        int64(f.Width),
		Height:       int64(f.Height),
		StorageKey:   f.StorageKey,
		CreatedAt:    toMS(f.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("create file: %w", mapError(err))
	}
	return nil
}

func (db *DB) FileByID(ctx context.Context, id domain.ID) (domain.File, error) {
	r, err := db.reader(ctx).GetFile(ctx, int64(id))
	if err != nil {
		return domain.File{}, fmt.Errorf("load file %d: %w", id, mapError(err))
	}
	return domain.File{
		ID:           domain.ID(r.ID),
		OwnerID:      fromNullID(r.OwnerID),
		Purpose:      domain.FilePurpose(r.Purpose),
		SHA256:       r.Sha256,
		Size:         r.Size,
		Mime:         r.Mime,
		OriginalName: r.OriginalName,
		Width:        int(r.Width),
		Height:       int(r.Height),
		StorageKey:   r.StorageKey,
		CreatedAt:    fromMS(r.CreatedAt),
	}, nil
}
