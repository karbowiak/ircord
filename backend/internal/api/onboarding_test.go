package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestOnboardingCompletionHTTP(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	first, other := cookieClient(), cookieClient()
	a := register(t, first, server.URL)
	b := register(t, other, server.URL)
	if a.Account.OnboardingNetworkID != "" || b.Account.OnboardingNetworkID != "" {
		t.Fatal("unassociated signup became eligible")
	}
	// Seed pending accounts for completion-route tests; auth tests cover eligibility.
	if _, err := store.DB.Exec("UPDATE accounts SET onboarding_network_id='home' WHERE id IN (?,?)", a.Account.ID, b.Account.ID); err != nil {
		t.Fatal(err)
	}
	second := cookieClient()
	var login struct {
		Account struct {
			OnboardingNetworkID string `json:"onboardingNetworkId"`
		} `json:"account"`
	}
	if err := json.Unmarshal(request(t, second, "POST", server.URL+"/api/v1/auth/login", a.credentials(), 200), &login); err != nil || login.Account.OnboardingNetworkID != "home" {
		t.Fatal("login omitted pending setup", err)
	}
	path := server.URL + "/api/v1/auth/onboarding/complete"
	for _, tc := range []struct {
		name, origin, bearer string
		client               *http.Client
		status               int
	}{
		{name: "anonymous", client: cookieClient(), status: 401},
		{name: "bot-token", client: cookieClient(), bearer: "bot-other-principal", status: 401},
		{name: "foreign-origin", client: first, origin: "https://attacker.invalid", status: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := http.NewRequest("POST", path, nil)
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("Origin", tc.origin)
			if tc.bearer != "" {
				r.Header.Set("Authorization", "Bearer "+tc.bearer)
			}
			response, err := tc.client.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("got %d, want %d", response.StatusCode, tc.status)
			}
		})
	}
	// Failed completion must leave setup resumable.
	if _, err := store.DB.Exec(`CREATE TRIGGER fail_completion BEFORE UPDATE OF onboarding_network_id ON accounts BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	request(t, first, "POST", path, nil, 503)
	if raw := request(t, first, "GET", server.URL+"/api/v1/auth/session", nil, 200); !strings.Contains(string(raw), `"onboardingNetworkId":"home"`) {
		t.Fatal("failed completion cleared pending setup")
	}
	if _, err := store.DB.Exec("DROP TRIGGER fail_completion"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		// No request body, and a foreign owner query cannot change the target account.
		r, err := http.NewRequest("POST", path+"?owner="+b.Account.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Origin", server.URL)
		response, err := first.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 204 || response.ContentLength > 0 {
			t.Fatal("completion must return an empty 204", response.StatusCode)
		}
	}
	for _, client := range []*http.Client{first, second} {
		if raw := request(t, client, "GET", server.URL+"/api/v1/auth/session", nil, 200); strings.Contains(string(raw), "onboardingNetworkId") {
			t.Fatal("completion not visible to every signed-in device")
		}
	}
	if raw := request(t, other, "GET", server.URL+"/api/v1/auth/session", nil, 200); !strings.Contains(string(raw), `"onboardingNetworkId":"home"`) {
		t.Fatal("completion changed another account")
	}
	request(t, first, "POST", server.URL+"/api/v1/auth/logout", nil, 204)
	request(t, first, "POST", path, nil, 401)
	if raw := request(t, cookieClient(), "POST", server.URL+"/api/v1/auth/login", a.credentials(), 200); strings.Contains(string(raw), "onboardingNetworkId") {
		t.Fatal("new login rearmed completed setup")
	}
}
