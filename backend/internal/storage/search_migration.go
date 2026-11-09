package storage

import (
	"context"
	"database/sql"
)

// Reconcile both branches' versions 25 and 26, which may have applied either set.
func reconcileBranchMigrations(ctx context.Context, tx *sql.Tx) error {
	if err := addMessageSearch(ctx, tx); err != nil {
		return err
	}
	if err := dropBareNameMentions(ctx, tx); err != nil {
		return err
	}
	if err := addNetworkSignedOut(ctx, tx); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS messages_offline_page ON messages(owner,created_ns DESC,id DESC)`)
	return err
}

// Legacy databases without a Goose ledger can already have newer columns.
func addMessageSearch(ctx context.Context, tx *sql.Tx) error {
	for _, column := range []string{"search_visible", "search_kinds"} {
		found, err := hasColumn(ctx, tx, "messages", column)
		if err != nil {
			return err
		}
		if !found {
			if _, err = tx.ExecContext(ctx, "ALTER TABLE messages ADD COLUMN "+column+" INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
		}
	}
	for _, statement := range []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS message_search USING fts5(text, content='', contentless_delete=1, tokenize='unicode61 remove_diacritics 2', prefix='2 3')`,
		`CREATE INDEX IF NOT EXISTS messages_search_page ON messages(owner, created_ns DESC, id DESC) WHERE search_visible=1`,
		`CREATE INDEX IF NOT EXISTS messages_search_sender ON messages(owner, sender, created_ns DESC, id DESC) WHERE search_visible=1`,
		`CREATE TRIGGER IF NOT EXISTS messages_search_insert AFTER INSERT ON messages WHEN new.search_visible=1 BEGIN
		  INSERT OR REPLACE INTO message_search(rowid,text) VALUES(new.rowid,new.text);
		END`,
		`CREATE TRIGGER IF NOT EXISTS messages_search_update AFTER UPDATE OF text,search_visible ON messages BEGIN
		  DELETE FROM message_search WHERE rowid=old.rowid;
		  INSERT OR REPLACE INTO message_search(rowid,text) SELECT new.rowid,new.text WHERE new.search_visible=1;
		END`,
		`CREATE TRIGGER IF NOT EXISTS messages_search_delete AFTER DELETE ON messages BEGIN
		  DELETE FROM message_search WHERE rowid=old.rowid;
		END`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
