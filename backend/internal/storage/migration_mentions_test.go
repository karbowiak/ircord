package storage

import (
	"errors"
	"testing"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/notify"
)

// Upgrading drops the mention markers the old rule gave to unread channel
// messages that only name the account (#209), and nothing else.
func TestUpgradeDropsUnreadBareNameMentions(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	account(t, s, "alice")
	account(t, s, "bob")
	settings := notify.Defaults()
	settings.HighlightWords = []string{"deploy"}
	if _, err = s.Commit("alice", "notification-settings.updated", settings); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	for _, owner := range []string{"alice", "bob"} {
		for _, c := range []model.Conversation{{ID: "room", Kind: "channel"}, {ID: "dm", Kind: "direct"}} {
			if _, err = s.Commit(owner, "conversation.updated", c); err != nil {
				t.Fatal(err)
			}
		}
	}
	messages := []model.Message{
		{ID: "read", ConversationID: "room", Text: "alice, from yesterday"},
		{ID: "bare", ConversationID: "room", Text: "Alice is on it"},
		{ID: "reply", ConversationID: "room", Text: "alice: yes"},
		{ID: "at", ConversationID: "room", Text: "hey @alice"},
		{ID: "word", ConversationID: "room", Text: "the deploy, alice"},
		{ID: "direct", ConversationID: "dm", Text: "alice?"},
	}
	for i, msg := range messages {
		msg.SenderID, msg.Kind, msg.Mention, msg.Reactions = "peer", "message", true, []model.Reaction{}
		msg.CreatedAt = at.Add(time.Duration(i) * time.Second)
		for _, owner := range []string{"alice", "bob"} {
			if err = s.SaveHistory(owner, msg); err != nil {
				t.Fatal(err)
			}
			reasons := []string{"mention"}
			if msg.ID == "reply" {
				reasons = append(reasons, "reply")
			}
			if _, err = s.Commit(owner, "inbox.added", model.InboxEntry{MessageID: msg.ID, ConversationID: msg.ConversationID, NetworkID: "net", SenderID: "peer", Reasons: reasons, CreatedAt: msg.CreatedAt}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = s.MarkRead("alice", "room", at); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("DELETE FROM goose_db_version WHERE version_id=25"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s, err = Open(dir); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	marked := func(owner, conversation, id string) bool {
		var n int
		if err := s.DB.QueryRow("SELECT count(*) FROM message_mentions WHERE owner=? AND conversation=? AND message=?", owner, conversation, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}
	for id, want := range map[string]bool{"read": true, "bare": false, "reply": false, "at": true, "word": true} {
		if got := marked("alice", "room", id); got != want {
			t.Errorf("alice's %s marked %v, want %v", id, got, want)
		}
	}
	if !marked("alice", "dm", "direct") {
		t.Error("a direct message lost its mention marker")
	}
	// bob has no highlight words: only the @mention stays among his unread.
	for id, want := range map[string]bool{"read": false, "bare": false, "reply": false, "at": true, "word": false} {
		if got := marked("bob", "room", id); got != want {
			t.Errorf("bob's %s marked %v, want %v", id, got, want)
		}
	}
	if n, err := s.Mentions("alice", "room"); err != nil || n != 2 {
		t.Fatalf("alice's unread mentions %d, want 2 (%v)", n, err)
	}
	if _, err := s.InboxItem("alice", "bare"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a bare name stayed in the inbox: %v", err)
	}
	if row, err := s.InboxItem("alice", "reply"); err != nil || len(row.Entry.Reasons) != 1 || row.Entry.Reasons[0] != "reply" {
		t.Fatalf("reply item: %+v %v", row.Entry.Reasons, err)
	}
	if row, err := s.InboxItem("alice", "read"); err != nil || len(row.Entry.Reasons) != 1 || row.Entry.Reasons[0] != "mention" {
		t.Fatalf("read history changed: %+v %v", row.Entry.Reasons, err)
	}
}
