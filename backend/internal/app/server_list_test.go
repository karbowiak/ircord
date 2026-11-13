package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerListFailureStopsBeforeOpeningStorage(t *testing.T) {
	for _, tc := range []struct {
		name, config, secret string
		debug                bool
	}{
		{"missing secret", "webirc:\n  - host: irc.example\n    passwordEnv: NO_SUCH_WEBIRC_FIXTURE\n", "", false},
		{"debug mode", "webirc:\n  - host: irc.example\n    passwordEnv: TEST_WEBIRC_APP_SECRET\n", "fixture-app-secret", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv("GIRC_DEBUG", "")
			t.Setenv("TEST_WEBIRC_APP_SECRET", tc.secret)
			t.Setenv("NO_SUCH_WEBIRC_FIXTURE", "")
			if tc.debug {
				t.Setenv("GIRC_DEBUG", "true")
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "servers.yaml")
			if err := os.WriteFile(path, []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("IRCORD_SERVER_LIST_FILE", path)
			dataDir := filepath.Join(dir, "state")
			t.Setenv("IRCORD_DATA_DIR", dataDir)
			err := Run(context.Background())
			if err == nil || strings.Contains(err.Error(), "fixture-app-secret") || strings.Contains(err.Error(), "NO_SUCH") || strings.Contains(err.Error(), path) {
				t.Fatalf("unsafe startup error: %v", err)
			}
			if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
				t.Fatal("invalid configuration opened storage")
			}
		})
	}
}
