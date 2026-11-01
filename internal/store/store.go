package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"livecheck/internal/domain"
)

// ErrSchemaNewer indicates the database was created by a newer application
// schema. Callers must not reset it automatically.
var ErrSchemaNewer = errors.New("database schema is newer than this build")

type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		sql: `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS app_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    revision INTEGER NOT NULL CHECK (revision >= 0),
    settings_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
INSERT OR IGNORE INTO app_settings (id, revision, settings_json, updated_at)
VALUES (1, 0, '{}', '1970-01-01T00:00:00Z');
`,
	},
}

// Store owns the local SQLite database. SQL and schema details stay here.
type Store struct {
	db   *sql.DB
	path string
}

func Open(ctx context.Context, path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	db, err := sql.Open("sqlite", abs)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", abs, err)
	}
	// One connection gives predictable serialized writes and keeps the
	// connection-scoped PRAGMAs in force.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %s: %w", abs, err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure busy timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure foreign keys: %w", err)
	}

	s := &Store{db: db, path: abs}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Path() string { return s.path }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	var current int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > len(migrations) {
		return fmt.Errorf("%w: database version %d, supported %d", ErrSchemaNewer, current, len(migrations))
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.version, err)
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", m.version, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			m.version, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.version, err)
		}
	}
	return nil
}

// SchemaVersion returns the highest applied migration version.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var version int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

func (s *Store) LoadSettings(ctx context.Context) (domain.Settings, error) {
	var revision int64
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT revision, settings_json FROM app_settings WHERE id = 1`).Scan(&revision, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Settings{}, nil
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("load settings: %w", err)
	}
	var exp domain.Expectations
	if err := json.Unmarshal([]byte(raw), &exp); err != nil {
		return domain.Settings{}, fmt.Errorf("decode settings: %w", err)
	}
	return domain.Settings{Revision: revision, Expectations: exp}, nil
}

// SaveSettings compares expectedRevision, validates the new expectations and
// increments the revision transactionally. A stale caller never overwrites
// newer data.
func (s *Store) SaveSettings(ctx context.Context, expectedRevision int64, exp domain.Expectations) (domain.Settings, error) {
	if expectedRevision < 0 {
		return domain.Settings{}, fmt.Errorf("expected revision must be non-negative")
	}
	if err := exp.Validate(); err != nil {
		return domain.Settings{}, err
	}
	raw, err := json.Marshal(exp)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("encode settings: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("begin settings update: %w", err)
	}
	defer tx.Rollback()

	var current int64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM app_settings WHERE id = 1`).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		if expectedRevision != 0 {
			return domain.Settings{}, domain.ErrRevisionConflict
		}
		newRevision := int64(1)
		if _, err := tx.ExecContext(ctx, `INSERT INTO app_settings (id, revision, settings_json, updated_at) VALUES (1, ?, ?, ?)`,
			newRevision, string(raw), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return domain.Settings{}, fmt.Errorf("insert settings: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return domain.Settings{}, fmt.Errorf("commit settings: %w", err)
		}
		return domain.Settings{Revision: newRevision, Expectations: exp}, nil
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("read settings revision: %w", err)
	}
	if current != expectedRevision {
		return domain.Settings{}, domain.ErrRevisionConflict
	}

	newRevision := current + 1
	res, err := tx.ExecContext(ctx, `UPDATE app_settings SET revision = ?, settings_json = ?, updated_at = ? WHERE id = 1 AND revision = ?`,
		newRevision, string(raw), time.Now().UTC().Format(time.RFC3339Nano), expectedRevision)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("update settings: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.Settings{}, fmt.Errorf("check settings update: %w", err)
	}
	if n != 1 {
		return domain.Settings{}, domain.ErrRevisionConflict
	}
	if err := tx.Commit(); err != nil {
		return domain.Settings{}, fmt.Errorf("commit settings: %w", err)
	}
	return domain.Settings{Revision: newRevision, Expectations: exp}, nil
}
