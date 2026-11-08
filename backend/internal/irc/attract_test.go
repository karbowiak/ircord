package irc

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/notify"
	"ircord/backend/internal/storage"
)

type attractFixture struct {
	m    *Manager
	w    *workspace
	c    *connection
	conv Conversation
}

func newAttractFixture(t *testing.T) attractFixture {
	t.Helper()
	m := New(Options{})
	t.Cleanup(m.Close)
	w := m.space("alice")
	w.members["peer"] = Member{ID: "peer", Nick: "Peer", Username: "user", Host: "example.test"}
	w.networks["network"] = Network{ID: "network", ISupport: map[string]string{"PREFIX": "(qaohv)~&@%+"}}
	c := &connection{owner: w, config: NetworkConfig{ID: "network", Nick: "Alice"}, ctx: m.ctx, ready: true, caps: map[string]bool{"message-tags": true}, nicks: map[string]string{"peer": "peer"}, historyBatches: map[string]bool{}, queue: make(chan []*girc.Event, 8)}
	w.connections["network"] = c
	conv := c.conversation("#test")
	conv.Joined = true
	conv.MemberIDs = append(conv.MemberIDs, w.self.ID, "peer")
	conv.MemberModes = map[string]string{w.self.ID: "@", "peer": "&"}
	w.conversations[conv.ID] = conv
	now := time.Now()
	w.messages[conv.ID] = []Message{
		{ID: "own-old", ConversationID: conv.ID, SenderID: w.self.ID, UpstreamID: "up-own-old", Kind: "message", CreatedAt: now.Add(-time.Minute)},
		{ID: "own-new", ConversationID: conv.ID, SenderID: w.self.ID, UpstreamID: "up-own-new", Kind: "message", CreatedAt: now.Add(-30 * time.Second)},
		{ID: "local", ConversationID: conv.ID, SenderID: w.self.ID, UpstreamID: "_local", Kind: "message", CreatedAt: now.Add(-20 * time.Second)},
		{ID: "peer-msg", ConversationID: conv.ID, SenderID: "peer", UpstreamID: "up-peer", Kind: "message", CreatedAt: now.Add(-10 * time.Second)},
	}
	return attractFixture{m: m, w: w, c: c, conv: conv}
}

func (f attractFixture) setModes(member, modes string) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	conv := f.w.conversations[f.conv.ID]
	conv.MemberModes[member] = modes
	f.w.conversations[f.conv.ID] = conv
}

func TestAttractSendsOneTagmsgAndRateLimits(t *testing.T) {
	f := newAttractFixture(t)
	activity, stop := f.m.SubscribeEphemeral("alice")
	defer stop()
	foreign, stopForeign := f.m.SubscribeEphemeral("bob")
	defer stopForeign()
	cursor := f.m.Snapshot("alice").Cursor

	target, err := f.m.Attract("alice", f.conv.ID)
	if err != nil || target.ID != "own-new" {
		t.Fatal(target.ID, err)
	}
	if len(f.c.queue) != 1 {
		t.Fatalf("queued %d batches", len(f.c.queue))
	}
	sent := (<-f.c.queue)[0]
	if sent.Command != "TAGMSG" || len(sent.Params) != 1 || sent.Params[0] != "#test" || len(sent.Tags) != 1 || sent.Tags[attractTag] != "up-own-new" {
		t.Fatalf("wire event: %+v", sent)
	}
	select {
	case next := <-activity:
		e := next.(model.AttractEvent)
		if e.Type != "attract" || e.Data != (model.AttractData{ConversationID: f.conv.ID, MemberID: f.w.self.ID, MessageID: "own-new"}) {
			t.Fatal(e)
		}
		raw, _ := json.Marshal(e)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		if _, ok := body["seq"]; ok {
			t.Fatal("attract has a sequence")
		}
	default:
		t.Fatal("sender did not see their own attract")
	}
	if len(foreign) != 0 {
		t.Fatal("attract leaked to another account")
	}
	if f.m.Snapshot("alice").Cursor != cursor {
		t.Fatal("attract changed the durable cursor")
	}

	_, err = f.m.Attract("alice", f.conv.ID)
	var limited RateLimitError
	if !errors.Is(err, ErrRateLimited) || !errors.As(err, &limited) || limited.Wait <= 25*time.Second || len(f.c.queue) != 0 {
		t.Fatal("second attract inside the cooldown", err, len(f.c.queue))
	}

	// The per-connection burst limit spans channels.
	f.c.mu.Lock()
	f.c.attractSent = map[string]time.Time{}
	f.c.attractRecent = []time.Time{time.Now(), time.Now(), time.Now(), time.Now(), time.Now()}
	f.c.mu.Unlock()
	if _, err = f.m.Attract("alice", f.conv.ID); !errors.Is(err, ErrRateLimited) || len(f.c.queue) != 0 {
		t.Fatal("burst limit", err)
	}
}

func TestAttractRefusals(t *testing.T) {
	f := newAttractFixture(t)
	for _, modes := range []string{"", "+", "%"} {
		f.setModes(f.w.self.ID, modes)
		if _, err := f.m.Attract("alice", f.conv.ID); !errors.Is(err, ErrAttractNeedsOperator) {
			t.Fatalf("modes %q: %v", modes, err)
		}
	}
	for _, modes := range []string{"@", "&", "~"} {
		f.setModes(f.w.self.ID, modes)
		f.c.mu.Lock()
		f.c.attractSent, f.c.attractRecent = nil, nil
		f.c.mu.Unlock()
		if _, err := f.m.Attract("alice", f.conv.ID); err != nil {
			t.Fatalf("modes %q: %v", modes, err)
		}
		<-f.c.queue
	}

	f = newAttractFixture(t)
	f.w.messages[f.conv.ID] = f.w.messages[f.conv.ID][2:]
	if _, err := f.m.Attract("alice", f.conv.ID); !errors.Is(err, ErrNotFound) || err.Error() == ErrNotFound.Error() {
		t.Fatal("no own message", err)
	}

	f = newAttractFixture(t)
	f.w.networks["network"] = Network{ID: "network", ISupport: map[string]string{"CLIENTTAGDENY": "ircord/attract"}}
	if _, err := f.m.Attract("alice", f.conv.ID); !errors.Is(err, ErrAttractUnsupported) {
		t.Fatal("denied tag", err)
	}
	f = newAttractFixture(t)
	f.c.caps = map[string]bool{}
	if _, err := f.m.Attract("alice", f.conv.ID); !errors.Is(err, ErrAttractUnsupported) {
		t.Fatal("without message-tags", err)
	}

	f = newAttractFixture(t)
	dm := f.c.conversation("Peer")
	f.w.conversations[dm.ID] = dm
	f.w.messages[dm.ID] = []Message{{ID: "dm", SenderID: f.w.self.ID, UpstreamID: "up-dm", Kind: "message"}}
	if _, err := f.m.Attract("alice", dm.ID); err == nil {
		t.Fatal("attract in a DM")
	}
	if _, err := f.m.Attract("bob", f.conv.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-owner attract", err)
	}
	if len(f.c.queue) != 0 {
		t.Fatal("a refused attract reached IRC")
	}
}

func TestAttractReceiveVerifiesAndCoalesces(t *testing.T) {
	f := newAttractFixture(t)
	activity, stop := f.m.SubscribeEphemeral("alice")
	defer stop()
	event := func(tags girc.Tags) girc.Event {
		return girc.Event{Command: "TAGMSG", Source: &girc.Source{Name: "Peer", Ident: "user", Host: "example.test"}, Params: []string{"#test"}, Tags: tags}
	}
	none := func(why string) {
		t.Helper()
		select {
		case e := <-activity:
			t.Fatal(why, e)
		default:
		}
	}

	// Spoof: the referenced message belongs to someone else.
	f.c.attract(event(girc.Tags{attractTag: "up-own-new"}))
	none("spoofed attract was delivered")
	// Unknown message.
	f.c.attract(event(girc.Tags{attractTag: "missing"}))
	none("attract for an unknown message")
	// History replay.
	f.c.historyBatches["old"] = true
	f.c.attract(event(girc.Tags{attractTag: "up-peer", "batch": "old"}))
	none("historical attract was delivered")
	// Sender below operator.
	f.setModes("peer", "%")
	f.c.attract(event(girc.Tags{attractTag: "up-peer"}))
	none("attract from a non-operator")
	f.setModes("peer", "&")
	// Ignored sender.
	f.w.policy = notify.New(notify.Settings{Ignore: []string{"*!user@example.test"}})
	f.c.attract(event(girc.Tags{attractTag: "up-peer"}))
	none("ignored sender produced attract")
	f.w.policy = notify.New(notify.Defaults())
	// Own echo: the own nick resolves to self, an operator, as restoreNicks maps it.
	f.c.nicks["alice"] = f.w.self.ID
	f.c.attract(girc.Event{Command: "TAGMSG", Source: &girc.Source{Name: "Alice"}, Params: []string{"#test"}, Tags: girc.Tags{attractTag: "up-own-new"}})
	none("own echo was delivered twice")

	f.c.attract(event(girc.Tags{attractTag: "up-peer"}))
	select {
	case next := <-activity:
		e := next.(model.AttractEvent)
		if e.Data != (model.AttractData{ConversationID: f.conv.ID, MemberID: "peer", MessageID: "peer-msg"}) {
			t.Fatal(e)
		}
	default:
		t.Fatal("missing incoming attract")
	}
	for range 100 {
		f.c.attract(event(girc.Tags{attractTag: "up-peer"}))
	}
	none("repeated attract was not coalesced")
	replay, _, _, unsub := f.m.Subscribe("alice", 0)
	defer unsub()
	for _, e := range replay {
		if e.Type == "attract" {
			t.Fatal("attract was replayed")
		}
	}
}

func TestAttractReceiveCoalescingRules(t *testing.T) {
	f := newAttractFixture(t)
	activity, stop := f.m.SubscribeEphemeral("alice")
	defer stop()
	f.w.members["other"] = Member{ID: "other", Nick: "Other", Username: "other", Host: "example.test"}
	f.c.nicks["other"] = "other"
	conv := f.w.conversations[f.conv.ID]
	conv.MemberIDs = append(conv.MemberIDs, "other")
	conv.MemberModes["other"] = "@"
	f.w.conversations[f.conv.ID] = conv
	f.w.messages[f.conv.ID] = append(f.w.messages[f.conv.ID], Message{ID: "other-msg", ConversationID: f.conv.ID, SenderID: "other", UpstreamID: "up-other", Kind: "message", CreatedAt: time.Now()})
	from := func(nick, msgid string) {
		f.c.attract(girc.Event{Command: "TAGMSG", Source: &girc.Source{Name: nick, Ident: "user", Host: "example.test"}, Params: []string{"#test"}, Tags: girc.Tags{attractTag: msgid}})
	}
	drain := func() int {
		n := 0
		for len(activity) > 0 {
			<-activity
			n++
		}
		return n
	}

	// Per channel: a second operator inside the running effect is dropped.
	from("Peer", "up-peer")
	from("Other", "up-other")
	if n := drain(); n != 1 {
		t.Fatalf("channel coalescing: %d events", n)
	}
	// Once the effect is over, another operator plays.
	f.c.mu.Lock()
	f.c.attractReceived["c:"+f.conv.ID] = time.Now().Add(-attractEffect)
	f.c.mu.Unlock()
	from("Other", "up-other")
	if n := drain(); n != 1 {
		t.Fatalf("after the effect: %d events", n)
	}
	// Per sender: past the channel window but inside attractRepeat, the same sender is dropped.
	f.c.mu.Lock()
	f.c.attractReceived["c:"+f.conv.ID] = time.Now().Add(-attractEffect)
	f.c.attractReceived["m:"+f.conv.ID+":peer"] = time.Now().Add(-attractEffect)
	f.c.mu.Unlock()
	from("Peer", "up-peer")
	if n := drain(); n != 0 {
		t.Fatalf("sender coalescing: %d events", n)
	}
}

func TestAttractFallsBackToStoredHistory(t *testing.T) {
	f := newAttractFixture(t)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err = store.CreateAccount("alice", "alice"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, msg := range []Message{
		{ID: "stored-old", ConversationID: f.conv.ID, SenderID: f.w.self.ID, UpstreamID: "up-stored-old", Kind: "message", CreatedAt: now.Add(-2 * time.Minute), Reactions: []Reaction{}},
		{ID: "stored-new", ConversationID: f.conv.ID, SenderID: f.w.self.ID, UpstreamID: "up-stored-new", Kind: "message", CreatedAt: now.Add(-time.Minute), Reactions: []Reaction{}},
		{ID: "stored-peer", ConversationID: f.conv.ID, SenderID: "peer", UpstreamID: "up-stored-peer", Kind: "message", CreatedAt: now, Reactions: []Reaction{}},
	} {
		if err = store.SaveHistory("alice", msg); err != nil {
			t.Fatal(err)
		}
	}
	f.w.mu.Lock()
	f.w.store = store
	f.w.messages[f.conv.ID] = nil // e.g. after a restart
	f.w.mu.Unlock()
	target, err := f.m.Attract("alice", f.conv.ID)
	if err != nil || target.ID != "stored-new" {
		t.Fatal(target.ID, err)
	}
	if sent := (<-f.c.queue)[0]; sent.Tags[attractTag] != "up-stored-new" {
		t.Fatalf("wire event: %+v", sent)
	}
}

func TestAttractFailedSendKeepsTheCooldownFree(t *testing.T) {
	f := newAttractFixture(t)
	f.c.queue = make(chan []*girc.Event, 1)
	f.c.queue <- nil // full
	if _, err := f.m.Attract("alice", f.conv.ID); !errors.Is(err, ErrBusy) {
		t.Fatal("full queue", err)
	}
	<-f.c.queue
	if _, err := f.m.Attract("alice", f.conv.ID); err != nil {
		t.Fatal("a refused send used up the cooldown", err)
	}
	if len(f.c.queue) != 1 {
		t.Fatal("retry did not send")
	}
}
