package irc

import (
	"context"
	"strings"
	"testing"

	"github.com/lrstanley/girc"
)

func TestNicknameValidationAcrossConnectionAndCommands(t *testing.T) {
	for _, nick := range []string{"Alice", "alice-123", "[alice]", "_alice", "`alice", strings.Repeat("a", 32)} {
		if err := validateNickname(nick, 32); err != nil {
			t.Fatal(nick, err)
		}
	}
	for _, nick := range []string{"", "Alice Smith", " Alice", "alice\n", "9alice", "-alice", "alice@example", "alice😀", "?alice", strings.Repeat("a", 33)} {
		if _, err := normalizeConfig(NetworkConfig{Nick: nick}); err == nil {
			t.Fatalf("connection accepted invalid nickname %q", nick)
		}
		if err := validateNickname(nick, 32); err == nil {
			t.Fatalf("identity endpoint accepted invalid nickname %q", nick)
		}
	}
	for _, line := range []string{"NICK Alice Smith", "NICK :Alice Smith", "NICK alice@example", "NICK 9alice"} {
		if _, err := rawEvent(line); err == nil {
			t.Fatalf("raw/slash command accepted invalid nickname: %s", line)
		}
	}
	if _, err := normalizeConfig(NetworkConfig{Nick: "Alice", AltNick: "Alice Smith"}); err == nil {
		t.Fatal("invalid alternate nickname accepted")
	}
}

func TestRealnameAndProfileNamePrecedence(t *testing.T) {
	member := Member{Nick: "alice", DisplayName: "alice", NameSource: "nick"}
	applyRealname(&member, "Alice Smith")
	if member.DisplayName != "Alice Smith" || member.NameSource != "realname" {
		t.Fatal(member)
	}
	applyRealname(&member, "Alice Jones")
	if member.DisplayName != "Alice Jones" {
		t.Fatal("realname update was lost", member)
	}
	member.Nick = "alice_new"
	resolveMemberName(&member)
	if member.DisplayName != "Alice Jones" {
		t.Fatal("nickname overwrote realname", member)
	}
	member.DisplayName = "Custom profile"
	member.NameSource = "profile"
	applyRealname(&member, "Another real name")
	if member.DisplayName != "Custom profile" {
		t.Fatal("realname overwrote profile", member)
	}
	member.NameSource = "nick"
	resolveMemberName(&member)
	if member.DisplayName != "Another real name" {
		t.Fatal("clearing profile lost realname fallback", member)
	}
	applyRealname(&member, "IRCord")
	if member.DisplayName != "alice_new" {
		t.Fatal("placeholder did not fall back to nickname", member)
	}
}

func profileName(name string) *string { return &name }

func TestProfileNameStartsUnsetAndIsOnlyExplicit(t *testing.T) {
	m := New(Options{AllowPrivate: true})
	defer m.Close()
	user := m.Snapshot("alice").User
	if user.DisplayName == "You" || user.NameSource == "profile" {
		t.Fatal("fresh account manufactured a profile name", user)
	}
	// Saving other fields without a name keeps it unset.
	if user, _ = m.SetProfile("alice", Profile{Bio: "about"}); user.NameSource == "profile" || user.Bio != "about" {
		t.Fatal("unrelated save set a profile name", user)
	}
	if user, _ = m.SetProfile("alice", Profile{DisplayName: profileName("  Alice  ")}); user.DisplayName != "Alice" || user.NameSource != "profile" {
		t.Fatal("explicit name not saved", user)
	}
	// Omitting the name keeps the explicit one.
	if user, _ = m.SetProfile("alice", Profile{Bio: "about"}); user.DisplayName != "Alice" || user.NameSource != "profile" {
		t.Fatal("omitted name cleared the profile", user)
	}
	if user, _ = m.SetProfile("alice", Profile{DisplayName: profileName("")}); user.NameSource == "profile" || user.DisplayName != user.Nick {
		t.Fatal("clearing did not restore the nickname fallback", user)
	}
	if _, err := m.SetProfile("alice", Profile{DisplayName: profileName("bad\nname")}); err == nil {
		t.Fatal("invalid name accepted")
	}
}

func TestStoredSelfNamesWithoutSource(t *testing.T) {
	for _, tc := range []struct{ in, want, source string }{
		{"You", "alice", "nick"},
		{"", "alice", "nick"},
		{"alice", "alice", "nick"},
		{"Alice Smith", "Alice Smith", "profile"},
	} {
		v := Member{Nick: "alice", DisplayName: tc.in}
		normalizeSelfName(&v)
		if v.DisplayName != tc.want || v.NameSource != tc.source {
			t.Fatal(tc.in, v)
		}
	}
	v := Member{Nick: "alice", DisplayName: "You", NameSource: "profile"}
	normalizeSelfName(&v)
	if v.DisplayName != "You" {
		t.Fatal("an explicit name with a source was rewritten at load", v)
	}
}

func TestPushProfileSharesOnlyExplicitNames(t *testing.T) {
	c := &connection{ctx: context.Background(), ready: true, queue: make(chan []*girc.Event, 8), caps: map[string]bool{"draft/metadata-3": true}}
	shared := func(p Member) []string {
		c.pushProfile(p)
		for {
			events := <-c.queue
			if events[0].Params[2] == "display-name" {
				for len(c.queue) > 0 {
					<-c.queue
				}
				return events[0].Params
			}
		}
	}
	if params := shared(Member{Nick: "alice", DisplayName: "alice", NameSource: "nick"}); len(params) != 3 {
		t.Fatal("nickname fallback shared as a profile name", params)
	}
	if params := shared(Member{Nick: "alice", DisplayName: "Alice", NameSource: "profile"}); len(params) != 4 || params[3] != "Alice" {
		t.Fatal("explicit name not shared", params)
	}
}

func TestPeerClearsDisplayName(t *testing.T) {
	m := New(Options{AllowPrivate: true})
	defer m.Close()
	w := m.space("alice")
	c := &connection{owner: w, config: NetworkConfig{ID: "net"}, nicks: map[string]string{}}
	event := func(line string) girc.Event { return *girc.ParseEvent(line) }
	c.metadata(event(":irc.test METADATA bob display-name * :Bobby"))
	c.metadata(event(":irc.test METADATA bob display-name *"))
	found := false
	for _, v := range m.Snapshot("alice").Members {
		if v.Nick == "bob" {
			found = true
			if v.DisplayName != "bob" || v.NameSource != "nick" {
				t.Fatal("cleared peer profile name was kept", v)
			}
		}
	}
	if !found {
		t.Fatal("peer metadata was ignored")
	}
}
