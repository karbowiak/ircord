package api

import (
	"encoding/json"
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
	for _, body := range []any{map[string]any{}, map[string]string{"registrationKey": "wrong"}} {
		request(t, client, "POST", server.URL+"/api/v1/auth/register", body, 403)
	}
	var count int
	store.DB.QueryRow("SELECT count(*) FROM accounts").Scan(&count)
	if count != 0 {
		t.Fatal("rejected HTTP registration created an account")
	}
	var created RegisteredAccount
	json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/auth/register", map[string]string{"registrationKey": "http-fixture-key"}, 201), &created)
	request(t, client, "POST", server.URL+"/api/v1/auth/logout", nil, 204)
	request(t, client, "POST", server.URL+"/api/v1/auth/login", map[string]string{"key": created.Key}, 200)
	request(t, client, "GET", server.URL+"/api/v1/bootstrap", nil, 200)
}
