package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ergoapi "ircord/backend/internal/ergo"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func TestPeopleHTTPAuthenticationScopeAndPrivateProjection(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ns/list" {
			t.Errorf("unexpected source %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "accounts": []any{
			map[string]any{"success": true, "accountName": "NativeAccount", "registeredAt": "2026-01-01T00:00:00Z", "channels": []string{"#private"}, "sessions": 7},
		}})
	}))
	defer upstream.Close()
	ergo, err := ergoapi.New(ergoapi.Options{Server: "irc.fixture:6697", URL: upstream.URL, Token: strings.Repeat("fixture-", 8)})
	if err != nil {
		t.Fatal(err)
	}
	defer ergo.Close()
	_, server, store, _ := authenticatedApp(t, func(options *irc.Options) { options.PartyAccounts = ergo })
	client := cookieClient()
	request(t, client, "GET", server.URL+"/api/v1/people?networkId=home&purpose=services", nil, 401)
	account := register(t, client, server.URL)
	if _, err = store.Commit(account.Account.ID, "network.updated", model.Network{ID: "home", Host: "irc.fixture", Port: 6697, TLS: true, Nick: "Caller"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Commit(account.Account.ID, "network.updated", model.Network{ID: "other", Host: "other.fixture", Port: 6697, TLS: true, Nick: "OtherCaller"}); err != nil {
		t.Fatal(err)
	}
	body := request(t, client, "GET", server.URL+"/api/v1/people?networkId=home&purpose=services&query=Native", nil, 200)
	var page model.PeoplePage
	if err = json.Unmarshal(body, &page); err != nil || len(page.Items) != 1 || page.Items[0].ServicesAccount != "NativeAccount" || !page.Items[0].Eligible || page.Items[0].BackendAccountID != "" || page.Items[0].Nickname != "" {
		t.Fatalf("public registered lookup: %s, %v", body, err)
	}
	for _, private := range []string{"registeredAt", "#private", "sessions", "createdAt", "lastSeenAt", "ownerSource", "username", "host"} {
		if strings.Contains(string(body), private) {
			t.Fatal("private metadata entered public people response", private, string(body))
		}
	}
	request(t, client, "GET", server.URL+"/api/v1/people?networkId=missing&purpose=services", nil, 404)
	request(t, client, "GET", server.URL+"/api/v1/people?networkId=home&purpose=mention", nil, 400)
	request(t, client, "GET", server.URL+"/api/v1/people?purpose=services", nil, 422)
	body = request(t, client, "GET", server.URL+"/api/v1/people?networkId=other&purpose=services&query=Native", nil, 200)
	if strings.Contains(string(body), "NativeAccount") {
		t.Fatal("directory crossed network boundary", string(body))
	}
	request(t, client, "GET", server.URL+administrationPrefix+"/identities?query=Native", nil, 403)
	request(t, cookieClient(), "GET", server.URL+"/api/v1/people?networkId=home&purpose=services", nil, 401)
}

func TestPeopleAdministrationMatchesDisplayNamesButRequiresActualLocalAccount(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	if err := store.GrantOwner(account.Account.ID, storage.OwnerFromOperator, "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Commit(account.Account.ID, "member.updated", model.Member{ID: account.Account.ID, DisplayName: "Peter Jensen"}); err != nil {
		t.Fatal(err)
	}
	body := request(t, client, "GET", server.URL+administrationPrefix+"/identities?query=Peter", nil, 200)
	var matches Items[IdentityMatch]
	if err := json.Unmarshal(body, &matches); err != nil || len(matches.Items) != 1 || !matches.Items[0].Eligible || matches.Items[0].User.ID != account.Account.ID {
		t.Fatal("shared display-name matching failed", string(body), err)
	}
	// A server account whose name happens to equal a local ID is not that login.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "present": true, "nickname": "NativeNick", "account": account.Account.ID})
	}))
	defer upstream.Close()
	ergo, err := ergoapi.New(ergoapi.Options{Server: "irc.fixture:6697", URL: upstream.URL, Token: strings.Repeat("fixture-", 8)})
	if err != nil {
		t.Fatal(err)
	}
	defer ergo.Close()
	app.options.Ergo = ergo
	body = request(t, client, "GET", server.URL+administrationPrefix+"/identities?query=NativeNick", nil, 200)
	matches = Items[IdentityMatch]{}
	if err = json.Unmarshal(body, &matches); err != nil || len(matches.Items) != 1 || matches.Items[0].Eligible || matches.Items[0].User != nil || matches.Items[0].Account != account.Account.ID {
		t.Fatal("server account name was confused with local account ID", string(body), err)
	}
}

func TestPeopleConversationTargetInputsAreMutuallyExclusive(t *testing.T) {
	_, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	register(t, client, server.URL)
	for _, input := range []map[string]string{
		{"networkId": "one", "nickname": "Nick", "memberId": "one:member"},
		{"networkId": "one", "nickname": "Nick", "name": "channel"},
		{"networkId": "one", "nickname": "Nick", "key": "channel-only"},
		{"networkId": "one"},
	} {
		request(t, client, "POST", server.URL+"/api/v1/conversations", input, 422)
	}
}
