package irc

import (
	"net"
	"testing"
	"time"

	"ircord/backend/internal/storage"
)

// dials counts connection attempts to a listener that never answers like IRC, so each
// registration times out and IRCord keeps retrying while the network is on.
func dials(t *testing.T) (port int, attempts <-chan struct{}) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	ch := make(chan struct{}, 256)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			select {
			case ch <- struct{}{}:
			default:
			}
			go func() { time.Sleep(time.Second); _ = conn.Close() }()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, ch
}

func expectDial(t *testing.T, attempts <-chan struct{}, want bool, why string) {
	t.Helper()
	wait := 600 * time.Millisecond
	if want {
		wait = 5 * time.Second
	}
	select {
	case <-attempts:
		if !want {
			t.Fatal("IRCord connected to IRC " + why)
		}
	case <-time.After(wait):
		if want {
			t.Fatal("IRCord did not connect to IRC " + why)
		}
	}
}

func drainDials(attempts <-chan struct{}) {
	for {
		select {
		case <-attempts:
		default:
			return
		}
	}
}

// Signing out of the last device takes the account off IRC until it signs in again, even
// across a backend restart; a network the user disconnected stays off after sign-in (#237).
func TestSignOutStaysOffIRCAcrossRestartsUntilSignIn(t *testing.T) {
	port, attempts := dials(t)
	dir := t.TempDir()
	open := func() (*storage.Store, *Manager) {
		t.Helper()
		store, err := storage.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		m := New(Options{Store: store, AllowPrivate: true, ReconnectMin: 20 * time.Millisecond, ConnectTimeout: 100 * time.Millisecond})
		if err = m.Restore(); err != nil {
			t.Fatal(err)
		}
		return store, m
	}
	store, m := open()
	if _, err := store.CreateAccount("alice", "alice-key-hash"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"chat", "quiet"} {
		if _, err := m.Connect("alice", NetworkConfig{ID: id, Name: id, Host: "127.0.0.1", Port: port, Nick: "alice"}); err != nil {
			t.Fatal(err)
		}
	}
	expectDial(t, attempts, true, "for a new network")
	// The user disconnects one network themselves, then signs out of their last device.
	if err := m.Disconnect("alice", "quiet"); err != nil {
		t.Fatal(err)
	}
	if err := m.SignOut("alice"); err != nil {
		t.Fatal(err)
	}
	for _, n := range m.Snapshot("alice").Networks {
		if n.Status != "disconnected" {
			t.Fatalf("network %s is %s after sign-out", n.ID, n.Status)
		}
	}
	drainDials(attempts)
	expectDial(t, attempts, false, "after the account signed out")

	m.Close()
	_ = store.Close()
	store, m = open()
	defer func() { m.Close(); _ = store.Close() }()
	expectDial(t, attempts, false, "after a restart while signed out")
	if _, err := m.connection("alice", "chat"); err == nil {
		t.Fatal("restore reconnected a signed-out network")
	}

	if err := m.SignIn("alice"); err != nil {
		t.Fatal(err)
	}
	expectDial(t, attempts, true, "after signing in again")
	if _, err := m.connection("alice", "chat"); err != nil {
		t.Fatal("sign-in left the signed-out network off:", err)
	}
	if _, err := m.connection("alice", "quiet"); err == nil {
		t.Fatal("sign-in reconnected the network the user disconnected")
	}
	if ids, err := store.SignedOutNetworks("alice"); err != nil || len(ids) != 0 {
		t.Fatal("sign-in kept sign-out marks", ids, err)
	}
	// Signing in again with nothing signed out changes nothing.
	if err := m.SignIn("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.connection("alice", "quiet"); err == nil {
		t.Fatal("a second sign-in reconnected the network the user disconnected")
	}
}

// A network that can't reconnect at sign-in keeps its mark and doesn't hold back the others.
func TestSignInReconnectsTheRestWhenOneNetworkFails(t *testing.T) {
	port, _ := dials(t)
	dir := t.TempDir()
	open := func(private bool) (*storage.Store, *Manager) {
		t.Helper()
		store, err := storage.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		m := New(Options{Store: store, AllowPrivate: private, ReconnectMin: 20 * time.Millisecond, ConnectTimeout: 100 * time.Millisecond})
		if err = m.Restore(); err != nil {
			t.Fatal(err)
		}
		return store, m
	}
	store, m := open(true)
	if _, err := store.CreateAccount("alice", "alice-key-hash"); err != nil {
		t.Fatal(err)
	}
	// "a" sorts first and is plain text; "b" uses TLS.
	for _, cfg := range []NetworkConfig{{ID: "a", Name: "a", Host: "127.0.0.1", Port: port, Nick: "alice"}, {ID: "b", Name: "b", Host: "127.0.0.1", Port: port, TLS: true, Nick: "alice"}} {
		if _, err := m.Connect("alice", cfg); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.SignOut("alice"); err != nil {
		t.Fatal(err)
	}
	m.Close()
	_ = store.Close()
	// The operator now requires TLS, so "a" can't connect.
	store, m = open(false)
	defer func() { m.Close(); _ = store.Close() }()
	if err := m.SignIn("alice"); err == nil {
		t.Fatal("sign-in hid the network that couldn't reconnect")
	}
	if _, err := m.connection("alice", "b"); err != nil {
		t.Fatal("one failed network kept the next one off:", err)
	}
	if ids, err := store.SignedOutNetworks("alice"); err != nil || len(ids) != 1 || ids[0] != "a" {
		t.Fatal("marks after a partly failed sign-in:", ids, err)
	}
}

// A network saved as on while the connection pool was full has no connection, but signing
// out still keeps it off after a restart.
func TestSignOutCoversNetworksWithoutAConnection(t *testing.T) {
	port, _ := dials(t)
	dir := t.TempDir()
	open := func() (*storage.Store, *Manager) {
		t.Helper()
		store, err := storage.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		m := New(Options{Store: store, AllowPrivate: true, MaxConnections: 1, ReconnectMin: 20 * time.Millisecond, ConnectTimeout: 100 * time.Millisecond})
		if err = m.Restore(); err != nil {
			t.Fatal(err)
		}
		return store, m
	}
	store, m := open()
	for _, owner := range []string{"alice", "bob"} {
		if _, err := store.CreateAccount(owner, owner+"-key-hash"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Connect("bob", NetworkConfig{ID: "x", Name: "x", Host: "127.0.0.1", Port: port, Nick: "bob"}); err != nil {
		t.Fatal(err)
	}
	// Bob holds the only connection, so Alice's home network is saved without one.
	if err := m.ProvisionHome("alice", NetworkConfig{ID: "home", Name: "home", Host: "127.0.0.1", Port: port, Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.connection("alice", "home"); err == nil {
		t.Fatal("the full pool still connected the home network")
	}
	if err := m.SignOut("alice"); err != nil {
		t.Fatal(err)
	}
	if err := m.Disconnect("bob", "x"); err != nil {
		t.Fatal(err)
	}
	m.Close()
	_ = store.Close()
	store, m = open()
	defer func() { m.Close(); _ = store.Close() }()
	if _, err := m.connection("alice", "home"); err == nil {
		t.Fatal("a restart connected a network while its account was signed out")
	}
	if err := m.SignIn("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.connection("alice", "home"); err != nil {
		t.Fatal("sign-in left the network off:", err)
	}
}

// Without durable storage, a full connection pool at sign-in keeps the mark for next time.
func TestSignInRetriesAfterAFullPoolWithoutStorage(t *testing.T) {
	port, _ := dials(t)
	m := New(Options{AllowPrivate: true, MaxConnections: 1, ReconnectMin: 20 * time.Millisecond, ConnectTimeout: 100 * time.Millisecond})
	defer m.Close()
	if _, err := m.Connect("alice", NetworkConfig{ID: "a", Name: "a", Host: "127.0.0.1", Port: port, Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := m.SignOut("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Connect("bob", NetworkConfig{ID: "x", Name: "x", Host: "127.0.0.1", Port: port, Nick: "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := m.SignIn("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.connection("alice", "a"); err == nil {
		t.Fatal("connected past a full pool")
	}
	if err := m.Disconnect("bob", "x"); err != nil {
		t.Fatal(err)
	}
	if err := m.SignIn("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.connection("alice", "a"); err != nil {
		t.Fatal("the second sign-in forgot the network:", err)
	}
}
