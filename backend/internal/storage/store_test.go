package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"ircord/backend/internal/model"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	if _, _, err = s.BeginCommand("alice", in); !errors.Is(err, ErrCommandIdentityConflict) {
		t.Fatal("client-id collision accepted")
	}
	changedConversation := model.SendInput{ClientID: "client", ConversationID: "another-room", Text: "hello"}
	if _, _, err = s.BeginCommand("alice", changedConversation); !errors.Is(err, ErrCommandIdentityConflict) {
		t.Fatal("client-id reuse in another conversation was accepted")
	}
	account(t, s, "bob")
	if _, fresh, err := s.BeginCommand("bob", in); err != nil || !fresh {
		t.Fatal("another account did not get an independent clientId namespace", err)
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

func TestOutboxIdentityRetriesAcrossStatesAndRestart(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	account(t, s, "alice")
	in := model.SendInput{ClientID: "stable", ConversationID: "room", Text: "same"}
	if _, fresh, err := s.BeginCommand("alice", in); err != nil || !fresh {
		t.Fatal("initial command was not reserved", err)
	}
	for _, state := range []string{"pending", "sent", "unconfirmed", "failed", "uncertain"} {
		if _, err := s.DB.Exec("UPDATE outbox SET state=? WHERE owner=? AND client_id=?", state, "alice", in.ClientID); err != nil {
			t.Fatal(err)
		}
		if _, fresh, err := s.BeginCommand("alice", in); err != nil || fresh {
			t.Fatalf("same payload did not deduplicate in %s state: fresh=%v err=%v", state, fresh, err)
		}
		changed := in
		changed.Text = "changed"
		if _, _, err := s.BeginCommand("alice", changed); !errors.Is(err, ErrCommandIdentityConflict) {
			t.Fatalf("changed payload was not rejected in %s state: %v", state, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, fresh, err := s.BeginCommand("alice", in); err != nil || fresh {
		t.Fatalf("same payload did not deduplicate after restart: fresh=%v err=%v", fresh, err)
	}
	changed := in
	changed.ConversationID = "other"
	if _, _, err := s.BeginCommand("alice", changed); !errors.Is(err, ErrCommandIdentityConflict) {
		t.Fatalf("conversation change after restart was not rejected: %v", err)
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
	if err = store.DeleteAccount("alice", false); err != nil {
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

// Servers number batches per session, so a reconnect reuses refs. Deleting a
// message must scrub only the batch instance that carried it, in both the
// server window and the replay log.
func TestScrubRedactedKeepsOtherBatchesWithTheSameRef(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	open := func(id string, tags map[string]string) model.ServerLine {
		return model.ServerLine{ID: id, NetworkID: "n", Command: "BATCH", Params: []string{"+1", "draft/multiline", "#room"}, Tags: tags}
	}
	member := func(id, text string, tags map[string]string) model.ServerLine {
		if tags == nil {
			tags = map[string]string{}
		}
		tags["batch"] = "1"
		return model.ServerLine{ID: id, NetworkID: "n", Command: "PRIVMSG", Params: []string{"#room", text}, Text: text, Tags: tags}
	}
	closing := func(id string) model.ServerLine {
		return model.ServerLine{ID: id, NetworkID: "n", Command: "BATCH", Params: []string{"-1"}}
	}
	attachment := map[string]string{"+ircord/attachment": "file"}
	lines := []model.ServerLine{
		// An earlier session's message in batch 1.
		open("before-open", map[string]string{"msgid": "up-before", "+ircord/attachment": "file"}),
		member("before-1", "keep before", nil),
		closing("before-close"),
		// The deleted message, received in batch 1 of a later session.
		open("in-open", map[string]string{"msgid": "up", "+ircord/attachment": "file"}),
		member("in-1", "secret one", nil),
		member("in-2", "secret two", map[string]string{"+ircord/attachment": "file"}),
		closing("in-close"),
		// The author's outgoing copy, where only a member names the message.
		open("out-open", attachment),
		member("out-1", "secret one", map[string]string{"+ircord/client-id": "client-1"}),
		closing("out-close"),
		// A later session's message in batch 1.
		open("after-open", map[string]string{"msgid": "up-after", "+ircord/attachment": "file"}),
		member("after-1", "keep after", map[string]string{"+ircord/attachment": "file"}),
		closing("after-close"),
	}
	for _, line := range lines {
		if _, err = s.Commit("alice", "server.line", line); err != nil {
			t.Fatal(err)
		}
	}
	deleted := model.Message{ID: "n:up", UpstreamID: "up", ConversationID: "room", Kind: "message", Text: "Message deleted", Deleted: true}
	if err = s.ScrubRedacted("alice", "n", deleted, "client-1"); err != nil {
		t.Fatal(err)
	}
	check := func(where string, got map[string]model.ServerLine) {
		t.Helper()
		if len(got) != len(lines) {
			t.Fatalf("%s has %d lines, want %d", where, len(got), len(lines))
		}
		for _, id := range []string{"in-open", "in-1", "in-2", "out-open", "out-1"} {
			if line := got[id]; strings.Contains(line.Text, "secret") || line.Tags["+ircord/attachment"] != "" {
				t.Errorf("%s kept deleted content on %s: %+v", where, id, line)
			}
		}
		for _, id := range []string{"before-open", "before-1", "after-open", "after-1"} {
			line := got[id]
			if line.Text == "Message deleted" || line.Tags["+ircord/attachment"] == "" && id != "before-1" {
				t.Errorf("%s scrubbed unrelated line %s: %+v", where, id, line)
			}
		}
	}
	events, _, err := s.Replay("alice", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	replay := map[string]model.ServerLine{}
	for _, e := range events {
		if e.Type == "server.line" {
			var line model.ServerLine
			if err = json.Unmarshal(e.Data, &line); err != nil {
				t.Fatal(err)
			}
			replay[line.ID] = line
		}
	}
	check("replay log", replay)
	page, err := s.ServerLog("alice", "n", "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]model.ServerLine{}
	for _, line := range page.Items {
		byID[line.ID] = line
	}
	check("server window", byID)
}

// A session can drop mid-batch, so a batch may never close. The next session's
// batch with the same ref is a different message, whatever its batch type.
func TestScrubServerLinesEndsUnclosedBatchAtTheNextOpening(t *testing.T) {
	for _, next := range [][]string{{"+1", "draft/multiline", "#room"}, {"+1", "chathistory", "#room"}} {
		t.Run(next[1], func(t *testing.T) {
			lines := []model.ServerLine{
				{ID: "cut-open", Command: "BATCH", Params: []string{"+1", "draft/multiline", "#room"}, Tags: map[string]string{"msgid": "up"}},
				{ID: "cut-1", Command: "PRIVMSG", Params: []string{"#room", "secret"}, Text: "secret", Tags: map[string]string{"batch": "1"}},
				{ID: "next-open", Command: "BATCH", Params: next, Tags: map[string]string{"msgid": "up-next"}},
				{ID: "next-1", Command: "PRIVMSG", Params: []string{"#room", "keep"}, Text: "keep", Tags: map[string]string{"batch": "1", "msgid": "other"}},
				{ID: "next-close", Command: "BATCH", Params: []string{"-1"}},
			}
			if got := ScrubServerLines(lines, "up", "", "Message deleted"); !slices.Equal(got, []int{1}) {
				t.Fatalf("changed %v, want [1]", got)
			}
			if lines[3].Text != "keep" {
				t.Fatalf("scrubbed the next session's batch: %+v", lines[3])
			}
		})
	}
}

func TestScrubRedactedRewritesReplayAndServerWindowCopies(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	msg := model.Message{ID: "n:up", UpstreamID: "up", ClientID: "client-1", ConversationID: "room", SenderID: "alice", Kind: "message", Text: "line one\nline two", CreatedAt: time.Now().UTC()}
	if _, err = s.Commit("alice", "message.created", msg); err != nil {
		t.Fatal(err)
	}
	// Outgoing multiline send as the server window records it, plus an unrelated line.
	for _, line := range []model.ServerLine{
		{ID: "b1", NetworkID: "n", Kind: "outgoing", Command: "BATCH", Params: []string{"+ref", "draft/multiline", "#room"}, Tags: map[string]string{"+ircord/client-id": "client-1"}},
		{ID: "l1", NetworkID: "n", Kind: "outgoing", Command: "PRIVMSG", Params: []string{"#room", "line one"}, Text: "line one", Tags: map[string]string{"batch": "ref"}},
		{ID: "l2", NetworkID: "n", Kind: "outgoing", Command: "PRIVMSG", Params: []string{"#room", "line two"}, Text: "line two", Tags: map[string]string{"batch": "ref"}},
		{ID: "x1", NetworkID: "n", Kind: "outgoing", Command: "PRIVMSG", Params: []string{"#room", "keep me"}, Text: "keep me", Tags: map[string]string{"+ircord/client-id": "other"}},
	} {
		if _, err = s.Commit("alice", "server.line", line); err != nil {
			t.Fatal(err)
		}
	}
	deleted := msg
	deleted.Text, deleted.Deleted = "Message deleted", true
	if err = s.ScrubRedacted("alice", "n", deleted, "client-1"); err != nil {
		t.Fatal(err)
	}
	events, _, err := s.Replay("alice", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	kept := false
	for _, e := range events {
		if strings.Contains(string(e.Data), "line one") || strings.Contains(string(e.Data), "line two") {
			t.Fatalf("replay still carries deleted text: %s %s", e.Type, e.Data)
		}
		kept = kept || strings.Contains(string(e.Data), "keep me")
	}
	if !kept {
		t.Fatal("scrubbed an unrelated line")
	}
	rows, err := s.DB.Query("SELECT payload FROM server_lines WHERE owner='alice'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		_ = rows.Scan(&raw)
		if strings.Contains(string(raw), "line one") || strings.Contains(string(raw), "line two") {
			t.Fatalf("server window still carries deleted text: %s", raw)
		}
	}
}

// Removing a network also removes the history of its conversations that were
// closed earlier, whose conversation entities are already gone.
func TestNetworkRemovalDeletesClosedConversationHistory(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	hex := strings.Repeat("ab", 12)
	closed := "one_" + hex
	// Another network whose ID starts with the removed one keeps its history.
	similar := "one_two_" + hex
	for i, conversation := range []string{closed, similar} {
		if _, err = s.DB.Exec("INSERT INTO messages(owner,conversation,id,upstream,created_ns,text,sender,kind,payload) VALUES('alice',?,?,'',?,'text','bob','message',x'7b7d')", conversation, fmt.Sprintf("m%d", i), i); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.Commit("alice", "network.removed", model.Removal{ID: "one"}); err != nil {
		t.Fatal(err)
	}
	var left []string
	rows, err := s.DB.Query("SELECT conversation FROM messages WHERE owner='alice'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var conversation string
		if err = rows.Scan(&conversation); err != nil {
			t.Fatal(err)
		}
		left = append(left, conversation)
	}
	if len(left) != 1 || left[0] != similar {
		t.Fatalf("messages left after removing the network: %v", left)
	}
}
