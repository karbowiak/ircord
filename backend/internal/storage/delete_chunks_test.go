package storage

import (
	"fmt"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

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
