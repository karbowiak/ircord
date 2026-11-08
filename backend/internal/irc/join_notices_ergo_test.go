package irc

import (
	"slices"
	"testing"
	"time"

	"ircord/backend/internal/storage"

	"github.com/lrstanley/girc"
)

// joinRows lists, oldest first, who each stored join notice in owner's copy of
// room is about. It reads durable history, not the bounded in-memory cache.
func joinRows(t *testing.T, store *storage.Store, m *Manager, owner, room string) []string {
	t.Helper()
	rows := []string{}
	for _, v := range m.Snapshot(owner).Conversations {
		if v.Target != room {
			continue
		}
		page, err := store.History(owner, v.ID, "", 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, msg := range page.Items {
			if msg.Kind == "event" && msg.Event != nil && msg.Event.Type == "join" {
				rows = append(rows, msg.Event.ActorID)
			}
		}
	}
	return rows
}

// Restoring an always-on account replays a JOIN for every channel the server
// kept it in. Neither that replay nor a second IRC session of a present
// identity is a new arrival; real part/kick/quit and rejoins still are.
func TestErgoRestoredMembershipAddsNoJoinNotice(t *testing.T) {
	f := newMetadataFixture(t, "jn")
	bobPassword := alwaysOnAccount(t, f.host, f.port, f.bob)
	f.connect(f.bobSide, "bob", f.config(f.bob, bobPassword), true)
	f.connect(f.aliceSide, "alice", f.config(f.alice, ""), true)
	bobRows := func() []string { return joinRows(t, f.store, f.bobSide, "bob", f.room) }
	aliceRows := func() []string { return joinRows(t, f.store, f.aliceSide, "alice", f.room) }
	eventually(t, func() bool { return len(bobRows()) == 2 })
	eventually(t, func() bool { return len(aliceRows()) == 1 })
	aliceID := f.peer().ID

	for range 2 {
		f.restartBob(func() {})
		eventually(t, func() bool {
			conv := findConversation(t, f.bobSide, "bob", "net", f.room)
			return f.bobSide.Snapshot("bob").Networks[0].Status == "connected" && conv.Joined && slices.Contains(conv.MemberIDs, aliceID)
		})
		// Let catch-up history, NAMES and WHO replies land before counting.
		time.Sleep(time.Second)
		if rows := bobRows(); len(rows) != 2 {
			t.Fatal("restored membership added join notices", rows)
		}
	}

	// Another IRC session attaching to bob's always-on account is invisible
	// to alice and is not a new arrival for bob's backend either.
	second := girc.New(girc.Config{Server: f.host, Port: f.port, Nick: f.bob, User: "test", Name: f.bob, AllowFlood: true, SASL: &girc.SASLPlain{User: f.bob, Pass: bobPassword}})
	ready := make(chan struct{})
	second.Handlers.Add(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) { close(ready) })
	go func() { _ = second.Connect() }()
	t.Cleanup(second.Close)
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("second session did not connect")
	}
	time.Sleep(time.Second)
	if rows := aliceRows(); len(rows) != 1 {
		t.Fatal("second session announced a join to a co-member", rows)
	}
	if rows := bobRows(); len(rows) != 2 {
		t.Fatal("second session announced a join to the account", rows)
	}

	// A channel the other session joins is announced once to the account, and
	// a saved channel whose name casefolds differently under RFC 1459 than
	// under Ergo's ascii mapping is too. Neither repeats on restart.
	other, bracketed := f.room+"x", f.room+"[q]"
	second.Cmd.Join(other)
	if _, err := f.bobSide.Join("bob", "net", bracketed); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		return len(joinRows(t, f.store, f.bobSide, "bob", other)) == 1 && len(joinRows(t, f.store, f.bobSide, "bob", bracketed)) == 1
	})
	for range 2 {
		f.restartBob(func() {})
		eventually(t, func() bool {
			return f.bobSide.Snapshot("bob").Networks[0].Status == "connected" && findConversation(t, f.bobSide, "bob", "net", bracketed).Joined
		})
		time.Sleep(time.Second)
		for _, channel := range []string{other, bracketed} {
			if rows := joinRows(t, f.store, f.bobSide, "bob", channel); len(rows) != 1 {
				t.Fatal("restart announced a join again in", channel, rows)
			}
		}
	}

	// A real part and immediate rejoin is announced, as is a rejoin after a kick.
	alice := findConversation(t, f.aliceSide, "alice", "net", f.room)
	if err := f.aliceSide.Part("alice", alice.ID); err != nil {
		t.Fatal(err)
	}
	f.join(f.aliceSide, "alice")
	eventually(t, func() bool { return len(bobRows()) == 3 })
	if err := f.bobSide.Raw("bob", "net", "KICK "+f.room+" "+f.alice+" :test"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return !findConversation(t, f.aliceSide, "alice", "net", f.room).Joined })
	f.join(f.aliceSide, "alice")
	eventually(t, func() bool { return len(bobRows()) == 4 })
	if rows := bobRows(); rows[2] != aliceID || rows[3] != aliceID {
		t.Fatal("rejoin notices name the wrong member", rows)
	}
}
