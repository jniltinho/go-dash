// Package database opens the SQLite database, applies embedded migrations
// and provides consistent backups.
package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// noForeignKeysDirective, as the first line of a migration, runs it with
// foreign key enforcement off (needed to rebuild a referenced table).
const noForeignKeysDirective = "-- migrate:no-foreign-keys"

// Open opens (creating if needed) the SQLite database at path. Pragmas are
// passed through the DSN so the driver applies them to every pooled connection.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("creating database directory: %w", err)
		}
	}

	query := url.Values{}
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "synchronous(NORMAL)")
	dsn := "file:" + path + "?" + query.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	return db, nil
}

// OpenAndMigrate opens the database and applies pending migrations.
func OpenAndMigrate(ctx context.Context, path string) (*sql.DB, error) {
	db, err := Open(ctx, path)
	if err != nil {
		return nil, err
	}

	if _, err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// Migrate applies pending embedded migrations in lexical order and returns
// the names of the migrations that were applied.
func Migrate(ctx context.Context, db *sql.DB) ([]string, error) {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("opening migrations: %w", err)
	}
	return migrate(ctx, db, sub)
}

// migrate applies the *.sql files of fsys that are not recorded yet.
func migrate(ctx context.Context, db *sql.DB, fsys fs.FS) ([]string, error) {
	const createTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
	)`
	if _, err := db.ExecContext(ctx, createTable); err != nil {
		return nil, fmt.Errorf("creating schema_migrations: %w", err)
	}

	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("listing migrations: %w", err)
	}
	sort.Strings(names)

	applied := []string{}
	for _, name := range names {
		version := strings.TrimSuffix(filepath.Base(name), ".sql")

		done, err := isApplied(ctx, db, version)
		if err != nil {
			return applied, err
		}
		if done {
			continue
		}

		if err := applyMigration(ctx, db, fsys, name, version); err != nil {
			return applied, err
		}
		applied = append(applied, version)
	}

	return applied, nil
}

func isApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var found string
	err := db.QueryRowContext(ctx, "SELECT version FROM schema_migrations WHERE version = ?", version).Scan(&found)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("checking migration %q: %w", version, err)
	}
	return true, nil
}

func applyMigration(ctx context.Context, db *sql.DB, fsys fs.FS, name, version string) error {
	body, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("reading migration %q: %w", name, err)
	}
	if strings.HasPrefix(string(body), noForeignKeysDirective) {
		return applyWithoutForeignKeys(ctx, db, string(body), version)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting migration %q: %w", version, err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := runMigration(ctx, tx, string(body), version, false); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration %q: %w", version, err)
	}
	return nil
}

// runMigration executes body and records version inside tx. With checkFK it
// fails when PRAGMA foreign_key_check reports any violation.
func runMigration(ctx context.Context, tx *sql.Tx, body, version string, checkFK bool) error {
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("applying migration %q: %w", version, err)
	}
	if checkFK {
		if err := foreignKeyCheck(ctx, tx); err != nil {
			return fmt.Errorf("applying migration %q: %w", version, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("recording migration %q: %w", version, err)
	}
	return nil
}

// applyWithoutForeignKeys runs one migration on a dedicated connection with
// foreign keys off. PRAGMA foreign_keys cannot change inside a transaction
// and is connection-local, so it is restored (and verified) before the
// connection goes back to the pool; if that fails the connection is discarded.
func applyWithoutForeignKeys(ctx context.Context, db *sql.DB, body, version string) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection for migration %q: %w", version, err)
	}
	defer func() {
		// Restore even when ctx is cancelled: the connection outlives it.
		if restoreErr := setForeignKeys(context.WithoutCancel(ctx), conn, true); restoreErr != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			err = errors.Join(err, fmt.Errorf("restoring foreign keys after migration %q: %w", version, restoreErr))
		}
		_ = conn.Close()
	}()

	if err := setForeignKeys(ctx, conn, false); err != nil {
		return fmt.Errorf("disabling foreign keys for migration %q: %w", version, err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting migration %q: %w", version, err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := runMigration(ctx, tx, body, version, true); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration %q: %w", version, err)
	}
	return nil
}

func setForeignKeys(ctx context.Context, conn *sql.Conn, on bool) error {
	want := 0
	if on {
		want = 1
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA foreign_keys = %d", want)); err != nil {
		return err
	}
	var got int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&got); err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("PRAGMA foreign_keys is %d, want %d", got, want)
	}
	return nil
}

func foreignKeyCheck(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("checking foreign keys: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var (
			table, parent string
			rowid         sql.NullInt64
			fkid          int
		)
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return fmt.Errorf("reading foreign key violation: %w", err)
		}
		return fmt.Errorf("foreign key violation: %s row %d references missing %s", table, rowid.Int64, parent)
	}
	return rows.Err()
}

// Backup writes a consistent copy of the database (including WAL content)
// to outPath using VACUUM INTO. outPath must not exist.
func Backup(ctx context.Context, db *sql.DB, outPath string) error {
	if _, err := os.Stat(outPath); err == nil {
		return fmt.Errorf("backup target %q already exists", outPath)
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", outPath); err != nil {
		return fmt.Errorf("writing backup: %w", err)
	}
	return nil
}
