package irc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"ircord/backend/internal/storage"
)

// plainIRC is a client that negotiates no capabilities at all, the way a
// classic IRC client sees a channel.
type plainIRC struct {
	mu    sync.Mutex
	lines []string
}

func joinPlain(t *testing.T, host string, port int, nick, room string) *plainIRC {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprint(port)), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	p := &plainIRC{}
	joined := make(chan struct{})
	var once sync.Once
	fmt.Fprintf(conn, "NICK %s\r\nUSER %s 0 * :plain\r\n", nick, nick)
	go func() {
		r := bufio.NewScanner(conn)
		for r.Scan() {
			line := r.Text()
			if strings.HasPrefix(line, "PING ") {
				fmt.Fprintf(conn, "PONG %s\r\n", strings.TrimPrefix(line, "PING "))
				continue
			}
			fields := strings.Fields(line)
			if len(fields) > 1 && fields[1] == "001" {
				fmt.Fprintf(conn, "JOIN %s\r\n", room)
			}
			if len(fields) > 1 && fields[1] == "366" {
				once.Do(func() { close(joined) })
			}
			p.mu.Lock()
			p.lines = append(p.lines, line)
			p.mu.Unlock()
		}
	}()
	select {
	case <-joined:
	case <-time.After(10 * time.Second):
		t.Fatal("plain client did not join")
	}
	return p
}

func (p *plainIRC) privmsgs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []string{}
	for _, line := range p.lines {
		if strings.Contains(line, " PRIVMSG ") {
			out = append(out, line)
		}
	}
	return out
}

func TestErgoEditAcrossClientsAndHistory(t *testing.T) {
	host, port := ergo(t)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"alice", "bob", "carol"} {
		if _, err = store.CreateAccount(owner, owner+"-key-hash"); err != nil {
			t.Fatal(err)
		}
	}
	suffix := id()[:8]
	room := "#edit" + suffix
	alice, bob, carol := "ea"+suffix, "eb"+suffix, "ec"+suffix
	password := id()
	registerErgoAccount(t, host, port, alice, password)
	m := New(Options{Store: store, AllowPrivate: true, ReconnectMin: time.Hour})
	t.Cleanup(func() { m.Close(); store.Close() })
	connect := func(owner string, v NetworkConfig) {
		t.Helper()
		if _, err := m.Connect(owner, v); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := m.WaitConnected(ctx, owner, "one"); err != nil {
			t.Fatal(err)
		}
	}
	connect("alice", NetworkConfig{ID: "one", Name: "one", Host: host, Port: port, Nick: alice, SASLUser: alice, SASLPassword: password})
	connect("bob", NetworkConfig{ID: "one", Name: "one", Host: host, Port: port, Nick: bob})
	if !m.Snapshot("alice").Networks[0].Features.Edit {
		t.Fatal("editing not exposed as a network feature on Ergo")
	}
	a, err := m.Join("alice", "one", room)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Join("bob", "one", room)
	if err != nil {
		t.Fatal(err)
	}
	plain := joinPlain(t, host, port, "ep"+suffix, room)
	eventually(t, func() bool { return len(findConversation(t, m, "alice", "one", room).MemberIDs) == 3 })

	if err = m.Send("alice", SendInput{ClientID: "typo", ConversationID: a.ID, Text: "helo wrold"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "bob", b.ID)) == 1 && len(page(m, "alice", a.ID)) == 1 })
	mine, theirs := page(m, "alice", a.ID)[0], page(m, "bob", b.ID)[0]
	if err = m.Send("bob", SendInput{ClientID: "quote", ConversationID: b.ID, Text: "what?", ReplyToID: theirs.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ToggleReaction("bob", b.ID, theirs.ID, "👍"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		v := page(m, "alice", a.ID)
		return len(v) == 2 && len(v[0].Reactions) == 1
	})
	bobs := page(m, "bob", b.ID)[1]

	// Only the author may edit; nothing reaches the network otherwise.
	if _, err = m.Edit("bob", b.ID, theirs.ID, "bob rewrites alice"); !errors.Is(err, ErrEditForbidden) {
		t.Fatalf("bob editing alice's message: %v", err)
	}
	if _, err = m.Edit("carol", a.ID, mine.ID, "cross-account"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-account edit: %v", err)
	}
	// Bob is not logged in, so receivers could not tell him from someone who
	// takes his nick later.
	if _, err = m.Edit("bob", b.ID, bobs.ID, "what??"); !errors.Is(err, ErrEditForbidden) {
		t.Fatalf("edit without an account: %v", err)
	}

	cursor := m.Snapshot("bob").Cursor
	edited, err := m.Edit("alice", a.ID, mine.ID, "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if edited.ID != mine.ID || edited.Text != "hello world" || edited.EditedAt == nil {
		t.Fatalf("edit result: %+v", edited)
	}
	eventually(t, func() bool {
		v := page(m, "bob", b.ID)
		return len(v) == 2 && v[0].Text == "hello world" && v[0].EditedAt != nil && v[1].ReplyTo != nil && v[1].ReplyTo.Text == "hello world"
	})
	for _, owner := range []string{"alice", "bob"} {
		v := page(m, owner, map[string]string{"alice": a.ID, "bob": b.ID}[owner])
		if len(v) != 2 || v[0].UpstreamID != mine.UpstreamID || v[0].OriginalText != "helo wrold" || len(v[0].Reactions) != 1 || len(v[0].Edits) != 1 {
			t.Fatalf("%s view after the edit: %+v", owner, v)
		}
	}
	// A client without message-tags sees the correction as a new line.
	eventually(t, func() bool {
		lines := plain.privmsgs()
		return len(lines) >= 3 && strings.HasSuffix(lines[len(lines)-1], " :hello world")
	})
	for _, line := range plain.privmsgs() {
		if strings.HasPrefix(line, "@") {
			t.Fatalf("a client without message-tags received tags: %s", line)
		}
	}

	// A browser resuming from an older cursor gets the edit as message.updated.
	replay, _, _, stop := m.Subscribe("bob", cursor)
	stop()
	found := false
	for _, e := range replay {
		var v Message
		if e.Type == "message.created" && json.Unmarshal(e.Data, &v) == nil && v.Text == "hello world" {
			t.Fatalf("the edit replayed as a new message: %+v", v)
		}
		if e.Type == "message.updated" && json.Unmarshal(e.Data, &v) == nil && v.ID == theirs.ID && v.Text == "hello world" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no message.updated for the edit in replay: %+v", replay)
	}

	// A new IRCord account reads the channel from CHATHISTORY and converges.
	connect("carol", NetworkConfig{ID: "one", Name: "one", Host: host, Port: port, Nick: carol})
	c, err := m.Join("carol", "one", room)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		v := page(m, "carol", c.ID)
		return len(v) == 2 && v[0].Text == "hello world" && v[0].EditedAt != nil && v[0].UpstreamID == mine.UpstreamID && v[1].ReplyTo != nil
	})
}
