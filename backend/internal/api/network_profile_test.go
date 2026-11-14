package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

func TestNetworkProfileRoutesAuthorizeOfflineNetworksBeforeAvatarUpload(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	alice, bob := cookieClient(), cookieClient()
	a := register(t, alice, server.URL)
	register(t, bob, server.URL)
	// Keep both networks offline; profile ownership must not depend on having
	// a live connection. These identities and files exist only in temp storage.
	for _, id := range []string{"one", "two"} {
		cfg := irc.NetworkConfig{ID: id, Name: id, Host: id + ".invalid", Port: 6697, TLS: true, Nick: id + "nick"}
		if err := store.SaveNetwork(a.Account.ID, cfg, false); err != nil {
			t.Fatal(err)
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	avatar := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
	body := map[string]any{"displayName": "One profile", "bio": "One bio", "nameColor": "#123456", "avatarDataUrl": avatar}
	request(t, bob, "GET", server.URL+"/api/v1/networks/one/profile", nil, 404)
	request(t, bob, "PATCH", server.URL+"/api/v1/networks/one/profile", body, 404)
	request(t, alice, "PATCH", server.URL+"/api/v1/networks/missing/profile", body, 404)
	request(t, http.DefaultClient, "PATCH", server.URL+"/api/v1/networks/one/profile", body, 401)
	var files int
	if err := store.DB.QueryRow("SELECT count(*) FROM files").Scan(&files); err != nil || files != 0 {
		t.Fatal("unauthorized request stored an avatar", files, err)
	}
	var saved model.Member
	if err := json.Unmarshal(request(t, alice, "PATCH", server.URL+"/api/v1/networks/one/profile", body, 200), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ID != a.Account.ID || saved.DisplayName != "One profile" || saved.Bio != "One bio" || saved.NameColor != "#123456" || saved.AvatarURL == "" {
		t.Fatal("scoped profile was not saved", saved)
	}
	var read model.Member
	if err := json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/networks/one/profile", nil, 200), &read); err != nil || read.AvatarURL != saved.AvatarURL {
		t.Fatal("profile read lost the avatar", read, err)
	}
	other, err := app.Manager.NetworkProfile(a.Account.ID, "two")
	if err != nil || other.NameSource == "profile" || other.Bio != "" || other.AvatarURL != "" || other.NameColor != "" {
		t.Fatal("offline network profile leaked", other, err)
	}
	// Keep omission/null semantics and all existing avatar validation rules.
	request(t, alice, "PATCH", server.URL+"/api/v1/networks/one/profile", map[string]any{"bio": "kept"}, 200)
	read, _ = app.Manager.NetworkProfile(a.Account.ID, "one")
	if read.DisplayName != saved.DisplayName || read.NameColor != saved.NameColor || read.AvatarURL != saved.AvatarURL {
		t.Fatal("omitted profile fields were cleared", read)
	}
	request(t, alice, "PATCH", server.URL+"/api/v1/networks/one/profile", map[string]any{"avatarUrl": nil}, 200)
	read, _ = app.Manager.NetworkProfile(a.Account.ID, "one")
	if read.AvatarURL != saved.AvatarURL {
		t.Fatal("null avatarUrl changed image", read)
	}
	for _, fields := range []map[string]any{
		{"bio": "", "avatarUrl": "http://untrusted.invalid/avatar.png"},
		{"bio": "", "avatarDataUrl": "data:image/svg+xml;base64,PHN2Zz4="},
		{"bio": "", "avatarDataUrl": "data:image/png;base64,bm90IGFuIGltYWdl"},
		{"bio": "", "avatarDataUrl": "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", (128<<10)+1)))},
		{"bio": "", "nameColor": "#zzzzzz"},
	} {
		request(t, alice, "PATCH", server.URL+"/api/v1/networks/one/profile", fields, 400)
	}
	request(t, alice, "PATCH", server.URL+"/api/v1/networks/one/profile", map[string]any{"displayName": nil, "bio": "", "nameColor": nil, "avatarDataUrl": nil, "avatarUrl": "https://example.com/avatar.png"}, 200)
	read, _ = app.Manager.NetworkProfile(a.Account.ID, "one")
	if read.NameSource == "profile" || read.DisplayName != "onenick" || read.AvatarURL != "" || read.NameColor != "" {
		t.Fatal("explicit clears did not stay on selected server", read)
	}
}

func TestProfilePatchOmitClearAndNull(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	alice := cookieClient()
	a := register(t, alice, server.URL)
	cfg := irc.NetworkConfig{ID: "one", Name: "one", Host: "one.invalid", Port: 6697, TLS: true, Nick: "one"}
	if err := store.SaveNetwork(a.Account.ID, cfg, false); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/profile", "/api/v1/networks/one/profile"} {
		request(t, alice, "PATCH", server.URL+path, map[string]any{"bio": "saved", "displayName": "Name", "nameColor": "#123456"}, 200)
		var got model.Member
		if err := json.Unmarshal(request(t, alice, "PATCH", server.URL+path, map[string]any{"displayName": "Changed"}, 200), &got); err != nil {
			t.Fatal(err)
		}
		if got.Bio != "saved" || got.NameColor != "#123456" || got.DisplayName != "Changed" {
			t.Fatal("omission lost fields", got)
		}
		request(t, alice, "PATCH", server.URL+path, map[string]any{"bio": nil}, 400)
		request(t, alice, "PATCH", server.URL+path, map[string]any{"bio": "bad\r", "nameColor": "#abcdef"}, 400)
		got = model.Member{}
		if err := json.Unmarshal(request(t, alice, "PATCH", server.URL+path, map[string]any{}, 200), &got); err != nil || got.Bio != "saved" || got.NameColor != "#123456" {
			t.Fatal("invalid patch changed profile", got, err)
		}
		got = model.Member{}
		if err := json.Unmarshal(request(t, alice, "PATCH", server.URL+path, map[string]any{"bio": "", "nameColor": nil, "displayName": nil}, 200), &got); err != nil {
			t.Fatal(err)
		}
		if got.Bio != "" || got.NameColor != "" || got.NameSource == "profile" {
			t.Fatal("clear failed", got)
		}
	}
	if _, err := app.Manager.NetworkProfile(a.Account.ID, "one"); err != nil {
		t.Fatal(err)
	}
}
