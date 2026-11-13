package irc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"ircord/backend/internal/storage"
)

// stubAttempt decides how the stub server treats one accepted connection.
type stubAttempt struct {
	// welcome completes registration; otherwise the connection closes at once.
	welcome bool
	// rename is a nickname the server forces on the client after its welcome.
	rename string
	// hold closes a welcomed connection after this long; zero keeps it open.
	hold time.Duration
	// collide answers registration with "nickname in use" instead of a welcome.
	collide bool
	// later is a raw line the server sends a moment after its welcome.
	later string
}

// stubServer is a minimal IRC server on loopback. plan picks the behaviour for
// each connection by its 1-based attempt number.
type stubServer struct {
	t        *testing.T
	listener net.Listener
	plan     func(attempt int) stubAttempt
	mu       sync.Mutex
	accepts  []time.Time
	closes   []time.Time
}

func newStubServer(t *testing.T, plan func(attempt int) stubAttempt) *stubServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &stubServer{t: t, listener: listener, plan: plan}
	done := make(chan struct{})
	var conns sync.WaitGroup
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.accepts = append(s.accepts, time.Now())
			attempt := s.plan(len(s.accepts))
			s.mu.Unlock()
			conns.Add(1)
			go func() {
				defer conns.Done()
				s.serve(conn, attempt)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
		conns.Wait()
	})
	return s
}

func (s *stubServer) port() int { return s.listener.Addr().(*net.TCPAddr).Port }

func (s *stubServer) attempts() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Time{}, s.accepts...)
}

func (s *stubServer) closed() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Time{}, s.closes...)
}

func (s *stubServer) serve(conn net.Conn, attempt stubAttempt) {
	defer func() {
		_ = conn.Close()
		s.mu.Lock()
		s.closes = append(s.closes, time.Now())
		s.mu.Unlock()
	}()
	if !attempt.welcome {
		return
	}
	deadline := time.Now().Add(time.Hour)
	if attempt.hold > 0 {
		deadline = time.Now().Add(attempt.hold)
	}
	stop := time.AfterFunc(time.Until(deadline), func() { _ = conn.Close() })
	defer stop.Stop()
	go func() {
		// Also end the session when the test finishes.
		<-s.t.Context().Done()
		_ = conn.Close()
	}()
	reader := bufio.NewReader(conn)
	nick := ""
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "CAP":
			if len(fields) > 1 && strings.EqualFold(fields[1], "LS") {
				_, _ = conn.Write([]byte(":stub CAP * LS :\r\n"))
			}
		case "NICK":
			if len(fields) > 1 {
				nick = fields[1]
			}
		case "USER":
			if attempt.collide {
				_, _ = conn.Write([]byte(":stub 433 * " + nick + " :Nickname is already in use\r\n"))
				continue
			}
			_, _ = conn.Write([]byte(":stub 001 " + nick + " :Welcome\r\n:stub 376 " + nick + " :End of MOTD\r\n"))
			if attempt.rename != "" {
				_, _ = conn.Write([]byte(":" + nick + "!u@stub NICK " + attempt.rename + "\r\n"))
				nick = attempt.rename
			}
			if attempt.later != "" {
				later := attempt.later
				time.AfterFunc(300*time.Millisecond, func() { _, _ = conn.Write([]byte(later + "\r\n")) })
			}
		case "PING":
			_, _ = conn.Write([]byte(":stub PONG stub " + strings.Join(fields[1:], " ") + "\r\n"))
		}
	}
}

func restoreStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err = store.CreateAccount("alice", "alice"); err != nil {
		t.Fatal(err)
	}
	return store
}

func waitConnected(t *testing.T, m *Manager, network string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.WaitConnected(ctx, "alice", network); err != nil {
		n, _ := m.Network("alice", network)
		t.Fatalf("network %s did not connect: status=%q detail=%q", network, n.Status, n.StatusDetail)
	}
}

func savedNetwork(t *testing.T, store *storage.Store, id string) storage.SavedNetwork {
	t.Helper()
	saved, err := store.Networks("alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range saved {
		if n.Config.ID == id {
			return n
		}
	}
	t.Fatalf("network %s is not saved", id)
	return storage.SavedNetwork{}
}

// A saved network that can no longer pass validation must not stop the
// backend from starting, nor keep the account's other networks offline.
func TestRestoreSkipsNetworkThatCannotConnect(t *testing.T) {
	server := newStubServer(t, func(int) stubAttempt { return stubAttempt{welcome: true} })
	store := restoreStore(t)
	first := New(Options{Store: store, AllowPrivate: true})
	for _, id := range []string{"broken", "working"} {
		if _, err := first.Connect("alice", NetworkConfig{ID: id, Name: id, Host: "127.0.0.1", Port: server.port(), Nick: "alice"}); err != nil {
			t.Fatal(err)
		}
		waitConnected(t, first, id)
	}
	first.Close()
	broken := savedNetwork(t, store, "broken")
	broken.Config.Nick = "042AAAAAB"
	if err := store.SaveNetwork("alice", broken.Config, true); err != nil {
		t.Fatal(err)
	}

	second := New(Options{Store: store, AllowPrivate: true})
	defer second.Close()
	if err := second.Restore(); err != nil {
		t.Fatalf("restore stopped at one network: %v", err)
	}
	waitConnected(t, second, "working")
	n, err := second.Network("alice", "broken")
	if err != nil {
		t.Fatal(err)
	}
	if n.Status != "error" || n.StatusDetail == "" {
		t.Fatalf("unrestorable network shows status=%q detail=%q, want an error with a reason", n.Status, n.StatusDetail)
	}
}

// Servers can assign a nickname IRCord rejects. Showing it is fine, but saving
// it would stop the network from connecting after the next restart.
func TestServerAssignedInvalidNicknameIsNotSaved(t *testing.T) {
	// Someone else's later rename also updates the network, through a
	// different save path.
	server := newStubServer(t, func(int) stubAttempt {
		return stubAttempt{welcome: true, rename: "042AAAAAB", later: ":bob!u@stub NICK bobby"}
	})
	store := restoreStore(t)
	m := New(Options{Store: store, AllowPrivate: true})
	defer m.Close()
	if _, err := m.Connect("alice", NetworkConfig{ID: "net", Name: "Net", Host: "127.0.0.1", Port: server.port(), Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, m, "net")
	waitNick(t, m, "042AAAAAB")
	// The later rename was sent 300 ms after the welcome; connecting took 2 s.
	time.Sleep(200 * time.Millisecond)
	if got := savedNetwork(t, store, "net").Config.Nick; got != "alice" {
		t.Fatalf("saved nickname = %q, want the configured %q", got, "alice")
	}
}

// A nickname change is saved with the network, so it stays within the length
// a restored network accepts even when the server allows longer nicknames.
func TestNicknameChangeStaysWithinSavedLimit(t *testing.T) {
	server := newStubServer(t, func(int) stubAttempt { return stubAttempt{welcome: true} })
	store := restoreStore(t)
	m := New(Options{Store: store, AllowPrivate: true})
	defer m.Close()
	if _, err := m.Connect("alice", NetworkConfig{ID: "net", Name: "Net", Host: "127.0.0.1", Port: server.port(), Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, m, "net")
	w := m.space("alice")
	w.mu.Lock()
	n := w.networks["net"]
	n.ISupport = map[string]string{"NICKLEN": "64"}
	w.networks["net"] = n
	w.mu.Unlock()
	if err := m.Nick("alice", "net", "a"+strings.Repeat("x", defaultNickLength)); err == nil {
		t.Fatalf("accepted a nickname longer than %d characters", defaultNickLength)
	}
	if err := m.Nick("alice", "net", strings.Repeat("x", defaultNickLength)); err != nil {
		t.Fatalf("rejected a %d-character nickname: %v", defaultNickLength, err)
	}
}

func waitNick(t *testing.T, m *Manager, nick string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		n, err := m.Network("alice", "net")
		if err != nil {
			t.Fatal(err)
		}
		if n.Nick == nick {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("network nickname = %q, want %q", n.Nick, nick)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A network that had already failed when the backend stopped, such as one
// with a wrong address, gets one attempt after a restart instead of endless retries.
func TestRestoredNetworkThatHadFailedIsNotRetried(t *testing.T) {
	server := newStubServer(t, func(int) stubAttempt { return stubAttempt{} })
	store := restoreStore(t)
	first := New(Options{Store: store, AllowPrivate: true})
	if _, err := first.Connect("alice", NetworkConfig{ID: "net", Name: "Net", Host: "127.0.0.1", Port: server.port(), Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, first, "error")
	first.Close()

	before := len(server.attempts())
	second := New(Options{Store: store, AllowPrivate: true, ReconnectMin: 50 * time.Millisecond})
	defer second.Close()
	if err := second.Restore(); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, second, "error")
	time.Sleep(time.Second)
	if got := len(server.attempts()) - before; got != 1 {
		t.Fatalf("connection attempts after restart = %d, want 1", got)
	}
}

// After an unclean stop the server can still hold the previous session's
// nickname. A restored network retries until the server releases it.
func TestRestoredNetworkRetriesNicknameInUse(t *testing.T) {
	var mu sync.Mutex
	collisions := 0
	server := newStubServer(t, func(int) stubAttempt {
		mu.Lock()
		defer mu.Unlock()
		if collisions > 0 {
			collisions--
			return stubAttempt{welcome: true, collide: true}
		}
		return stubAttempt{welcome: true}
	})
	store := restoreStore(t)
	first := New(Options{Store: store, AllowPrivate: true})
	if _, err := first.Connect("alice", NetworkConfig{ID: "net", Name: "Net", Host: "127.0.0.1", Port: server.port(), Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, first, "net")
	first.Close()

	mu.Lock()
	collisions = 1
	mu.Unlock()
	second := New(Options{Store: store, AllowPrivate: true, ReconnectMin: 50 * time.Millisecond})
	defer second.Close()
	if err := second.Restore(); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, second, "net")
}

// A network never connected stops at its first nickname conflict, as before.
func TestNewNetworkNicknameInUseFails(t *testing.T) {
	server := newStubServer(t, func(int) stubAttempt { return stubAttempt{welcome: true, collide: true} })
	store := restoreStore(t)
	m := New(Options{Store: store, AllowPrivate: true, ReconnectMin: 50 * time.Millisecond})
	defer m.Close()
	if _, err := m.Connect("alice", NetworkConfig{ID: "net", Name: "Net", Host: "127.0.0.1", Port: server.port(), Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	n := waitStatus(t, m, "error")
	if n.StatusDetail != "Nickname is already in use" {
		t.Fatalf("status detail = %q, want the nickname conflict", n.StatusDetail)
	}
}

func waitStatus(t *testing.T, m *Manager, status string) Network {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		n, err := m.Network("alice", "net")
		if err == nil && n.Status == status {
			return n
		}
		if time.Now().After(deadline) {
			t.Fatalf("network status = %q (%q), want %q", n.Status, n.StatusDetail, status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Joining is limited to what a restored network accepts, so a join can never
// save a channel list that stops the network from reconnecting after a restart.
func TestJoinStaysWithinRestorableLimits(t *testing.T) {
	server := newStubServer(t, func(int) stubAttempt { return stubAttempt{welcome: true} })
	store := restoreStore(t)
	m := New(Options{Store: store, AllowPrivate: true})
	defer m.Close()
	if _, err := m.Connect("alice", NetworkConfig{ID: "net", Name: "Net", Host: "127.0.0.1", Port: server.port(), Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, m, "net")
	if _, err := m.JoinKey("alice", "net", "#"+strings.Repeat("k", 99), strings.Repeat("k", 100)); err == nil {
		t.Fatal("joined with a channel entry longer than a restored network accepts")
	}
	join := func(channel string) error {
		for {
			// The stub server does not drain the send queue as fast as this loop joins.
			if _, err := m.Join("alice", "net", channel); !errors.Is(err, ErrBusy) {
				return err
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	for i := range maxSavedChannels {
		if err := join(fmt.Sprintf("#room%d", i)); err != nil {
			t.Fatalf("join %d: %v", i, err)
		}
	}
	if err := join("#one-too-many"); err == nil || !strings.Contains(err.Error(), "up to 100 channels") {
		t.Fatalf("joining past the limit = %v, want the channel limit error", err)
	}
	if err := join("#room0"); err != nil {
		t.Fatalf("rejoining a saved channel at the limit: %v", err)
	}
	if got := len(savedNetwork(t, store, "net").Config.Channels); got != maxSavedChannels {
		t.Fatalf("saved channels = %d, want %d", got, maxSavedChannels)
	}
	m.Close()
	restored := New(Options{Store: store, AllowPrivate: true})
	defer restored.Close()
	if err := restored.Restore(); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, restored, "net")
}

// After a restart, a network whose server is briefly unreachable keeps
// retrying instead of failing for good on its first attempt.
func TestRestoredNetworkRetriesFailedFirstConnection(t *testing.T) {
	var mu sync.Mutex
	drops := 0
	server := newStubServer(t, func(int) stubAttempt {
		mu.Lock()
		defer mu.Unlock()
		if drops > 0 {
			drops--
			return stubAttempt{}
		}
		return stubAttempt{welcome: true}
	})
	store := restoreStore(t)
	first := New(Options{Store: store, AllowPrivate: true})
	if _, err := first.Connect("alice", NetworkConfig{ID: "net", Name: "Net", Host: "127.0.0.1", Port: server.port(), Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, first, "net")
	first.Close()

	mu.Lock()
	drops = 2
	mu.Unlock()
	before := len(server.attempts())
	second := New(Options{Store: store, AllowPrivate: true, ReconnectMin: 50 * time.Millisecond})
	defer second.Close()
	if err := second.Restore(); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, second, "net")
	if got := len(server.attempts()) - before; got != 3 {
		t.Fatalf("connection attempts after restart = %d, want 3 (two dropped, then connected)", got)
	}
}

// A connection that stayed up resets the reconnect delay, so a later drop is
// retried quickly instead of waiting out the longest backoff.
func TestReconnectDelayResetsAfterStableConnection(t *testing.T) {
	previous := stableConnection
	stableConnection = 100 * time.Millisecond
	t.Cleanup(func() { stableConnection = previous })
	server := newStubServer(t, func(attempt int) stubAttempt {
		switch attempt {
		case 1, 5:
			// girc reports a connection two seconds after the welcome.
			return stubAttempt{welcome: true, hold: 2500 * time.Millisecond}
		case 2, 3, 4:
			return stubAttempt{}
		default:
			return stubAttempt{welcome: true}
		}
	})
	store := restoreStore(t)
	m := New(Options{Store: store, AllowPrivate: true, ReconnectMin: 200 * time.Millisecond})
	defer m.Close()
	if _, err := m.Connect("alice", NetworkConfig{ID: "net", Name: "Net", Host: "127.0.0.1", Port: server.port(), Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for len(server.attempts()) < 6 {
		if time.Now().After(deadline) {
			n, _ := m.Network("alice", "net")
			t.Fatalf("connection attempts = %d, want 6; status=%q detail=%q", len(server.attempts()), n.Status, n.StatusDetail)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Three dropped attempts grew the delay to 3.2 s. The fifth connection
	// stayed up, so the sixth attempt follows after about ReconnectMin.
	gap := server.attempts()[5].Sub(server.closed()[4])
	if gap > time.Second {
		t.Fatalf("reconnect after a stable connection waited %v, want about 200ms", gap)
	}
}
