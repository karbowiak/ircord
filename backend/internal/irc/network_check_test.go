package irc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNetworkCheckTLSAndAddressPolicy(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("address verification must not send application requests")
	}))
	defer server.Close()
	host, value, _ := net.SplitHostPort(server.Listener.Addr().String())
	port, _ := strconv.Atoi(value)
	private := New(Options{})
	defer private.Close()
	if err := private.CheckNetwork(context.Background(), host, port, true); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("verification bypassed private-address policy: %v", err)
	}
	untrusted := New(Options{AllowPrivate: true})
	defer untrusted.Close()
	if err := untrusted.CheckNetwork(context.Background(), host, port, true); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("verification accepted an invalid certificate: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	trusted := New(Options{AllowPrivate: true, TLSConfig: &tls.Config{RootCAs: roots}})
	defer trusted.Close()
	if err := trusted.CheckNetwork(context.Background(), host, port, true); err != nil {
		t.Fatal(err)
	}
	if len(trusted.Snapshot("owner").Networks) != 0 {
		t.Fatal("verification created a saved connection")
	}
}

func TestConnectionRegistrationDeadlineAndCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 2)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted <- conn
		}
	}()
	m := New(Options{AllowPrivate: true, ConnectTimeout: 150 * time.Millisecond})
	defer m.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if _, err = m.Connect("owner", NetworkConfig{ID: "silent", Name: "Silent server", Host: "127.0.0.1", Port: port, Nick: "deadline"}); err != nil {
		t.Fatal(err)
	}
	conn := <-accepted
	defer conn.Close()
	eventually(t, func() bool {
		n := m.Snapshot("owner").Networks[0]
		return n.Status == "connecting" && strings.Contains(n.StatusDetail, "timed out")
	})
	if err = m.Reconnect("owner", "silent"); err != nil {
		t.Fatal(err)
	}
	conn = <-accepted
	defer conn.Close()
	start := time.Now()
	if err = m.Disconnect("owner", "silent"); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancel waited for registration timeout")
	}
}
