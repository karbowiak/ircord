package storage

import (
	"context"
	"errors"
	"testing"

	"ircord/backend/internal/model"
)

func TestNeutralPeopleMatchingAndExactUsernameBoundary(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, tc := range []struct{ id, username, display string }{
		{"local-peter", "peter", "Different"}, {"local-profile", "bob", "Peter Jensen"},
		{"local-prefix", "PeterPowers", "Different"}, {"local-substring", "ppetermann", "Different"},
	} {
		if _, err = s.CreateAccount(tc.id, tc.username); err != nil {
			t.Fatal(err)
		}
		if _, err = s.Commit(tc.id, "member.updated", model.Member{ID: tc.id, DisplayName: tc.display}); err != nil {
			t.Fatal(err)
		}
	}
	items, more, err := s.LocalPeople(context.Background(), "peter", 3)
	if err != nil || !more || len(items) != 3 || items[0].Username != "peter" || items[1].Username != "PeterPowers" || items[2].Username != "bob" {
		t.Fatalf("ranked neutral matches: %+v, %v, %v", items, more, err)
	}
	if _, err = s.LocalPersonByUsername(context.Background(), "local-peter"); !errors.Is(err, ErrNotFound) {
		t.Fatal("a local account ID was accepted as a username", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = s.LocalPeople(ctx, "", 10); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled storage lookup continued", err)
	}
}
