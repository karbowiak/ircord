package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"ircord/backend/internal/auth"
)

func TestRegistrationGateHTTPAndPublicPolicy(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	service := auth.New(store, auth.Options{Registration: true, RegistrationKey: "http-fixture-key"})
	defer service.Close()
	app.options.Auth = service
	client := cookieClient()
	policy := request(t, client, "GET", server.URL+"/api/v1/auth/registration", nil, 200)
	if strings.Contains(string(policy), "http-fixture-key") || !strings.Contains(string(policy), `"registrationKeyRequired":true`) {
		t.Fatal("public policy does not describe the gate safely")
	}
	for _, key := range []string{"", "wrong"} {
		request(t, client, "POST", server.URL+"/api/v1/auth/signup", map[string]string{"username": "gated", "password": "correct-horse", "registrationKey": key}, 403)
	}
	var count int
	store.DB.QueryRow("SELECT count(*) FROM accounts").Scan(&count)
	if count != 0 {
		t.Fatal("rejected HTTP registration created an account")
	}
	request(t, client, "POST", server.URL+"/api/v1/auth/signup", map[string]string{"username": "gated", "password": "correct-horse", "registrationKey": "http-fixture-key"}, 201)
	request(t, client, "POST", server.URL+"/api/v1/auth/logout", nil, 204)
	request(t, client, "POST", server.URL+"/api/v1/auth/login", map[string]string{"username": "gated", "password": "correct-horse"}, 200)
	request(t, client, "GET", server.URL+"/api/v1/bootstrap", nil, 200)
	// The username check answers only with the sign-up key, and then says why a name can't be used.
	request(t, cookieClient(), "GET", server.URL+"/api/v1/auth/username?name=GATED", nil, 403)
	check := func(name, key string) string {
		t.Helper()
		r, _ := http.NewRequest("GET", server.URL+"/api/v1/auth/username?name="+name, nil)
		r.Header.Set("X-Registration-Key", key)
		res, err := cookieClient().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return fmt.Sprint(res.StatusCode, " ", string(body))
	}
	if got := check("GATED", "wrong"); !strings.HasPrefix(got, "403 ") {
		t.Fatal("a wrong key got an answer:", got)
	}
	if got := check("GATED", "http-fixture-key"); !strings.Contains(got, `"available":false`) {
		t.Fatal("taken username reported available", got)
	}
	if got := check("fresh", "http-fixture-key"); !strings.Contains(got, `"available":true`) {
		t.Fatal("free username reported taken", got)
	}
}
