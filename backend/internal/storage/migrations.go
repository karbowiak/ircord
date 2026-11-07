package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const schemaVersion = 4

func (s *Store) migrate(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var legacy, ledger bool
	if err := s.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='accounts'), EXISTS(SELECT 1 FROM sqlite_master WHERE name='goose_db_version')").Scan(&legacy, &ledger); err != nil {
		return err
	}
	if legacy && !ledger {
		backup := filepath.Join(dir, "pre-goose-"+time.Now().UTC().Format("20060102T150405.000000000")+".db")
		if _, err := s.DB.ExecContext(ctx, "VACUUM INTO ?", backup); err != nil {
			return fmt.Errorf("back up legacy database before migrations: %w", err)
		}
		if err := os.Chmod(backup, 0600); err != nil {
			return err
		}
	}
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, s.DB, files,
		goose.WithDisableGlobalRegistry(true),
		goose.WithGoMigrations(
			goose.NewGoMigration(2, &goose.GoFunc{RunTx: upgradePosition}, nil),
			goose.NewGoMigration(4, &goose.GoFunc{RunTx: upgradeLegacyInvitations}, nil)))
	if err != nil {
		return err
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return err
	}
	if version > schemaVersion {
		return fmt.Errorf("database schema %d is newer than supported version %d", version, schemaVersion)
	}
	if _, err = provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}

// Early development databases may already contain this column without a ledger.
func upgradePosition(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA table_info(messages)")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var id, notnull, pk int
		var name, kind string
		var defaultValue any
		if err = rows.Scan(&id, &name, &kind, &notnull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "position" {
			found = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !found {
		if _, err = tx.ExecContext(ctx, "ALTER TABLE messages ADD COLUMN position INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS messages_position ON messages(owner,conversation,created_ns DESC,position DESC,id DESC)")
	return err
}
