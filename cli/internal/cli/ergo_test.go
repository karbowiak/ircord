package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCapabilityAwareCommandTreeAndStatus(t *testing.T) {
	cleanEnvironment(t)
	token := strings.Repeat("ab", 32)
	t.Setenv("IRCORD_ADMIN_TOKEN", token)
	var associated atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("discovery did not use admin authentication")
		}
		switch r.URL.Path {
		case "/api/v1/admin/capabilities":
			features := []string{}
			if associated.Load() {
				features = append(features, "ergo")
			}
			json.NewEncoder(w).Encode(map[string]any{"features": features})
		case "/api/v1/admin/status":
			value := map[string]any{"status": "ok", "database": "available", "version": "dev", "commit": "test", "goVersion": "go-test", "startedAt": time.Now().UTC(), "uptimeSeconds": 1}
			if associated.Load() {
				value["ergo"] = map[string]any{"server": "irc.example.com:6697", "state": "unavailable", "checkedAt": time.Now().UTC(), "error": "offline"}
				value["status"] = "degraded"
			}
			json.NewEncoder(w).Encode(value)
		case "/api/v1/admin/doctor":
			json.NewEncoder(w).Encode(map[string]any{"status": "ok", "checks": []any{map[string]string{"name": "backend", "status": "ok", "detail": "reachable"}}})
		case "/api/v1/admin/ergo/channels":
			json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"name": "#test", "users": 2, "topic": "hello"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("IRCORD_SERVER_URL", server.URL)
	for _, enabled := range []bool{false, true, false} {
		associated.Store(enabled)
		code, out, errOut := execute(t, "--help", "--json")
		if code != 0 || errOut != "" {
			t.Fatal("help failed", errOut)
		}
		var help struct{ Commands []helpEntry }
		if json.Unmarshal([]byte(out), &help) != nil {
			t.Fatal("invalid help")
		}
		found := false
		for _, cmd := range help.Commands {
			if cmd.Name == "ergo" {
				found = true
			}
		}
		if found != enabled {
			t.Fatal("association was not reflected in command discovery")
		}
		_, out, _ = execute(t, "__complete", "")
		if strings.Contains(out, "ergo\t") != enabled {
			t.Fatal("completion advertises an unavailable integration", out)
		}
		code, out, _ = execute(t, "status", "--json")
		if strings.Contains(out, `"ergo"`) != enabled {
			t.Fatal("incorrect optional status output")
		}
		if enabled && code != 1 || !enabled && code != 0 {
			t.Fatal("partial status exit code incorrect", code)
		}
	}
	associated.Store(true)
	code, out, errOut := execute(t, "ergo", "channels", "list", "--json")
	if code != 0 || !strings.Contains(out, "#test") || errOut != "" {
		t.Fatal("read-only command failed", code, errOut)
	}
	code, out, errOut = execute(t, "doctor", "--json")
	if code != 0 || !json.Valid([]byte(out)) || errOut != "" {
		t.Fatal("doctor failed")
	}
}
