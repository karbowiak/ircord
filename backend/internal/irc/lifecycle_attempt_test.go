package irc

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
)

func lifecycleConfig(port int) NetworkConfig {
	return NetworkConfig{ID: "net", Name: "Net", Host: "127.0.0.1", Port: port, Nick: "alice"}
}

// girc invokes CONNECTED two seconds after the welcome, even if the socket
// already closed. Keep the next attempt far enough away to observe that callback.
func TestLifecycleDroppedRegistrationCannotBecomeReady(t *testing.T) {
	s := newStubServer(t, func(int) stubAttempt { return stubAttempt{welcome: true, hold: 150 * time.Millisecond} })
	m := New(Options{AllowPrivate: true, ReconnectMin: 5 * time.Second})
	defer m.Close()
	if _, err := m.Connect("alice", lifecycleConfig(s.port())); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(s.closed()) == 1 })
	time.Sleep(2300 * time.Millisecond)
	n, err := m.Network("alice", "net")
	if err != nil || n.Status != "connecting" {
		t.Fatalf("dead registration status = %q, %v", n.Status, err)
	}
	c, _ := m.connection("alice", "net")
	c.mu.Lock()
	ready := c.ready
	old := c.client
	c.mu.Unlock()
	if ready {
		t.Fatal("ended attempt became ready")
	}
	c.receiveFromAttempt(old, girc.Event{Command: "464", Params: []string{"alice", "late failure"}})
	c.mu.Lock()
	permanent := c.permanent
	c.mu.Unlock()
	if permanent {
		t.Fatal("ended attempt changed terminal failure state")
	}
}

func TestLifecycleInitialTransientFailuresRetryAndKeepOneOwner(t *testing.T) {
	s := newStubServer(t, func(attempt int) stubAttempt { return stubAttempt{welcome: attempt >= 3} })
	m := New(Options{AllowPrivate: true, ReconnectMin: 30 * time.Millisecond})
	defer m.Close()
	cfg := lifecycleConfig(s.port())
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Connect("alice", cfg); err == nil {
		t.Fatal("second owner accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := m.WaitConnected(ctx, "alice", cfg.ID); err != nil {
		t.Fatal(err)
	}
	attempts := s.attempts()
	if got := len(attempts); got != 3 {
		t.Fatalf("got %d attempts, want 3", got)
	}
	if attempts[1].Sub(attempts[0]) < 30*time.Millisecond || attempts[2].Sub(attempts[1]) < 60*time.Millisecond {
		t.Fatalf("temporary failures did not back off: %v", attempts)
	}
}

func TestLifecycleCanceledWelcomeCannotBecomeReady(t *testing.T) {
	s := newStubServer(t, func(int) stubAttempt { return stubAttempt{welcome: true} })
	m := New(Options{AllowPrivate: true, ReconnectMin: 30 * time.Millisecond})
	defer m.Close()
	if _, err := m.Connect("alice", lifecycleConfig(s.port())); err != nil {
		t.Fatal(err)
	}
	c, err := m.connection("alice", "net")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		c.mu.Lock()
		client := c.client
		c.mu.Unlock()
		return client != nil && client.GetNick() == "alice"
	})
	if err := m.Disconnect("alice", "net"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2200 * time.Millisecond)
	n, err := m.Network("alice", "net")
	if err != nil || n.Status != "disconnected" || len(s.attempts()) != 1 {
		t.Fatalf("canceled attempt status = %q, attempts = %d, error = %v", n.Status, len(s.attempts()), err)
	}
}

func TestLifecycleRegistrationTimeoutRetriesAndDisconnectCancels(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted <- conn
		}
	}()
	m := New(Options{AllowPrivate: true, ConnectTimeout: 100 * time.Millisecond, ReconnectMin: 30 * time.Millisecond})
	defer m.Close()
	if _, err := m.Connect("alice", lifecycleConfig(listener.Addr().(*net.TCPAddr).Port)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case conn := <-accepted:
			defer conn.Close()
		case <-time.After(3 * time.Second):
			t.Fatal("registration timeout did not retry")
		}
	}
	start := time.Now()
	if err := m.Disconnect("alice", "net"); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("disconnect waited for registration timeout")
	}
	select {
	case conn := <-accepted:
		conn.Close()
		t.Fatal("attempt started after disconnect")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestLifecycleBadAuthenticationIsTerminal(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{}, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted <- struct{}{}
			go func() {
				defer conn.Close()
				r := bufio.NewReader(conn)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if strings.HasPrefix(line, "USER ") {
						_, _ = conn.Write([]byte(":stub 464 alice :Password incorrect test-secret\r\n"))
					}
				}
			}()
		}
	}()
	m := New(Options{AllowPrivate: true, ReconnectMin: 30 * time.Millisecond})
	defer m.Close()
	cfg := lifecycleConfig(listener.Addr().(*net.TCPAddr).Port)
	cfg.ServerPassword = "test-secret"
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	n := waitStatus(t, m, "error")
	if n.StatusDetail != "Password incorrect [redacted]" {
		t.Fatalf("authentication detail = %q", n.StatusDetail)
	}
	<-accepted
	select {
	case <-accepted:
		t.Fatal("authentication failure retried")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestLifecycleInvalidTransportConfigurationIsTerminal(t *testing.T) {
	s := newStubServer(t, func(int) stubAttempt { return stubAttempt{welcome: true} })
	m := New(Options{AllowPrivate: true, ReconnectMin: 30 * time.Millisecond})
	defer m.Close()
	cfg := lifecycleConfig(s.port())
	cfg.Username = "*" // accepted by IRCord's input check but rejected by girc
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	n := waitStatus(t, m, "error")
	if !strings.Contains(n.StatusDetail, "invalid configuration") || len(s.attempts()) != 0 {
		t.Fatalf("invalid configuration: status %q, attempts %d", n.StatusDetail, len(s.attempts()))
	}
}

func TestLifecyclePrivateAddressPolicyIsTerminal(t *testing.T) {
	s := newStubServer(t, func(int) stubAttempt { return stubAttempt{welcome: true} })
	m := New(Options{ReconnectMin: 30 * time.Millisecond})
	defer m.Close()
	cfg := lifecycleConfig(s.port())
	cfg.TLS = true
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	n := waitStatus(t, m, "error")
	if !strings.Contains(n.StatusDetail, "private") || len(s.attempts()) != 0 {
		t.Fatalf("private address: status %q, attempts %d", n.StatusDetail, len(s.attempts()))
	}
}

func TestLifecycleTLSCertificateFailureIsTerminal(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	addr := server.Listener.Addr().(*net.TCPAddr)
	m := New(Options{AllowPrivate: true, ReconnectMin: 30 * time.Millisecond, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}})
	defer m.Close()
	cfg := lifecycleConfig(addr.Port)
	cfg.TLS = true
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	n := waitStatus(t, m, "error")
	if !strings.Contains(n.StatusDetail, "certificate") {
		t.Fatalf("TLS failure detail = %q", n.StatusDetail)
	}
}

func TestLifecycleVerifiedTLSRegistration(t *testing.T) {
	source := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	roots := x509.NewCertPool()
	roots.AddCert(source.Certificate())
	cert := source.TLS.Certificates[0]
	source.Close()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "CAP LS"):
				_, _ = conn.Write([]byte(":stub CAP * LS :\r\n"))
			case strings.HasPrefix(line, "USER "):
				_, _ = conn.Write([]byte(":stub 001 alice :Welcome\r\n:stub 376 alice :End of MOTD\r\n"))
			}
		}
	}()
	m := New(Options{AllowPrivate: true, TLSConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}})
	defer m.Close()
	cfg := lifecycleConfig(listener.Addr().(*net.TCPAddr).Port)
	cfg.TLS = true
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
}

// Keep girc's synchronous event handler busy while the TLS peer sends
// close_notify and the delayed CONNECTED handler fires. Teardown must drain
// both handlers and return the network to reconnecting.
func TestLifecycleTLSCloseNotifyDuringDelayedConnected(t *testing.T) {
	source := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	roots := x509.NewCertPool()
	roots.AddCert(source.Certificate())
	cert := source.TLS.Certificates[0]
	source.Close()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	welcome := make(chan struct{})
	closePeer := make(chan struct{})
	closed := make(chan struct{})
	finishPeer := make(chan struct{})
	defer close(finishPeer)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			close(closed)
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "CAP LS"):
				_, _ = conn.Write([]byte(":stub CAP * LS :\r\n"))
			case strings.HasPrefix(line, "USER "):
				_, _ = conn.Write([]byte(":stub 001 alice :Welcome\r\n"))
				close(welcome)
				<-closePeer
				_, _ = conn.Write([]byte(":stub NOTICE alice :hold handler\r\n"))
				_ = conn.(*tls.Conn).CloseWrite()
				close(closed)
				<-finishPeer // keep TCP open so only TLS reports EOF
				return
			}
		}
	}()
	m := New(Options{AllowPrivate: true, ReconnectMin: 5 * time.Second, TLSConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}})
	defer m.Close()
	cfg := lifecycleConfig(listener.Addr().(*net.TCPAddr).Port)
	cfg.TLS = true
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	select {
	case <-welcome:
	case <-time.After(5 * time.Second):
		t.Fatal("TLS server did not receive registration")
	}
	c, err := m.connection("alice", "net")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		c.mu.Lock()
		client := c.client
		c.mu.Unlock()
		return client != nil && client.GetNick() == "alice"
	})
	w := m.space("alice")
	w.mu.Lock()
	locked := true
	defer func() {
		if locked {
			w.mu.Unlock()
		}
	}()
	close(closePeer)
	<-closed
	time.Sleep(2300 * time.Millisecond)
	c.mu.Lock()
	transientReady := c.ready
	transportAlive := c.transportAliveLocked()
	client := c.client
	attemptDone := c.attemptDone
	c.mu.Unlock()
	libraryConnected := client.IsConnected()
	w.mu.Unlock()
	locked = false
	select {
	case <-attemptDone:
	case <-time.After(5 * time.Second):
		t.Fatal("TLS close deadlocked attempt teardown")
	}
	connectedEvent := false
	eventually(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		connecting := 0
		for _, event := range w.events {
			if event.Type != "network.updated" {
				continue
			}
			var network Network
			if json.Unmarshal(event.Data, &network) == nil {
				if network.Status == "connecting" {
					connecting++
				}
				connectedEvent = connectedEvent || network.Status == "connected"
			}
		}
		return connecting >= 2 && w.networks["net"].Status == "connecting"
	})
	c.mu.Lock()
	ready := c.ready
	c.mu.Unlock()
	if ready {
		t.Fatal("TLS close left the attempt ready after its handlers drained")
	}
	t.Logf("while teardown was blocked: ready=%v transport_alive=%v girc_connected=%v; connected event=%v", transientReady, transportAlive, libraryConnected, connectedEvent)
}

func TestLifecycleStaleMetadataTimersLeaveNewAttemptAlone(t *testing.T) {
	old := girc.New(girc.Config{Server: "old", Nick: "alice", User: "alice"})
	current := girc.New(girc.Config{Server: "new", Nick: "alice", User: "alice"})
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	c := &connection{
		ctx: context.Background(), client: current, transport: conn, ready: true,
		metadataSubscribed: true, metadataJoinTimer: timer,
		metadataJoins:     map[string]map[string]string{"#new": {"": "#new", "new": "new"}},
		metadataRequested: map[string]bool{}, queue: make(chan []*girc.Event, 1),
	}
	c.flushJoinSyncs(old)
	c.requestMetadataSyncForAttempt(old, "stale")
	if c.metadataJoinTimer != timer || len(c.metadataJoins) != 1 || len(c.metadataRequested) != 0 || len(c.queue) != 0 {
		t.Fatal("stale timer modified the current attempt")
	}
}

func TestLifecycleQueuedWritesStayWithTheirAttempt(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()
	old := girc.New(girc.Config{Server: "old", Nick: "alice", User: "alice"})
	current := girc.New(girc.Config{Server: "new", Nick: "alice", User: "alice"})
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	c := &connection{manager: m, owner: m.space("alice"), ctx: ctx, config: NetworkConfig{ID: "net"},
		client: old, transport: conn, ready: true, queue: make(chan []*girc.Event, 2)}
	if err := c.enqueue(&girc.Event{Command: "PRIVMSG", Params: []string{"#private", "old attempt"}}); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	c.client = current
	c.mu.Unlock()
	m.wg.Add(1)
	go c.writer()
	eventually(t, func() bool { return len(c.queue) == 0 })
	time.Sleep(200 * time.Millisecond)
	cancel()
	c.owner.mu.Lock()
	lines := len(c.owner.trace["net"])
	c.owner.mu.Unlock()
	if lines != 0 {
		t.Fatal("queued write from old attempt reached the replacement")
	}
}

func TestLifecycleTLSHandshakeDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	m := New(Options{AllowPrivate: true, ConnectTimeout: 100 * time.Millisecond, ReconnectMin: time.Second})
	defer m.Close()
	cfg := lifecycleConfig(listener.Addr().(*net.TCPAddr).Port)
	cfg.TLS = true
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	conn := <-accepted
	defer conn.Close()
	eventually(t, func() bool {
		n, err := m.Network("alice", "net")
		return err == nil && n.Status == "connecting" && strings.Contains(n.StatusDetail, "timed out")
	})
}
