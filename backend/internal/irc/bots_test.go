package irc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

// botFixture is a connection to a network that advertises a bot mode.
func botFixture(t *testing.T, isupport map[string]string) *connection {
	t.Helper()
	m := New(Options{})
	t.Cleanup(m.Close)
	w := m.space("alice")
	cfg := NetworkConfig{ID: "net", Name: "net", Host: "irc.invalid", Port: 6667, Nick: "Me"}
	c := &connection{owner: w, manager: m, config: cfg, ctx: m.ctx, ready: true, caps: map[string]bool{"message-tags": true}, queue: make(chan []*girc.Event, 8), nicks: map[string]string{}, historyBatches: map[string]bool{}, batches: map[string]*multiline{}}
	w.mu.Lock()
	w.connections[cfg.ID] = c
	w.configs[cfg.ID] = cfg
	w.networks[cfg.ID] = Network{ID: cfg.ID, ISupport: isupport}
	w.mu.Unlock()
	return c
}

func (c *connection) observeLines(t *testing.T, lines ...string) {
	t.Helper()
	for _, line := range lines {
		e := girc.ParseEvent(line)
		if e == nil {
			t.Fatal("bad line", line)
		}
		c.observeBot(*e)
	}
}

func roleOf(c *connection, nick string) string {
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	id := c.nicks[c.fold(nick)]
	if id == "" {
		return ""
	}
	return w.members[id].Role
}

func TestBotStatusFollowsTheBotTagOfLiveMessages(t *testing.T) {
	c := botFixture(t, map[string]string{"BOT": "B", "WHOX": ""})
	c.observeLines(t, "@bot :Robo!r@host PRIVMSG #test :beep")
	if got := roleOf(c, "Robo"); got != "bot" {
		t.Fatalf("a tagged message did not mark its sender: %q", got)
	}
	// History describes the sender as they were then.
	c.historyBatches["old"] = true
	c.observeLines(t, "@batch=old :Robo!r@host PRIVMSG #test :from before")
	if got := roleOf(c, "Robo"); got != "bot" {
		t.Fatalf("a replayed message changed the mark: %q", got)
	}
	c.observeLines(t, ":Robo!r@host PRIVMSG #test :no longer a bot")
	if got := roleOf(c, "Robo"); got != "member" {
		t.Fatalf("an untagged live message kept the mark: %q", got)
	}
	c.observeLines(t, "@draft/bot :Other!o@host JOIN #test")
	if got := roleOf(c, "Other"); got != "bot" {
		t.Fatalf("a JOIN with the draft tag did not mark the member: %q", got)
	}
	// Server notices are not people.
	c.observeLines(t, "@bot :irc.example.test NOTICE Me :hello")
	if id := c.nicks[c.fold("irc.example.test")]; id != "" {
		t.Fatal("a server notice created a member")
	}
}

// openBatch registers a batch the way batch() does when its opening line arrives.
func (c *connection) openBatch(ref, kind, parent string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if kind == "chathistory" {
		c.historyBatches[ref] = true
	} else {
		c.batches[ref] = &multiline{historyRef: parent, historical: c.historyBatches[parent]}
	}
}

// Ergo puts the bot tag only on the opening BATCH line of a multiline message;
// the lines inside carry just the batch reference (seen on Ergo v2.19.1).
func TestMultilineMessagesFromABotKeepItsMark(t *testing.T) {
	c := botFixture(t, map[string]string{"BOT": "B", "WHOX": ""})
	c.observeLines(t, "@bot :Other!o@host PRIVMSG #test :the server sends the tag")
	c.observeLines(t, "@time=2026-11-15T00:00:00Z;msgid=m1;bot :Robo!~u@host BATCH +ml1 draft/multiline :#test")
	if roleOf(c, "Robo") != "bot" {
		t.Fatal("the tagged BATCH line did not mark its sender")
	}
	c.openBatch("ml1", "draft/multiline", "")
	c.observeLines(t,
		"@batch=ml1 :Robo!~u@host PRIVMSG #test :line one",
		"@batch=ml1 :Robo!~u@host PRIVMSG #test :line two",
	)
	if got := roleOf(c, "Robo"); got != "bot" {
		t.Fatalf("a live multiline message from a bot cleared its mark: %q", got)
	}
	// Replayed history describes the sender as they were then, multiline or not.
	c.openBatch("hist", "chathistory", "")
	c.observeLines(t, "@batch=hist :Robo!~u@host BATCH +ml2 draft/multiline :#test")
	c.openBatch("ml2", "draft/multiline", "hist")
	c.observeLines(t, "@batch=ml2 :Robo!~u@host PRIVMSG #test :old line")
	if got := roleOf(c, "Robo"); got != "bot" {
		t.Fatalf("a replayed multiline message changed the mark: %q", got)
	}
	// A live multiline message without the tag clears it.
	c.observeLines(t, ":Robo!~u@host BATCH +ml3 draft/multiline :#test")
	if got := roleOf(c, "Robo"); got != "member" {
		t.Fatalf("an untagged multiline message kept the mark: %q", got)
	}
}

func TestUntaggedMessagesKeepWhoMarksUntilTheServerSendsTheTag(t *testing.T) {
	c := botFixture(t, map[string]string{"BOT": "B"})
	c.observeLines(t, ":srv 352 Me #test helper h srv Helper HB :0 Helper", ":Helper!h@host PRIVMSG #test :hi")
	if roleOf(c, "Helper") != "bot" {
		t.Fatal("a server that never sent the bot tag cleared a WHO mark")
	}
	c.observeLines(t, "@bot :Other!o@host PRIVMSG #test :beep", ":Helper!h@host PRIVMSG #test :hi again")
	if roleOf(c, "Helper") != "member" {
		t.Fatal("an untagged message kept the mark on a server that sends the tag")
	}
}

func TestBotStatusFollowsWhoFlagsAndWhois(t *testing.T) {
	c := botFixture(t, map[string]string{"BOT": "B", "WHOX": ""})
	c.observeLines(t,
		":srv 352 Me #test helper h srv Helper H@B :0 Helper Bot",
		":srv 352 Me #test person h srv Person H+ :0 A Person",
	)
	if roleOf(c, "Helper") != "bot" || roleOf(c, "Person") != "member" {
		t.Fatalf("352 flags: Helper %q, Person %q", roleOf(c, "Helper"), roleOf(c, "Person"))
	}
	// girc's own WHOX (token 1) carries no flags and is ignored.
	c.observeLines(t, ":srv 354 Me 1 #test helper h Helper 0 :Helper Bot")
	if roleOf(c, "Helper") != "bot" {
		t.Fatal("a WHOX reply without flags cleared the mark")
	}
	if c.nicks[c.fold("#test")] != "" {
		t.Fatal("girc's WHOX reply was read as a bot-flags reply")
	}
	c.observeLines(t, ":srv 354 Me "+botWhoToken+" Helper H")
	if roleOf(c, "Helper") != "member" {
		t.Fatal("WHOX flags without B kept the mark")
	}
	c.observeLines(t, ":srv 335 Me Person :is a Bot on Example")
	if roleOf(c, "Person") != "bot" {
		t.Fatal("RPL_WHOISBOT did not mark the member")
	}
}

func TestBotStatusNeedsTheNetworksBotMode(t *testing.T) {
	c := botFixture(t, map[string]string{"WHOX": ""})
	c.observeLines(t, "@bot :Robo!r@host PRIVMSG #test :beep", ":srv 352 Me #test r h srv Helper HB :0 x")
	if roleOf(c, "Robo") == "bot" || roleOf(c, "Helper") == "bot" {
		t.Fatal("a network without BOT in ISUPPORT marked a bot")
	}
	c.requestBotFlags("#test")
	if len(c.queue) != 0 {
		t.Fatal("asked for bot flags on a network without a bot mode")
	}
}

func TestJoiningAChannelAsksForBotFlags(t *testing.T) {
	c := botFixture(t, map[string]string{"BOT": "B", "WHOX": ""})
	c.requestBotFlags("#test")
	sent := <-c.queue
	if got := sent[0].Command + " " + strings.Join(sent[0].Params, " "); got != "WHO #test %tnf,"+botWhoToken {
		t.Fatalf("sent %q", got)
	}
	// One request at a time: further channels wait for the outstanding one to end.
	c.requestBotFlags("#two")
	c.requestBotFlags("#test")
	if len(c.queue) != 0 {
		t.Fatal("sent a second bot-flags WHO while one was outstanding")
	}
	// girc's own WHO of the same channel ends first; that does not count.
	c.observeLines(t, ":srv 315 Me #test :End of WHO list")
	if len(c.queue) != 0 {
		t.Fatal("girc's end of WHO released the next request")
	}
	c.observeLines(t, ":srv 354 Me "+botWhoToken+" Me H", ":srv 315 Me #test :End of WHO list")
	sent = <-c.queue
	if got := strings.Join(sent[0].Params, " "); got != "#two %tnf,"+botWhoToken {
		t.Fatalf("next request %q", got)
	}
	// While the send queue is more than half full, requests wait for room.
	busy := botFixture(t, map[string]string{"BOT": "B", "WHOX": ""})
	for range 5 {
		busy.queue <- nil
	}
	busy.requestBotFlags("#test")
	if len(busy.queue) != 5 {
		t.Fatal("a bot-flags WHO joined a busy send queue")
	}
	for range 5 {
		<-busy.queue
	}
	busy.observeLines(t, ":srv 315 Me #other :End of WHO list")
	if len(busy.queue) != 1 {
		t.Fatal("the waiting request was not sent once the queue had room")
	}
	// Without WHOX the server answers girc's WHO with 352 lines, which have the flags.
	plain := botFixture(t, map[string]string{"BOT": "B"})
	plain.requestBotFlags("#test")
	if len(plain.queue) != 0 {
		t.Fatal("sent WHOX to a network without it")
	}
}

func TestServerOwnerMarksABotThroughTheModerationLogin(t *testing.T) {
	m, store, c, _, server := ownerFixture(t, "right-password", func(_ string, params []string) string {
		if params[0] == "Gone" {
			return ":fake 401 ircordsvc Gone :No such nick"
		}
		return ":fake MODE " + params[0] + " " + params[1]
	})
	c.owner.mu.Lock()
	c.owner.networks["network"] = Network{ID: "network", ISupport: map[string]string{"BOT": "B"}}
	c.owner.members["gone"] = Member{ID: "gone", Nick: "Gone"}
	c.nicks["gone"] = "gone"
	// Known in this workspace, but from another network.
	c.owner.members["elsewhere:x"] = Member{ID: "elsewhere:x", Nick: "Stranger"}
	c.owner.mu.Unlock()
	if _, err := m.SetBot(context.Background(), "alice", "network", "elsewhere:x", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("marked someone from another network: %v", err)
	}
	// Another account on the associated server that has seen Peer.
	bob := m.space("bob")
	bob.mu.Lock()
	bc := &connection{owner: bob, manager: m, config: c.config, ctx: m.ctx, ready: true, nicks: map[string]string{"peer": "network:p"}, queue: make(chan []*girc.Event, 8)}
	bob.connections["network"] = bc
	bob.members["network:p"] = Member{ID: "network:p", Nick: "Peer", Role: "member"}
	bob.mu.Unlock()

	mark, err := m.SetBot(context.Background(), "alice", "network", "peer", true)
	if err != nil {
		t.Fatal(err)
	}
	if mark.Member.Role != "bot" || mark.Nick != "Peer" || mark.Account != "peeracct" || roleOf(c, "Peer") != "bot" {
		t.Fatalf("mark: %+v, stored %q", mark, roleOf(c, "Peer"))
	}
	if roleOf(bc, "Peer") != "bot" {
		t.Fatal("another account on the associated server did not see the mark")
	}
	if !strings.Contains(strings.Join(server.received(), "\n"), "SAMODE Peer +B") {
		t.Fatalf("the moderation login did not send SAMODE:\n%s", strings.Join(server.received(), "\n"))
	}
	if len(c.queue) != 0 {
		t.Fatal("the owner's own connection sent the mode change")
	}
	if mark, err = m.SetBot(context.Background(), "alice", "network", "peer", false); err != nil || mark.Member.Role != "member" {
		t.Fatalf("unmark: %v %+v", err, mark)
	}
	if _, err = m.SetBot(context.Background(), "alice", "network", "gone", true); !errors.Is(err, ErrBotRefused) || !strings.Contains(err.Error(), "No such nick") {
		t.Fatalf("refusal: %v", err)
	}
	// Your own network member resolves on this network, and is still refused.
	c.config.Nick = "Me"
	c.owner.mu.Lock()
	self := c.owner.members[c.owner.self.ID]
	self.Nick = "Me"
	c.owner.members[self.ID] = self
	resolved := c.knownMember("Me", "") == self.ID
	c.owner.mu.Unlock()
	if !resolved {
		t.Fatal("fixture: your own nick does not resolve to you")
	}
	sent := len(server.received())
	if _, err = m.SetBot(context.Background(), "alice", "network", self.ID, true); !errors.Is(err, ErrNotFound) || len(server.received()) != sent {
		t.Fatalf("marked yourself: %v", err)
	}
	// Without the moderation login, or for someone who is no longer an owner, nothing is sent.
	saved := m.moderator
	m.moderator = nil
	if _, err = m.SetBot(context.Background(), "alice", "network", "peer", true); !errors.Is(err, ErrBotForbidden) || !errors.Is(err, ErrForbidden) {
		t.Fatalf("without the moderation login: %v", err)
	}
	m.moderator = saved
	if err = store.GrantOwner("bob", storage.OwnerFromOwner, "test"); err != nil {
		t.Fatal(err)
	}
	if err = store.RevokeOwner("alice", false); err != nil {
		t.Fatal(err)
	}
	before := len(server.received())
	if _, err = m.SetBot(context.Background(), "alice", "network", "peer", true); !errors.Is(err, ErrBotForbidden) {
		t.Fatalf("a revoked owner marked a bot: %v", err)
	}
	if len(server.received()) != before {
		t.Fatal("a refused mark reached the IRC server")
	}
}
