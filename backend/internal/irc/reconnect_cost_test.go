package irc

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

type burstHarness struct {
	m     *Manager
	store *storage.Store
	peer  *capabilityPeer
}

func newBurstHarness(t *testing.T) *burstHarness {
	t.Helper()
	port, peers := capabilityServer(t, "message-tags batch server-time", "")
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateAccount("alice", "alice-key-hash"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store, AllowPrivate: true, ReconnectMin: time.Hour})
	t.Cleanup(func() { m.Close(); store.Close() })
	if _, err = m.Connect("alice", NetworkConfig{ID: "net", Name: "net", Host: "127.0.0.1", Port: port, Nick: "app"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	return &burstHarness{m: m, store: store, peer: <-peers}
}

// settle sends a marker notice and waits until the backend has applied every
// line sent before it.
func (h *burstHarness) settle(t *testing.T) {
	t.Helper()
	marker := "settled " + id()
	h.peer.send(":test NOTICE app :" + marker)
	w := h.m.space("alice")
	deadline := time.Now().Add(5 * time.Minute)
	for {
		w.mu.Lock()
		logs := w.serverLogs["net"]
		done := len(logs) > 0 && logs[len(logs)-1].Text == marker
		w.mu.Unlock()
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("lines were not applied")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (h *burstHarness) member(t *testing.T, nick string) Member {
	t.Helper()
	for _, v := range h.m.Snapshot("alice").Members {
		if v.Nick == nick {
			return v
		}
	}
	t.Fatalf("no member %s", nick)
	return Member{}
}

// burst replays what a server sends after a reconnect: a message of the day,
// then NAMES and WHO replies for every user in a channel. It returns how many
// durable events the burst committed and how long it took to apply.
func (h *burstHarness) burst(t *testing.T, users int) (uint64, time.Duration) {
	t.Helper()
	start, err := h.store.Head("alice")
	if err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	h.peer.send(":test 375 app :- test Message of the day -")
	for i := range 40 {
		h.peer.send(fmt.Sprintf(":test 372 app :- line %d", i))
	}
	h.peer.send(":test 376 app :End of /MOTD command.")
	h.peer.send(":app!u@h JOIN #big")
	nicks := make([]string, users)
	for i := range nicks {
		nicks[i] = "user" + strconv.Itoa(i)
	}
	for i := 0; i < users; i += 20 {
		h.peer.send(":test 353 app = #big :" + strings.Join(nicks[i:min(i+20, users)], " "))
	}
	h.peer.send(":test 366 app #big :End of /NAMES list.")
	for _, nick := range nicks {
		h.peer.send(":test 352 app #big " + nick + " host.example test " + nick + " H :0 Real " + nick)
	}
	h.peer.send(":test 315 app #big :End of /WHO list.")
	h.settle(t)
	elapsed := time.Since(began)
	end, err := h.store.Head("alice")
	if err != nil {
		t.Fatal(err)
	}
	return end - start, elapsed
}

// A reconnect into a busy channel must cost a bounded number of durable
// commits per user, and the roster must still match the server's replies.
func TestReconnectBurstCommitsInBatches(t *testing.T) {
	users := 200
	if v, err := strconv.Atoi(os.Getenv("IRCORD_BURST_USERS")); err == nil && v > 0 {
		users = v
	}
	h := newBurstHarness(t)
	events, elapsed := h.burst(t, users)
	t.Logf("users=%d durable events=%d elapsed=%s", users, events, elapsed)
	// One event when a member first appears and one when WHO fills in their
	// details. Before batching, every WHO, NAMES and MOTD line added its own.
	if limit := uint64(2*users + 20); events > limit {
		t.Fatalf("burst committed %d events, want at most %d", events, limit)
	}
	conv := findConversation(t, h.m, "alice", "net", "#big")
	if len(conv.MemberIDs) != users+1 {
		t.Fatalf("channel lists %d members, want %d", len(conv.MemberIDs), users+1)
	}
	if v := h.member(t, "user7"); v.Username != "user7" || v.Host != "host.example" || v.Realname != "Real user7" {
		t.Fatalf("WHO details not applied: %+v", v)
	}
	info, err := h.m.ConnectionInfo("alice", "net")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.MOTD) != 40 || info.MOTD[39] != "- line 39" {
		t.Fatalf("message of the day: %q", info.MOTD)
	}
	rows, err := h.store.DB.Query("SELECT json_extract(payload,'$.command') FROM server_lines WHERE owner='alice' AND network='net' ORDER BY seq")
	if err != nil {
		t.Fatal(err)
	}
	commands := []string{}
	for rows.Next() {
		var command string
		if err = rows.Scan(&command); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, command)
	}
	rows.Close()
	for _, command := range []string{"372", "376", "353", "366", "352", "315", "NOTICE"} {
		if !slices.Contains(commands, command) {
			t.Fatalf("server window lacks %s: %v", command, commands)
		}
	}
	if i, j := slices.Index(commands, "315"), slices.Index(commands, "NOTICE"); i > j {
		t.Fatalf("server window out of order: %v", commands)
	}
}

// Events that name a user or channel refresh only those, and still keep the
// channel's member list and member details current.
func TestRosterFollowsEventsAfterBurst(t *testing.T) {
	h := newBurstHarness(t)
	h.burst(t, 30)
	memberIDs := func() []string { return findConversation(t, h.m, "alice", "net", "#big").MemberIDs }
	gone := h.member(t, "user1").ID
	quit := h.member(t, "user2").ID
	renamed := h.member(t, "user3").ID
	h.peer.send(":user1!user1@host.example PART #big")
	h.peer.send(":user2!user2@host.example QUIT :bye")
	h.peer.send(":user3!user3@host.example NICK user3b")
	h.peer.send(":fresh!f@elsewhere JOIN #big")
	h.peer.send(":user4!user4@host.example AWAY :lunch")
	h.peer.send(":test MODE #big +o user5")
	h.settle(t)
	ids := memberIDs()
	if slices.Contains(ids, gone) || slices.Contains(ids, quit) {
		t.Fatalf("departed users still listed")
	}
	if !slices.Contains(ids, renamed) || h.member(t, "user3b").ID != renamed {
		t.Fatalf("renamed user lost their member")
	}
	if fresh := h.member(t, "fresh"); !slices.Contains(ids, fresh.ID) {
		t.Fatalf("joined user not listed")
	}
	if len(ids) != 30 {
		t.Fatalf("channel lists %d members, want 30", len(ids))
	}
	if v := h.member(t, "user4"); v.Status != "away" || v.AwayMessage != "lunch" {
		t.Fatalf("away not applied: %+v", v)
	}
	if conv := findConversation(t, h.m, "alice", "net", "#big"); conv.MemberModes[h.member(t, "user5").ID] != "@" {
		t.Fatalf("operator mode not applied: %v", conv.MemberModes)
	}
	h.peer.send(":app!u@h PART #big")
	h.settle(t)
	if findConversation(t, h.m, "alice", "net", "#big").Joined {
		t.Fatal("channel still joined after leaving it")
	}
}

func TestJoinBatches(t *testing.T) {
	join := func(events []*girc.Event) []string {
		out := []string{}
		for _, e := range events {
			out = append(out, strings.Join(e.Params, " "))
		}
		return out
	}
	channels := []string{"#a", "#b", "#c", "#d", "#e"}
	if got := join(joinBatches(channels, 2)); !slices.Equal(got, []string{"#a,#b", "#c,#d", "#e"}) {
		t.Fatalf("by count: %v", got)
	}
	long := []string{"#" + strings.Repeat("x", 250), "#" + strings.Repeat("y", 250), "#z"}
	if got := join(joinBatches(long, 10)); len(got) != 2 || got[1] != long[1]+",#z" {
		t.Fatalf("by length: %v", got)
	}
	if len(joinBatches(nil, 10)) != 0 {
		t.Fatal("empty list sent a JOIN")
	}
	for targmax, want := range map[string]int{"": 10, "PRIVMSG:3,JOIN:4": 4, "JOIN:": 10, "join:50": 10, "JOIN:x": 10} {
		if got := joinTargetLimit(targmax); got != want {
			t.Fatalf("TARGMAX %q: got %d, want %d", targmax, got, want)
		}
	}
}

// Stored members nobody refers to are dropped when the account loads.
func TestLoadPrunesUnreferencedMembers(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err = store.CreateAccount("alice", "alice-key-hash"); err != nil {
		t.Fatal(err)
	}
	commit := func(kind string, data any) {
		t.Helper()
		if _, err := store.Commit("alice", kind, data); err != nil {
			t.Fatal(err)
		}
	}
	for _, nick := range []string{"present", "writer", "kicker", "partner", "stranger", "target"} {
		commit("member.updated", model.Member{ID: "net:" + nick, Nick: nick, DisplayName: nick, Status: "online", Role: "member", Color: "lavender"})
	}
	commit("member.updated", model.Member{ID: "party-principal", Nick: "p", DisplayName: "p", Status: "online", Role: "member", Color: "lavender"})
	commit("conversation.updated", model.Conversation{ID: "net_chan", NetworkID: "net", Kind: "channel", Name: "chan", MemberIDs: []string{"alice", "net:present"}, MemberModes: map[string]string{}, Kicked: &model.Kick{ByID: "net:kicker"}})
	commit("conversation.updated", model.Conversation{ID: "net_dm", NetworkID: "net", Kind: "direct", Name: "Partner", MemberIDs: []string{"alice"}, MemberModes: map[string]string{}})
	commit("message.created", model.Message{ID: "m1", ConversationID: "net_chan", SenderID: "net:writer", Text: "hi", Kind: "message", CreatedAt: time.Now(), Reactions: []model.Reaction{}})
	commit("message.created", model.Message{ID: "m2", ConversationID: "net_chan", SenderID: "net:kicker", Kind: "event", CreatedAt: time.Now(), Event: &model.MembershipEvent{Type: "kick", ActorID: "net:kicker", TargetID: "net:target"}, Reactions: []model.Reaction{}})
	m := New(Options{Store: store})
	t.Cleanup(m.Close)
	got := []string{}
	for _, v := range m.Snapshot("alice").Members {
		got = append(got, v.ID)
	}
	slices.Sort(got)
	want := []string{"alice", "net:kicker", "net:partner", "net:present", "net:target", "net:writer", "party-principal"}
	if !slices.Equal(got, want) {
		t.Fatalf("members after load: %v, want %v", got, want)
	}
	entities, err := store.Entities("alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range entities["member"] {
		if strings.Contains(string(raw), "stranger") {
			t.Fatal("pruned member still stored")
		}
	}
}
