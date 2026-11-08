package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// resetBackend answers password resets like the backend, recording what it received.
type resetBackend struct {
	*httptest.Server
	calls    atomic.Int32
	received atomic.Value
}

func newResetBackend(t *testing.T, token string, answer func(w http.ResponseWriter, password string)) *resetBackend {
	t.Helper()
	b := &resetBackend{}
	b.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.calls.Add(1)
		if r.Method != "POST" || r.URL.Path != "/api/v1/admin/accounts/alice/password" || r.Header.Get("Authorization") != "Bearer "+token || r.URL.RawQuery != "" {
			t.Error("incorrect reset request", r.Method, r.URL.Path)
		}
		var body struct{ Password string }
		raw, _ := io.ReadAll(r.Body)
		if json.Unmarshal(raw, &body) != nil {
			t.Error("reset body is not JSON")
		}
		b.received.Store(body.Password)
		answer(w, body.Password)
	}))
	t.Cleanup(b.Close)
	return b
}

func problem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "detail": detail})
}

func completed(w http.ResponseWriter, _ string) {
	_ = json.NewEncoder(w).Encode(map[string]any{"username": "alice", "status": "completed", "sessionsRevoked": 2, "associated": true})
}

func withStdin(t *testing.T, input string) {
	t.Helper()
	previous := stdin
	stdin = strings.NewReader(input)
	t.Cleanup(func() { stdin = previous })
}

func TestResetPasswordFromStdinAndFile(t *testing.T) {
	cleanEnvironment(t)
	token := strings.Repeat("ab", 32)
	t.Setenv("IRCORD_ADMIN_TOKEN", token)
	secret := "operator-chosen-pass"
	backend := newResetBackend(t, token, completed)
	withStdin(t, secret+"\n")
	code, out, errOut := execute(t, "--json", "--server", backend.URL, "accounts", "reset-password", "alice", "--password-stdin")
	var result struct {
		Username, Status string
		SessionsRevoked  int
	}
	if code != 0 || errOut != "" || json.Unmarshal([]byte(out), &result) != nil || result.Status != "completed" || result.SessionsRevoked != 2 {
		t.Fatal("stdin reset", code, out, errOut)
	}
	if backend.received.Load() != secret || strings.Contains(out, secret) || strings.Contains(out, token) {
		t.Fatal("password not sent as given, or echoed")
	}
	file := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(file, []byte(secret+"\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut = execute(t, "--server", backend.URL, "accounts", "reset-password", "alice", "--password-file", file)
	if code != 0 || errOut != "" || !strings.Contains(out, "Sessions signed out") || !strings.Contains(out, "updated") || strings.Contains(out, secret) {
		t.Fatal("file reset", code, out, errOut)
	}
	if backend.received.Load() != secret {
		t.Fatal("password file not read as given")
	}
}

func TestResetPasswordInputIsNeverAnArgumentAndFailsClosed(t *testing.T) {
	cleanEnvironment(t)
	token := strings.Repeat("ab", 32)
	t.Setenv("IRCORD_ADMIN_TOKEN", token)
	backend := newResetBackend(t, token, completed)
	withStdin(t, "")
	previous := terminalInput
	terminalInput = func(io.Reader) (int, bool) { return 0, false }
	t.Cleanup(func() { terminalInput = previous })
	dir := t.TempDir()
	for _, args := range [][]string{
		{"accounts", "reset-password", "alice"},                                // no terminal for the prompt
		{"accounts", "reset-password", "alice", "--password", "in-argv"},       // no argument form exists
		{"accounts", "reset-password", "alice", "--password-stdin"},            // empty input
		{"accounts", "reset-password", "alice", "--password-file", dir},        // not a regular file
		{"accounts", "reset-password", "alice", "--password-file", dir + "/x"}, // missing
		{"accounts", "reset-password", "alice", "--password-stdin", "--password-file", dir + "/x"},
		{"accounts", "reset-password"},
	} {
		code, out, errOut := execute(t, append([]string{"--json", "--server", backend.URL}, args...)...)
		if code != 2 || out != "" || !json.Valid([]byte(errOut)) || strings.Contains(errOut, "in-argv") {
			t.Fatalf("%v: code=%d err=%s", args, code, errOut)
		}
	}
	withStdin(t, strings.Repeat("x", 5000))
	if code, _, _ := execute(t, "--server", backend.URL, "accounts", "reset-password", "alice", "--password-stdin"); code != 2 {
		t.Fatal("oversized input accepted")
	}
	if backend.calls.Load() != 0 {
		t.Fatal("the backend was called without a usable password")
	}
}

func TestResetPasswordHiddenPrompt(t *testing.T) {
	cleanEnvironment(t)
	token := strings.Repeat("ab", 32)
	t.Setenv("IRCORD_ADMIN_TOKEN", token)
	backend := newResetBackend(t, token, completed)
	previousTerminal, previousRead := terminalInput, readHidden
	t.Cleanup(func() { terminalInput, readHidden = previousTerminal, previousRead })
	terminalInput = func(io.Reader) (int, bool) { return 7, true }
	entries := []string{}
	readHidden = func(_ context.Context, fd int) ([]byte, error) {
		if fd != 7 || len(entries) == 0 {
			return nil, errors.New("unexpected read")
		}
		next := entries[0]
		entries = entries[1:]
		return []byte(next), nil
	}
	entries = []string{"first-choice", "second-choice"}
	code, _, errOut := execute(t, "--server", backend.URL, "accounts", "reset-password", "alice")
	if code != 2 || backend.calls.Load() != 0 || !strings.Contains(errOut, "don’t match") || strings.Contains(errOut, "choice") {
		t.Fatal("mismatched entries", code, errOut)
	}
	entries = []string{"prompted-password", "prompted-password"}
	code, out, errOut := execute(t, "--server", backend.URL, "accounts", "reset-password", "alice")
	if code != 0 || backend.received.Load() != "prompted-password" || !strings.Contains(errOut, "New password for alice") || strings.Contains(errOut+out, "prompted-password") {
		t.Fatal("prompted reset", code, out, errOut)
	}
}

func TestResetPasswordOutcomesAndExitCodes(t *testing.T) {
	cleanEnvironment(t)
	token := strings.Repeat("ab", 32)
	t.Setenv("IRCORD_ADMIN_TOKEN", token)
	secret := "operator-chosen-pass"
	for _, c := range []struct {
		name   string
		answer func(http.ResponseWriter, string)
		code   int
		stdout bool
		says   string
	}{
		{"pending", func(w http.ResponseWriter, _ string) {
			_ = json.NewEncoder(w).Encode(map[string]any{"username": "alice", "status": "pending", "sessionsRevoked": 1, "associated": true, "detail": "The associated IRC server couldn’t be reached; the backend keeps retrying."})
		}, 1, true, "pending"},
		{"invalid", func(w http.ResponseWriter, _ string) {
			problem(w, 422, "Your password can’t contain spaces.")
		}, 2, false, "can’t contain spaces"},
		{"reflected-secret", func(w http.ResponseWriter, password string) {
			problem(w, 422, "rejected "+password+" with "+token+"\x1b[31m")
		}, 2, false, "[redacted]"},
		{"unknown", func(w http.ResponseWriter, _ string) { problem(w, 404, "No IRCord account has that username.") }, 5, false, "No IRCord account"},
		{"no-accounts", func(w http.ResponseWriter, _ string) { problem(w, 501, "this backend has no IRCord accounts") }, 5, false, "no IRCord accounts"},
		{"admin-rejected", func(w http.ResponseWriter, _ string) { problem(w, 401, "admin authentication required") }, 3, false, "admin authentication rejected"},
		{"unavailable", func(w http.ResponseWriter, _ string) {
			problem(w, 503, "The IRC server couldn’t be reached; try again shortly.")
		}, 4, false, "couldn’t be reached"},
		{"malformed", func(w http.ResponseWriter, _ string) { _, _ = w.Write([]byte(`{"status":"done"}`)) }, 4, false, "invalid password reset response"},
	} {
		t.Run(c.name, func(t *testing.T) {
			backend := newResetBackend(t, token, c.answer)
			withStdin(t, secret)
			code, out, errOut := execute(t, "--json", "--server", backend.URL, "accounts", "reset-password", "alice", "--password-stdin")
			var failure struct {
				Error string
				Code  int
			}
			if code != c.code || json.Unmarshal([]byte(errOut), &failure) != nil || failure.Code != c.code || !strings.Contains(failure.Error+out, c.says) {
				t.Fatalf("code=%d out=%s err=%s", code, out, errOut)
			}
			if (out != "") != c.stdout || !json.Valid([]byte(out)) && out != "" {
				t.Fatal("stdout", out)
			}
			if strings.Contains(out+errOut, secret) || strings.Contains(out+errOut, token) || strings.Contains(errOut, "\x1b") {
				t.Fatal("secret or control sequence in output")
			}
		})
	}
}

func TestResetPasswordTimeoutAndCancellation(t *testing.T) {
	cleanEnvironment(t)
	token := strings.Repeat("ab", 32)
	t.Setenv("IRCORD_ADMIN_TOKEN", token)
	release := make(chan struct{})
	backend := newResetBackend(t, token, func(w http.ResponseWriter, _ string) { <-release })
	defer close(release)
	withStdin(t, "operator-chosen-pass")
	code, _, errOut := execute(t, "--server", backend.URL, "--timeout", "200ms", "accounts", "reset-password", "alice", "--password-stdin")
	if code != 4 || !strings.Contains(errOut, "run the command again with the same password") {
		t.Fatal("timeout", code, errOut)
	}
	var out, errBuf strings.Builder
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	withStdin(t, "operator-chosen-pass")
	if code := Execute(cancelled, []string{"--server", backend.URL, "accounts", "reset-password", "alice", "--password-stdin"}, &out, &errBuf); code != 130 {
		t.Fatal("cancelled reset", code)
	}
}
