package storage

import (
	"errors"
	"testing"
	"time"
)

func deviceStore(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRevokeLiveSessionSucceedsOnce(t *testing.T) {
	s := deviceStore(t, t.TempDir())
	defer s.Close()
	if _, err := s.CreateUser("owner", "owner", "hash"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateVerifiedSession("live", "owner", []byte("t1"), time.Now().Add(time.Hour), "hash", "ua"); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeLiveSession("owner", "live"); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeLiveSession("owner", "live"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second revoke of the same session: %v", err)
	}
	if err := s.CreateVerifiedSession("expired", "owner", []byte("t2"), time.Now().Add(-time.Second), "hash", "ua"); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeLiveSession("owner", "expired"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoke of an expired session: %v", err)
	}
	if err := s.RevokeLiveSession("someone-else", "expired"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoke of another account's session: %v", err)
	}
}

// Revoking and removing the session's push subscriptions happen together or not at all.
func TestRevokeAndPushCleanupAreAtomic(t *testing.T) {
	s := deviceStore(t, t.TempDir())
	defer s.Close()
	if _, err := s.CreateUser("owner", "owner", "hash"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"phone", "laptop"} {
		if err := s.CreateVerifiedSession(id, "owner", []byte(id), time.Now().Add(time.Hour), "hash", "ua"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(`CREATE TABLE push_subscriptions(id TEXT PRIMARY KEY,owner TEXT NOT NULL,session TEXT NOT NULL,device TEXT NOT NULL,payload BLOB NOT NULL,created_ns INTEGER NOT NULL);
INSERT INTO push_subscriptions VALUES('p1','owner','phone','',x'',0),('p2','owner','laptop','',x'',0);
CREATE TRIGGER no_delete BEFORE DELETE ON push_subscriptions BEGIN SELECT RAISE(ABORT,'disk trouble'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeLiveSession("owner", "phone"); err == nil {
		t.Fatal("revoke reported success although its push cleanup failed")
	}
	if !s.SessionActive("owner", "phone") {
		t.Fatal("a failed revoke left the session half signed out")
	}
	if _, err := s.DB.Exec("DROP TRIGGER no_delete"); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeLiveSession("owner", "phone"); err != nil {
		t.Fatal(err)
	}
	var phone, laptop int
	s.DB.QueryRow("SELECT count(*) FROM push_subscriptions WHERE session='phone'").Scan(&phone)
	s.DB.QueryRow("SELECT count(*) FROM push_subscriptions WHERE session='laptop'").Scan(&laptop)
	if phone != 0 || laptop != 1 {
		t.Fatalf("push rows after revoke: phone %d laptop %d", phone, laptop)
	}
	// A password change revokes every other session and their push rows in one go.
	if err := s.SetPassword("owner", "hash2", "laptop"); err != nil {
		t.Fatal(err)
	}
	s.DB.QueryRow("SELECT count(*) FROM push_subscriptions WHERE session='laptop'").Scan(&laptop)
	if laptop != 1 {
		t.Fatal("password change removed the kept session's push subscription")
	}
}

func TestDeviceMetadataSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	s := deviceStore(t, dir)
	if _, err := s.CreateUser("owner", "owner", "hash"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateVerifiedSession("phone", "owner", []byte("t"), time.Now().Add(time.Hour), "hash", "Mozilla/5.0 (iPhone)"); err != nil {
		t.Fatal(err)
	}
	s.DB.Exec("UPDATE sessions SET last_seen_ns=? WHERE id='phone'", time.Now().Add(-2*time.Minute).UnixNano())
	if err := s.TouchSession("phone"); err != nil {
		t.Fatal(err)
	}
	before, err := s.DeviceSessions("owner")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = deviceStore(t, dir)
	defer s.Close()
	after, err := s.DeviceSessions("owner")
	if err != nil || len(after) != 1 {
		t.Fatal(after, err)
	}
	if after[0].UserAgent != "Mozilla/5.0 (iPhone)" || !after[0].LastSeenAt.Equal(before[0].LastSeenAt) || time.Since(after[0].LastSeenAt) > time.Minute {
		t.Fatalf("metadata changed across restart: %+v -> %+v", before[0], after[0])
	}
}
