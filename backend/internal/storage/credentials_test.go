package storage

import (
	"errors"
	"testing"
	"time"
)

func TestVerifiedSessionCannotUseRevokedPasswordVersion(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.CreateUser("owner", "alice", "old-hash"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword("owner", "new-hash", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateVerifiedSession("stale", "owner", []byte("token1"), time.Now().Add(time.Hour), "old-hash", ""); !errors.Is(err, ErrNotFound) {
		t.Fatal("stale password created session", err)
	}
	if err := s.CreateVerifiedSession("fresh", "owner", []byte("token2"), time.Now().Add(time.Hour), "new-hash", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCredentialOperation("alice", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateVerifiedSession("pending", "owner", []byte("token3"), time.Now().Add(time.Hour), "new-hash", ""); !errors.Is(err, ErrNotFound) {
		t.Fatal("pending change allowed login", err)
	}
}
