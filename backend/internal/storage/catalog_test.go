package storage

import (
	"testing"
	"time"

	"ircord/backend/internal/model"
)

type catalogRow struct {
	url, kind, identity string
	previewed           bool
}

func catalogRowsOf(t *testing.T, s *Store, message string) []catalogRow {
	t.Helper()
	rows, err := s.DB.Query("SELECT url,kind,identity,previewed FROM message_links WHERE owner='alice' AND message=? ORDER BY idx", message)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []catalogRow{}
	for rows.Next() {
		var r catalogRow
		if err = rows.Scan(&r.url, &r.kind, &r.identity, &r.previewed); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// Galleries and the link catalog share one index: a text image link is one
// row serving both, other links are catalogue-only rows, and gallery-only
// entries (uploads, picked GIFs) carry no catalog identity.
func TestLinkCatalogSharesTheMessageIndexWithGalleries(t *testing.T) {
	s := galleryStore(t)
	msg := model.Message{ID: "net:1", ConversationID: "net:#pics", SenderID: "carol", Kind: "message", CreatedAt: time.Now().UTC(),
		Text: "http://plain.test/a.png then https://i.imgur.com/a.png and https://news.test/story?id=1#top",
		GIF:  nil}
	if _, err := s.Commit("alice", "message.created", msg); err != nil {
		t.Fatal(err)
	}
	got := catalogRowsOf(t, s, "net:1")
	want := []catalogRow{
		{"https://i.imgur.com/a.png", "image", "https://i.imgur.com/a.png", true},
		{"http://plain.test/a.png", "link", "http://plain.test/a.png", false},
		{"https://news.test/story?id=1#top", "link", "https://news.test/story?id=1", false},
	}
	if len(got) != len(want) {
		t.Fatalf("rows: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	// The gallery still shows only the HTTPS image.
	if urls := galleryURLs(t, s, "alice", "net:#pics"); len(urls) != 1 || urls[0] != "https://i.imgur.com/a.png" {
		t.Fatalf("gallery: %v", urls)
	}
	groups, _, err := s.Links("alice", LinkFilter{}, "", 10, 5)
	if err != nil || len(groups) != 3 {
		t.Fatalf("catalog: %+v %v", groups, err)
	}

	// An upload is gallery-only; its link in the text gains no second row.
	upload := model.Message{ID: "net:2", ConversationID: "net:#pics", SenderID: "carol", Kind: "message", CreatedAt: time.Now().UTC(),
		Text: "https://chat.example/files/abc/x/cat.png", Attachment: &model.Attachment{URL: "https://chat.example/files/abc/x/cat.png", Name: "cat.png", ContentType: "image/png"}}
	if _, err = s.Commit("alice", "message.created", upload); err != nil {
		t.Fatal(err)
	}
	if rows := catalogRowsOf(t, s, "net:2"); len(rows) != 1 || rows[0].identity == "" {
		t.Fatalf("upload rows: %+v", rows)
	}
}

// Rows indexed under the gallery-only rules (version 1) gain their catalog
// identity when the backfill runs again under the current version.
func TestBackfillAddsCatalogIdentitiesToOlderRows(t *testing.T) {
	s := galleryStore(t)
	msg := model.Message{ID: "net:1", ConversationID: "net:#pics", SenderID: "carol", Kind: "message", Text: "https://i.imgur.com/a.png https://news.test/", CreatedAt: time.Now().UTC()}
	if err := s.SaveHistory("alice", msg); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("DELETE FROM message_links WHERE identity!='' AND kind='link'; UPDATE message_links SET identity='',host='',previewed=0; INSERT INTO index_jobs(name,version,last_rowid,done,updated_ns) VALUES('message-links',1,1000000,1,0) ON CONFLICT(name) DO UPDATE SET version=1,last_rowid=1000000,done=1"); err != nil {
		t.Fatal(err)
	}
	if groups, _, _ := s.Links("alice", LinkFilter{}, "", 10, 5); len(groups) != 0 {
		t.Fatalf("catalog before re-index: %+v", groups)
	}
	for {
		done, err := s.BackfillLinksStep()
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
	}
	if groups, _, _ := s.Links("alice", LinkFilter{}, "", 10, 5); len(groups) != 2 {
		t.Fatalf("catalog after re-index: %+v", groups)
	}
	if urls := galleryURLs(t, s, "alice", "net:#pics"); len(urls) != 1 {
		t.Fatalf("gallery after re-index: %v", urls)
	}
}
