package irc

import (
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/storage"

	"github.com/lrstanley/girc"
)

type presenceFixture struct {
	t     *testing.T
	m     *Manager
	store *storage.Store
	c     *connection
	clock time.Time
}

func newPresenceFixture(t *testing.T, saved ...string) *presenceFixture {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err = store.CreateAccount("alice", "alicekey"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store})
	t.Cleanup(m.Close)
	cfg := NetworkConfig{ID: "net", Name: "net", Host: "irc.invalid", Port: 6667, Nick: "Me"}
	c := &connection{owner: m.space("alice"), config: cfg, ctx: m.ctx, ready: true, queue: make(chan []*girc.Event, 64), nicks: map[string]string{}, historyBatches: map[string]bool{}, closedTargets: map[string]bool{}, desired: map[string]string{}, roster: map[string]map[string]bool{}, arrivals: map[string]bool{}}
	w := c.owner
	w.mu.Lock()
	w.connections[cfg.ID] = c
	w.configs[cfg.ID] = cfg
	for _, channel := range saved {
		// A saved channel's conversation survives restarts and reconnects.
		c.conversation(channel)
	}
	w.mu.Unlock()
	for _, channel := range saved {
		c.desired[strings.ToLower(channel)] = channel
	}
	return &presenceFixture{t: t, m: m, store: store, c: c, clock: time.Now().UTC()}
}

// feed runs IRC lines through the membership path in order.
func (f *presenceFixture) feed(lines ...string) {
	for _, line := range lines {
		e := girc.ParseEvent(line)
		if e == nil {
			f.t.Fatal("bad line", line)
		}
		// Distinct server times keep stored history in feed order.
		f.clock = f.clock.Add(time.Millisecond)
		if e.Tags == nil {
			e.Tags = girc.Tags{}
		}
		e.Tags["time"] = f.clock.Format(time.RFC3339Nano)
		f.c.membership(*e)
	}
}

// notices lists the stored membership events of a channel as "type:nick".
func (f *presenceFixture) notices(channel string) []string {
	f.t.Helper()
	w := f.c.owner
	w.mu.Lock()
	id := f.c.conversationID(channel)
	nicks := map[string]string{}
	for _, v := range w.members {
		nicks[v.ID] = v.Nick
	}
	nicks[w.self.ID] = "Me"
	w.mu.Unlock()
	page, err := f.store.History("alice", id, "", 100)
	if err != nil {
		f.t.Fatal(err)
	}
	rows := []string{}
	for _, msg := range page.Items {
		if msg.Kind == "event" {
			rows = append(rows, msg.Event.Type+":"+nicks[msg.Event.ActorID])
		}
	}
	return rows
}

func (f *presenceFixture) expect(channel string, want ...string) {
	f.t.Helper()
	got := f.notices(channel)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		f.t.Fatalf("%s notices = %v, want %v", channel, got, want)
	}
}

func TestPresenceRestoredSelfJoinIsSilent(t *testing.T) {
	f := newPresenceFixture(t, "#room")
	// A bouncer reattach or reconnect replays our JOIN of a channel we have.
	f.feed(":Me!u@h JOIN #room", ":srv 353 Me = #room :Me @Bob +carol", ":Me!u@h JOIN #room")
	f.expect("#room")
	// A channel we did not have, such as one another client of the account or
	// the server joined, is announced once; its replay after a reconnect is not.
	f.feed(":Me!u@h JOIN #other")
	f.expect("#other", "join:Me")
	f.c.resetRoster()
	f.feed(":Me!u@h JOIN #other")
	f.expect("#other", "join:Me")
}

func TestPresenceFirstJoinOfConfiguredChannelIsAnnounced(t *testing.T) {
	f := newPresenceFixture(t)
	// A new network's initial channel is saved but has no conversation yet.
	f.c.desired["#fresh"] = "#fresh"
	f.feed(":Me!u@h JOIN #fresh")
	f.expect("#fresh", "join:Me")
}

func TestPresenceReconnectDropsStaleRoster(t *testing.T) {
	f := newPresenceFixture(t, "#room", "#side")
	f.feed(":Me!u@h JOIN #room", ":Me!u@h JOIN #side", ":srv 353 Me = #side :Me Bob")
	// After the reconnect we are back in #room only; Bob's later QUIT must
	// not reach #side through the previous connection's roster.
	f.c.resetRoster()
	f.feed(":Me!u@h JOIN #room", ":Bob!u@h QUIT :bye")
	f.expect("#side")
}

func TestPresenceRequestedAndRejoinedChannelsAreAnnounced(t *testing.T) {
	f := newPresenceFixture(t)
	f.c.arrivals["#room"] = true
	f.c.desired["#room"] = "#room"
	f.feed(":Me!u@h JOIN #room")
	f.expect("#room", "join:Me")
	// Leaving a saved channel without the Part action, then rejoining.
	f.feed(":Me!u@h PART #room", ":Me!u@h JOIN #room")
	f.expect("#room", "join:Me", "part:Me", "join:Me")
	f.feed(":Op!u@h KICK #room Me :out", ":Me!u@h JOIN #room")
	f.expect("#room", "join:Me", "part:Me", "join:Me", "kick:Op", "join:Me")
	// A reconnect replay afterwards is still a restoration.
	f.c.roster = map[string]map[string]bool{}
	f.feed(":Me!u@h JOIN #room")
	f.expect("#room", "join:Me", "part:Me", "join:Me", "kick:Op", "join:Me")
}

func TestPresenceMembersFromNamesAndWhoAreNotArrivals(t *testing.T) {
	f := newPresenceFixture(t, "#room")
	f.feed(
		":Me!u@h JOIN #room",
		":srv 353 Me = #room :Me @Bob +carol!c@host",
		":srv 354 Me 1 #room du dh Dave acct :Dave",
		":srv 352 Me #room eu eh srv Erin H :0 Erin",
		// Duplicates of members we already know, in other case variants.
		":bob!u@h JOIN #room", ":CAROL!u@h JOIN #room", ":dave!u@h JOIN #room", ":erin!u@h JOIN #room",
		// NAMES for a channel we are not in must not seed a roster.
		":srv 353 Me = #elsewhere :Frank",
		":Frank!u@h JOIN #room",
	)
	f.expect("#room", "join:Frank")
}

func TestPresenceGenuineTransitions(t *testing.T) {
	f := newPresenceFixture(t, "#room", "#side")
	f.feed(":Me!u@h JOIN #room", ":Me!u@h JOIN #side", ":srv 353 Me = #room :Me Bob", ":srv 353 Me = #side :Me")
	f.feed(
		// Part, quit and kick followed by an immediate rejoin all show.
		":Bob!u@h PART #room", ":Bob!u@h JOIN #room",
		":Bob!u@h QUIT :bye", ":Bob!u@h JOIN #room",
		":Op!u@h KICK #room Bob :out", ":Bob!u@h JOIN #room",
		// The same user joining another channel is its own arrival.
		":Bob!u@h JOIN #side", ":Bob!u@h JOIN #side",
	)
	f.expect("#room", "part:Bob", "join:Bob", "quit:Bob", "join:Bob", "kick:Op", "join:Bob")
	f.expect("#side", "join:Bob")
	// After a rename the old nick is free; whoever takes it next is arriving.
	f.feed(":Bob!u@h NICK Robert", ":Robert!u@h JOIN #side", ":Bob!x@y JOIN #side")
	// (The fixture skips receive's nick map rename, so both rows name Bob's member.)
	f.expect("#side", "join:Bob", "nick:Bob", "join:Bob")
}

func TestPresenceQuitReachesChannelsAfterRosterSync(t *testing.T) {
	f := newPresenceFixture(t, "#room")
	f.feed(":Me!u@h JOIN #room", ":srv 353 Me = #room :Me Bob")
	// The conversation's member list never had Bob (no roster sync ran);
	// the connection's own roster still routes his QUIT here.
	f.feed(":Bob!u@h QUIT :bye")
	f.expect("#room", "quit:Bob")
}

func TestPresenceRedeliveredAndHistoricalJoinsAreIdempotent(t *testing.T) {
	f := newPresenceFixture(t, "#room")
	f.feed(":Me!u@h JOIN #room")
	f.feed("@msgid=abc :Bob!u@h JOIN #room", ":Bob!u@h PART #room", "@msgid=abc :Bob!u@h JOIN #room")
	f.expect("#room", "join:Bob", "part:Bob")
	f.c.historyBatches["h1"] = true
	f.feed("@batch=h1 :Carol!u@h JOIN #room", "@batch=h1 :Carol!u@h PART #room")
	f.expect("#room", "join:Bob", "part:Bob")
	f.feed(":Carol!u@h JOIN #room")
	f.expect("#room", "join:Bob", "part:Bob", "join:Carol")
}

func TestPresenceRosterIsBoundedByJoinedChannels(t *testing.T) {
	f := newPresenceFixture(t, "#room", "#side")
	f.feed(":Me!u@h JOIN #room", ":Me!u@h JOIN #side")
	for _, nick := range []string{"a", "b", "c"} {
		f.feed(":"+nick+"!u@h JOIN #room", ":"+nick+"!u@h QUIT :bye")
	}
	// The Part action closes the target, so its PART echo never reaches membership.
	if err := f.m.Part("alice", f.c.conversationID("#room")); err != nil {
		t.Fatal(err)
	}
	f.c.mu.Lock()
	_, kept := f.c.roster["#room"]
	f.c.mu.Unlock()
	if kept {
		t.Fatal("roster kept a channel the Part action left")
	}
	f.feed(":Me!u@h PART #side")
	f.c.mu.Lock()
	defer f.c.mu.Unlock()
	if len(f.c.roster) != 0 {
		t.Fatal("roster kept a channel we left", f.c.roster)
	}
}
