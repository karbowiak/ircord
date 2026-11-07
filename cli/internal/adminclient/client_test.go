package adminclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ircord/cli/internal/contract"
)

func testStatus() contract.Status {
	return contract.Status{Status: "ok", Database: "available", StartedAt: time.Now().UTC(), Version: "dev", Commit: "test", GoVersion: "go-test"}
}

func TestClientCredentialTransportAndRedirects(t *testing.T) {
	token := strings.Repeat("ab", 32)
	var destinationCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationCalls.Add(1)
	}))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prefix/api/v1/admin/status" || r.Header.Get("Authorization") != "Bearer "+token || r.URL.RawQuery != "" || len(r.Cookies()) != 0 {
			t.Error("credential transport or path is incorrect")
		}
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	c, err := New(server.URL+"/prefix", token, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, err = c.Status(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 307 || destinationCalls.Load() != 0 {
		t.Fatal("redirect followed or incorrect failure", err)
	}
}

func TestClientURLAndTLSValidation(t *testing.T) {
	token := strings.Repeat("ab", 32)
	for _, server := range []string{"http://example.com", "https://user:" + token + "@example.com", "https://example.com?token=" + token, "https://example.com#fragment", "https://example.com?", "file:///tmp/server", "https://example.com/%2fprivate"} {
		_, err := New(server, token, time.Second)
		if err == nil || strings.Contains(err.Error(), token) {
			t.Fatal("unsafe URL accepted or leaked")
		}
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(testStatus())
	}))
	defer server.Close()
	c, err := New(server.URL, token, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Status(context.Background()); err == nil {
		t.Fatal("untrusted TLS certificate accepted")
	}
	// Trust the disposable CA only in this test; production has no insecure flag.
	c.http.Transport = server.Client().Transport
	if _, err := c.Status(context.Background()); err != nil {
		t.Fatal("verified TLS request failed", err)
	}
}

func TestClientTimeoutCancellationAndRedaction(t *testing.T) {
	token := strings.Repeat("ab", 32)
	for _, flush := range []bool{false, true} {
		t.Run(map[bool]string{false: "headers", true: "body"}[flush], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if flush {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer server.Close()
			c, _ := New(server.URL, token, 30*time.Millisecond)
			defer c.Close()
			if _, err := c.Status(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("request did not time out", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := c.Status(ctx); !errors.Is(err, context.Canceled) {
				t.Fatal("request ignored cancellation", err)
			}
		})
	}
	for _, code := range []int{200, 401, 503} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
			status := testStatus()
			status.Version = token
			_ = json.NewEncoder(w).Encode(status)
		}))
		c, _ := New(server.URL, token, time.Second)
		status, err := c.Status(context.Background())
		c.Close()
		server.Close()
		if err != nil && strings.Contains(err.Error(), token) || strings.Contains(status.Version, token) {
			t.Fatal("upstream reflected a credential into output")
		}
	}
}
