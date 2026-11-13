package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ircord/backend/internal/irc"
)

func TestPresetAPIExposesOnlyPublicFields(t *testing.T) {
	t.Setenv("TEST_WEBIRC_API_SECRET", "fixture-api-secret")
	path := filepath.Join(t.TempDir(), "servers.yaml")
	data := "servers:\n  - name: Example\n    host: irc.example\n    authMethod: server-password\nwebirc:\n  - host: irc.example\n    passwordEnv: TEST_WEBIRC_API_SECRET\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	list, err := irc.LoadServerList(path, map[string]string{"irc.example:6697": "fixture-server-secret"}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, server, _, _ := authenticatedApp(t, func(o *irc.Options) { o.ServerList = list })
	request(t, http.DefaultClient, http.MethodGet, server.URL+"/api/v1/networks/presets", nil, http.StatusUnauthorized)
	client := cookieClient()
	register(t, client, server.URL)
	raw := request(t, client, http.MethodGet, server.URL+"/api/v1/networks/presets", nil, http.StatusOK)
	var page NetworkPresetPage
	if err := json.Unmarshal(raw, &page); err != nil || !page.Configured || len(page.Items) != 1 || !page.Items[0].PasswordProvided {
		t.Fatalf("bad preset response: %s %v", raw, err)
	}
	for _, private := range []string{"fixture-api-secret", "fixture-server-secret", "TEST_WEBIRC_API_SECRET", "webirc"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("preset response disclosed private configuration")
		}
	}
}
