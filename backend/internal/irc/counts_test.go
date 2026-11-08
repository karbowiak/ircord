package irc

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"ircord/backend/internal/model"
)

// eventsAfter returns the durable events after cursor.
func eventsAfter(t *testing.T, m *Manager, owner string, cursor uint64) []Event {
	t.Helper()
	events, _, ok, stop := m.Subscribe(owner, cursor)
	defer stop()
	if !ok {
		t.Fatal("replay unavailable")
	}
	return events
}

func types(events []Event) []string {
	out := []string{}
	for _, e := range events {
		out = append(out, e.Type)
	}
	return out
}

func countsOf(t *testing.T, e Event) model.ConversationCounts {
	t.Helper()
	if e.Type != "conversation.counts" {
		t.Fatalf("event %s, want conversation.counts", e.Type)
	}
	var fields map[string]any
	if err := json.Unmarshal(e.Data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"memberIds", "memberModes", "topic", "name"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("counts event carries %q: %s", key, e.Data)
		}
	}
	var counts model.ConversationCounts
	if err := json.Unmarshal(e.Data, &counts); err != nil {
		t.Fatal(err)
	}
	return counts
}

// withRoster gives #dev a large member list, as a busy channel has after NAMES.
func withRoster(f *inboxFixture, n int) string {
	w := f.c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	conv := f.c.conversation("#dev")
	if conv.MemberModes == nil {
		conv.MemberModes = map[string]string{}
	}
	for i := range n {
		id := fmt.Sprintf("net:member-%d", i)
		conv.MemberIDs = append(conv.MemberIDs, id)
		if i%50 == 0 {
			conv.MemberModes[id] = "v"
		}
	}
	w.conversations[conv.ID] = conv
	return conv.ID
}

func TestIncomingMessageEmitsCountsWithoutRoster(t *testing.T) {
	f := newInboxFixture(t)
	f.say(t, "bob", "#dev", "m1", "first")
	id := withRoster(f, 2000)
	cursor := f.m.Snapshot("alice").Cursor

	f.say(t, "bob", "#dev", "m2", "hello alice")
	events := eventsAfter(t, f.m, "alice", cursor)
	if slices.Contains(types(events), "conversation.updated") {
		t.Fatalf("an incoming message resent the whole conversation: %v", types(events))
	}
	if len(events) < 2 || events[1].Type != "message.created" {
		t.Fatalf("events: %v", types(events))
	}
	counts := countsOf(t, events[0])
	if counts.ID != id || counts.UnreadCount != 2 || counts.MentionCount != 1 || counts.UnreadFrom != "" {
		t.Fatalf("counts: %+v", counts)
	}
	if len(events[0].Data) > 200 {
		t.Fatalf("counts event is %d bytes", len(events[0].Data))
	}
}

func TestReadAndMarkUnreadEmitCountsAndFavoriteStaysFull(t *testing.T) {
	f := newInboxFixture(t)
	f.say(t, "bob", "#dev", "m1", "first")
	f.say(t, "bob", "#dev", "m2", "second")
	id := withRoster(f, 10)

	cursor := f.m.Snapshot("alice").Cursor
	if _, err := f.m.MarkUnread("alice", id, "net:m2"); err != nil {
		t.Fatal(err)
	}
	events := eventsAfter(t, f.m, "alice", cursor)
	if got := types(events); !slices.Equal(got, []string{"conversation.counts"}) {
		t.Fatalf("mark unread events: %v", got)
	}
	if counts := countsOf(t, events[0]); counts.UnreadCount != 1 || counts.UnreadFrom != "net:m2" {
		t.Fatalf("mark unread counts: %+v", counts)
	}

	cursor = f.m.Snapshot("alice").Cursor
	read := true
	if _, err := f.m.UpdateConversation("alice", id, &read, nil); err != nil {
		t.Fatal(err)
	}
	events = eventsAfter(t, f.m, "alice", cursor)
	if got := types(events); !slices.Equal(got, []string{"conversation.counts"}) {
		t.Fatalf("read events: %v", got)
	}
	if counts := countsOf(t, events[0]); counts.UnreadCount != 0 || counts.MentionCount != 0 || counts.UnreadFrom != "" {
		t.Fatalf("read counts: %+v", counts)
	}

	cursor = f.m.Snapshot("alice").Cursor
	favorite := true
	if _, err := f.m.UpdateConversation("alice", id, nil, &favorite); err != nil {
		t.Fatal(err)
	}
	events = eventsAfter(t, f.m, "alice", cursor)
	if got := types(events); !slices.Equal(got, []string{"conversation.updated"}) {
		t.Fatalf("favorite events: %v", got)
	}
	var conv Conversation
	if err := json.Unmarshal(events[0].Data, &conv); err != nil {
		t.Fatal(err)
	}
	if !conv.Favorite || len(conv.MemberIDs) < 10 {
		t.Fatalf("favorite event: favorite=%v members=%d", conv.Favorite, len(conv.MemberIDs))
	}
}

func TestNotificationSettingsRecountEmitsCounts(t *testing.T) {
	f := newInboxFixture(t)
	f.say(t, "bob", "#dev", "m1", "first")
	cursor := f.m.Snapshot("alice").Cursor
	f.settings(t, `{"ignore":["bob"]}`)
	events := eventsAfter(t, f.m, "alice", cursor)
	if got := types(events); !slices.Equal(got, []string{"notification-settings.updated", "conversation.counts"}) {
		t.Fatalf("settings events: %v", got)
	}
	if counts := countsOf(t, events[1]); counts.UnreadCount != 0 {
		t.Fatalf("ignoring bob kept his message unread: %+v", counts)
	}
}

// Count-only events leave the stored conversation's counts behind; after a restart
// live counts must continue from storage, matching the bootstrap.
func TestCountsContinueFromStorageAfterRestart(t *testing.T) {
	f := newInboxFixture(t)
	f.say(t, "bob", "#dev", "m1", "first")
	f.say(t, "bob", "#dev", "m2", "second")
	f.m.Close()

	m := New(Options{Store: f.store})
	t.Cleanup(m.Close)
	c := &connection{owner: m.space("alice"), manager: m, config: f.c.config, nicks: map[string]string{}, historyBatches: map[string]bool{}, historyResponses: map[string]*historyRequest{}}
	f.m, f.c = m, c
	cursor := m.Snapshot("alice").Cursor
	f.say(t, "bob", "#dev", "m3", "third")

	events := eventsAfter(t, m, "alice", cursor)
	i := slices.IndexFunc(events, func(e Event) bool { return e.Type == "conversation.counts" })
	if i < 0 {
		t.Fatalf("no counts event: %v", types(events))
	}
	counts := countsOf(t, events[i])
	snapshot := m.Snapshot("alice")
	if len(snapshot.Conversations) != 1 {
		t.Fatalf("conversations: %d", len(snapshot.Conversations))
	}
	if counts.UnreadCount != 3 || counts.UnreadCount != snapshot.Conversations[0].UnreadCount || counts.MentionCount != snapshot.Conversations[0].MentionCount {
		t.Fatalf("live counts %+v, bootstrap %d/%d", counts, snapshot.Conversations[0].UnreadCount, snapshot.Conversations[0].MentionCount)
	}
}
