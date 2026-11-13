package irc

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebIRCFirstCommandAndAttemptOrigin(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("TEST_WEBIRC_PASSWORD", "fixture-webirc-secret")
	text := fmt.Sprintf("webirc:\n  - host: 127.0.0.1\n    port: %d\n    tls: false\n    gateway: ircord\n    passwordEnv: TEST_WEBIRC_PASSWORD\n", listener.Addr().(*net.TCPAddr).Port)
	list, err := LoadServerList(writeServerList(t, text), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	firstLines := make(chan string, 3)
	accepted := make(chan net.Conn, 3)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted <- conn
			go func() {
				defer conn.Close()
				r := bufio.NewReader(conn)
				first := true
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if first {
						firstLines <- strings.TrimSpace(line)
						first = false
					}
					if strings.HasPrefix(line, "CAP LS") {
						_, _ = conn.Write([]byte(":stub CAP * LS :\r\n"))
					}
					if strings.HasPrefix(line, "USER ") {
						_, _ = conn.Write([]byte(":stub 001 alice :Welcome\r\n:stub 376 alice :End\r\n:stub NOTICE alice :fixture-webirc-secret 198.51.100.25\r\n"))
					}
				}
			}()
		}
	}()
	var origin atomic.Value
	origin.Store("::ffff:198.51.100.25")
	m := New(Options{AllowPrivate: true, ServerList: list, ReconnectMin: 20 * time.Millisecond, Origin: func(string) (string, error) { return origin.Load().(string), nil }})
	defer m.Close()
	cfg := lifecycleConfig(listener.Addr().(*net.TCPAddr).Port)
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	first := <-accepted
	if line := <-firstLines; line != "WEBIRC fixture-webirc-secret ircord 198.51.100.25 198.51.100.25" {
		t.Fatalf("first command: %q", line)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	origin.Store("::1")
	select {
	case conn := <-accepted:
		conn.Close()
		t.Fatal("origin change reconnected an established session")
	case <-time.After(150 * time.Millisecond):
	}
	_ = first.Close()
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("no replacement attempt")
	}
	select {
	case line := <-firstLines:
		if line != "WEBIRC fixture-webirc-secret ircord 0::1 0::1" {
			t.Fatalf("replacement command: %q", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("replacement registration stalled")
	}
	w := m.space("alice")
	w.mu.Lock()
	public, _ := json.Marshal(struct {
		Trace    []ServerLine
		Networks map[string]Network
	}{w.trace["net"], w.networks})
	w.mu.Unlock()
	for _, private := range []string{"fixture-webirc-secret", "198.51.100.25", "0::1"} {
		if strings.Contains(string(public), private) {
			t.Fatal("gateway credential or client address appeared in public state")
		}
	}
}

func TestWebIRCMissingOriginUsesOrdinaryConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("TEST_WEBIRC_PASSWORD", "fixture-webirc-secret")
	text := fmt.Sprintf("webirc:\n  - host: 127.0.0.1\n    port: %d\n    tls: false\n    passwordEnv: TEST_WEBIRC_PASSWORD\n", listener.Addr().(*net.TCPAddr).Port)
	list, err := LoadServerList(writeServerList(t, text), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		line, _ := bufio.NewReader(conn).ReadString('\n')
		lines <- line
	}()
	m := New(Options{AllowPrivate: true, ServerList: list, Origin: func(string) (string, error) { return "", nil }, ReconnectMin: time.Hour})
	defer m.Close()
	if _, err := m.Connect("alice", lifecycleConfig(listener.Addr().(*net.TCPAddr).Port)); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-lines:
		if !strings.HasPrefix(line, "CAP LS") {
			t.Fatalf("unexpected first command: %q", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("registration did not start")
	}
}

func TestWebIRCUsesVerifiedTLS(t *testing.T) {
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
	t.Setenv("TEST_WEBIRC_PASSWORD", "fixture-webirc-secret")
	text := fmt.Sprintf("webirc:\n  - host: 127.0.0.1\n    port: %d\n    passwordEnv: TEST_WEBIRC_PASSWORD\n", listener.Addr().(*net.TCPAddr).Port)
	list, err := LoadServerList(writeServerList(t, text), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		line, _ := bufio.NewReader(conn).ReadString('\n')
		lines <- line
	}()
	m := New(Options{AllowPrivate: true, ServerList: list, TLSConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, Origin: func(string) (string, error) { return "198.51.100.1", nil }, ReconnectMin: time.Hour})
	defer m.Close()
	cfg := lifecycleConfig(listener.Addr().(*net.TCPAddr).Port)
	cfg.TLS = true
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-lines:
		if !strings.HasPrefix(line, "WEBIRC fixture-webirc-secret ") {
			t.Fatal("verified TLS did not carry WEBIRC first")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("TLS registration did not start")
	}
}

func TestWebIRCRejectionDoesNotFallback(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("TEST_WEBIRC_PASSWORD", "fixture-webirc-secret")
	text := fmt.Sprintf("webirc:\n  - host: 127.0.0.1\n    port: %d\n    tls: false\n    passwordEnv: TEST_WEBIRC_PASSWORD\n", listener.Addr().(*net.TCPAddr).Port)
	list, err := LoadServerList(writeServerList(t, text), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 2)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				r := bufio.NewReader(conn)
				line, err := r.ReadString('\n')
				if err != nil {
					return
				}
				lines <- line
				for {
					line, err = r.ReadString('\n')
					if err != nil {
						return
					}
					if strings.HasPrefix(line, "USER ") {
						_, _ = conn.Write([]byte("ERROR :WEBIRC rejected fixture-webirc-secret\r\n"))
						for {
							if _, err = r.ReadString('\n'); err != nil {
								return
							}
						}
					}
				}
			}()
		}
	}()
	m := New(Options{AllowPrivate: true, ServerList: list, Origin: func(string) (string, error) { return "198.51.100.4", nil }, ReconnectMin: 20 * time.Millisecond})
	defer m.Close()
	if _, err := m.Connect("alice", lifecycleConfig(listener.Addr().(*net.TCPAddr).Port)); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-lines:
		if !strings.HasPrefix(line, "WEBIRC ") {
			t.Fatal("gateway command was not first")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no registration")
	}
	n := waitStatus(t, m, "error")
	if strings.Contains(n.StatusDetail, "fixture-webirc-secret") || !strings.Contains(n.StatusDetail, "[redacted]") {
		t.Fatal("rejection exposed credential")
	}
	select {
	case <-lines:
		t.Fatal("WEBIRC rejection retried")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestWebIRCOfflineRestoreUsesPersistedOrigin(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("TEST_WEBIRC_PASSWORD", "fixture-webirc-secret")
	text := fmt.Sprintf("webirc:\n  - host: 127.0.0.1\n    port: %d\n    tls: false\n    passwordEnv: TEST_WEBIRC_PASSWORD\n", listener.Addr().(*net.TCPAddr).Port)
	list, err := LoadServerList(writeServerList(t, text), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				r := bufio.NewReader(conn)
				first := true
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if first {
						lines <- strings.TrimSpace(line)
						first = false
					}
					if strings.HasPrefix(line, "CAP LS") {
						_, _ = conn.Write([]byte(":stub CAP * LS :\r\n"))
					}
					if strings.HasPrefix(line, "USER ") {
						_, _ = conn.Write([]byte(":stub 001 alice :Welcome\r\n:stub 376 alice :End\r\n"))
					}
				}
			}()
		}
	}()
	store := restoreStore(t)
	if err := store.SaveOrigin("alice", "198.51.100.1"); err != nil {
		t.Fatal(err)
	}
	options := Options{Store: store, AllowPrivate: true, ServerList: list, Origin: store.Origin, ReconnectMin: 20 * time.Millisecond}
	first := New(options)
	defer first.Close()
	if _, err := first.Connect("alice", lifecycleConfig(listener.Addr().(*net.TCPAddr).Port)); err != nil {
		t.Fatal(err)
	}
	if line := <-lines; !strings.HasSuffix(line, "198.51.100.1 198.51.100.1") {
		t.Fatalf("first origin %q", line)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := first.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	first.Close()
	if err := store.SaveOrigin("alice", "198.51.100.2"); err != nil {
		t.Fatal(err)
	}
	restored := New(options)
	defer restored.Close()
	if err := restored.Restore(); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-lines:
		if !strings.HasSuffix(line, "198.51.100.2 198.51.100.2") {
			t.Fatalf("restored origin %q", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("offline restore did not connect")
	}
}

func TestWebIRCEndpointEditDoesNotForwardGateway(t *testing.T) {
	first, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	t.Setenv("TEST_WEBIRC_PASSWORD", "fixture-webirc-secret")
	text := fmt.Sprintf("webirc:\n  - host: 127.0.0.1\n    port: %d\n    tls: false\n    passwordEnv: TEST_WEBIRC_PASSWORD\n", first.Addr().(*net.TCPAddr).Port)
	list, err := LoadServerList(writeServerList(t, text), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 2)
	serve := func(listener net.Listener) {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		initial := true
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if initial {
				lines <- line
				initial = false
			}
			if strings.HasPrefix(line, "CAP LS") {
				_, _ = conn.Write([]byte(":stub CAP * LS :\r\n"))
			}
			if strings.HasPrefix(line, "USER ") {
				_, _ = conn.Write([]byte(":stub 001 alice :Welcome\r\n:stub 376 alice :End\r\n"))
			}
		}
	}
	go serve(first)
	go serve(second)
	m := New(Options{AllowPrivate: true, ServerList: list, Origin: func(string) (string, error) { return "198.51.100.1", nil }})
	defer m.Close()
	cfg := lifecycleConfig(first.Addr().(*net.TCPAddr).Port)
	if _, err := m.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	if line := <-lines; !strings.HasPrefix(line, "WEBIRC ") {
		t.Fatal("authorized endpoint did not receive WEBIRC")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	cfg.Port = second.Addr().(*net.TCPAddr).Port
	if _, err := m.UpdateNetwork("alice", "net", cfg); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-lines:
		if !strings.HasPrefix(line, "CAP LS") {
			t.Fatal("gateway command reached edited endpoint")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("edited endpoint did not connect")
	}
}
