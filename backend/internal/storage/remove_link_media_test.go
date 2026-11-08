package storage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

func TestRemoveLinkMediaMigrationPreservesHistoryAndReplay(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	account(t, s, "alice")
	msg := model.Message{ID: "post", ConversationID: "room", SenderID: "alice", Kind: "message", Text: "https://example.com/photo.jpg", CreatedAt: time.Now(), Reactions: []model.Reaction{}}
	if _, err = s.Commit("alice", "message.created", msg); err != nil {
		t.Fatal(err)
	}
	// Recreate the schema and payloads the previous version left on disk.
	for _, name := range []string{"00017_media_cdn.sql", "00018_message_links.sql"} {
		raw, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		up := strings.Split(string(raw), "-- +goose Down")[0]
		if _, err = s.DB.Exec(up); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.DB.Exec(`UPDATE messages SET payload=json_set(payload, '$.preview',json('{"title":"old card"}'), '$.previewSource','server', '$.embeds',json('[{"status":"ready"}]'), '$.media',json('[{"url":"https://cdn.example/old"}]'))`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec(`UPDATE events SET payload=json_set(payload, '$.data.preview',json('{"title":"old card"}'), '$.data.previewSource','server', '$.data.embeds',json('[]'), '$.data.media',json('[]'))`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("DELETE FROM goose_db_version WHERE version_id=23"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, table := range []string{"media_objects", "media_sources", "media_references", "media_denied_sources"} {
		var count int
		if err = s.DB.QueryRow("SELECT count(*) FROM sqlite_master WHERE name=?", table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("retired table %s: count=%d err=%v", table, count, err)
		}
	}
	page, err := s.History("alice", "room", "", 40)
	if err != nil || len(page.Items) != 1 || page.Items[0].Text != msg.Text {
		t.Fatalf("history was not preserved: %+v %v", page, err)
	}
	for _, table := range []string{"messages", "events"} {
		var raw []byte
		if err = s.DB.QueryRow("SELECT payload FROM " + table + " WHERE owner='alice'").Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var payload map[string]json.RawMessage
		if err = json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		if table == "events" {
			if err = json.Unmarshal(payload["data"], &payload); err != nil {
				t.Fatal(err)
			}
		}
		for _, field := range []string{"preview", "previewSource", "embeds", "media"} {
			if _, ok := payload[field]; ok {
				t.Fatalf("%s retained retired field %s", table, field)
			}
		}
	}
	// The retained Links catalog still indexes new message writes.
	msg.ID = "next"
	if _, err = s.Commit("alice", "message.created", msg); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.DB.QueryRow("SELECT count(*) FROM message_links WHERE owner='alice' AND message='next'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("catalog write: count=%d err=%v", count, err)
	}
}

func TestLinkCatalogRepairsEarlierRewriteDatabase(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Earlier local rewrite versions 19/20 had already removed these tables.
	_, err = s.DB.Exec("DROP TABLE message_links; DROP TABLE index_jobs; DELETE FROM goose_db_version WHERE version_id IN (22,23); INSERT INTO goose_db_version(version_id,is_applied) VALUES(20,1)")
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.BackfillLinksStep(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.DB.QueryRow("SELECT count(*) FROM pragma_table_info('message_links') WHERE name IN ('identity','host','previewed')").Scan(&count); err != nil || count != 3 {
		t.Fatalf("repaired catalog columns: %d, %v", count, err)
	}
}
