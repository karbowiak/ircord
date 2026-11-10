package irc

import (
	"errors"
	"testing"
)

func TestPeopleDirectTargetsSeparateNetworkMemberIDsAndExplicitNicknames(t *testing.T) {
	m, w := peopleFixture(t)
	w.connections["one"] = &connection{manager: m, owner: w, config: NetworkConfig{ID: "one", Nick: "Caller"}, nicks: map[string]string{}, ctx: m.ctx}
	w.members["Collision"] = Member{ID: "Collision", Nick: "WrongNetworkTarget", DisplayName: "Other network person"}
	w.conversations["secret"] = Conversation{ID: "secret", NetworkID: "two", MemberIDs: []string{"Collision", "two:secret"}}
	for _, memberID := range []string{"Collision", "two:secret"} {
		if _, err := m.DirectPerson("caller", "one", memberID, ""); !errors.Is(err, ErrNotFound) {
			t.Fatal("foreign member ID became a nickname or crossed networks", memberID, err)
		}
	}
	conv, err := m.DirectPerson("caller", "one", "", "Collision")
	if err != nil || conv.NetworkID != "one" || conv.Name != "Collision" {
		t.Fatal("explicit nickname was confused with a foreign member ID", conv, err)
	}
	conv, err = m.DirectPerson("caller", "one", "one:a", "")
	if err != nil || conv.Name != "PeterPowers" || conv.NetworkID != "one" {
		t.Fatal("selected network member did not resolve", conv, err)
	}
	conv, err = m.DirectPerson("caller", "one", "FreshLegacyNick", "")
	if err != nil || conv.Name != "FreshLegacyNick" {
		t.Fatal("unambiguous legacy nickname stopped working", conv, err)
	}
	for _, input := range []struct{ member, nickname string }{{"one:a", "ExplicitNick"}, {"", "bad nickname"}, {"", ""}} {
		if _, err = m.DirectPerson("caller", "one", input.member, input.nickname); err == nil {
			t.Fatal("invalid or ambiguous direct target was accepted", input)
		}
	}
	if _, err = m.DirectPerson("different-caller", "one", "", "Collision"); !errors.Is(err, ErrNotFound) {
		t.Fatal("direct target crossed account scope", err)
	}
}
