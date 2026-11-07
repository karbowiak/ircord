package irc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

// registerErgoAccount creates a NickServ account; Ergo only lets logged-in
// accounts redact their own messages.
func registerErgoAccount(t *testing.T, host string, port int, nick, password string) {
	t.Helper()
	registered := make(chan struct{})
	var once sync.Once
	peer := girc.New(girc.Config{Server: host, Port: port, Nick: nick, User: "test", Name: "test", AllowFlood: true})
	peer.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		c.Send(&girc.Event{Command: "PRIVMSG", Params: []string{"NickServ", "REGISTER " + password}, Sensitive: true})
	})
	peer.Handlers.Add(girc.NOTICE, func(_ *girc.Client, e girc.Event) {
		if strings.Contains(e.Last(), "Account created") {
			once.Do(func() { close(registered) })
		}
	})
	done := make(chan struct{})
	go func() { defer close(done); _ = peer.Connect() }()
	defer func() { peer.Close(); <-done }()
	select {
	case <-registered:
	case <-time.After(10 * time.Second):
		t.Fatal("NickServ test-account registration failed")
	}
}

func TestErgoRedactionAcrossClientsAndHistory(t *testing.T) {
	host, port := ergo(t)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner+"-key-hash"); err != nil {
			t.Fatal(err)
		}
	}
	suffix := id()[:8]
	room := "#redact" + suffix
	alice, bob := "ra"+suffix, "rb"+suffix
	password := id()
	registerErgoAccount(t, host, port, alice, password)
	m := New(Options{Store: store, AllowPrivate: true, ReconnectMin: time.Hour})
	t.Cleanup(func() { m.Close(); store.Close() })
	for _, v := range []NetworkConfig{
		{ID: "one", Name: "one", Host: host, Port: port, Nick: alice, SASLUser: alice, SASLPassword: password},
		{ID: "one", Name: "one", Host: host, Port: port, Nick: bob},
	} {
		owner := map[string]string{alice: "alice", bob: "bob"}[v.Nick]
		if _, err = m.Connect(owner, v); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = m.WaitConnected(ctx, owner, "one")
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
	if !m.Snapshot("alice").Networks[0].Features.Redact {
		t.Fatal("redaction capability not exposed as a network feature")
	}
	a, err := m.Join("alice", "one", room)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Join("bob", "one", room)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(findConversation(t, m, "alice", "one", room).MemberIDs) == 2 })
	if err = m.Send("alice", SendInput{ClientID: "doomed", ConversationID: a.ID, Text: "delete me"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "bob", b.ID)) == 1 && len(page(m, "alice", a.ID)) == 1 })
	theirs := page(m, "bob", b.ID)[0]
	mine := page(m, "alice", a.ID)[0]
	if err = m.Send("bob", SendInput{ClientID: "quote", ConversationID: b.ID, Text: "quoting it", ReplyToID: theirs.ID}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "bob", b.ID)) == 2 && len(page(m, "alice", a.ID)) == 2 })
	bobs := page(m, "bob", b.ID)[1]
	if _, err = m.ToggleReaction("bob", b.ID, theirs.ID, "👍"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "alice", a.ID)[0].Reactions) == 1 })

	// Denied: another person's message, other accounts and conversations, and a
	// network-side refusal for an account Ergo does not let redact.
	if _, _, err = m.Redact("bob", b.ID, theirs.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("bob deleting alice's message: %v", err)
	}
	// Workspaces are per account: an account without this conversation cannot
	// reach the message even with valid ids from another workspace.
	if _, _, err = m.Redact("carol", a.ID, mine.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-account target: %v", err)
	}
	if _, _, err = m.Redact("bob", b.ID, bobs.ID); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("unregistered self-redaction should be refused by Ergo: %v", err)
	}
	if page(m, "bob", b.ID)[1].Deleted || page(m, "alice", a.ID)[1].Deleted {
		t.Fatal("a refused deletion changed history")
	}

	cursor := m.Snapshot("bob").Cursor
	deleted, attachment, err := m.Redact("alice", a.ID, mine.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.Deleted || attachment != nil {
		t.Fatalf("unexpected result: %+v %+v", deleted, attachment)
	}
	eventually(t, func() bool {
		v := page(m, "bob", b.ID)
		return v[0].Deleted && v[0].Text == "Message deleted" && len(v[0].Reactions) == 0 && v[1].ReplyTo != nil && v[1].ReplyTo.Text == "Message deleted"
	})
	if v := page(m, "alice", a.ID); !v[0].Deleted || len(v[0].Reactions) != 0 || v[1].ReplyTo.Text != "Message deleted" {
		t.Fatalf("author's own view: %+v", v)
	}
	for _, owner := range []string{"alice", "bob"} {
		conv := map[string]string{"alice": a.ID, "bob": b.ID}[owner]
		saved, err := store.Message(owner, conv, mine.UpstreamID)
		if err != nil || !saved.Deleted || saved.Text != "Message deleted" {
			t.Fatalf("%s stored: %+v %v", owner, saved, err)
		}
		if r, _ := store.Reactions(owner, conv, mine.UpstreamID); len(r) != 0 {
			t.Fatalf("%s kept reactions on a deleted message", owner)
		}
	}
	replay, _, _, stop := m.Subscribe("bob", cursor)
	stop()
	found := false
	for _, e := range replay {
		var v Message
		if e.Type == "message.updated" && json.Unmarshal(e.Data, &v) == nil && v.UpstreamID == mine.UpstreamID && v.Deleted {
			found = true
		}
	}
	if !found {
		t.Fatalf("a browser device resuming from an older cursor would not replay the deletion: %+v", replay)
	}
	if _, _, err = m.Redact("alice", a.ID, mine.ID); !errors.Is(err, ErrAlreadyDeleted) {
		t.Fatalf("repeat deletion: %v", err)
	}

	// Bob's IRCord account is disconnected from IRC while Alice deletes. Ergo
	// drops the message from CHATHISTORY rather than replaying the REDACT, so the
	// reconnect catch-up has to notice the gap in the history window.
	if err = m.Send("alice", SendInput{ClientID: "offline-doomed", ConversationID: a.ID, Text: "deleted while bob is away"}); err != nil {
		t.Fatal(err)
	}
	if err = m.Send("alice", SendInput{ClientID: "offline-kept", ConversationID: a.ID, Text: "kept"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "bob", b.ID)) == 4 && len(page(m, "alice", a.ID)) == 4 })
	doomed := page(m, "alice", a.ID)[2]
	if err = m.Disconnect("bob", "one"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = m.Redact("alice", a.ID, doomed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Connect("bob", NetworkConfig{ID: "one", Name: "one", Host: host, Port: port, Nick: bob}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = m.WaitConnected(ctx, "bob", "one")
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Join("bob", "one", room); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		saved, err := store.Message("bob", b.ID, doomed.UpstreamID)
		return err == nil && saved.Deleted && saved.Text == "Message deleted"
	})
	if kept, err := store.Message("bob", b.ID, page(m, "alice", a.ID)[3].UpstreamID); err != nil || kept.Deleted {
		t.Fatalf("reconciliation deleted a message that still exists: %+v %v", kept, err)
	}
	if first, err := store.Message("bob", b.ID, bobs.UpstreamID); err != nil || first.Deleted {
		t.Fatalf("reconciliation deleted bob's own surviving message: %+v %v", first, err)
	}
}
