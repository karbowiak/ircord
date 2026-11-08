package irc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

func markerWorkspace(t *testing.T, capable bool) (*Manager, *connection, *storage.Store, Conversation) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateAccount("owner", "read-marker-test"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store})
	t.Cleanup(func() { m.Close(); store.Close() })
	w := m.space("owner")
	c := &connection{manager: m, owner: w, config: NetworkConfig{ID: "net", Nick: "reader"}, ctx: context.Background(), queue: make(chan []*girc.Event, 64), ready: true, caps: map[string]bool{"draft/read-marker": capable, "server-time": capable}, nicks: map[string]string{}}
	w.mu.Lock()
	w.connections["net"] = c
	w.networks["net"] = Network{ID: "net", Status: "connected"}
	v := c.conversation("#markers")
	w.mu.Unlock()
	return m, c, store, v
}

func TestIncomingReadMarkersAreMonotonicAndReconcileOnlyOnce(t *testing.T) {
	m, c, store, conv := markerWorkspace(t, true)
	at := time.Now().UTC().Truncate(time.Millisecond)
	for i := 0; i < 3; i++ {
		msg := Message{ID: fmt.Sprint(i), UpstreamID: fmt.Sprint("server-", i), ConversationID: conv.ID, SenderID: "peer", Kind: "message", Mention: i != 1, CreatedAt: at.Add(time.Duration(i) * time.Second)}
		if err := store.SaveHistory("owner", msg); err != nil {
			t.Fatal(err)
		}
	}
	check := func(unread, mentions int) {
		t.Helper()
		for _, v := range m.Snapshot("owner").Conversations {
			if v.ID == conv.ID && v.UnreadCount == unread && v.MentionCount == mentions {
				return
			}
		}
		t.Fatalf("expected unread=%d mentions=%d, got %+v", unread, mentions, m.Snapshot("owner").Conversations)
	}
	marker := func(value string) {
		c.receiveReadMarker(girc.Event{Command: "MARKREAD", Params: []string{"#markers", value}})
	}
	cursor := m.Snapshot("owner").Cursor
	marker("timestamp=" + at.Format(ircTimeLayout))
	check(2, 1)
	if counts := countsAfter(t, m, "owner", cursor); counts.ID != conv.ID || counts.UnreadCount != 2 || counts.MentionCount != 1 {
		t.Fatalf("incoming read marker sent %+v", counts)
	}
	marker("timestamp=" + at.Add(-time.Second).Format(ircTimeLayout))
	marker("*")
	marker("timestamp=invalid")
	check(2, 1)
	if len(c.queue) != 0 {
		t.Fatal("stale markers caused an echo loop")
	}
	read := true
	if _, err := m.UpdateConversation("owner", conv.ID, &read, nil); err != nil {
		t.Fatal(err)
	}
	<-c.queue // Normal local mark-read command.
	check(0, 0)
	// A fresh connection receives an older server marker. Reconcile the saved
	// local position once, even if the server clamps and echoes it repeatedly.
	c.readMarkersSeen = nil
	marker("*")
	if len(c.queue) != 1 {
		t.Fatal("offline local position was not reconciled")
	}
	batch := <-c.queue
	if batch[0].Last() != "timestamp="+at.Add(2*time.Second).Format(ircTimeLayout) {
		t.Fatal("incorrect reconciliation timestamp")
	}
	marker("*")
	marker("timestamp=" + at.Format(ircTimeLayout))
	check(0, 0)
	if len(c.queue) != 0 {
		t.Fatal("reconciliation kept echoing a stale server marker")
	}
}

func TestReadMarkerWithoutCapabilityKeepsLocalState(t *testing.T) {
	m, c, store, conv := markerWorkspace(t, false)
	at := time.Now().UTC().Truncate(time.Millisecond)
	if err := store.SaveHistory("owner", Message{ID: "unread", ConversationID: conv.ID, SenderID: "peer", Kind: "message", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	c.receiveReadMarker(girc.Event{Command: "MARKREAD", Params: []string{"#markers", "timestamp=" + at.Format(ircTimeLayout)}})
	if n, _ := store.Unread("owner", conv.ID); n != 1 {
		t.Fatal("unnegotiated marker changed read state")
	}
	read := true
	if _, err := m.UpdateConversation("owner", conv.ID, &read, nil); err != nil {
		t.Fatal(err)
	}
	if n, _ := store.Unread("owner", conv.ID); n != 0 {
		t.Fatal("local account read state requires upstream capability")
	}
	if len(c.queue) != 0 {
		t.Fatal("sent an unnegotiated command")
	}
}

func TestReadMarkerUsesLatestServerMessageTime(t *testing.T) {
	for _, capable := range []bool{false, true} {
		t.Run(map[bool]string{false: "local fallback", true: "upstream marker"}[capable], func(t *testing.T) {
			m, c, store, conv := markerWorkspace(t, capable)
			// A server clock ahead of the backend must not leave a read message unread.
			at := time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond)
			message := Message{ID: "first", UpstreamID: "upstream-first", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "reader: hello", Mention: true, CreatedAt: at}
			if err := store.SaveHistory("owner", message); err != nil {
				t.Fatal(err)
			}
			read := true
			if _, err := m.UpdateConversation("owner", conv.ID, &read, nil); err != nil {
				t.Fatal(err)
			}
			if n, err := store.Unread("owner", conv.ID); err != nil || n != 0 {
				t.Fatalf("marked message is still unread: %d %v", n, err)
			}
			if n, err := store.Mentions("owner", conv.ID); err != nil || n != 0 {
				t.Fatalf("marked mention is still unread: %d %v", n, err)
			}
			if capable {
				select {
				case batch := <-c.queue:
					if len(batch) != 1 || batch[0].Command != "MARKREAD" || batch[0].Last() != "timestamp="+at.Format(ircTimeLayout) {
						t.Fatalf("marker does not use the server message timestamp: %v", batch)
					}
				default:
					t.Fatal("missing upstream marker")
				}
			} else if len(c.queue) != 0 {
				t.Fatal("MARKREAD sent without negotiated capability")
			}
		})
	}
}

func TestNewDirectRequestsInitialReadMarker(t *testing.T) {
	m, c, _, _ := markerWorkspace(t, true)
	if _, err := m.Direct("owner", "net", "peer"); err != nil {
		t.Fatal(err)
	}
	select {
	case batch := <-c.queue:
		if len(batch) != 1 || batch[0].Command != "MARKREAD" || len(batch[0].Params) != 1 || batch[0].Params[0] != "peer" {
			t.Fatalf("unexpected initial marker request: %v", batch)
		}
	default:
		t.Fatal("new DM did not request its upstream read position")
	}
}

func TestManualUnreadStaysLocalUntilReadAgain(t *testing.T) {
	m, c, store, conv := markerWorkspace(t, true)
	at := time.Now().UTC().Truncate(time.Millisecond)
	for i := 0; i < 3; i++ {
		msg := Message{ID: fmt.Sprint(i), UpstreamID: fmt.Sprint("server-", i), ConversationID: conv.ID, SenderID: "peer", Kind: "message", CreatedAt: at.Add(time.Duration(i) * time.Second)}
		if err := store.SaveHistory("owner", msg); err != nil {
			t.Fatal(err)
		}
	}
	read := true
	if _, err := m.UpdateConversation("owner", conv.ID, &read, nil); err != nil {
		t.Fatal(err)
	}
	<-c.queue
	check := func(unread int, from string) {
		t.Helper()
		for _, v := range m.Snapshot("owner").Conversations {
			if v.ID == conv.ID && v.UnreadCount == unread && v.UnreadFrom == from {
				return
			}
		}
		t.Fatalf("expected unread=%d from=%q, got %+v", unread, from, m.Snapshot("owner").Conversations)
	}
	if _, err := m.MarkUnread("owner", conv.ID, "missing"); err != ErrNotFound {
		t.Fatal("unknown message accepted", err)
	}
	v, err := m.MarkUnread("owner", conv.ID, "1")
	if err != nil || v.UnreadCount != 2 || v.UnreadFrom != "1" {
		t.Fatalf("mark unread: %+v %v", v, err)
	}
	if len(c.queue) != 0 {
		t.Fatal("manual unread was sent upstream")
	}
	// A reconnect replays the server's (equal) marker: the boundary survives it.
	c.readMarkersSeen = nil
	c.receiveReadMarker(girc.Event{Command: "MARKREAD", Params: []string{"#markers", "timestamp=" + at.Add(2*time.Second).Format(ircTimeLayout)}})
	check(2, "1")
	if len(c.queue) != 0 {
		t.Fatal("equal marker caused a reconcile")
	}
	// Explicit read clears it, even with no newer message to advance the position.
	if _, err := m.UpdateConversation("owner", conv.ID, &read, nil); err != nil {
		t.Fatal(err)
	}
	check(0, "")
	// Another client reading further upstream also ends the boundary.
	if _, err := m.MarkUnread("owner", conv.ID, "0"); err != nil {
		t.Fatal(err)
	}
	check(3, "0")
	later := Message{ID: "3", UpstreamID: "server-3", ConversationID: conv.ID, SenderID: "peer", Kind: "message", CreatedAt: at.Add(3 * time.Second)}
	if err := store.SaveHistory("owner", later); err != nil {
		t.Fatal(err)
	}
	c.receiveReadMarker(girc.Event{Command: "MARKREAD", Params: []string{"#markers", "timestamp=" + later.CreatedAt.Format(ircTimeLayout)}})
	check(0, "")
}

// Without a store the boundary lives in memory and follows the same counting rules.
func TestManualUnreadWithoutStore(t *testing.T) {
	m := New(Options{})
	t.Cleanup(m.Close)
	w := m.space("owner")
	c := &connection{manager: m, owner: w, config: NetworkConfig{ID: "net", Nick: "reader"}, ctx: context.Background(), queue: make(chan []*girc.Event, 64), ready: true, caps: map[string]bool{"draft/read-marker": true, "server-time": true}, nicks: map[string]string{}}
	at := time.Now().UTC().Truncate(time.Millisecond)
	w.mu.Lock()
	w.connections["net"] = c
	w.networks["net"] = Network{ID: "net", Status: "connected"}
	conv := c.conversation("#markers")
	for i, sender := range []string{"peer", w.self.ID, "peer", "system", "peer"} {
		kind := "message"
		if sender == "system" {
			kind, sender = "system", ""
		}
		w.messages[conv.ID] = append(w.messages[conv.ID], Message{ID: fmt.Sprint(i), ConversationID: conv.ID, SenderID: sender, Kind: kind, Mention: i == 4, CreatedAt: at.Add(time.Duration(i) * time.Second)})
	}
	w.mu.Unlock()
	read := true
	if _, err := m.UpdateConversation("owner", conv.ID, &read, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := m.MarkUnread("owner", conv.ID, "3"); err != ErrNotFound {
		t.Fatal("a system row was accepted as the boundary", err)
	}
	// Your own chosen message, then what others sent after it; system rows don't count.
	v, err := m.MarkUnread("owner", conv.ID, "1")
	if err != nil || v.UnreadCount != 3 || v.MentionCount != 1 || v.UnreadFrom != "1" {
		t.Fatalf("mark unread: %+v %v", v, err)
	}
	if len(c.queue) != 0 {
		t.Fatal("manual unread was sent upstream")
	}
	for _, s := range m.Snapshot("owner").Conversations {
		if s.ID == conv.ID && (s.UnreadCount != 3 || s.UnreadFrom != "1") {
			t.Fatalf("snapshot lost the boundary: %+v", s)
		}
	}
	v, err = m.UpdateConversation("owner", conv.ID, &read, nil)
	if err != nil || v.UnreadCount != 0 || v.UnreadFrom != "" {
		t.Fatalf("read did not clear the boundary: %+v %v", v, err)
	}
}
