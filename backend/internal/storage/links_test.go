package storage

import (
	"fmt"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

func galleryStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	account(t, s, "alice")
	account(t, s, "bob")
	for _, owner := range []string{"alice", "bob"} {
		for _, c := range []model.Conversation{{ID: "net:#pics", NetworkID: "net", Kind: "channel", Name: "pics"}, {ID: "other:#pics", NetworkID: "other", Kind: "channel", Name: "pics"}} {
			if _, err = s.Commit(owner, "conversation.updated", c); err != nil {
				t.Fatal(err)
			}
		}
	}
	return s
}

func galleryURLs(t *testing.T, s *Store, owner, conv string) []string {
	t.Helper()
	rows, _, err := s.Gallery(owner, conv, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, r := range rows {
		out = append(out, r.URL)
	}
	return out
}

func TestGalleryFollowsStoredMessages(t *testing.T) {
	s := galleryStore(t)
	at := time.Now().UTC()
	msg := model.Message{ID: "net:1", UpstreamID: "1", ConversationID: "net:#pics", SenderID: "carol", Kind: "message", Text: "https://i.imgur.com/a.png https://i.imgur.com/b.gif", CreatedAt: at}
	if _, err := s.Commit("alice", "message.created", msg); err != nil {
		t.Fatal(err)
	}
	// Replay, catch-up and a second account's copy of the same post never multiply rows.
	if err := s.SaveHistory("alice", msg); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit("alice", "message.updated", msg); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveHistory("bob", msg); err != nil {
		t.Fatal(err)
	}
	rows, _, err := s.Gallery("alice", "net:#pics", "", 10)
	if err != nil || len(rows) != 2 || rows[0].URL != "https://i.imgur.com/a.png" || rows[1].Kind != "gif" || rows[0].NetworkID != "net" || rows[0].SenderID != "carol" || rows[0].Text != msg.Text {
		t.Fatalf("%+v %v", rows, err)
	}
	if n, _ := s.GalleryCount("alice", "net:#pics"); n != 2 {
		t.Fatalf("count %d", n)
	}
	// The same-named channel on another network and other accounts stay separate.
	if got := galleryURLs(t, s, "alice", "other:#pics"); len(got) != 0 {
		t.Fatalf("leaked into another network: %v", got)
	}
	if n, _ := s.GalleryCount("bob", "net:#pics"); n != 2 {
		t.Fatalf("bob's own copy: %d", n)
	}
	// An edit replaces the message's images.
	edited := msg
	edited.Text = "only https://i.imgur.com/c.jpg now"
	if _, err = s.Commit("alice", "message.updated", edited); err != nil {
		t.Fatal(err)
	}
	if got := galleryURLs(t, s, "alice", "net:#pics"); len(got) != 1 || got[0] != "https://i.imgur.com/c.jpg" {
		t.Fatalf("after edit: %v", got)
	}
	// A deletion removes them, and only for the account whose copy was deleted.
	deleted := edited
	deleted.Text, deleted.Deleted = "Message deleted", true
	if _, err = s.Commit("alice", "message.updated", deleted); err != nil {
		t.Fatal(err)
	}
	if got := galleryURLs(t, s, "alice", "net:#pics"); len(got) != 0 {
		t.Fatalf("after delete: %v", got)
	}
	if n, _ := s.GalleryCount("bob", "net:#pics"); n != 2 {
		t.Fatalf("bob lost his copy: %d", n)
	}
	if err = s.ClearRedacted("bob", "net:#pics", msg.ID, msg.UpstreamID); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.GalleryCount("bob", "net:#pics"); n != 0 {
		t.Fatalf("redaction kept rows: %d", n)
	}
}

func TestGalleryHidesClosedConversationsIgnoredSendersAndRemovedNetworks(t *testing.T) {
	s := galleryStore(t)
	at := time.Now().UTC()
	for i, sender := range []string{"carol", "mallory"} {
		msg := model.Message{ID: fmt.Sprintf("net:%d", i), ConversationID: "net:#pics", SenderID: sender, Kind: "message", Text: "https://i.imgur.com/" + sender + ".png", CreatedAt: at.Add(time.Duration(i) * time.Second)}
		if _, err := s.Commit("alice", "message.created", msg); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Commit("alice", "member.updated", model.Member{ID: "mallory", Ignored: true}); err != nil {
		t.Fatal(err)
	}
	if got := galleryURLs(t, s, "alice", "net:#pics"); len(got) != 1 || got[0] != "https://i.imgur.com/carol.png" {
		t.Fatalf("ignored sender shown: %v", got)
	}
	if _, err := s.Commit("alice", "conversation.removed", model.Removal{ID: "net:#pics"}); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.GalleryCount("alice", "net:#pics"); n != 0 {
		t.Fatalf("closed conversation shown: %d", n)
	}
	if _, err := s.Commit("alice", "conversation.updated", model.Conversation{ID: "net:#pics", NetworkID: "net", Kind: "channel", Name: "pics"}); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.GalleryCount("alice", "net:#pics"); n != 1 {
		t.Fatalf("rejoined conversation: %d", n)
	}
	if _, err := s.Commit("alice", "network.removed", model.Removal{ID: "net"}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.DB.QueryRow("SELECT count(*) FROM message_links WHERE owner='alice'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("network removal kept %d rows: %v", n, err)
	}
}

func TestGalleryPagesWithoutGapsOrRepeats(t *testing.T) {
	s := galleryStore(t)
	at := time.Now().UTC()
	// Several posts share a timestamp and carry several images each.
	for i := range 7 {
		msg := model.Message{ID: fmt.Sprintf("net:%d", i), ConversationID: "net:#pics", SenderID: "carol", Kind: "message", Text: fmt.Sprintf("https://i.imgur.com/%d-a.png https://i.imgur.com/%d-b.png", i, i), CreatedAt: at.Add(time.Duration(i/3) * time.Second)}
		if _, err := s.Commit("alice", "message.created", msg); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		rows, more, err := s.Gallery("alice", "net:#pics", cursor, 3)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if seen[r.URL] {
				t.Fatalf("repeated %s", r.URL)
			}
			seen[r.URL] = true
		}
		pages++
		if !more {
			break
		}
		cursor = GalleryCursor(rows[len(rows)-1])
	}
	if len(seen) != 14 || pages != 5 {
		t.Fatalf("saw %d in %d pages", len(seen), pages)
	}
	if _, _, err := s.Gallery("alice", "net:#pics", "not-a-cursor", 3); err != ErrInvalidCursor {
		t.Fatalf("bad cursor: %v", err)
	}
}

func TestBackfillIndexesStoredHistoryAndResumes(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	account(t, s, "alice")
	if _, err = s.Commit("alice", "conversation.updated", model.Conversation{ID: "net:#pics", NetworkID: "net", Kind: "channel", Name: "pics"}); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	for i := range 5 {
		msg := model.Message{ID: fmt.Sprintf("net:%d", i), ConversationID: "net:#pics", SenderID: "carol", Kind: "message", Text: fmt.Sprintf("https://i.imgur.com/%d.png", i), CreatedAt: at.Add(time.Duration(i) * time.Second)}
		if err = s.SaveHistory("alice", msg); err != nil {
			t.Fatal(err)
		}
	}
	// History stored before the index existed has no rows.
	if _, err = s.DB.Exec("DELETE FROM message_links"); err != nil {
		t.Fatal(err)
	}
	if complete, _ := s.LinkIndexComplete(); complete {
		t.Fatal("complete before backfill")
	}
	old := LinkBackfillBatch
	LinkBackfillBatch = 2
	defer func() { LinkBackfillBatch = old }()
	if done, err := s.BackfillLinksStep(); err != nil || done {
		t.Fatal(done, err)
	}
	if n, _ := s.GalleryCount("alice", "net:#pics"); n != 2 {
		t.Fatalf("first batch: %d", n)
	}
	// A restart resumes at the checkpoint.
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if s, err = Open(dir); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for range 5 {
		if done, err := s.BackfillLinksStep(); err != nil {
			t.Fatal(err)
		} else if done {
			break
		}
	}
	if n, _ := s.GalleryCount("alice", "net:#pics"); n != 5 {
		t.Fatalf("after backfill: %d", n)
	}
	if complete, err := s.LinkIndexComplete(); err != nil || !complete {
		t.Fatal(complete, err)
	}
	// Repeating a finished backfill changes nothing.
	if done, err := s.BackfillLinksStep(); err != nil || !done {
		t.Fatal(done, err)
	}
	var rows int
	if err = s.DB.QueryRow("SELECT count(*) FROM message_links").Scan(&rows); err != nil || rows != 5 {
		t.Fatal(rows, err)
	}
}
