package irc

import (
	"testing"

	"github.com/lrstanley/girc"
)

func TestChannelCommandsPreserveWirePrefix(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	w := m.space("owner")
	c := &connection{owner: w, config: NetworkConfig{ID: "network"}, ctx: m.ctx, ready: true,
		queue: make(chan []*girc.Event, 8), desired: map[string]string{}, closedTargets: map[string]bool{}}
	w.connections["network"] = c
	w.configs["network"] = c.config
	for _, target := range []string{"#general", "&local", "+modeless", "!ABCDEsafe", "#&local"} {
		t.Run(target, func(t *testing.T) {
			conv, err := m.JoinKey("owner", "network", target, "secret")
			if err != nil {
				t.Fatal(err)
			}
			join := (<-c.queue)[0]
			if join.Command != "JOIN" || len(join.Params) != 2 || join.Params[0] != target || join.Params[1] != "secret" {
				t.Fatalf("wrong JOIN target: %#v", join)
			}
			if conv.Target != target || c.target(conv) != target || conv.Name != target[1:] {
				t.Fatalf("display name changed the IRC target: %#v", conv)
			}
			if err := m.PartReason("owner", conv.ID, "bye"); err != nil {
				t.Fatal(err)
			}
			part := (<-c.queue)[0]
			if part.Command != "PART" || part.Params[0] != target {
				t.Fatalf("wrong PART target: %#v", part)
			}
		})
	}
	conv, err := m.Join("owner", "network", "bare")
	if err != nil || conv.Target != "#bare" {
		t.Fatalf("bare channel: %#v %v", conv, err)
	}
}
