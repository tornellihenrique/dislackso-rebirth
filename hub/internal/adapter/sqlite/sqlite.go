// Package sqlite implements the repositories and the transaction runner on
// SQLite: WAL mode, one writer connection and a pool of read-only ones.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	sqlite3 "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/sqlite/sqlcgen"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

//go:embed migrations/*.sql
var migrations embed.FS

type DB struct {
	write  *sql.DB
	read   *sql.DB
	writeQ *sqlcgen.Queries
	readQ  *sqlcgen.Queries
}

// Open opens (creating if needed) the database at path and applies pending
// migrations.
func Open(ctx context.Context, path string) (*DB, error) {
	write, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	// SQLite allows one writer at a time; a single connection turns lock
	// contention into Go-side queueing instead of SQLITE_BUSY errors.
	write.SetMaxOpenConns(1)
	if err := write.PingContext(ctx); err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := migrate(ctx, write); err != nil {
		_ = write.Close()
		return nil, err
	}

	read, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("open %s read-only: %w", path, err)
	}
	read.SetMaxOpenConns(max(4, runtime.NumCPU()))

	return &DB{write: write, read: read, writeQ: sqlcgen.New(write), readQ: sqlcgen.New(read)}, nil
}

func (db *DB) Close() error {
	return errors.Join(db.read.Close(), db.write.Close())
}

// Ping checks that the database answers; used as a readiness probe.
func (db *DB) Ping(ctx context.Context) error {
	if err := db.read.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

func (db *DB) Name() string { return "database" }

func (db *DB) Check(ctx context.Context) error { return db.Ping(ctx) }

func dsn(path string, readOnly bool) string {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	if readOnly {
		q.Add("_pragma", "query_only(1)")
	} else {
		q.Add("_pragma", "journal_mode(WAL)")
		// Take the write lock when the transaction starts, so two writers
		// never deadlock upgrading from a read lock.
		q.Set("_txlock", "immediate")
	}
	return path + "?" + q.Encode()
}

func migrate(ctx context.Context, db *sql.DB) error {
	dir, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("open migrations: %w", err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, db, dir, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return fmt.Errorf("prepare migrations: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

type txKey struct{}

type txState struct {
	tx          *sql.Tx
	afterCommit []func()
}

func txFrom(ctx context.Context) (*txState, bool) {
	st, ok := ctx.Value(txKey{}).(*txState)
	return st, ok
}

// InTx runs fn inside a write transaction. A nested call joins the outer one.
func (db *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFrom(ctx); ok {
		return fn(ctx)
	}
	tx, err := db.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	st := &txState{tx: tx}
	if err := fn(context.WithValue(ctx, txKey{}, st)); err != nil {
		return errors.Join(err, rollback(tx))
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	for _, hook := range st.afterCommit {
		hook()
	}
	return nil
}

// AfterCommit runs fn once the transaction in ctx commits, or right away
// outside a transaction. A rolled-back transaction drops it.
func (db *DB) AfterCommit(ctx context.Context, fn func()) {
	if st, ok := txFrom(ctx); ok {
		st.afterCommit = append(st.afterCommit, fn)
		return
	}
	fn()
}

func rollback(tx *sql.Tx) error {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("rollback: %w", err)
	}
	return nil
}

// reader queries through the caller's transaction when there is one, so a
// transaction sees its own writes.
func (db *DB) reader(ctx context.Context) *sqlcgen.Queries {
	if st, ok := txFrom(ctx); ok {
		return db.writeQ.WithTx(st.tx)
	}
	return db.readQ
}

func (db *DB) writer(ctx context.Context) *sqlcgen.Queries {
	if st, ok := txFrom(ctx); ok {
		return db.writeQ.WithTx(st.tx)
	}
	return db.writeQ
}

// mapError turns driver errors into domain errors: no rows into ErrNotFound
// and unique or primary key violations into ErrConflict naming the (first) column.
func mapError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	var se *sqlite3.Error
	if errors.As(err, &se) && (se.Code() == sqlite3lib.SQLITE_CONSTRAINT_UNIQUE || se.Code() == sqlite3lib.SQLITE_CONSTRAINT_PRIMARYKEY) {
		// The message reads "UNIQUE constraint failed: table.column (2067)".
		_, after, ok := strings.Cut(se.Error(), "constraint failed: ")
		if !ok {
			return domain.ErrConflict
		}
		_, column, _ := strings.Cut(after, ".")
		column, _, _ = strings.Cut(column, " ")
		column = strings.TrimSuffix(column, ",")
		return domain.Conflict(column)
	}
	return err
}

func toMS(t time.Time) int64 { return t.UnixMilli() }

func fromMS(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

func toNullMS(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UnixMilli(), Valid: true}
}

func fromNullMS(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := fromMS(v.Int64)
	return &t
}

func toNullID(id domain.ID) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(id), Valid: id != 0}
}

func fromNullID(v sql.NullInt64) domain.ID {
	if !v.Valid {
		return 0
	}
	return domain.ID(v.Int64)
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
