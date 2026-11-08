package ergo

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixtureStatus() map[string]any {
	return map[string]any{"success": true, "version": "ergo-test", "go_version": "go-test", "start_time": time.Now().Add(-time.Hour).UTC(), "users": map[string]int{"total": 3, "invisible": 1, "operators": 0, "unknown": 0, "max": 5}, "channels": 2, "servers": 1}
}
func newTestClient(t *testing.T, url string) *Client {
	t.Helper()
	c, err := New(Options{Server: "irc.example.com:6697", URL: url, Token: strings.Repeat("ab", 32)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}
func TestReadOnlyAdapter(t *testing.T) {
	token := strings.Repeat("ab", 32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer "+token || r.URL.RawQuery != "" {
			t.Error("invalid vendor transport")
		}
		var result any
		switch r.URL.Path {
		case "/v1/status":
			result = fixtureStatus()
		case "/v1/list":
			result = map[string]any{"success": true, "channels": []any{map[string]any{"name": "#test", "userCount": 2, "topic": "safe " + token, "secret": true, "hasKey": true}}}
		case "/v1/whois":
			result = map[string]any{"success": true, "present": true, "nickname": "tester", "session_count": 2, "hostname": "private.example", "channels": []any{map[string]string{"name": "#test", "mode": "o"}}}
		case "/v1/ns/list":
			result = map[string]any{"success": true, "accounts": []any{map[string]any{"success": true, "accountName": "tester", "email": "private@example.com"}}}
		case "/v1/ns/info":
			result = map[string]any{"success": true, "accountName": "tester", "registeredAt": time.Now().UTC(), "email": "private@example.com", "channels": []string{"#test"}}
		default:
			t.Error("unexpected vendor operation")
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	c := newTestClient(t, server.URL)
	status := c.Status(context.Background())
	if status.State != "ok" || status.Statistics == nil || status.Statistics.Users.Total != 3 || status.Statistics.UptimeSeconds < 3590 {
		t.Fatal("invalid normalized statistics", status)
	}
	channels, err := c.Channels(context.Background())
	if err != nil || len(channels) != 1 || !channels[0].Secret || strings.Contains(channels[0].Topic, token) {
		t.Fatal("invalid/redaction failure", err)
	}
	u, err := c.User(context.Background(), "tester")
	if err != nil || u.Sessions != 2 || len(u.Channels) != 1 {
		t.Fatal("invalid nickname response", err)
	}
	a, err := c.Accounts(context.Background())
	if err != nil || len(a) != 1 || a[0].Channels == nil {
		t.Fatal("invalid account list", err)
	}
	one, err := c.Account(context.Background(), "tester")
	if err != nil || one.RegisteredAt == nil {
		t.Fatal("invalid account details", err)
	}
	b, _ := json.Marshal([]any{u, a, one})
	if strings.Contains(string(b), "private@example.com") || strings.Contains(string(b), "private.example") {
		t.Fatal("unselected private vendor fields were forwarded")
	}
}
func TestVerifiedTLSAndPrivateTransport(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(fixtureStatus()) }))
	defer server.Close()
	c := newTestClient(t, server.URL)
	if c.Status(context.Background()).State != "unavailable" {
		t.Fatal("untrusted certificate accepted")
	}
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	trusted, err := New(Options{Server: "irc.example.com:6697", URL: server.URL, Token: strings.Repeat("ab", 32), Roots: roots, TLSServerName: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	defer trusted.Close()
	if trusted.Status(context.Background()).State != "ok" {
		t.Fatal("verified TLS with service-name override failed")
	}
	if trusted.http.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("vendor credential can use an environment proxy")
	}
}
func TestFailuresAreBoundedAndSanitized(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer target.Close()
	for _, tc := range []struct {
		name     string
		code     int
		body     string
		redirect bool
	}{
		{"redirect", 307, "", true}, {"auth", 401, strings.Repeat("ab", 32), false}, {"oversize", 200, strings.Repeat("x", (2<<20)+1), false}, {"missing", 200, `{"success":true,"version":"fake","start_time":"2026-01-01T00:00:00Z"}`, false}, {"invalid", 200, `{"success":false}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.redirect {
					http.Redirect(w, r, target.URL, tc.code)
					return
				}
				w.WriteHeader(tc.code)
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			c := newTestClient(t, server.URL)
			s := c.Status(context.Background())
			b, _ := json.Marshal(s)
			if s.State != "unavailable" || s.Statistics != nil || strings.Contains(string(b), strings.Repeat("ab", 32)) || strings.Contains(string(b), server.URL) {
				t.Fatal("unsafe/unavailable vendor status", s.State)
			}
		})
	}
	if redirected.Load() != 0 {
		t.Fatal("credential followed a redirect")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(io.Discard, r.Body); <-r.Context().Done() }))
	defer server.Close()
	c := newTestClient(t, server.URL)
	c.http.Timeout = 25 * time.Millisecond
	start := time.Now()
	if c.Status(context.Background()).State != "unavailable" || time.Since(start) > time.Second {
		t.Fatal("request deadline not applied")
	}
	for i := 0; i < cap(c.slots); i++ {
		c.slots <- struct{}{}
	}
	start = time.Now()
	if c.Status(context.Background()).State != "unavailable" || time.Since(start) > time.Second {
		t.Fatal("queue deadline not applied")
	}
}
func TestOptionalConfiguration(t *testing.T) {
	c, err := FromEnvironment(func(string) string { return "" })
	if c != nil || err != nil {
		t.Fatal("unconfigured association was not absent")
	}
	for _, url := range []string{"http://ergo.ergo.svc:8089", "https://user:secret@ergo.example", "https://ergo.example?token=secret", "https://ergo.example#fragment"} {
		_, err := New(Options{Server: "irc.example.com:6697", URL: url, Token: strings.Repeat("ab", 32)})
		if err == nil || strings.Contains(err.Error(), "user:secret") {
			t.Fatal("unsafe configuration accepted")
		}
	}
	if _, err := FromEnvironment(func(k string) string {
		if k == "IRCORD_ERGO_API_URL" {
			return "https://ergo.example"
		}
		return ""
	}); err == nil {
		t.Fatal("partial association silently accepted")
	}
}

func TestAccountProvisioning(t *testing.T) {
	accounts := map[string]string{"taken": "x"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct{ AccountName, Passphrase string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		name := strings.ToLower(in.AccountName)
		result := map[string]any{"success": true}
		switch r.URL.Path {
		case "/v1/ns/saregister":
			if _, ok := accounts[name]; ok {
				result = map[string]any{"success": false, "errorCode": "ACCOUNT_EXISTS"}
			} else if strings.ContainsAny(in.AccountName, "!#") {
				result = map[string]any{"success": false, "errorCode": "UNKNOWN_ERROR", "error": "Account could not be created"}
			} else if strings.ContainsAny(in.Passphrase, " \t") {
				result = map[string]any{"success": false, "errorCode": "INVALID_PASSPHRASE", "error": "Passphrase " + in.Passphrase + " is invalid"}
			} else {
				accounts[name] = in.Passphrase
			}
		case "/v1/ns/passwd":
			if _, ok := accounts[name]; !ok {
				result = map[string]any{"success": false, "errorCode": "ACCOUNT_DOES_NOT_EXIST"}
			} else {
				accounts[name] = in.Passphrase
			}
		case "/v1/check_auth":
			result = map[string]any{"success": accounts[name] == in.Passphrase}
		case "/v1/ns/info":
			if _, ok := accounts[name]; !ok {
				result = map[string]any{"success": false}
			} else {
				result = map[string]any{"success": true, "accountName": in.AccountName}
			}
		default:
			t.Error("unexpected vendor operation", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	c := newTestClient(t, server.URL)
	ctx := context.Background()
	if err := c.Register(ctx, "Alice", "secret-pass"); err != nil || accounts["alice"] != "secret-pass" {
		t.Fatal("registration failed", err)
	}
	if err := c.Register(ctx, "TAKEN", "secret-pass"); !errors.Is(err, ErrAccountExists) {
		t.Fatal("existing account not reported", err)
	}
	// Ergo's own error text can echo the passphrase; it never reaches the caller.
	if err := c.Register(ctx, "bob", "has a space"); !errors.Is(err, ErrInvalidPassphrase) || strings.Contains(err.Error(), "has a space") {
		t.Fatal("invalid passphrase not reported safely", err)
	}
	if err := c.Register(ctx, "bad!name", "secret-pass"); !errors.Is(err, ErrAccountRefused) {
		t.Fatal("refused account name not reported", err)
	}
	if err := c.SetPassword(ctx, "alice", "next-pass"); err != nil || accounts["alice"] != "next-pass" {
		t.Fatal("password change failed", err)
	}
	for password, want := range map[string]bool{"next-pass": true, "wrong-password": false} {
		if got, err := c.CheckPassword(ctx, "alice", password); err != nil || got != want {
			t.Fatal("credential verification failed", err)
		}
	}
	if err := c.SetPassword(ctx, "nobody", "next-pass"); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing account not reported", err)
	}
	for name, want := range map[string]bool{"ALICE": true, "nobody": false} {
		if got, err := c.AccountExists(ctx, name); err != nil || got != want {
			t.Fatalf("AccountExists(%q) = %v, %v", name, got, err)
		}
	}
}
