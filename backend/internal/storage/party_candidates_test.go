package storage

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

func TestPartyCandidatesMatchUsernamesAndDisplayNames(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, fixture := range []struct{ id, username, name string }{
		{"username-exact", "peter", "A Person"},
		{"name-exact", "zebra", "Peter"},
		{"username-prefix", "petersen", "Another Person"},
		{"name-prefix", "aardvark", "Peter Jensen"},
		{"substring", "a-peter", "Some Person"},
		{"unrelated", "nobody", "Someone Else"},
		{"placeholder", "placeholder", "ircord"},
		{"unicode", "unicode", "Søren Petersen"},
		{"deleted", "peter-deleted", "Peter"},
		{"expired", "peter-expired", "Peter"},
	} {
		if _, err = s.CreateAccount(fixture.id, fixture.username); err != nil {
			t.Fatal(err)
		}
		if _, err = s.Commit(fixture.id, "member.updated", model.Member{ID: fixture.id, DisplayName: fixture.name, AvatarURL: "https://example.com/avatar"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.DB.Exec("UPDATE accounts SET deleted=1 WHERE id='deleted'"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("UPDATE accounts SET last_seen_ns=? WHERE id='expired'", time.Now().Add(-AccountRetention-time.Hour).UnixNano()); err != nil {
		t.Fatal(err)
	}
	items, err := s.PartyCandidates(" PeTeR ")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.AccountID
		if item.AvatarURL != "https://example.com/avatar" {
			t.Fatal("candidate profile was lost")
		}
	}
	want := []string{"username-exact", "name-exact", "username-prefix", "name-prefix", "substring", "unicode"}
	if !slices.Equal(ids, want) {
		t.Fatalf("ordered candidates = %v, want %v", ids, want)
	}
	for search, id := range map[string]string{"søren": "unicode", "placeholder": "placeholder"} {
		items, err = s.PartyCandidates(search)
		if err != nil || len(items) != 1 || items[0].AccountID != id {
			t.Fatalf("search %q = %+v, %v", search, items, err)
		}
		one, err := s.PartyCandidate(id)
		if err != nil || one != items[0] {
			t.Fatalf("search profile differs from candidate profile: %+v, %v", one, err)
		}
	}
	items, err = s.PartyCandidates("ircord")
	if err != nil || len(items) != 0 {
		t.Fatalf("placeholder profile matched as a display name: %+v, %v", items, err)
	}
}

func TestPartyCandidatesBoundFocusSuggestionsAndUseOwnProfile(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := range 55 {
		id := fmt.Sprintf("person-%02d", i)
		if _, err = s.CreateAccount(id, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.Commit("person-00", "member.updated", model.Member{ID: "person-01", DisplayName: "Other workspace name"}); err != nil {
		t.Fatal(err)
	}
	// Missing or malformed optional profiles still use the account username.
	if _, err = s.DB.Exec("UPDATE entities SET payload='invalid' WHERE owner='person-02' AND id='person-02'"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("DELETE FROM entities WHERE owner='person-03' AND id='person-03'"); err != nil {
		t.Fatal(err)
	}
	for _, search := range []string{"", "  ", "person"} {
		items, err := s.PartyCandidates(search)
		if err != nil || len(items) != 50 || items[0].AccountID != "person-00" || items[49].AccountID != "person-49" {
			t.Fatalf("bounded focus suggestions: count=%d, err=%v", len(items), err)
		}
		for _, item := range items {
			if item.DisplayName != item.Username {
				t.Fatalf("unexpected profile: %+v", item)
			}
		}
	}
	items, err := s.PartyCandidates("Other workspace")
	if err != nil || len(items) != 0 {
		t.Fatalf("searched another account's cached profile: %+v, %v", items, err)
	}
}
