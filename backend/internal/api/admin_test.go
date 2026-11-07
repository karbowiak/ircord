package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ircord/backend/internal/admin"
	"ircord/backend/internal/buildinfo"
)

func TestAdminAuthenticationIsSeparateFromBrowserSessions(t *testing.T) {
	base, _, store, sessions := authenticatedApp(t)
	token := strings.Repeat("ab", 32)
	verifier, _ := admin.NewVerifier(token)
	app := New(base.Manager, Options{Store: store, Auth: sessions, Files: base.options.Files, Admin: verifier,
		Build: buildinfo.Info{Version: "v-test", Commit: "test-commit", GoVersion: "go-test"}})
	server := httptest.NewServer(app.Handler)
	defer server.Close()
	user := cookieClient()
	register(t, user, server.URL)
	for _, tc := range []struct {
		name, path, bearer, origin string
		client                     *http.Client
		want                       int
	}{
		{"anonymous", admin.Prefix + "/status", "", "", http.DefaultClient, 401},
		{"user-cookie", admin.Prefix + "/status", "", "", user, 401},
		{"wrong-bearer-with-cookie", admin.Prefix + "/status", strings.Repeat("cd", 32), "", user, 401},
		{"bot-token", admin.Prefix + "/status", "bot-other-principal", "", http.DefaultClient, 401},
		{"query-token", admin.Prefix + "/status?token=" + token, "", "", http.DefaultClient, 401},
		{"operator-without-cookie", admin.Prefix + "/status", token, "", http.DefaultClient, 200},
		{"foreign-origin", admin.Prefix + "/status", token, "https://untrusted.example", http.DefaultClient, 403},
		{"same-origin", admin.Prefix + "/status", token, server.URL, http.DefaultClient, 200},
		{"admin-is-not-user", "/api/v1/bootstrap", token, "", http.DefaultClient, 401},
		{"browser-unchanged", "/api/v1/bootstrap", "", "", user, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", server.URL+tc.path, nil)
			if tc.bearer != "" {
				r.Header.Set("Authorization", "Bearer "+tc.bearer)
			}
			r.Header.Set("Origin", tc.origin)
			response, err := tc.client.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode != tc.want || !strings.Contains(response.Header.Get("Cache-Control"), "no-store") || strings.Contains(string(body), token) {
				t.Fatalf("unexpected status/cache/redaction: %d", response.StatusCode)
			}
			if tc.want == 200 && strings.HasPrefix(tc.path, admin.Prefix) {
				var status admin.Status
				if json.Unmarshal(body, &status) != nil || status.Status != "ok" || status.Database != "available" || status.Version != "v-test" || status.StartedAt.IsZero() {
					t.Fatal("incorrect typed admin status")
				}
			}
		})
	}
	// Cross-origin preflights expose Authorization only on the admin group.
	r, _ := http.NewRequest("OPTIONS", server.URL+admin.Prefix+"/status", nil)
	r.Header.Set("Origin", server.URL)
	response, err := user.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 204 || !strings.Contains(response.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatal("admin preflight is incomplete")
	}
	security := app.API.OpenAPI().Paths[admin.Prefix+"/status"].Get.Security
	if len(security) != 1 || security[0]["adminBearer"] == nil {
		t.Fatal("admin operation does not declare its bearer boundary")
	}
	if _, exists := security[0]["sessionCookie"]; exists {
		t.Fatal("admin operation advertises cookie authentication")
	}
}

func TestAdminUnconfiguredAndDegraded(t *testing.T) {
	base, disabledServer, store, sessions := authenticatedApp(t)
	request(t, http.DefaultClient, "GET", disabledServer.URL+admin.Prefix+"/status", nil, 503)
	token := strings.Repeat("ab", 32)
	verifier, _ := admin.NewVerifier(token)
	app := New(base.Manager, Options{Store: store, Auth: sessions, Admin: verifier})
	if err := store.DB.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", admin.Prefix+"/status", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Handler.ServeHTTP(w, r)
	var status admin.Status
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &status) != nil || status.Status != "degraded" || status.Database != "unavailable" {
		t.Fatal("storage failure did not produce sanitized degraded status", w.Code)
	}
}
