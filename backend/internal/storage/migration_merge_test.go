package storage

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

// Version 38 was used independently for two changes. Both histories must be
// repaired even though the ledger says that version 38 has already run.
func TestMigration39ReconcilesBothVersion38Histories(t *testing.T) {
	indexes := []string{"events_message", "events_line_msgid", "events_line_edit", "events_line_client", "events_line_label", "events_line_batch", "events_line_batch_ref", "messages_replies"}
	columns := []struct{ name, kind string }{{"at_position", "INTEGER"}, {"at_id", "TEXT"}, {"unread_from_position", "INTEGER"}}
	type readState struct {
		at             int64
		unreadAt       sql.NullInt64
		unreadID       sql.NullString
		position       sql.NullInt64
		id             sql.NullString
		unreadPosition sql.NullInt64
	}
	for _, history := range []string{"released-indexes", "local-read-boundaries"} {
		t.Run(history, func(t *testing.T) {
			dir := t.TempDir()
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			account(t, s, "alice")
			at := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
			for _, id := range []string{"a", "b"} {
				if err := s.SaveHistory("alice", model.Message{ID: id, ConversationID: "room", SenderID: "peer", Kind: "message", Text: "retained " + id, CreatedAt: at}); err != nil {
					t.Fatal(err)
				}
			}
			if found, applied, err := s.MarkReadThrough("alice", "room", "a"); err != nil || !found || !applied {
				t.Fatalf("mark read: %t %t %v", found, applied, err)
			}
			if found, err := s.MarkUnread("alice", "room", "b"); err != nil || !found {
				t.Fatalf("mark unread: %t %v", found, err)
			}
			originalHistory, err := s.History("alice", "room", "", 20)
			if err != nil || len(originalHistory.Items) != 2 {
				t.Fatalf("original history: %+v %v", originalHistory.Items, err)
			}
			read := func(store *Store) readState {
				t.Helper()
				var state readState
				err := store.DB.QueryRow(`SELECT at_ns,unread_from_ns,unread_from_id,at_position,at_id,unread_from_position FROM conversation_read WHERE owner='alice' AND conversation='room'`).Scan(&state.at, &state.unreadAt, &state.unreadID, &state.position, &state.id, &state.unreadPosition)
				if err != nil {
					t.Fatal(err)
				}
				return state
			}
			want := read(s)
			if !want.position.Valid || !want.id.Valid || !want.unreadPosition.Valid || !want.unreadAt.Valid || !want.unreadID.Valid {
				t.Fatalf("test did not establish read boundaries: %+v", want)
			}
			indexSQL := map[string]string{}
			for _, name := range indexes {
				var statement string
				if err := s.DB.QueryRow("SELECT sql FROM sqlite_master WHERE type='index' AND name=?", name).Scan(&statement); err != nil {
					t.Fatalf("fresh %s: %v", name, err)
				}
				indexSQL[name] = statement
			}
			if _, err = s.DB.Exec("DELETE FROM goose_db_version WHERE version_id=39"); err != nil {
				t.Fatal(err)
			}
			if history == "released-indexes" {
				for _, column := range columns {
					if _, err = s.DB.Exec("ALTER TABLE conversation_read DROP COLUMN " + column.name); err != nil {
						t.Fatal(err)
					}
				}
				want.position = sql.NullInt64{}
				want.id = sql.NullString{}
				want.unreadPosition = sql.NullInt64{}
			} else {
				for _, name := range indexes {
					if _, err = s.DB.Exec("DROP INDEX " + name); err != nil {
						t.Fatal(err)
					}
				}
			}
			var applied38 int
			if err = s.DB.QueryRow("SELECT count(*) FROM goose_db_version WHERE version_id=38 AND is_applied=1").Scan(&applied38); err != nil || applied38 != 1 {
				t.Fatalf("version 38 ledger: %d %v", applied38, err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				s, err = Open(dir)
				if err != nil {
					t.Fatalf("reopen %d: %v", attempt, err)
				}
				for _, column := range columns {
					var kind string
					if err = s.DB.QueryRow("SELECT type FROM pragma_table_info('conversation_read') WHERE name=?", column.name).Scan(&kind); err != nil || kind != column.kind {
						t.Errorf("column %s type %q: %v", column.name, kind, err)
					}
				}
				for _, name := range indexes {
					var statement string
					if err = s.DB.QueryRow("SELECT sql FROM sqlite_master WHERE type='index' AND name=?", name).Scan(&statement); err != nil || statement != indexSQL[name] {
						t.Errorf("index %s = %q, want %q: %v", name, statement, indexSQL[name], err)
					}
				}
				if got := read(s); !reflect.DeepEqual(got, want) {
					t.Errorf("read state changed: got %+v, want %+v", got, want)
				}
				if exists, err := s.AccountExistsByID("alice"); err != nil || !exists {
					t.Errorf("account lost: %t %v", exists, err)
				}
				page, err := s.History("alice", "room", "", 20)
				if err != nil || !reflect.DeepEqual(page.Items, originalHistory.Items) {
					t.Errorf("history changed: got %+v, want %+v: %v", page.Items, originalHistory.Items, err)
				}
				var count39 int
				if err = s.DB.QueryRow("SELECT count(*) FROM goose_db_version WHERE version_id=39 AND is_applied=1").Scan(&count39); err != nil || count39 != 1 {
					t.Errorf("version 39 ledger: %d %v", count39, err)
				}
				// Force a second application to check the repair itself, not only ordinary startup.
				if attempt == 0 {
					if _, err = s.DB.Exec("DELETE FROM goose_db_version WHERE version_id=39"); err != nil {
						t.Fatal(err)
					}
				}
				if err = s.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
