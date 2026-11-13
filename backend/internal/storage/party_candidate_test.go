package storage

import (
	"testing"
	"time"

	"ircord/backend/internal/model"
)

func TestPartyCandidateUsesOwnProfileAndRejectsUnavailableAccounts(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, username := range []string{"alice", "bob", "expired"} {
		if _, err := s.CreateAccount(username, username); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Commit("alice", "member.updated", model.Member{ID: "alice", DisplayName: "Alice Example", AvatarURL: "https://example.test/avatar"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit("bob", "member.updated", model.Member{ID: "alice", DisplayName: "Wrong workspace"}); err != nil {
		t.Fatal(err)
	}
	candidate, err := s.PartyCandidate("alice")
	if err != nil || candidate.DisplayName != "Alice Example" || candidate.AvatarURL != "https://example.test/avatar" {
		t.Fatalf("own profile lost: %+v %v", candidate, err)
	}
	if _, err = s.DB.Exec("UPDATE accounts SET last_seen_ns=? WHERE id='expired'", time.Now().Add(-AccountRetention-time.Hour).UnixNano()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PartyCandidate("expired"); err != ErrNotFound {
		t.Fatalf("expired account accepted: %v", err)
	}
	if _, err = s.DB.Exec("UPDATE accounts SET deleted=1 WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PartyCandidate("alice"); err != ErrNotFound {
		t.Fatalf("deleted account accepted: %v", err)
	}
}
