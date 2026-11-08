package irc

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func redactFixture(t *testing.T, caps map[string]bool) (*Manager, *connection, Conversation, Conversation) {
	t.Helper()
	m := New(Options{})
	t.Cleanup(m.Close)
	w := m.space("alice")
	w.members["peer"] = Member{ID: "peer", Nick: "Peer"}
	c := &connection{owner: w, manager: m, config: NetworkConfig{ID: "network", Nick: "Alice"}, ctx: m.ctx, ready: true, caps: caps, nicks: map[string]string{"peer": "peer"}, historyBatches: map[string]bool{}, batchLabels: map[string]string{}, queue: make(chan []*girc.Event, 8)}
	w.connections["network"] = c
	conv := c.conversation("#test")
	conv.Joined = true
	w.conversations[conv.ID] = conv
	other := c.conversation("#other")
	w.messages[conv.ID] = []Message{
		{ID: "network:m1", UpstreamID: "m1", ConversationID: conv.ID, SenderID: "alice", Kind: "message", Text: "https://files.example/a.png", Reactions: []Reaction{{Emoji: "👍", MemberIDs: []string{"peer"}}}, Attachment: &model.Attachment{Name: "a.png", URL: "https://files.example/a.png"}, Preview: &model.LinkPreview{URL: "https://files.example/a.png", Title: "a"}},
		{ID: "network:m2", UpstreamID: "m2", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "peer text", Reactions: []Reaction{}},
		{ID: "local", ConversationID: conv.ID, SenderID: "alice", Kind: "message", Text: "unconfirmed", Reactions: []Reaction{}},
		{ID: "network:m3", UpstreamID: "m3", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "reply", Reactions: []Reaction{}, ReplyTo: &Reply{ID: "network:m1", Nick: "Alice", Text: "https://files.example/a.png"}},
	}
	return m, c, conv, other
}
func stored(c *connection, conv, id string) Message {
	c.owner.mu.Lock()
	defer c.owner.mu.Unlock()
	msg, _ := c.lookupMessage(conv, id)
	return msg
}

func TestRedactRejectsForeignTargetsAndUnsupportedNetworks(t *testing.T) {
	m, c, conv, other := redactFixture(t, map[string]bool{"message-tags": true})
	for _, tc := range []struct {
		owner, conversation, message string
		want                         error
	}{
		{"alice", conv.ID, "network:m2", ErrForbidden},     // someone else's message
		{"alice", other.ID, "network:m1", ErrNotFound},     // right message, wrong conversation
		{"bob", conv.ID, "network:m1", ErrNotFound},        // another account
		{"alice", conv.ID, "network:missing", ErrNotFound}, // unknown message
		{"alice", conv.ID, "network:m1", ErrUnsupported},   // cap not negotiated
	} {
		if _, _, err := m.Redact(tc.owner, tc.conversation, tc.message); !errors.Is(err, tc.want) {
			t.Fatalf("%+v: got %v", tc, err)
		}
	}
	c.caps["draft/message-redaction"] = true
	if _, _, err := m.Redact("alice", conv.ID, "local"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("message without a network id: %v", err)
	}
	if len(c.queue) != 0 {
		t.Fatal("a rejected deletion reached the network")
	}
	if got := stored(c, conv.ID, "network:m2"); got.Deleted {
		t.Fatal("foreign message changed")
	}
}

func TestRedactAppliesOwnDeletionAndUpdatesReplies(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{"message-tags": true, "draft/message-redaction": true})
	msg, attachment, err := m.Redact("alice", conv.ID, "network:m1")
	if err != nil {
		t.Fatal(err)
	}
	if !msg.Deleted || msg.Text != "Message deleted" || msg.Attachment != nil || msg.Preview != nil || len(msg.Reactions) != 0 {
		t.Fatalf("not cleared: %+v", msg)
	}
	if attachment == nil || attachment.URL != "https://files.example/a.png" {
		t.Fatal("the retained attachment was not reported")
	}
	sent := <-c.queue
	if len(sent) != 1 || sent[0].Command != "REDACT" || sent[0].Params[0] != "#test" || sent[0].Params[1] != "m1" {
		t.Fatalf("wire: %+v", sent)
	}
	if got := stored(c, conv.ID, "network:m3"); got.ReplyTo == nil || got.ReplyTo.Text != "Message deleted" {
		t.Fatalf("reply still quotes the deleted text: %+v", got.ReplyTo)
	}
	if _, _, err = m.Redact("alice", conv.ID, "network:m1"); !errors.Is(err, ErrAlreadyDeleted) {
		t.Fatalf("second deletion: %v", err)
	}
}

func TestRedactWaitsForLabeledOutcome(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{"message-tags": true, "draft/message-redaction": true, "labeled-response": true})
	run := func() (chan error, string) {
		done := make(chan error, 1)
		go func() { _, _, err := m.Redact("alice", conv.ID, "network:m1"); done <- err }()
		sent := <-c.queue
		return done, sent[0].Tags["label"]
	}
	done, label := run()
	if !strings.HasPrefix(label, "redact:") {
		t.Fatalf("label %q", label)
	}
	c.redactionResult(label, "You are not allowed to redact that message")
	if err := <-done; err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("refusal: %v", err)
	}
	if stored(c, conv.ID, "network:m1").Deleted {
		t.Fatal("a refused deletion changed history")
	}
	done, label = run()
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "Alice", Ident: "a", Host: "h"}, Params: []string{"#test", "m1"}, Tags: girc.Tags{"label": label}})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !stored(c, conv.ID, "network:m1").Deleted {
		t.Fatal("confirmed deletion not applied")
	}
	old := RedactTimeout
	RedactTimeout = 20 * time.Millisecond
	defer func() { RedactTimeout = old }()
	c.owner.messages[conv.ID][1].SenderID = "alice"
	if _, _, err := m.Redact("alice", conv.ID, "network:m2"); !errors.Is(err, ErrRedactUnconfirmed) {
		t.Fatalf("timeout: %v", err)
	}
	if stored(c, conv.ID, "network:m2").Deleted {
		t.Fatal("an unconfirmed deletion changed history")
	}
}

func TestIncomingRedactRequiresAuthorOperatorOrServer(t *testing.T) {
	_, c, conv, _ := redactFixture(t, map[string]bool{"message-tags": true, "draft/message-redaction": true})
	redact := func(nick, ident, msgid string) {
		c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: nick, Ident: ident, Host: map[bool]string{true: "h", false: ""}[ident != ""]}, Params: []string{"#test", msgid}})
	}
	redact("Mallory", "m", "m1")
	if stored(c, conv.ID, "network:m1").Deleted {
		t.Fatal("a non-author erased someone else's message")
	}
	redact("Peer", "p", "m2")
	if !stored(c, conv.ID, "network:m2").Deleted {
		t.Fatal("the author's own REDACT was ignored")
	}
	c.owner.mu.Lock()
	op := c.member("Oper", "")
	v := c.owner.conversations[conv.ID]
	v.MemberModes = map[string]string{op.ID: "@"}
	c.owner.conversations[conv.ID] = v
	c.owner.mu.Unlock()
	redact("Oper", "o", "m3")
	if !stored(c, conv.ID, "network:m3").Deleted {
		t.Fatal("a channel operator's REDACT was ignored")
	}
	redact("irc.example.test", "", "m1")
	if !stored(c, conv.ID, "network:m1").Deleted {
		t.Fatal("a server REDACT was ignored")
	}
}

func storedRedactFixture(t *testing.T, caps map[string]bool) (*Manager, *storage.Store, *connection, Conversation) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateAccount("alice", "alice-key-hash"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store})
	t.Cleanup(func() { m.Close(); store.Close() })
	w := m.space("alice")
	w.mu.Lock()
	defer w.mu.Unlock()
	w.members["peer"] = Member{ID: "peer", Nick: "Peer"}
	c := &connection{owner: w, manager: m, config: NetworkConfig{ID: "network", Nick: "Alice"}, ctx: m.ctx, ready: true, caps: caps, nicks: map[string]string{"peer": "peer"}, historyBatches: map[string]bool{}, batchLabels: map[string]string{}, queue: make(chan []*girc.Event, 8)}
	w.connections["network"] = c
	conv := c.conversation("#test")
	conv.Joined = true
	w.conversations[conv.ID] = conv
	at := time.Now().UTC()
	for _, msg := range []Message{
		{ID: "network:m1", UpstreamID: "m1", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "secret for Alice", CreatedAt: at, Mention: true, Reactions: []Reaction{}},
		{ID: "network:m2", UpstreamID: "m2", ClientID: "client-2", ConversationID: conv.ID, SenderID: "alice", Kind: "message", Text: "my secret", CreatedAt: at.Add(time.Second), Reactions: []Reaction{}},
	} {
		w.emit("message.created", msg)
		w.messages[conv.ID] = append(w.messages[conv.ID], msg)
	}
	conv = w.conversations[conv.ID]
	conv.MentionCount = 1
	w.conversations[conv.ID] = conv
	w.trace["network"] = []ServerLine{
		{ID: "t1", Command: "PRIVMSG", Params: []string{"#test", "secret for Alice"}, Text: "secret for Alice", Tags: map[string]string{"msgid": "m1"}},
		{ID: "t2", Command: "PRIVMSG", Params: []string{"#test", "unrelated"}, Text: "unrelated", Tags: map[string]string{"msgid": "m9"}},
	}
	if w.storageErr != nil {
		t.Fatal(w.storageErr)
	}
	return m, store, c, conv
}

func TestIncomingRedactHasNoSideEffectsForUnknownTargetsOrSenders(t *testing.T) {
	_, c, conv, _ := redactFixture(t, map[string]bool{"draft/message-redaction": true})
	w := c.owner
	conversations, members := len(w.conversations), len(w.members)
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "Stranger", Ident: "s", Host: "h"}, Params: []string{"Alice", "m1"}})
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "Stranger", Ident: "s", Host: "h"}, Params: []string{"#elsewhere", "m1"}})
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "Stranger", Ident: "s", Host: "h"}, Params: []string{"#test", "m1"}})
	if len(w.conversations) != conversations || len(w.members) != members {
		t.Fatalf("unsolicited REDACT created state: %d->%d conversations, %d->%d members", conversations, len(w.conversations), members, len(w.members))
	}
	if stored(c, conv.ID, "network:m1").Deleted {
		t.Fatal("an unknown sender erased a message")
	}
}

func TestRedactionScrubsCopiesAndRecountsMentions(t *testing.T) {
	m, store, c, conv := storedRedactFixture(t, map[string]bool{"draft/message-redaction": true})
	head, _ := store.Head("alice")
	// A REDACT inside a chathistory batch is persisted without a live event.
	c.historyBatches["hist"] = true
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "Peer", Ident: "p", Host: "h"}, Params: []string{"#test", "m1"}, Tags: girc.Tags{"batch": "hist"}})
	saved, err := store.Message("alice", conv.ID, "m1")
	if err != nil || !saved.Deleted || saved.Text != "Message deleted" {
		t.Fatalf("historical redaction not persisted: %+v %v", saved, err)
	}
	events, _, err := store.Replay("alice", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.Seq > head && e.Type == "message.updated" {
			t.Fatal("a historical redaction emitted a live message.updated")
		}
		if strings.Contains(string(e.Data), "secret for Alice") {
			t.Fatalf("replay log still carries the deleted text: %s", e.Data)
		}
	}
	if n, _ := store.Mentions("alice", conv.ID); n != 0 || c.owner.conversations[conv.ID].MentionCount != 0 {
		t.Fatalf("mention badge kept counting a deleted message: store=%d live=%d err=%v", n, c.owner.conversations[conv.ID].MentionCount, c.owner.storageErr)
	}
	if counts := countsAfter(t, m, "alice", head); counts.ID != conv.ID || counts.MentionCount != 0 {
		t.Fatalf("redaction recount sent %+v", counts)
	}
	if trace := c.owner.trace["network"]; trace[0].Text != "Message deleted" || trace[0].Params[1] != "Message deleted" || trace[1].Text != "unrelated" {
		t.Fatalf("raw trace: %+v", trace)
	}
}

func TestRedactReportsStorageFailure(t *testing.T) {
	m, store, _, conv := storedRedactFixture(t, map[string]bool{"draft/message-redaction": true})
	store.Close()
	if _, _, err := m.Redact("alice", conv.ID, "network:m2"); !errors.Is(err, ErrStorage) {
		t.Fatalf("a deletion IRCord could not record reported %v", err)
	}
}
