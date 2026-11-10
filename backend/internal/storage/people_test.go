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

func TestNetworkPeopleUseOnlyTheMatchingServerAccountProfile(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.CreateAccount("local", "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit("local", "profile.updated", model.Member{ID: "local", DisplayName: "Legacy private", AvatarURL: "https://legacy.invalid/avatar"}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []model.Network{
		{ID: "home", Host: "home.invalid", Port: 6697, TLS: true, Auth: model.NetworkAuth{Method: "sasl-plain", Account: "alice"}, Profile: &model.Member{ID: "local", DisplayName: "Home visible", AvatarURL: "https://home.invalid/avatar"}},
		{ID: "other", Host: "other.invalid", Port: 6697, TLS: true, Auth: model.NetworkAuth{Method: "sasl-plain", Account: "alice"}, Profile: &model.Member{ID: "local", DisplayName: "Other private", AvatarURL: "https://other.invalid/avatar"}},
		{ID: "foreign", Host: "foreign.invalid", Port: 6697, TLS: true, Auth: model.NetworkAuth{Method: "sasl-plain", Account: "bob"}, Profile: &model.Member{ID: "local", DisplayName: "Foreign private"}},
	} {
		if _, err := s.Commit("local", "network.updated", n); err != nil {
			t.Fatal(err)
		}
	}
	home := model.Network{Host: "home.invalid", Port: 6697, TLS: true}
	ctx := context.Background()
	people, _, err := s.LocalPeople(ctx, "Home visible", 10, home)
	if err != nil || len(people) != 1 || people[0].DisplayName != "Home visible" || people[0].AvatarURL != "https://home.invalid/avatar" {
		t.Fatal("server directory lost its own profile", people, err)
	}
	for _, query := range []string{"Legacy private", "Other private", "Foreign private"} {
		people, _, err := s.LocalPeople(ctx, query, 10, home)
		if err != nil || len(people) != 0 {
			t.Fatal("server directory searched another profile", query, people, err)
		}
	}
	for _, n := range []model.Network{{Host: "foreign.invalid", Port: 6697, TLS: true}, {Host: "missing.invalid", Port: 6697, TLS: true}, {Host: "home.invalid", Port: 6697, TLS: false}} {
		p, err := s.LocalPersonByUsername(ctx, "alice", n)
		if err != nil || p.DisplayName != "alice" || p.AvatarURL != "" {
			t.Fatal("missing server identity fell back to another profile", p, err)
		}
	}
}
