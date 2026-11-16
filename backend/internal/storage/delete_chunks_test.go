package storage

import (
	"fmt"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

// Removing a network clears its open and closed conversations' history and its
// server window in bounded transactions, and nothing of another network.
func TestClearNetworkRowsInChunks(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	saved := deleteChunkRows
	deleteChunkRows = 2
	t.Cleanup(func() { deleteChunkRows = saved })
	at := time.Now().UTC()
	conversations := map[string]string{
		"gone_000000000000000000000001": "gone", // open: has an entity
		"gone_000000000000000000000002": "gone", // closed earlier: ID only
		"kept_000000000000000000000001": "kept",
	}
	for conv, network := range conversations {
		if conv != "gone_000000000000000000000002" {
			if _, err = s.Commit("alice", "conversation.updated", model.Conversation{ID: conv, NetworkID: network, Kind: "channel", Name: "c", MemberIDs: []string{}, MemberModes: map[string]string{}}); err != nil {
				t.Fatal(err)
			}
		}
		for i := range 5 {
			msg := model.Message{ID: fmt.Sprintf("%s:%d", conv, i), ConversationID: conv, SenderID: network + ":x", Kind: "message", Text: "hello", CreatedAt: at.Add(time.Duration(i) * time.Second)}
			if _, err = s.Commit("alice", "message.created", msg); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, network := range []string{"gone", "kept"} {
		lines := []model.ServerLine{}
		for i := range 5 {
			lines = append(lines, model.ServerLine{ID: fmt.Sprintf("%s-%d", network, i), NetworkID: network, Time: at, Kind: "numeric", Command: "352"})
		}
		if _, err = s.Commit("alice", "server-log.updated", model.ServerLogBatch{NetworkID: network, Lines: lines}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.ClearNetworkRows("alice", "gone"); err != nil {
		t.Fatal(err)
	}
	count := func(query string, args ...any) (n int) {
		t.Helper()
		if err := s.DB.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count("SELECT count(*) FROM messages WHERE owner='alice' AND conversation LIKE 'gone_%'"); n != 0 {
		t.Fatalf("removed network kept %d messages", n)
	}
	if n := count("SELECT count(*) FROM server_lines WHERE owner='alice' AND network='gone'"); n != 0 {
		t.Fatalf("removed network kept %d server lines", n)
	}
	if count("SELECT count(*) FROM messages WHERE owner='alice' AND conversation LIKE 'kept_%'") != 5 || count("SELECT count(*) FROM server_lines WHERE owner='alice' AND network='kept'") != 5 {
		t.Fatal("another network's rows were removed")
	}
}

// A deleting account's rows go in bounded transactions, and only its own rows.
func TestDeleteAccountRemovesRowsInChunks(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	account(t, s, "bob")
	saved := deleteChunkRows
	deleteChunkRows = 3
	t.Cleanup(func() { deleteChunkRows = saved })
	at := time.Now().UTC()
	for _, owner := range []string{"alice", "bob"} {
		for i := range 10 {
			msg := model.Message{ID: fmt.Sprintf("%s:%d", owner, i), ConversationID: "room", SenderID: "net:x", Kind: "message", Text: "hello", CreatedAt: at.Add(time.Duration(i) * time.Second)}
			if _, err = s.Commit(owner, "message.created", msg); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Before MarkDeleting, nothing is removed piecemeal.
	if err = s.clearDeletingAccount("alice"); err != nil {
		t.Fatal(err)
	}
	count := func(table, owner string) (n int) {
		t.Helper()
		if err := s.DB.QueryRow("SELECT count(*) FROM "+table+" WHERE owner=?", owner).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count("messages", "alice") != 10 {
		t.Fatal("rows removed from an account that is not being deleted")
	}
	if err = s.MarkDeleting("alice"); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteAccount("alice", false); err != nil {
		t.Fatal(err)
	}
	for _, table := range accountBulkTables {
		if n := count(table, "alice"); n != 0 {
			t.Fatalf("%s kept %d rows", table, n)
		}
	}
	if count("messages", "bob") != 10 || count("events", "bob") != 10 {
		t.Fatal("another account's rows were removed")
	}
	var n int
	if err = s.DB.QueryRow("SELECT count(*) FROM accounts WHERE id='alice'").Scan(&n); err != nil || n != 0 {
		t.Fatal("account row kept", err)
	}
}
