package irc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/lrstanley/girc"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReplayIsolationAndSlowConsumers(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	_, slow, _, stop := m.Subscribe("alice", 0)
	defer stop()
	for i := 0; i < 300; i++ {
		m.Preferences("alice", Preferences{Theme: "dark", Compact: i%2 == 0})
	}
	_, _, valid, cancel := m.Subscribe("alice", 0)
	defer cancel()
	if valid {
		t.Fatal("expired cursor must resync")
	}
	replay, _, valid, cancel2 := m.Subscribe("alice", 299)
	defer cancel2()
	if !valid || len(replay) != 1 {
		t.Fatal("incorrect exact replay")
	}
	if m.Snapshot("bob").Cursor != 0 {
		t.Fatal("cross-owner event leakage")
	}
	for range slow {
	} // a full queue was closed instead of blocking the producer
	replay[0].Data[0] = '!'
	again, _, _, cancel3 := m.Subscribe("alice", 299)
	defer cancel3()
	if again[0].Data[0] == '!' {
		t.Fatal("replay payloads are mutable")
	}
}
func TestInputAndOwnership(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	if _, err := m.Connect("alice", NetworkConfig{Name: "n", Host: "localhost", Port: 6667, Nick: "a"}); err == nil {
		t.Fatal("plaintext enabled by default")
	}
	if err := m.Send("alice", SendInput{ClientID: "c", ConversationID: "other", Text: "hello"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := (safeDialer{ctx: ctx}).Dial("tcp", "127.0.0.1:6667"); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("private destination accepted: %v", err)
	}
}
func TestConcurrentDisconnectPreservesReplacement(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	w := m.space("owner")
	secondArrived := make(chan struct{})
	releaseSecond := make(chan struct{})
	var calls atomic.Int32
	done := make(chan struct{})
	close(done)
	old := &connection{owner: w, done: done, cancel: func() {
		if calls.Add(1) == 1 {
			<-secondArrived
		} else {
			close(secondArrived)
			<-releaseSecond
		}
	}}
	w.connections["net"] = old
	results := make(chan error, 2)
	go func() { results <- m.Disconnect("owner", "net") }()
	go func() { results <- m.Disconnect("owner", "net") }()
	if err := <-results; err != nil {
		t.Fatal(err)
	}
	replacement := &connection{}
	w.mu.Lock()
	w.connections["net"] = replacement
	w.mu.Unlock()
	close(releaseSecond)
	if err := <-results; err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.connections["net"] != replacement {
		t.Fatal("stale disconnect removed the new owner connection")
	}
}
func ergo(t *testing.T) (string, int) {
	t.Helper()
	addr := os.Getenv("IRCORD_ERGO_ADDR")
	if addr == "" {
		t.Skip("set IRCORD_ERGO_ADDR or run make test-integration")
	}
	host, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}
func eventually(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition did not converge")
}
func findConversation(t *testing.T, m *Manager, owner, network, name string) Conversation {
	t.Helper()
	for _, v := range m.Snapshot(owner).Conversations {
		if v.NetworkID == network && v.Name == strings.TrimPrefix(name, "#") {
			return v
		}
	}
	t.Fatal("conversation not found")
	return Conversation{}
}
func page(m *Manager, owner, conv string) []Message {
	p, _ := m.History(owner, conv, "", 100)
	return slices.DeleteFunc(p.Items, func(v Message) bool { return v.Kind != "message" })
}
func TestErgoMessagesProfilesAndHistory(t *testing.T) {
	host, port := ergo(t)
	m := New(Options{AllowPrivate: true, ReconnectMin: 50 * time.Millisecond})
	defer m.Close()
	suffix := id()[:8]
	room := "#t" + suffix
	alice := "a" + suffix
	bob := "b" + suffix
	for _, v := range []struct{ owner, net, nick string }{{"alice", "one", alice}, {"alice", "two", "a2" + suffix}, {"bob", "one", bob}} {
		if _, err := m.Connect(v.owner, NetworkConfig{ID: v.net, Name: v.net, Host: host, Port: port, Nick: v.nick}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := m.WaitConnected(ctx, v.owner, v.net)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
	caps := m.Snapshot("alice").Networks[0].Capabilities
	for _, cap := range []string{"echo-message", "batch", "message-tags", "server-time", "draft/chathistory", "draft/multiline", "draft/metadata-3", "draft/message-redaction", "draft/read-marker"} {
		if !slices.Contains(caps, cap) {
			t.Fatalf("Ergo not configured for %s: %v", cap, caps)
		}
	}
	a, err := m.Join("alice", "one", room)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Join("bob", "one", room)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("alice history: %+v; bob history: %+v", page(m, "alice", a.ID), page(m, "bob", b.ID))
		}
	})
	eventually(t, func() bool { return len(findConversation(t, m, "alice", "one", room).MemberIDs) == 2 })
	input := SendInput{ClientID: "hello", ConversationID: a.ID, Text: "hi :ircord: 👋"}
	if err = m.Send("alice", input); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "alice", a.ID)) == 1 && len(page(m, "bob", b.ID)) == 1 })
	sent := page(m, "alice", a.ID)[0]
	if sent.ClientID != "hello" || sent.UpstreamID == "" || sent.SenderID != "alice" {
		t.Fatalf("echo was not correlated: %+v", sent)
	}
	if err = m.Send("alice", input); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if len(page(m, "bob", b.ID)) != 1 {
		t.Fatal("duplicate delivery on retry")
	}
	if err = m.Send("bob", SendInput{ClientID: "reply", ConversationID: b.ID, Text: "reply", ReplyToID: page(m, "bob", b.ID)[0].ID}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "alice", a.ID)) == 2 })
	if page(m, "alice", a.ID)[1].ReplyTo == nil {
		t.Fatal("reply tag not rendered")
	}
	if _, err = m.ToggleReaction("alice", a.ID, sent.ID, "👍"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "bob", b.ID)[0].Reactions) == 1 })
	if _, err = m.ToggleReaction("alice", a.ID, sent.ID, "👍"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "bob", b.ID)[0].Reactions) == 0 })
	multiline := "first line\n" + strings.Repeat("long 🐈 text ", 40) + "\nlast line"
	if err = m.Send("alice", SendInput{ClientID: "multiline", ConversationID: a.ID, Text: multiline}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range page(m, "bob", b.ID) {
			if v.Text == multiline {
				return true
			}
		}
		return false
	})
	if err = m.SetTopic("alice", a.ID, "frontend-compatible topic"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return findConversation(t, m, "bob", "one", room).Topic == "frontend-compatible topic" })
	color := "#ab34ef"
	avatar := "https://example.com/avatar.png"
	if _, err = m.SetProfile("alice", Profile{DisplayName: profileName("Alice custom"), Bio: "Profile bio", NameColor: &color, AvatarURL: &avatar}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range m.Snapshot("bob").Members {
			if v.Nick == alice && v.DisplayName == "Alice custom" && v.NameColor == color && v.AvatarURL == avatar && v.Bio == "Profile bio" {
				return true
			}
		}
		return false
	})
	if _, err = m.SetProfile("alice", Profile{DisplayName: profileName("Alice custom"), Bio: "Profile bio"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range m.Snapshot("bob").Members {
			if v.Nick == alice && v.NameColor == "" {
				return true
			}
		}
		return false
	})
	if err = m.SetAway("alice", "one", "coffee"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range m.Snapshot("bob").Members {
			if v.Nick == alice && v.Status == "away" {
				return true
			}
		}
		return false
	})
	if err = m.Nick("alice", "one", "new"+suffix); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range m.Snapshot("bob").Members {
			if v.Nick == "new"+suffix && v.DisplayName == "Alice custom" {
				return true
			}
		}
		return false
	})
	dm, err := m.Direct("alice", "one", bob)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Send("alice", SendInput{ClientID: "dm", ConversationID: dm.ID, Text: "private dm"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range m.Snapshot("bob").Conversations {
			if v.Kind == "direct" {
				for _, msg := range page(m, "bob", v.ID) {
					if msg.Text == "private dm" {
						return true
					}
				}
			}
		}
		return false
	})
	if len(m.Snapshot("mallory").Conversations) != 0 {
		t.Fatal("owner leakage")
	}
	g := &GIF{ID: "gif-1", Slug: "hello-1", Title: "Hello", URL: "https://static.klipy.com/example.gif", PreviewURL: "https://static.klipy.com/example.webp", StillURL: "https://static.klipy.com/example.jpg", Width: 400, Height: 300, Provider: "klipy"}
	if err = m.Send("alice", SendInput{ClientID: "gif", ConversationID: a.ID, Text: g.URL, GIF: g}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range page(m, "bob", b.ID) {
			if v.GIF != nil && v.GIF.ID == g.ID {
				return true
			}
		}
		return false
	})
	if _, err = m.History("mallory", a.ID, "", 40); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-owner history accepted")
	}
	// A fresh connection imports server history and does not duplicate overlaps.
	c, err := m.Join("alice", "two", room)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "alice", c.ID)) >= 4 })
	count := len(page(m, "alice", c.ID))
	if err = m.RequestHistory("alice", c.ID, "", 100); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if len(page(m, "alice", c.ID)) != count {
		t.Fatal("history replay duplicates")
	}
	events := map[string]bool{}
	for _, msg := range allMessages(m, "alice", a.ID) {
		if msg.Event != nil {
			events[msg.Event.Type] = true
		}
	}
	for _, kind := range []string{"join", "nick", "topic"} {
		if !events[kind] {
			t.Fatal("missing inline membership event", kind)
		}
	}
	if err = m.Part("bob", b.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range m.Snapshot("bob").Conversations {
			if v.ID == b.ID {
				return false
			}
		}
		return true
	})
	eventually(t, func() bool {
		for _, msg := range allMessages(m, "alice", a.ID) {
			if msg.Event != nil && msg.Event.Type == "part" {
				return true
			}
		}
		return false
	})
	if _, err = m.Join("bob", "one", room); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range m.Snapshot("bob").Conversations {
			if v.ID == b.ID {
				return v.Joined
			}
		}
		return false
	})
	if err = m.Raw("alice", "one", "KICK "+room+" "+bob+" :membership test"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, msg := range allMessages(m, "alice", a.ID) {
			if msg.Event != nil && msg.Event.Type == "kick" && msg.Event.TargetID != "" {
				return true
			}
		}
		return false
	})
	if _, err = m.Join("bob", "one", room); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range m.Snapshot("bob").Conversations {
			if v.ID == b.ID {
				return v.Joined
			}
		}
		return false
	})
	if err = m.Disconnect("bob", "one"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, msg := range allMessages(m, "alice", a.ID) {
			if msg.Event != nil && msg.Event.Type == "quit" {
				return true
			}
		}
		return false
	})
}
func TestErgoConcurrentConnections(t *testing.T) {
	host, port := ergo(t)
	m := New(Options{AllowPrivate: true})
	defer m.Close()
	suffix := id()[:6]
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		for j := 0; j < 2; j++ {
			wg.Add(1)
			go func(i, j int) {
				defer wg.Done()
				owner := fmt.Sprint("user", i)
				network := fmt.Sprint("network", j)
				_, err := m.Connect(owner, NetworkConfig{ID: network, Name: network, Host: host, Port: port, Nick: fmt.Sprintf("c%s_%d_%d", suffix, i, j)})
				if err != nil {
					t.Error(err)
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				if err := m.WaitConnected(ctx, owner, network); err != nil {
					t.Error(err)
				}
			}(i, j)
		}
	}
	wg.Wait()
	for i := 0; i < 50; i++ {
		s := m.Snapshot(fmt.Sprint("user", i))
		if len(s.Networks) != 2 {
			t.Fatal("connections overwritten across users/networks")
		}
	}
}
func TestErgoSASLTLSAndReconnect(t *testing.T) {
	host, port := ergo(t)
	nick := "sasl" + id()[:8]
	password := id()
	registered := make(chan struct{})
	var once sync.Once
	peer := girc.New(girc.Config{Server: host, Port: port, Nick: nick, User: "test", Name: "test", AllowFlood: true})
	peer.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		c.Send(&girc.Event{Command: "PRIVMSG", Params: []string{"NickServ", "REGISTER " + password}, Sensitive: true})
	})
	peer.Handlers.Add(girc.NOTICE, func(_ *girc.Client, e girc.Event) {
		if strings.Contains(e.Last(), "Account created") {
			once.Do(func() { close(registered) })
		}
	})
	done := make(chan struct{})
	go func() { defer close(done); _ = peer.Connect() }()
	select {
	case <-registered:
	case <-time.After(10 * time.Second):
		peer.Close()
		t.Fatal("NickServ test-account registration failed")
	}
	peer.Close()
	<-done
	cert, err := os.ReadFile("../../../.data/ergo/fullchain.pem")
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(cert) {
		t.Fatal("invalid Ergo test certificate")
	}
	m := New(Options{AllowPrivate: true, ReconnectMin: 50 * time.Millisecond, TLSConfig: &tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12}})
	defer m.Close()
	cfg := NetworkConfig{ID: "tls", Name: "TLS", Host: host, Port: 16697, TLS: true, Nick: nick, SASLUser: nick, SASLPassword: password}
	if _, err = m.Connect("owner", cfg); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err = m.WaitConnected(ctx, "owner", "tls"); err != nil {
		t.Fatal(err)
	}
	c, _ := m.connection("owner", "tls")
	if !c.has("sasl") {
		t.Fatal("SASL not negotiated")
	}
	room, err := m.Join("owner", "tls", "#r"+id()[:8])
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return m.Snapshot("owner").Conversations[0].Joined })
	// Simulate a transport failure. The single owner reconnects, authenticates,
	// and rejoins without requiring a browser or creating a second IRC session.
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	client.Close()
	eventually(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.client != client && c.ready })
	eventually(t, func() bool { return findConversation(t, m, "owner", "tls", room.Name).Joined })
	if err = m.Send("owner", SendInput{ClientID: "after-reconnect", ConversationID: room.ID, Text: "still here"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "owner", room.ID)) == 1 })
	if err = m.Disconnect("owner", "tls"); err != nil {
		t.Fatal(err)
	}
	cfg.ID = "bad"
	cfg.SASLPassword = "wrong-test-password"
	if _, err = m.Connect("bad-owner", cfg); err != nil {
		t.Fatal(err)
	}
	bad, _ := m.connection("bad-owner", "bad")
	select {
	case <-bad.done:
	case <-time.After(10 * time.Second):
		t.Fatal("bad SASL credentials kept retrying")
	}
	if bad.ready {
		t.Fatal("failed SASL silently connected anonymously")
	}
	state := m.Snapshot("bad-owner").Networks[0]
	if state.Status != "error" || state.StatusDetail == "" {
		t.Fatal("authentication failure did not expose connection status detail")
	}
}

func TestDeletedOwnerCannotRecreateConnections(t *testing.T) {
	manager := New(Options{AllowPrivate: true})
	defer manager.Close()
	_ = manager.Snapshot("deleted")
	if err := manager.DropOwner("deleted"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Connect("deleted", NetworkConfig{ID: "net", Name: "Net", Host: "127.0.0.1", Port: 16667, Nick: "deletedtest"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted workspace recreated a connection: %v", err)
	}
	if _, err := manager.SetProfile("deleted", Profile{DisplayName: profileName("Revived")}); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted workspace accepted a profile", err)
	}
	if err := manager.StorageError("deleted"); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted workspace bootstrap accepted", err)
	}
}

func allMessages(m *Manager, owner, conversation string) []Message {
	page, _ := m.History(owner, conversation, "", 100)
	return page.Items
}
