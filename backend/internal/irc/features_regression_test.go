package irc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
)

func TestWireBoundaryPreservesWhitespaceAndMultilineAssociation(t *testing.T) {
	port, peers := capabilityServer(t, "message-tags batch draft/multiline=max-bytes=4096,max-lines=100", "")
	m := New(Options{AllowPrivate: true})
	t.Cleanup(m.Close)
	if _, err := m.Connect("alice", NetworkConfig{ID: "wire", Name: "wire", Host: "127.0.0.1", Port: port, Nick: "boundary"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.WaitConnected(ctx, "alice", "wire"); err != nil {
		t.Fatal(err)
	}
	peer := <-peers
	conv, err := m.Join("alice", "wire", "#wire")
	if err != nil {
		t.Fatal(err)
	}
	peer.send(":boundary!u@h JOIN #wire")
	eventually(t, func() bool { return findConversation(t, m, "alice", "wire", "#wire").Joined })
	network := m.Snapshot("alice").Networks[0]
	c, _ := m.connection("alice", "wire")
	c.mu.Lock()
	threshold := c.client.MaxEventLength()
	c.mu.Unlock()
	budget := network.Features.MessageLineBytes - len("PRIVMSG #wire :")
	// Repeated spaces exercise girc's word-based splitter, not just its length
	// calculation. Check actual TCP output for single, CTCP and batch messages.
	for _, tc := range []struct{ id, kind, text string }{
		{"single", "message", "a  " + strings.Repeat("b", budget-3)},
		{"action", "action", "a  " + strings.Repeat("b", budget-12)},
		{"multiline", "message", "a  " + strings.Repeat("b", budget-3) + "\nlast"},
	} {
		if err = m.Send("alice", SendInput{ClientID: tc.id, ConversationID: conv.ID, Kind: tc.kind, Text: tc.text}); err != nil {
			t.Fatal(err)
		}
		var assembled, batch string
		lines := 0
		reading := true
		for reading {
			select {
			case event := <-peer.commands:
				switch event.Command {
				case "BATCH":
					if strings.HasPrefix(event.Params[0], "+") {
						batch = event.Params[0][1:]
					} else {
						reading = false
					}
				case "PRIVMSG":
					if event.LenOpts(false) >= threshold {
						t.Fatalf("wire line reached girc splitter: %d >= %d", event.LenOpts(false), threshold)
					}
					if batch != "" && event.Tags["batch"] != batch {
						t.Fatalf("lost multiline association: %q", event.String())
					}
					if lines > 0 {
						if _, concat := event.Tags["draft/multiline-concat"]; !concat {
							assembled += "\n"
						}
					}
					assembled += event.Last()
					lines++
					if batch == "" {
						reading = false
					}
				}
			case <-time.After(3 * time.Second):
				t.Fatal("no complete message reached the TCP peer")
			}
		}
		want := tc.text
		if tc.kind == "action" {
			want = "\x01ACTION " + want + "\x01"
		}
		if assembled != want {
			t.Fatalf("%s was rewritten: want %q, got %q", tc.id, want, assembled)
		}
	}
}

func TestMultilineRequiresMessageTagsAndSafeFallbackBudget(t *testing.T) {
	c := &connection{ready: true, caps: map[string]bool{"batch": true, "draft/multiline": true}, capValues: map[string]string{"draft/multiline": "max-bytes=4096"}}
	if c.features(nil).Multiline != nil {
		t.Fatal("multiline enabled without message-tags")
	}
	if budget := c.features(nil).MessageLineBytes; budget >= girc.DefaultMaxLineLength-girc.DefaultMaxPrefixLength {
		t.Fatal("fallback budget reaches girc splitter")
	}
	c.caps["message-tags"] = true
	if c.features(nil).Multiline == nil {
		t.Fatal("multiline did not become available after message-tags ACK")
	}
	delete(c.caps, "message-tags")
	if c.features(nil).Multiline != nil {
		t.Fatal("multiline survived loss of message-tags")
	}
}

func TestListModeQueriesDoNotRequireMutationPrivileges(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{})
	conv.MemberModes = map[string]string{}
	c.owner.conversations[conv.ID] = conv
	c.owner.networks[c.config.ID] = Network{ID: c.config.ID, ISupport: map[string]string{"CHANMODES": "bexI,k,l,imnt"}}
	for _, line := range []string{"MODE #test +b", "MODE #test +e", "MODE #test +I", "MODE #test +x", "MODE #test +bI"} {
		if err := m.Raw("alice", c.config.ID, line); err != nil {
			t.Fatalf("read-only query %q was blocked: %v", line, err)
		}
	}
	for _, line := range []string{"MODE #test +b *!*@*", "MODE #test -b *!*@*", "MODE #test +bx *!*@*", "MODE #test +bi", "MODE #test +i"} {
		if err := m.Raw("alice", c.config.ID, line); err == nil {
			t.Fatalf("unauthorized mutation %q was allowed", line)
		}
	}
}
