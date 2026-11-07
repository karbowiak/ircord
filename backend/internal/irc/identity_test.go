package irc

import (
	"strings"
	"testing"
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
