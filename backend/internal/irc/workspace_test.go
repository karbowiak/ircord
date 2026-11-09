package irc

import (
	"context"
	"errors"
	"testing"
	"time"

	"ircord/backend/internal/storage"
)

func TestTargetedReadsAreIsolatedAndSkipConversationQueries(t *testing.T) {
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.CreateAccount("alice", "key"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: s})
	defer m.Close()
	w := m.space("alice")
	w.networks["network"] = Network{ID: "network", ISupport: map[string]string{"NETWORK": "private"}}
	w.members["peer"] = Member{ID: "peer", Nick: "peer"}
	w.conversations["room"] = Conversation{ID: "room"}
	w.self.NameColor = "#123456"
	w.seq = 42
	// A full Snapshot would issue unread queries for room; these lookups must not.
	before := s.DB.Stats().WaitCount
	conn, err := s.DB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	type result struct {
		network      Network
		member, self Member
		err          error
		snapshot     Snapshot
		cursor       uint64
	}
	done := make(chan result, 1)
	go func() {
		network, err := m.Network("alice", "network")
		if err != nil {
			done <- result{err: err}
			return
		}
		member, err := m.Member("alice", "peer")
		done <- result{network: network, member: member, self: m.Self("alice"), snapshot: m.Snapshot("alice"), cursor: m.Cursor("alice"), err: err}
	}()
	var got result
	select {
	case got = <-done:
	case <-time.After(time.Second):
		t.Fatal("targeted read waited for conversation storage")
	}
	conn.Close()
	if got.err != nil {
		t.Fatal(got.err)
	}
	if s.DB.Stats().WaitCount != before {
		t.Fatal("targeted read queried conversation storage")
	}
	if got.member.Nick != "peer" || got.self.NameColor != "#123456" {
		t.Fatal("targeted read returned the wrong entity")
	}
	if got.cursor != 42 || got.snapshot.Cursor != 42 || len(got.snapshot.Conversations) != 1 {
		t.Fatal("snapshot did not capture the shared account view and cursor")
	}
	got.network.ISupport["NETWORK"] = "changed"
	if w.networks["network"].ISupport["NETWORK"] != "private" {
		t.Fatal("network result aliases account state")
	}
	if _, err := m.Network("bob", "network"); !errors.Is(err, ErrNotFound) {
		t.Fatal("network leaked across accounts")
	}
	if _, err := m.Member("bob", "peer"); !errors.Is(err, ErrNotFound) {
		t.Fatal("member leaked across accounts")
	}
}
