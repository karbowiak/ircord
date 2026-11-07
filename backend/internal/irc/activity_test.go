package irc

import (
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

func TestHistoryStatusWindowProjectionAndIsolation(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner+"key"); err != nil {
			t.Fatal(err)
		}
		if _, err = store.Commit(owner, "network.updated", Network{ID: "net", Name: "Ergo"}); err != nil {
			t.Fatal(err)
		}
		if _, err = store.Commit(owner, "conversation.updated", Conversation{ID: "room", NetworkID: "net", Kind: "channel", Name: "test", Target: "#test"}); err != nil {
			t.Fatal(err)
		}
	}
	at := time.Now().UTC()
	if _, err = store.HistoryInsert("alice", Message{ID: "net:_old", UpstreamID: "_old", ConversationID: "room", Kind: "system", Text: "a legacy join notice", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store})
	defer m.Close()
	page, err := m.ServerLog("alice", "net", "", 100, false)
	if err != nil || len(page.Items) != 1 || page.Items[0].Text != "[#test] a legacy join notice" {
		t.Fatal(page, err)
	}
	if again, err := m.ServerLog("alice", "net", "", 100, false); err != nil || len(again.Items) != 1 {
		t.Fatal("projection duplicated", again, err)
	}
	if other, err := m.ServerLog("bob", "net", "", 100, false); err != nil || len(other.Items) != 0 {
		t.Fatal("projection escaped account", other, err)
	}
	w := m.space("alice")
	c := &connection{owner: w, config: NetworkConfig{ID: "net"}}
	e := girc.Event{Command: "NOTICE", Source: &girc.Source{Name: "HistServ", Ident: "service", Host: "localhost"}, Params: []string{"#test", "another history event"}, Tags: girc.Tags{"msgid": "_new", "time": at.Format(time.RFC3339Nano)}}
	c.capture(e, false)
	c.capture(e, false)
	page, err = m.ServerLog("alice", "net", "", 100, false)
	if err != nil || len(page.Items) != 2 {
		t.Fatal("history notice missing or duplicated", page, err)
	}
	if !strings.Contains(page.Items[1].Text, "#test") {
		t.Fatal("missing channel context")
	}
	history, err := store.History("alice", "room", "", 100)
	if err != nil || len(history.Items) != 1 {
		t.Fatal("history/cursor records were removed", err)
	}
}

func TestInviteActivityDoesNotJoinOrLeak(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner+"key"); err != nil {
			t.Fatal(err)
		}
	}
	m := New(Options{Store: store})
	w := m.space("alice")
	c := &connection{owner: w, config: NetworkConfig{ID: "net", Nick: "Alice"}, nicks: map[string]string{}, historyBatches: map[string]bool{}}
	event := girc.Event{Command: "INVITE", Source: &girc.Source{Name: "Inviter"}, Params: []string{"Alice", "#private"}}
	for range 10 {
		c.membership(event)
	}
	snap := m.Snapshot("alice")
	if len(snap.Invitations) != 1 || len(snap.Conversations) != 0 {
		t.Fatal("invite changed membership or duplicated", snap)
	}
	if len(m.Snapshot("bob").Invitations) != 0 {
		t.Fatal("invite escaped account")
	}
	invitation := snap.Invitations[0]
	if err = m.RejectInvitation("bob", invitation.ID); err != ErrNotFound {
		t.Fatal("foreign rejection accepted", err)
	}
	if err = m.RejectInvitation("alice", invitation.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.RejectInvitation("alice", invitation.ID); err != nil {
		t.Fatal("rejection not idempotent", err)
	}
	event.Source.Name = "AnotherInviter"
	for range 10 {
		c.membership(event)
	}
	if len(m.Snapshot("alice").Invitations) != 0 {
		t.Fatal("rejected invitation repeated")
	}
	m.Close()
	store.Close()
	store, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m = New(Options{Store: store})
	defer m.Close()
	w = m.space("alice")
	c.owner = w
	c.membership(event)
	if len(m.Snapshot("alice").Invitations) != 0 {
		t.Fatal("restart lost rejection")
	}
	event.Params[1] = "#other"
	c.membership(event)
	if len(m.Snapshot("alice").Invitations) != 1 {
		t.Fatal("different channel suppressed")
	}
	w.mu.Lock()
	w.acceptInvitation("net", c.fold("#other"))
	w.mu.Unlock()
	if len(m.Snapshot("alice").Invitations) != 0 {
		t.Fatal("accepted invitation remained")
	}
	event.Params = []string{"Unrelated", "#unrelated"}
	c.membership(event)
	if len(m.Snapshot("alice").Invitations) != 0 || len(m.Snapshot("alice").Conversations) != 0 {
		t.Fatal("unrelated invitation changed workspace")
	}
	replay, valid, err := store.Replay("alice", 0, 100)
	if err != nil || !valid || len(replay) != 4 {
		t.Fatal("invitation state and private durable events diverged", len(replay), err)
	}
}
