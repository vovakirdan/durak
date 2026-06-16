package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/pressly/goose/v3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

const (
	sqliteStatusActive    = "active"
	sqliteStatusCompleted = "completed"
)

var (
	// ErrInvalidSQLiteStore means the SQLite store configuration or data is invalid.
	ErrInvalidSQLiteStore = errors.New("invalid sqlite store")
	// ErrSQLiteConflict means persisted match data conflicts with existing rows.
	ErrSQLiteConflict = errors.New("sqlite storage conflict")
)

//go:embed migrations/*.sql
var sqliteMigrations embed.FS

// SQLiteStore stores indexed match history in SQLite.
type SQLiteStore struct {
	db *bun.DB
}

// OpenSQLiteStore opens a SQLite database file and applies embedded migrations.
func OpenSQLiteStore(ctx context.Context, path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: path is empty", ErrInvalidSQLiteStore)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve path %q: %w", ErrInvalidSQLiteStore, path, err)
	}
	sqldb, err := sql.Open(sqliteshim.ShimName, sqliteDSN(abs))
	if err != nil {
		return nil, fmt.Errorf("%w: open %q: %w", ErrInvalidSQLiteStore, path, err)
	}
	sqldb.SetMaxOpenConns(1)

	store := &SQLiteStore{db: bun.NewDB(sqldb, sqlitedialect.New())}
	if err := store.Migrate(ctx); err != nil {
		closeErr := store.Close()
		if closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return store, nil
}

// Close closes the underlying database.
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Migrate applies embedded Goose migrations.
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return fmt.Errorf("%w: store is nil", ErrInvalidSQLiteStore)
	}
	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(sqliteMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("%w: set migration dialect: %w", ErrInvalidSQLiteStore, err)
	}
	if err := goose.Up(s.db.DB, "migrations"); err != nil {
		return fmt.Errorf("%w: run migrations: %w", ErrInvalidSQLiteStore, err)
	}
	return nil
}

func sqliteDSN(absPath string) string {
	return "file:" + filepath.ToSlash(absPath) + "?cache=shared&mode=rwc&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}
