package storage

import (
	"testing"
	"time"

	"ircord/backend/internal/model"
)

func TestReplayWithFullyExpiredHistoryRequiresSnapshot(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.CreateAccount("alice", "key"); err != nil {
		t.Fatal(err)
	}
	event, err := s.Commit("alice", "profile.updated", model.Member{ID: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.PruneEvents(time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, valid, err := s.Replay("alice", 0, 4096); err != nil || valid {
		t.Fatalf("expired cursor accepted: %v", err)
	}
	if events, valid, err := s.Replay("alice", event.Seq, 4096); err != nil || !valid || len(events) != 0 {
		t.Fatalf("current cursor rejected: %v", err)
	}
}
