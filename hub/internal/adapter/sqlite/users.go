package sqlite

import (
	"context"
	"fmt"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/sqlite/sqlcgen"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

func (db *DB) CreateUser(ctx context.Context, u domain.User) error {
	err := db.writer(ctx).CreateUser(ctx, sqlcgen.CreateUserParams{
		ID:              int64(u.ID),
		Username:        u.Username,
		Email:           u.Email,
		EmailVerifiedAt: toNullMS(u.EmailVerifiedAt),
		PasswordHash:    u.PasswordHash,
		DisplayName:     u.DisplayName,
		StatusPref:      string(u.StatusPref),
		CustomStatus:    u.CustomStatus,
		IsInstanceAdmin: boolInt(u.IsInstanceAdmin),
		DisabledAt:      toNullMS(u.DisabledAt),
		CreatedAt:       toMS(u.CreatedAt),
		UpdatedAt:       toMS(u.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("create user: %w", mapError(err))
	}
	return nil
}

func (db *DB) UserByID(ctx context.Context, id domain.ID) (domain.User, error) {
	row, err := db.reader(ctx).GetUserByID(ctx, int64(id))
	if err != nil {
		return domain.User{}, fmt.Errorf("load user %d: %w", id, mapError(err))
	}
	return toUser(row), nil
}

func (db *DB) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	row, err := db.reader(ctx).GetUserByEmail(ctx, email)
	if err != nil {
		return domain.User{}, fmt.Errorf("load user by email: %w", mapError(err))
	}
	return toUser(row), nil
}

func (db *DB) UserByLogin(ctx context.Context, login string) (domain.User, error) {
	row, err := db.reader(ctx).GetUserByLogin(ctx, login)
	if err != nil {
		return domain.User{}, fmt.Errorf("load user by login: %w", mapError(err))
	}
	return toUser(row), nil
}

func (db *DB) UpdateUser(ctx context.Context, u domain.User) error {
	n, err := db.writer(ctx).UpdateUser(ctx, sqlcgen.UpdateUserParams{
		Username:        u.Username,
		Email:           u.Email,
		EmailVerifiedAt: toNullMS(u.EmailVerifiedAt),
		PasswordHash:    u.PasswordHash,
		DisplayName:     u.DisplayName,
		StatusPref:      string(u.StatusPref),
		CustomStatus:    u.CustomStatus,
		IsInstanceAdmin: boolInt(u.IsInstanceAdmin),
		DisabledAt:      toNullMS(u.DisabledAt),
		UpdatedAt:       toMS(u.UpdatedAt),
		ID:              int64(u.ID),
	})
	if err != nil {
		return fmt.Errorf("update user %d: %w", u.ID, mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("update user %d: %w", u.ID, domain.ErrNotFound)
	}
	return nil
}

func (db *DB) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := db.reader(ctx).ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", mapError(err))
	}
	users := make([]domain.User, len(rows))
	for i, r := range rows {
		users[i] = toUser(r)
	}
	return users, nil
}

func (db *DB) CountUsers(ctx context.Context) (int, error) {
	n, err := db.reader(ctx).CountUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", mapError(err))
	}
	return int(n), nil
}

func toUser(r sqlcgen.User) domain.User {
	return domain.User{
		ID:              domain.ID(r.ID),
		Username:        r.Username,
		Email:           r.Email,
		EmailVerifiedAt: fromNullMS(r.EmailVerifiedAt),
		PasswordHash:    r.PasswordHash,
		DisplayName:     r.DisplayName,
		StatusPref:      domain.StatusPref(r.StatusPref),
		CustomStatus:    r.CustomStatus,
		IsInstanceAdmin: r.IsInstanceAdmin != 0,
		DisabledAt:      fromNullMS(r.DisabledAt),
		CreatedAt:       fromMS(r.CreatedAt),
		UpdatedAt:       fromMS(r.UpdatedAt),
	}
}
