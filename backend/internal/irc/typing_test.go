package irc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/notify"
)

func TestTypingPolicyIsolationAndNoReplay(t *testing.T) {
	for _, tc := range []struct {
		policy  string
		allowed bool
	}{{"", true}, {"*", false}, {"*,-typing,-channel-context", true}, {"typing", false}, {"draft/*", true}} {
		if tagAllowed(tc.policy, "typing") != tc.allowed {
			t.Fatalf("CLIENTTAGDENY %q", tc.policy)
		}
	}
	m := New(Options{})
	defer m.Close()
	w := m.space("alice")
	w.members["peer"] = Member{ID: "peer", Nick: "Peer", Username: "user", Host: "example.test"}
	c := &connection{owner: w, config: NetworkConfig{ID: "network", Nick: "Alice"}, ctx: m.ctx, ready: true, caps: map[string]bool{"message-tags": true}, nicks: map[string]string{"peer": "peer"}, historyBatches: map[string]bool{}, queue: make(chan []*girc.Event, 8)}
	w.connections["network"] = c
	conv := c.conversation("#test")
	conv.Joined = true
	conv.MemberIDs = append(conv.MemberIDs, "peer")
	w.conversations[conv.ID] = conv
	activity, stop := m.SubscribeTyping("alice")
	defer stop()
	foreign, stopForeign := m.SubscribeTyping("bob")
	defer stopForeign()
	cursor := m.Snapshot("alice").Cursor
	event := girc.Event{Command: "TAGMSG", Source: &girc.Source{Name: "Peer", Ident: "user", Host: "example.test"}, Params: []string{"#test"}, Tags: girc.Tags{"+typing": "active"}}
	c.typing(event)
	c.capture(event, false)
	c.capture(girc.Event{Command: "TAGMSG", Params: []string{"#test"}, Tags: girc.Tags{"+typing": "active"}}, true)
	select {
	case e := <-activity:
		if e.Data.MemberID != "peer" || e.Data.State != model.TypingActive || e.Data.ConversationID != conv.ID {
			t.Fatal(e)
		}
		raw, _ := json.Marshal(e)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		if _, ok := body["seq"]; ok {
			t.Fatal("typing has a sequence")
		}
	default:
		t.Fatal("missing incoming typing")
	}
	for range 100 {
		c.typing(event)
	}
	if len(activity) != 0 {
		t.Fatal("repeated remote activity was not coalesced")
	}
	select {
	case e := <-foreign:
		t.Fatal("typing leaked", e)
	default:
	}
	if m.Snapshot("alice").Cursor != cursor {
		t.Fatal("typing changed durable cursor")
	}
	replay, _, _, unsub := m.Subscribe("alice", cursor)
	defer unsub()
	if len(replay) != 0 {
		t.Fatal("typing was replayed")
	}
	w.policy = notify.New(notify.Settings{Ignore: []string{"*!user@example.test"}})
	event.Tags["+typing"] = "paused"
	c.typing(event)
	select {
	case e := <-activity:
		t.Fatal("ignored sender produced typing", e)
	default:
	}
	w.policy = notify.New(notify.Defaults())
	c.historyBatches["old"] = true
	event.Tags["batch"] = "old"
	c.typing(event)
	select {
	case e := <-activity:
		t.Fatal("historical typing was delivered", e)
	default:
	}
	if err := m.SetTyping("bob", conv.ID, model.TypingActive); err != ErrNotFound {
		t.Fatal("cross-owner typing accepted", err)
	}
	if err := m.SetTyping("alice", conv.ID, "bad"); err == nil {
		t.Fatal("invalid state accepted")
	}
}

func TestOutgoingTypingThrottleAndBoundedQueues(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	w := m.space("owner")
	c := &connection{owner: w, config: NetworkConfig{ID: "net"}, ctx: m.ctx, ready: true, caps: map[string]bool{"message-tags": true}, queue: make(chan []*girc.Event, 8)}
	w.connections["net"] = c
	conv := c.conversation("&local")
	conv.Joined = true
	w.conversations[conv.ID] = conv
	for range 5 {
		if err := m.SetTyping("owner", conv.ID, model.TypingActive); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.queue) != 1 {
		t.Fatal("active typing flooded the IRC queue")
	}
	first := (<-c.queue)[0]
	if first.Command != "TAGMSG" || first.Params[0] != "&local" || first.Tags["+typing"] != "active" {
		t.Fatal(first)
	}
	// A refresh after the protocol's three-second window must extend the indicator.
	c.mu.Lock()
	previous := c.typingSent[conv.ID]
	previous.ActiveAt = time.Now().Add(-4 * time.Second)
	c.typingSent[conv.ID] = previous
	c.mu.Unlock()
	if err := m.SetTyping("owner", conv.ID, model.TypingActive); err != nil {
		t.Fatal(err)
	}
	if len(c.queue) != 1 {
		t.Fatal("active refresh never emitted")
	}
	if err := m.SetTyping("owner", conv.ID, model.TypingDone); err != nil {
		t.Fatal(err)
	}
	if len(c.queue) != 2 {
		t.Fatal("done was not emitted")
	}
	for range 100 {
		_ = m.SetTyping("owner", conv.ID, model.TypingPaused)
		_ = m.SetTyping("owner", conv.ID, model.TypingDone)
	}
	if len(c.queue) > 7 {
		t.Fatal("state transitions bypassed the outgoing rate limit")
	}
	activity, stop := m.SubscribeTyping("owner")
	defer stop()
	for range 100 {
		w.publishTyping(model.TypingData{ConversationID: conv.ID, MemberID: "peer", State: model.TypingActive})
	}
	if len(activity) != 16 {
		t.Fatal("ephemeral queue is not bounded")
	}
}
