package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const schemaVersion = 21

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
		// Independent feature migrations may land in adjacent PRs out of order.
		// Apply a newly introduced lower version rather than skipping it or
		// preventing startup after a higher version was already deployed.
		goose.WithAllowOutofOrder(true),
		goose.WithGoMigrations(
			goose.NewGoMigration(2, &goose.GoFunc{RunTx: upgradePosition}, nil),
			goose.NewGoMigration(4, &goose.GoFunc{RunTx: upgradeLegacyInvitations}, nil),
			goose.NewGoMigration(5, &goose.GoFunc{RunTx: upgradeSelfDisplayNames}, nil),
			goose.NewGoMigration(6, &goose.GoFunc{RunTx: upgradeUsernames}, nil),
			goose.NewGoMigration(8, &goose.GoFunc{RunTx: addManualUnread}, nil),
			goose.NewGoMigration(13, &goose.GoFunc{RunTx: addSessionDevices}, nil),
			goose.NewGoMigration(19, &goose.GoFunc{RunTx: addLinkCatalog}, nil)))
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
	found, err := hasColumn(ctx, tx, "messages", "position")
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

// upgradeUsernames adds username and password accounts (#64). Key accounts are retired: rows
// without a username are deleted by the account cleanup, and key_hash only satisfies the old
// schema. Columns are checked first so the migration can run again on a legacy database.
func upgradeUsernames(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT name FROM pragma_table_info('accounts')")
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, column := range []string{"username", "password_hash"} {
		if !columns[column] {
			if _, err = tx.ExecContext(ctx, "ALTER TABLE accounts ADD COLUMN "+column+" TEXT"); err != nil {
				return err
			}
		}
	}
	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS accounts_username ON accounts(username COLLATE NOCASE) WHERE username IS NOT NULL")
	return err
}

func hasColumn(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var id, notnull, pk int
		var name, kind string
		var defaultValue any
		if err = rows.Scan(&id, &name, &kind, &notnull, &defaultValue, &pk); err != nil {
			return false, err
		}
		found = found || name == column
	}
	return found, rows.Err()
}

// A manual "mark as unread" boundary. It overrides the monotonic read position for
// unread counting until the conversation is read again; it is never sent upstream.
func addManualUnread(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []string{"unread_from_ns INTEGER", "unread_from_id TEXT"} {
		name := column[:strings.IndexByte(column, ' ')]
		found, err := hasColumn(ctx, tx, "conversation_read", name)
		if err != nil {
			return err
		}
		if !found {
			if _, err = tx.ExecContext(ctx, "ALTER TABLE conversation_read ADD COLUMN "+column); err != nil {
				return err
			}
		}
	}
	return nil
}

// Devices & sessions (#85): when each session was last seen and which browser signed it in.
// Sessions from before this keep a zero last-seen time and an empty user agent, and are
// shown as unknown.
func addSessionDevices(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []struct{ name, definition string }{
		{"last_seen_ns", "INTEGER NOT NULL DEFAULT 0"},
		{"user_agent", "TEXT NOT NULL DEFAULT ''"},
	} {
		found, err := hasColumn(ctx, tx, "sessions", column.name)
		if err != nil {
			return err
		}
		if !found {
			if _, err = tx.ExecContext(ctx, "ALTER TABLE sessions ADD COLUMN "+column.name+" "+column.definition); err != nil {
				return err
			}
		}
	}
	return nil
}

// The link catalog (#102) reads the message link index: every link of a
// message's text gets an identity that groups posts of the same resource, its
// host for domain filters, and whether it is the link a server preview
// describes. Rows without an identity (uploads, picked GIFs, preview media)
// are gallery-only. msgindex.Version 2 re-indexes stored history so existing
// rows gain these columns. Columns are checked first so the migration can run
// again on a legacy database.
func addLinkCatalog(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []struct{ name, definition string }{
		{"identity", "TEXT NOT NULL DEFAULT ''"},
		{"host", "TEXT NOT NULL DEFAULT ''"},
		{"previewed", "INTEGER NOT NULL DEFAULT 0"},
	} {
		found, err := hasColumn(ctx, tx, "message_links", column.name)
		if err != nil {
			return err
		}
		if !found {
			if _, err = tx.ExecContext(ctx, "ALTER TABLE message_links ADD COLUMN "+column.name+" "+column.definition); err != nil {
				return err
			}
		}
	}
	// The predicate must match catalogRows in storage/catalog.go.
	for _, index := range []string{
		"CREATE INDEX IF NOT EXISTS message_links_catalog ON message_links(owner, created_ns DESC) WHERE identity!=''",
		"CREATE INDEX IF NOT EXISTS message_links_catalog_identity ON message_links(owner, identity, created_ns DESC) WHERE identity!=''",
	} {
		if _, err := tx.ExecContext(ctx, index); err != nil {
			return err
		}
	}
	return nil
}
