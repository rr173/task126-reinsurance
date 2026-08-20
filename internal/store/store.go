package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"task126-reinsurance/internal/domain"
)

// DBTX is the common interface satisfied by *sql.DB and *sql.Tx so that store
// read functions can run inside the caller's transaction. With
// SetMaxOpenConns(1) on the SQLite handle, running a second connection-bound
// query mid-transaction would deadlock; routing every tx-scope read through
// the same tx avoids that.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Store is the persistence facade. Every entity-specific collection lives in its
// own file but shares the same handle and schema.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite database at dsn, applies migrations and
// configures the connection for single-writer concurrency.
func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Single writer: modernc.org/sqlite serialises writes at the DB level. One
	// connection avoids "database is locked" during nested tx reads.
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set foreign_keys: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying handle for advanced callers (health check).
func (s *Store) DB() *sql.DB { return s.db }

// InTx runs fn inside a single transaction, committing on a nil return and
// rolling back on any error. The tx is passed to fn as a DBTX so all reads in
// scope use the same connection.
func (s *Store) InTx(ctx context.Context, fn func(tx DBTX) error) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
			return
		}
		if err = tx.Commit(); err != nil {
			err = fmt.Errorf("commit tx: %w", err)
		}
	}()
	return fn(tx)
}

// ensureRowsAffected returns an error if res did not change exactly one row.
func ensureRowsAffected(res sql.Result, want int64, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != want {
		return domain.Newf(domain.ErrNotFound, "%s not changed (rows=%d, want %d)", what, n, want)
	}
	return nil
}
