package model

import "testing"

func TestPeopleIdentityMappingAndRanking(t *testing.T) {
	for _, tc := range []struct{ mapping, input, want string }{
		{"ascii", "Nick[\\^", "nick[\\^"},
		{"rfc1459", "Nick[\\^", "nick{|~"},
		{"strict-rfc1459", "Nick[\\^", "nick{|^"},
	} {
		if got := FoldIdentity(tc.input, tc.mapping); got != tc.want {
			t.Fatalf("%s fold: %q", tc.mapping, got)
		}
	}
	for _, tc := range []struct {
		display, nick string
		want          int
	}{
		{"Different", "peter", 0}, {"Peter", "bob", 1}, {"Different", "PeterPowers", 2},
		{"Peter Jensen", "bob", 3}, {"Different", "ppetermann", 4}, {"Different", "bob", 99},
	} {
		if got := PersonMatchRank("PeTeR", "ascii", tc.display, tc.nick); got != tc.want {
			t.Fatalf("rank %q / %q: %d", tc.display, tc.nick, got)
		}
	}
}
