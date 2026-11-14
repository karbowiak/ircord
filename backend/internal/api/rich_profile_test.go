package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"ircord/backend/internal/irc"
)

func TestRichProfileAPIPatchAndClears(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	cfg := irc.NetworkConfig{ID: "one", Name: "one", Host: "one.invalid", Port: 6697, TLS: true, Nick: "one"}
	if err := store.SaveNetwork(account.Account.ID, cfg, false); err != nil {
		t.Fatal(err)
	}
	path := server.URL + "/api/v1/networks/one/profile"
	values := map[string]any{"pronouns": "they/them", "homepageUrl": "https://example.com/", "statusText": "Busy", "statusEmoji": "☕", "statusExpiresAt": "2099-01-01T00:00:00Z", "headline": "Hello", "bannerUrl": "https://example.com/banner", "profileBackgroundUrl": "https://example.com/bg", "profileAccentColor": "#abcdef", "avatarFrame": "round-1", "location": "Copenhagen", "timezone": "Europe/Copenhagen", "languages": []string{"en-US", "da"}, "interests": []string{"IRC", "Music"}}
	request(t, client, "PATCH", path, values, 200)
	request(t, client, "PATCH", path, map[string]any{"bio": "retained"}, 200)
	var got map[string]any
	if err := json.Unmarshal(request(t, client, "GET", path, nil, 200), &got); err != nil {
		t.Fatal(err)
	}
	for key, want := range values {
		encodedWant, _ := json.Marshal(want)
		encodedGot, _ := json.Marshal(got[key])
		if string(encodedGot) != string(encodedWant) {
			t.Errorf("%s got %s, want %s", key, encodedGot, encodedWant)
		}
	}
	for _, invalid := range []map[string]any{{"homepageUrl": "https://user@example.com/", "pronouns": "changed"}, {"profileAccentColor": "#zzzzzz"}, {"timezone": "not/a-zone"}, {"languages": []string{"en_US"}}, {"interests": []string{strings.Repeat("x", 33)}}, {"statusExpiresAt": "invalid"}} {
		request(t, client, "PATCH", path, invalid, 400)
	}
	request(t, client, "PATCH", path, map[string]any{"statusExpiresAt": strings.Repeat("x", 65)}, 422)
	for _, body := range []string{`{"pronouns":"\ud800"}`, "{\"pronouns\":\"" + string([]byte{0xff}) + "\"}"} {
		req, err := http.NewRequest(http.MethodPatch, path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode < 400 {
			t.Fatalf("malformed Unicode was accepted: %d", res.StatusCode)
		}
	}
	if err := json.Unmarshal(request(t, client, "GET", path, nil, 200), &got); err != nil || got["pronouns"] != "they/them" {
		t.Fatal("invalid patch applied", got, err)
	}
	clears := map[string]any{}
	for key := range values {
		clears[key] = nil
	}
	request(t, client, "PATCH", path, clears, 200)
	got = map[string]any{}
	if err := json.Unmarshal(request(t, client, "GET", path, nil, 200), &got); err != nil {
		t.Fatal(err)
	}
	for key := range values {
		if value, ok := got[key]; ok && value != nil {
			t.Errorf("%s was not cleared: %v", key, value)
		}
	}
	if got["bio"] != "retained" {
		t.Fatal("clears erased unrelated field", got)
	}
	request(t, client, "PATCH", path, values, 200)
	empty := map[string]any{}
	for key := range values {
		empty[key] = ""
	}
	empty["languages"], empty["interests"] = []string{}, []string{}
	request(t, client, "PATCH", path, empty, 200)
	got = map[string]any{}
	if err := json.Unmarshal(request(t, client, "GET", path, nil, 200), &got); err != nil {
		t.Fatal(err)
	}
	for key := range values {
		if value, ok := got[key]; ok && value != nil {
			t.Errorf("%s was not emptied: %v", key, value)
		}
	}
}
