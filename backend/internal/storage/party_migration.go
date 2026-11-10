package storage

import (
	"context"
	"database/sql"
)

// The local autoreloader can have applied an earlier development version of
// migration 31. Finish that shape without discarding any account or party data.
func reconcilePartySchema(ctx context.Context, tx *sql.Tx) error {
	raw, e := migrationFiles.ReadFile("migrations/00031_parties.sql")
	if e != nil {
		return e
	}
	schema := string(raw)
	if _, e = tx.ExecContext(ctx, schema); e != nil {
		return e
	}
	found, e := hasColumn(ctx, tx, "party_history", "revision")
	if e != nil {
		return e
	}
	if !found {
		if _, e = tx.ExecContext(ctx, "ALTER TABLE party_history ADD COLUMN revision INTEGER NOT NULL DEFAULT 1"); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO party_identities(account_id,username,payload) SELECT json_extract(m.value,'$.accountId'),json_extract(m.value,'$.username'),json_remove(m.value,'$.joinedAt') FROM parties p,json_each(p.payload,'$.members') m"); e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO party_participants(party,account_id,username) SELECT p.id,json_extract(m.value,'$.accountId'),json_extract(m.value,'$.username') FROM parties p,json_each(p.payload,'$.members') m")
	return e
}

func partyFileScopeIndex(ctx context.Context, tx *sql.Tx) error {
	_, e := tx.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS party_file_urls ON message_links(url,conversation)")
	return e
}
