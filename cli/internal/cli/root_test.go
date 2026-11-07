package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ircord/cli/internal/contract"
)

func cleanEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"IRCORD_ADMIN_TOKEN", "IRCORD_ADMIN_TOKEN_FILE", "IRCORD_CLI_CONFIG", "IRCORD_SERVER_URL", "IRCORD_CLI_TIMEOUT", "NO_COLOR"} {
		t.Setenv(name, "")
	}
	// Even an unusable backend data path and server-only secrets cannot affect
	// this independently built client.
	t.Setenv("IRCORD_DATA_DIR", "/dev/null/no-backend-storage")
	t.Setenv("IRCORD_NETWORK_PASSWORDS_FILE", "/missing-server-only-file")
}

func execute(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestOfflineCommandsIgnoreRuntimeConfiguration(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("IRCORD_CLI_CONFIG", "/does-not-exist")
	t.Setenv("IRCORD_ADMIN_TOKEN_FILE", "/does-not-exist")
	for _, args := range [][]string{nil, {"--help"}, {"help", "status"}, {"version"}, {"--version"}, {"completion", "zsh"}, {"completion", "bash"}, {"completion", "fish"}, {"completion", "powershell"}} {
		code, out, errOut := execute(t, args...)
		if code != 0 || out == "" || errOut != "" || strings.Contains(out, "\x1b") {
			t.Fatalf("offline command %v failed: code=%d error=%s", args, code, errOut)
		}
	}
	for _, args := range [][]string{{"--json", "version"}, {"--json", "--version"}, {"--json", "--help"}, {"--json", "completion", "zsh"}} {
		code, out, errOut := execute(t, args...)
		if code != 0 || !json.Valid([]byte(out)) || errOut != "" || strings.Contains(out, "\x1b") {
			t.Fatalf("invalid JSON command output: %v code=%d", args, code)
		}
	}
	code, out, errOut := execute(t, "--json", "unknown-command")
	if code != 2 || out != "" || !json.Valid([]byte(errOut)) {
		t.Fatal("parse failure is not structured or has incorrect exit status")
	}
	code, _, _ = execute(t, "serve")
	if code != 2 {
		t.Fatal("remote client must not expose backend startup")
	}
}

func TestRemoteStatusConfigurationAndOutput(t *testing.T) {
	cleanEnvironment(t)
	token := strings.Repeat("ab", 32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/status" || r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("Origin") != "" {
			t.Error("incorrect remote administration request")
		}
		_ = json.NewEncoder(w).Encode(contract.Status{Status: "ok", Database: "available", StartedAt: time.Now().UTC(), Version: "dev", Commit: "test", GoVersion: "go-test"})
	}))
	defer server.Close()
	dir := t.TempDir()
	file := filepath.Join(dir, "token")
	if err := os.WriteFile(file, []byte(token+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "client.json")
	b, _ := json.Marshal(remoteConfig{ServerURL: server.URL, AdminTokenFile: "token", Timeout: "1s"})
	if err := os.WriteFile(config, b, 0600); err != nil {
		t.Fatal(err)
	}
	// Resolve a relative credential file from its config directory, not cwd.
	code, out, errOut := execute(t, "--config", config, "status", "--json")
	if code != 0 || !json.Valid([]byte(out)) || errOut != "" || strings.Contains(out, token) {
		t.Fatal("config-file remote status failed", code, errOut)
	}
	t.Setenv("IRCORD_CLI_CONFIG", config)
	t.Setenv("IRCORD_SERVER_URL", "https://wrong.example")
	t.Setenv("IRCORD_ADMIN_TOKEN", "invalid-environment-token")
	t.Setenv("IRCORD_CLI_TIMEOUT", "invalid")
	code, out, errOut = execute(t, "status", "--server", server.URL, "--admin-token-file", file, "--timeout", "1s", "--no-color")
	if code != 0 || !strings.Contains(out, "Database") || errOut != "" || strings.Contains(out, "\x1b") {
		t.Fatal("explicit flags did not override environment", code, errOut)
	}
	t.Setenv("IRCORD_SERVER_URL", server.URL)
	t.Setenv("IRCORD_ADMIN_TOKEN", token)
	t.Setenv("IRCORD_CLI_TIMEOUT", "1s")
	code, _, errOut = execute(t, "status")
	if code != 0 || errOut != "" {
		t.Fatal("environment did not override config", code, errOut)
	}
}

func TestStatusFailuresAndCancellation(t *testing.T) {
	cleanEnvironment(t)
	code, out, errOut := execute(t, "--json", "status")
	if code != 2 || out != "" || !json.Valid([]byte(errOut)) {
		t.Fatal("missing credential did not produce a configuration failure")
	}
	token := strings.Repeat("ab", 32)
	t.Setenv("IRCORD_ADMIN_TOKEN", token)
	for _, status := range []int{401, 403, 404, 503} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(token)) // A reflected secret must not enter diagnostics.
		}))
		code, out, errOut = execute(t, "status", "--json", "--server", server.URL)
		server.Close()
		want := 4
		if status == 401 || status == 403 {
			want = 3
		}
		if code != want || out != "" || !json.Valid([]byte(errOut)) || strings.Contains(errOut, token) {
			t.Fatal("incorrect remote failure handling", code, want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code = Execute(ctx, []string{"status", "--server", "http://127.0.0.1:1"}, &stdout, &stderr)
	if code != 130 {
		t.Fatal("cancellation did not propagate", code)
	}
}

func TestInvalidConfigAndTimeoutDoNotLeakSecrets(t *testing.T) {
	cleanEnvironment(t)
	token := strings.Repeat("ab", 32)
	file := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(file, []byte(`{"adminToken":"`+token+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := execute(t, "--config", file, "status")
	if code != 2 || out != "" || strings.Contains(errOut, token) {
		t.Fatal("invalid config accepted or disclosed")
	}
	t.Setenv("IRCORD_ADMIN_TOKEN", token)
	code, _, _ = execute(t, "status", "--timeout", "0s")
	if code != 2 {
		t.Fatal("unbounded timeout accepted")
	}
}
