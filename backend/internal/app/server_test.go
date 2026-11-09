package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func cleanEnvironment(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "IRCORD_") {
			t.Setenv(name, "")
		}
	}
}

func TestInvalidConfigurationDoesNotOpenStorage(t *testing.T) {
	for _, settings := range []map[string]string{
		{"IRCORD_PUBLIC_URL": "ftp://example.invalid"},
		{"IRCORD_TRUSTED_PROXIES": "not-a-network"},
		{"IRCORD_REQUIRE_REGISTRATION_KEY": "true"},
		{"IRCORD_NETWORK_PASSWORDS": "{}", "IRCORD_NETWORK_PASSWORDS_FILE": "missing"},
		{"IRCORD_LINKMEDIA_TIMEOUT": "invalid"},
		{"IRCORD_FILE_STORAGE": "unknown"},
	} {
		t.Run(fmtSettings(settings), func(t *testing.T) {
			cleanEnvironment(t)
			dataDir := filepath.Join(t.TempDir(), "state")
			t.Setenv("IRCORD_DATA_DIR", dataDir)
			for name, value := range settings {
				t.Setenv(name, value)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := Run(ctx); err == nil || errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("invalid configuration did not fail before startup: %v", err)
			}
			if _, err := os.Stat(dataDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid configuration acquired storage: %v", err)
			}
		})
	}
}

// Names alone keep configuration values out of test output.
func fmtSettings(settings map[string]string) string {
	for name := range settings {
		return name
	}
	return "defaults"
}

func TestHTTPShutdownClosesListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listening := make(chan string, 1)
	server := &http.Server{Addr: "127.0.0.1:0", Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ready") }), BaseContext: func(listener net.Listener) context.Context {
		listening <- listener.Addr().String()
		return context.Background()
	}}
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server) }()
	var address string
	select {
	case address = <-listening:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not start")
	}
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://" + address)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal(response.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not return")
	}
	if conn, err := net.DialTimeout("tcp", address, time.Second); err == nil {
		conn.Close()
		t.Fatal("listener survived shutdown")
	}
}

func TestHTTPBindFailureReturns(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := serve(ctx, &http.Server{Addr: listener.Addr().String()}); err == nil || ctx.Err() != nil {
		t.Fatalf("listener failure was not reported immediately: %v", err)
	}
}
