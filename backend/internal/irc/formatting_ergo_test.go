package irc

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

// A fenced code block is one canonical multiline message: the same bytes reach
// the other IRCord account live, its private events, storage, a reloaded
// manager, and the chathistory replay after a reconnect.
func TestErgoFencedCodeRoundTrip(t *testing.T) {
	host, port := ergo(t)
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner+"-key-hash"); err != nil {
			t.Fatal(err)
		}
	}
	suffix := id()[:8]
	room := "#code" + suffix
	m := New(Options{Store: store, AllowPrivate: true, ReconnectMin: time.Hour})
	closed := false
	t.Cleanup(func() {
		if !closed {
			m.Close()
			store.Close()
		}
	})
	for owner, nick := range map[string]string{"alice": "ca" + suffix, "bob": "cb" + suffix} {
		if _, err = m.Connect(owner, NetworkConfig{ID: "net", Name: "Ergo", Host: host, Port: port, Nick: nick}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = m.WaitConnected(ctx, owner, "net")
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
	a, err := m.Join("alice", "net", room)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Join("bob", "net", room)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(findConversation(t, m, "alice", "net", room).MemberIDs) == 2 })
	if err = m.Send("bob", SendInput{ClientID: "question", ConversationID: b.ID, Text: "how?"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "alice", a.ID)) == 1 })
	cursor := m.Snapshot("bob").Cursor
	if err = m.Send("alice", SendInput{ClientID: "code", ConversationID: a.ID, Text: fencedCode, ReplyToID: page(m, "alice", a.ID)[0].ID}); err != nil {
		t.Fatal(err)
	}
	var received Message
	eventually(t, func() bool {
		v := page(m, "bob", b.ID)
		if len(v) == 2 {
			received = v[1]
		}
		return received.Text == fencedCode && received.ReplyTo != nil
	})
	eventually(t, func() bool {
		v := page(m, "alice", a.ID)
		return len(v) == 2 && v[1].Text == fencedCode
	})
	replay, _, valid, stop := m.Subscribe("bob", cursor)
	stop()
	events := 0
	for _, event := range replay {
		var msg Message
		if event.Type == "message.created" && json.Unmarshal(event.Data, &msg) == nil && msg.ID == received.ID {
			if msg.Text != fencedCode {
				t.Fatalf("private event changed the text: %q", msg.Text)
			}
			events++
		}
	}
	if !valid || events == 0 {
		t.Fatal("no private message.created event for the code block")
	}
	for owner, conv := range map[string]string{"alice": a.ID, "bob": b.ID} {
		saved, err := store.Message(owner, conv, received.UpstreamID)
		if err != nil || saved.Text != fencedCode {
			t.Fatalf("%s stored %q: %v", owner, saved.Text, err)
		}
	}

	// Another client sends a code block while IRCord is down; the replay keeps it whole.
	peer := girc.New(girc.Config{Server: host, Port: port, Nick: "cp" + suffix, User: "test", Name: "test", AllowFlood: true,
		SupportedCaps: map[string][]string{"batch": nil, "draft/multiline": nil, "message-tags": nil}})
	ready, done := make(chan struct{}), make(chan struct{})
	peer.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) { c.Cmd.Join(room); close(ready) })
	go func() { defer close(done); _ = peer.Connect() }()
	t.Cleanup(func() { peer.Close(); <-done })
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("peer did not connect")
	}
	if !peer.HasCapability("draft/multiline") {
		t.Fatal("the Ergo fixture did not negotiate draft/multiline")
	}
	eventually(t, func() bool { return len(findConversation(t, m, "alice", "net", room).MemberIDs) == 3 })
	m.Close()
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	closed = true
	missed := "```\n  indented\n\n\tlast `line`  \n```"
	peer.Send(&girc.Event{Command: "BATCH", Params: []string{"+code", "draft/multiline", room}})
	for _, line := range strings.Split(missed, "\n") {
		peer.Send(&girc.Event{Command: "PRIVMSG", Params: []string{room, line}, Tags: girc.Tags{"batch": "code"}})
	}
	peer.Send(&girc.Event{Command: "BATCH", Params: []string{"-code"}})

	store, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m = New(Options{Store: store, AllowPrivate: true})
	defer m.Close()
	if err = m.Restore(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(40 * time.Second)
	for {
		var v []Message
		if p, e := m.HistoryContext(context.Background(), "alice", a.ID, "", 40); e == nil {
			v = slices.DeleteFunc(p.Items, func(v Message) bool { return v.Kind != "message" })
		}
		if len(v) >= 3 && v[1].Text == fencedCode && v[1].ReplyTo != nil && v[2].Text == missed {
			break
		}
		if time.Now().After(deadline) {
			texts := []string{}
			for _, msg := range v {
				texts = append(texts, msg.Text)
			}
			t.Fatalf("history after reload and replay: %q", texts)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
