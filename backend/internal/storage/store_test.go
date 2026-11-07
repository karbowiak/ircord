package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"ircord/backend/internal/model"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func account(t *testing.T, s *Store, id string) {
	t.Helper()
	if _, err := s.CreateAccount(id, "test-key-hash-"+id); err != nil {
		t.Fatal(err)
	}
}
func TestDurableHistoryEventsAndOwnership(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	account(t, s, "alice")
	account(t, s, "bob")
	conv := model.Conversation{ID: "room", NetworkID: "net", Kind: "channel", Name: "test"}
	if _, err = s.Commit("alice", "conversation.updated", conv); err != nil {
		t.Fatal(err)
	}
	same := time.Now().UTC()
	for _, id := range []string{"a", "b", "c"} {
		msg := model.Message{ID: id, ConversationID: "room", Text: id, CreatedAt: same, Reactions: []model.Reaction{}}
		if _, err = s.Commit("alice", "message.created", msg); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.History("alice", "room", "", 2)
	if err != nil || len(p.Items) != 2 || p.NextCursor == nil {
		t.Fatal(p, err)
	}
	older, err := s.History("alice", "room", *p.NextCursor, 2)
	if err != nil || len(older.Items) != 1 || older.Items[0].ID != "a" {
		t.Fatal(older, err)
	}
	other, err := s.History("bob", "room", "", 40)
	if err != nil || len(other.Items) != 0 {
		t.Fatal("history leaked")
	}
	events, valid, err := s.Replay("alice", 1, 100)
	if err != nil || !valid || len(events) != 3 {
		t.Fatal(events, valid, err)
	}
	if _, err = Open(dir); err == nil {
		t.Fatal("second process store lock accepted")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	events, valid, err = s.Replay("alice", 1, 100)
	if err != nil || !valid || len(events) != 3 {
		t.Fatal("restart lost replay")
	}
	if _, err = s.History("alice", "room", "not-a-cursor", 40); err == nil {
		t.Fatal("invalid cursor accepted")
	}
}
func TestEncryptedNetworksAndConcurrentAccountSequences(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	account(t, s, "bob")
	cfg := model.NetworkConfig{ID: "n", SASLUser: "test", SASLPassword: "password-must-not-be-plaintext"}
	if err = s.SaveNetwork("alice", cfg, true); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err = s.DB.QueryRow("SELECT config FROM networks WHERE owner='alice'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(cfg.SASLPassword)) {
		t.Fatal("plaintext network credential")
	}
	if _, err = s.open("bob", "n", raw); err == nil {
		t.Fatal("ciphertext can be copied to another owner")
	}
	nets, err := s.Networks("alice")
	if err != nil || nets[0].Config.SASLPassword != cfg.SASLPassword {
		t.Fatal(nets, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Commit("alice", "preferences.updated", model.Preferences{Theme: "dark"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	events, valid, err := s.Replay("alice", 0, 100)
	if err != nil || !valid || len(events) != 20 {
		t.Fatal(events, valid, err)
	}
	for i, e := range events {
		if e.Seq != uint64(i+1) {
			t.Fatal("out-of-order account commit")
		}
	}
	before, _ := s.Head("alice")
	_, err = s.Commit("missing", "preferences.updated", model.Preferences{})
	if err == nil {
		t.Fatal("non-account event accepted")
	}
	after, _ := s.Head("alice")
	if before != after {
		t.Fatal("failed commit advanced sequence")
	}
	key, err := os.ReadFile(filepath.Join(filepath.Dir(s.lock.Path()), "master.key"))
	if err != nil || len(key) != 32 {
		t.Fatal("missing encryption key")
	}
}
func TestOutboxIdempotencyAndReactionReplayOrdering(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	in := model.SendInput{ClientID: "client", ConversationID: "room", Text: "hello"}
	if _, fresh, err := s.BeginCommand("alice", in); err != nil || !fresh {
		t.Fatal(err)
	}
	if _, fresh, err := s.BeginCommand("alice", in); err != nil || fresh {
		t.Fatal("retry duplicated intent")
	}
	in.Text = "different"
	if _, _, err = s.BeginCommand("alice", in); err == nil {
		t.Fatal("client-id collision accepted")
	}
	msg := model.Message{ID: "message", UpstreamID: "server-id", ClientID: "client", ConversationID: "room", Text: "hello", CreatedAt: time.Now().UTC()}
	if _, err = s.Commit("alice", "message.created", msg); err != nil {
		t.Fatal(err)
	}
	command, err := s.Command("alice", "client")
	if err != nil || command.State != "sent" || command.MessageID != msg.ID {
		t.Fatal(command, err)
	}
	now := time.Now()
	if err = s.Reaction("alice", "room", "parent", "sender", "👍", false, now); err != nil {
		t.Fatal(err)
	}
	if err = s.Reaction("alice", "room", "parent", "sender", "👍", true, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	reactions, err := s.Reactions("alice", "room", "parent")
	if err != nil || len(reactions) != 0 {
		t.Fatal("old history replay resurrected removed reaction")
	}
	raw, _ := json.Marshal(command)
	if bytes.Contains(raw, []byte("fingerprint")) {
		t.Fatal("internal command data leaked")
	}
}

func TestDenseTimestampPaginationWhileOlderBatchesArrive(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	at := time.Now().UTC().Truncate(time.Millisecond)
	for i := 160; i < 260; i++ {
		msg := model.Message{ID: fmt.Sprintf("msg-%03d", i), ConversationID: "room", CreatedAt: at, Text: fmt.Sprint(i)}
		if _, err = s.Commit("alice", "message.created", msg); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 2; page++ {
		p, e := s.History("alice", "room", cursor, 40)
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range p.Items {
			seen[m.ID] = true
		}
		cursor = *p.NextCursor
	}
	base, err := s.HistoryBase("alice", "room")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 160; i++ {
		msg := model.Message{ID: fmt.Sprintf("msg-%03d", i), ConversationID: "room", CreatedAt: at, Text: fmt.Sprint(i), Position: base + int64(i) + 1}
		if _, err = s.HistoryInsert("alice", msg); err != nil {
			t.Fatal(err)
		}
	}
	for page := 0; page < 10; page++ {
		p, e := s.History("alice", "room", cursor, 40)
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range p.Items {
			if seen[m.ID] {
				t.Fatal("duplicate across dense cursor pages")
			}
			seen[m.ID] = true
		}
		if p.NextCursor == nil {
			break
		}
		cursor = *p.NextCursor
	}
	if len(seen) != 260 {
		t.Fatalf("dense timestamps lost %d messages", 260-len(seen))
	}
	oldest, e := s.Oldest("alice", "room")
	if e != nil || oldest.ID != "msg-000" {
		t.Fatal("upstream paging boundary lost server order", oldest.ID, e)
	}
}

func TestLargeChannelDirectoryCacheIsolationAndConnectionFence(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	account(t, store, "alice")
	account(t, store, "bob")
	items := make([]model.ChannelListing, 25000)
	for i := range items {
		items[i] = model.ChannelListing{Name: fmt.Sprintf("#channel%05d", i), Users: i, Topic: "Danish ØL"}
	}
	started := time.Now()
	if err = store.SaveChannels("alice", "net", items, started); err != nil {
		t.Fatal(err)
	}
	page, valid, err := store.CachedChannels("alice", "net", "channel24999", 50, started.Add(-time.Second))
	if err != nil || !valid || len(page) != 1 || page[0].Users != 24999 {
		t.Fatal("large directory was truncated", err)
	}
	page, valid, err = store.CachedChannels("alice", "net", "øl", 50, started.Add(-time.Second))
	if err != nil || !valid || len(page) != 50 || page[0].Users != 24999 {
		t.Fatal("directory search/ranking lost Unicode", err)
	}
	if _, valid, err = store.CachedChannels("bob", "net", "", 50, time.Time{}); err != nil || valid {
		t.Fatal("channel directory leaked")
	}
	if _, valid, err = store.CachedChannels("alice", "net", "", 50, started.Add(time.Second)); err != nil || valid {
		t.Fatal("a replacement connection accepted an old directory")
	}
	if err = store.DeleteAccount("alice"); err != nil {
		t.Fatal(err)
	}
	var n int
	store.DB.QueryRow("SELECT count(*) FROM channel_cache WHERE owner='alice'").Scan(&n)
	if n != 0 {
		t.Fatal("account deletion retained directory rows")
	}
}
func TestRedactionClearsDerivedRowsAndFindsQuotingReplies(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	at := time.Now().UTC()
	parent := model.Message{ID: "n:p", UpstreamID: "p", ConversationID: "room", SenderID: "bob", Kind: "message", Text: "hi alice", CreatedAt: at, Mention: true}
	reply := model.Message{ID: "n:r", UpstreamID: "r", ConversationID: "room", SenderID: "carol", Kind: "message", Text: "re", CreatedAt: at.Add(time.Second), ReplyTo: &model.Reply{ID: parent.ID, Nick: "bob", Text: parent.Text}}
	foreign := reply
	foreign.ID, foreign.UpstreamID, foreign.ConversationID = "n:f", "f", "elsewhere"
	for _, m := range []model.Message{parent, reply, foreign} {
		if _, err = s.Commit("alice", "message.created", m); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.SetReaction("alice", "room", "p", "carol", "👍", true, at, false); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.Mentions("alice", "room"); n != 1 {
		t.Fatalf("mention setup: %d", n)
	}
	replies, err := s.Replies("alice", "room", parent.ID)
	if err != nil || len(replies) != 1 || replies[0].ID != reply.ID {
		t.Fatalf("replies %+v %v", replies, err)
	}
	if err = s.ClearRedacted("alice", "room", parent.ID, parent.UpstreamID); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Reactions("alice", "room", "p"); len(r) != 0 {
		t.Fatalf("reactions survived: %+v", r)
	}
	if n, _ := s.Mentions("alice", "room"); n != 0 {
		t.Fatalf("mention survived: %d", n)
	}
}
