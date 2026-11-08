package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/admin"
	"ircord/backend/internal/auth"
)

func adminPost(t *testing.T, client *http.Client, url, bearer string, body any) (int, []byte) {
	t.Helper()
	raw, _ := json.Marshal(body)
	r, _ := http.NewRequest("POST", url, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	out, _ := io.ReadAll(response.Body)
	if !strings.Contains(response.Header.Get("Cache-Control"), "no-store") {
		t.Fatal("admin response is cacheable")
	}
	return response.StatusCode, out
}

func TestAdminPasswordReset(t *testing.T) {
	base, _, store, sessions := authenticatedApp(t)
	token := strings.Repeat("ab", 32)
	verifier, _ := admin.NewVerifier(token)
	app := New(base.Manager, Options{Store: store, Auth: sessions, Files: base.options.Files, Admin: verifier})
	server := httptest.NewServer(app.Handler)
	defer server.Close()
	user := cookieClient()
	target := register(t, user, server.URL)
	bystander := cookieClient()
	unrelated := register(t, bystander, server.URL)
	socket := accountSocket(t, user, server.URL, 0)
	url := func(name string) string { return server.URL + admin.Prefix + "/accounts/" + name + "/password" }
	secret := "operator-chosen-pass"

	// Browser sessions and bot-style credentials can't reset anyone, including themselves.
	for _, c := range []struct {
		client *http.Client
		bearer string
	}{{http.DefaultClient, ""}, {user, ""}, {user, strings.Repeat("cd", 32)}, {http.DefaultClient, "bot-other-principal"}} {
		if code, _ := adminPost(t, c.client, url(target.Username), c.bearer, map[string]string{"password": secret}); code != 401 {
			t.Fatalf("unauthorized reset answered %d", code)
		}
	}
	code, body := adminPost(t, http.DefaultClient, url(target.Username), token, map[string]string{"password": "short"})
	if code != 422 || !strings.Contains(string(body), "at least 8 characters") {
		t.Fatalf("weak password: %d %s", code, body)
	}
	// Rejected bodies never echo the password back, here or on any other endpoint.
	long := "SECRETMARK" + strings.Repeat("y", 290)
	code, body = adminPost(t, http.DefaultClient, url(target.Username), token, map[string]string{"password": long})
	if code != 422 || strings.Contains(string(body), "SECRETMARK") || !strings.Contains(string(body), "128 characters or fewer") {
		t.Fatalf("long password: %d %s", code, body)
	}
	code, body = adminPost(t, http.DefaultClient, url(target.Username), token, map[string]any{"password": "SECRETMARK-ok", "extra": 1})
	if code != 422 || strings.Contains(string(body), "SECRETMARK") {
		t.Fatalf("unexpected property: %d %s", code, body)
	}
	code, body = adminPost(t, http.DefaultClient, server.URL+"/api/v1/auth/login", "", map[string]string{"username": target.Username, "password": long})
	if code != 422 || strings.Contains(string(body), "SECRETMARK") || !strings.Contains(string(body), "body.password") {
		t.Fatalf("login validation echoed the password: %d %s", code, body)
	}
	if code, body = adminPost(t, http.DefaultClient, url("nobody_here"), token, map[string]string{"password": secret}); code != 404 {
		t.Fatalf("unknown account: %d %s", code, body)
	}
	// Nothing changed yet.
	request(t, user, "GET", server.URL+"/api/v1/bootstrap", nil, 200)

	code, body = adminPost(t, http.DefaultClient, url(target.Username), token, map[string]string{"password": secret})
	var result auth.PasswordResetResult
	if code != 200 || json.Unmarshal(body, &result) != nil {
		t.Fatalf("reset: %d %s", code, body)
	}
	if result.Username != target.Username || result.Status != "completed" || result.SessionsRevoked != 1 || result.Associated {
		t.Fatalf("result = %+v", result)
	}
	if strings.Contains(string(body), secret) || strings.Contains(string(body), "argon2") || strings.Contains(string(body), token) {
		t.Fatal("credential material in the reset response")
	}
	// The prior browser session and its live connection end; the old password stops working.
	request(t, user, "GET", server.URL+"/api/v1/bootstrap", nil, 401)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if _, _, err := socket.Read(ctx); err != nil {
			if ctx.Err() != nil {
				t.Fatal("live connection stayed open after the reset")
			}
			break
		}
	}
	request(t, cookieClient(), "POST", server.URL+"/api/v1/auth/login", target.credentials(), 401)
	request(t, cookieClient(), "POST", server.URL+"/api/v1/auth/login", map[string]string{"username": target.Username, "password": secret}, 200)
	// Other accounts keep their sessions and passwords.
	request(t, bystander, "GET", server.URL+"/api/v1/bootstrap", nil, 200)
	request(t, cookieClient(), "POST", server.URL+"/api/v1/auth/login", unrelated.credentials(), 200)

	events, err := store.AdminEvents(10)
	if err != nil || len(events) != 4 || events[0].Target != target.Username || events[0].Outcome != "completed" {
		t.Fatal("audit log", events, err)
	}

	operation := app.API.OpenAPI().Paths[admin.Prefix+"/accounts/{username}/password"].Post
	if operation == nil || len(operation.Security) != 1 || operation.Security[0]["adminBearer"] == nil {
		t.Fatal("reset operation does not declare its admin boundary")
	}
}

func TestAdminPasswordResetWithoutAccounts(t *testing.T) {
	base, _, store, _ := authenticatedApp(t)
	token := strings.Repeat("ab", 32)
	verifier, _ := admin.NewVerifier(token)
	app := New(base.Manager, Options{Store: store, Admin: verifier, Identity: func(http.ResponseWriter, *http.Request) (string, error) { return "", nil }})
	server := httptest.NewServer(app.Handler)
	defer server.Close()
	if code, _ := adminPost(t, http.DefaultClient, server.URL+admin.Prefix+"/accounts/alice/password", token, map[string]string{"password": "operator-chosen"}); code != 501 {
		t.Fatalf("reset without accounts answered %d", code)
	}
}
