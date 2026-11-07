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
	marker("timestamp=" + at.Format(ircTimeLayout))
	check(2, 1)
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
