package api

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/storage"
)

func TestHistoryCursorHTTPScopesAndLegacy(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	alice, bob := cookieClient(), cookieClient()
	a := register(t, alice, server.URL)
	b := register(t, bob, server.URL)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, owner := range []string{a.Account.ID, b.Account.ID} {
		for _, id := range []string{"room", "other"} {
			if _, err := store.Commit(owner, "conversation.updated", irc.Conversation{ID: id, NetworkID: "offline", Kind: "channel", Name: id}); err != nil {
				t.Fatal(err)
			}
			for i := range 2 {
				msg := irc.Message{ID: id + string(rune('a'+i)), ConversationID: id, SenderID: "peer", Kind: "message", CreatedAt: at.Add(time.Duration(i) * time.Second)}
				if err := store.SaveHistory(owner, msg); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	base := server.URL + "/api/v1/conversations/"
	var first irc.MessagePage
	if err := json.Unmarshal(request(t, alice, "GET", base+"room/messages?limit=1", nil, 200), &first); err != nil || first.NextCursor == nil {
		t.Fatal(first, err)
	}
	query := "?limit=1&before=" + url.QueryEscape(*first.NextCursor)
	request(t, bob, "GET", base+"room/messages"+query, nil, 400)
	request(t, alice, "GET", base+"other/messages"+query, nil, 400)
	request(t, alice, "GET", base+"room/messages?before=123", nil, 400)
	request(t, alice, "GET", base+"room/messages?before="+strings.Repeat("x", storage.MaxHistoryCursorLength+1), nil, 400)
	request(t, alice, "GET", base+"room/messages?before="+url.QueryEscape(storage.SearchCursor(first.Items[0])), nil, 400)
	var older irc.MessagePage
	legacy := storage.Cursor(first.Items[0])
	if err := json.Unmarshal(request(t, alice, "GET", base+"room/messages?limit=1&before="+url.QueryEscape(legacy), nil, 200), &older); err != nil || len(older.Items) != 1 || older.Items[0].ID != "rooma" {
		t.Fatal(older, err)
	}
}
