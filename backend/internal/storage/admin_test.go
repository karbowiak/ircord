package storage

import (
	"context"
	"testing"
	"time"
)

func TestAdminOverviewExcludesExpiredPrincipals(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"active", "expired"} {
		if _, err = s.CreateAccount(id, id+"-hash"); err != nil {
			t.Fatal(err)
		}
		if err = s.CreateSession(id+"-session", id, []byte(id), time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.DB.Exec("UPDATE accounts SET last_seen_ns=? WHERE id='expired'", time.Now().Add(-AccountRetention-time.Hour).UnixNano()); err != nil {
		t.Fatal(err)
	}
	if err = s.CreateSession("old-session", "active", []byte("old"), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = s.CreateSession("revoked-session", "active", []byte("revoked"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = s.RevokeSession("active", "revoked-session"); err != nil {
		t.Fatal(err)
	}
	overview, err := s.AdminOverview(context.Background())
	if err != nil || overview.Accounts != 1 || overview.Sessions != 1 || overview.DatabaseBytes <= 0 {
		t.Fatal("incorrect aggregate overview", overview, err)
	}
}
