package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

func TestSearchHTTPFiltersValidationAndIsolation(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	request(t, cookieClient(), "GET", server.URL+"/api/v1/search?has=image", nil, http.StatusUnauthorized)
	alice, bob := cookieClient(), cookieClient()
	a := register(t, alice, server.URL)
	register(t, bob, server.URL)
	for _, id := range []string{"one", "two"} {
		_, err := store.Commit(a.Account.ID, "conversation.updated", model.Conversation{ID: id, Name: id, Kind: "channel", NetworkID: "net", MemberIDs: []string{}, MemberModes: map[string]string{}})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range []model.Member{{ID: "peer", Nick: "Jul", Role: "member"}, {ID: "bot", Nick: "Helper", Role: "bot"}} {
		if _, err := store.Commit(a.Account.ID, "member.updated", m); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 4; i++ {
		msg := model.Message{ID: string(rune('a' + i)), ConversationID: []string{"one", "two"}[i%2], SenderID: "bot", Text: "Hello @Jul https://x.test/a.png", CreatedAt: time.Unix(1000, 0).UTC(), Kind: "message", Reactions: []model.Reaction{}}
		if err := store.SaveHistory(a.Account.ID, msg); err != nil {
			t.Fatal(err)
		}
	}
	get := func(client *http.Client, q string, want int) irc.SearchPage {
		t.Helper()
		var p irc.SearchPage
		if err := json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/search?"+q, nil, 200), &p); err != nil || len(p.Items) != want {
			t.Fatalf("%s: %+v %v", q, p, err)
		}
		return p
	}
	get(alice, "conversationId=one&conversationId=two", 4)
	get(alice, "from=peer&from=bot&mentions=peer&has=image&has=file&author=bot", 4)
	get(alice, "after=1970-01-01T00:16:40Z&before=1970-01-01T00:16:41Z", 4)
	get(alice, "before=1970-01-01T00:16:40Z", 0)
	get(alice, "q=h&has=image", 4)
	get(alice, "from=foreign", 0)
	get(bob, "q=hello", 0)
	p := get(alice, "has=image&limit=2", 2)
	if p.NextCursor == nil {
		t.Fatal("missing cursor")
	}
	p2 := get(alice, "has=image&limit=2&cursor="+url.QueryEscape(*p.NextCursor), 2)
	if p2.NextCursor != nil || p.Items[0].Message.ID != "d" || p2.Items[0].Message.ID != "b" {
		t.Fatal(p, p2)
	}
	request(t, bob, "GET", server.URL+"/api/v1/search?conversationId=one&conversationId=two", nil, 404)
	request(t, alice, "GET", server.URL+"/api/v1/search?conversationId=one&conversationId=foreign", nil, 404)
	for _, q := range []string{"", "q=x", "has=invalid", "author=invalid", "limit=101&has=image", "limit=0&has=image", "q=" + strings.Repeat("x", 201), "before=bad", "before=2026-01-01T00:00:00Z&after=2026-01-01T00:00:00Z", strings.Repeat("from=x&", 51), strings.Repeat("has=image&", 51)} {
		request(t, alice, "GET", server.URL+"/api/v1/search?"+q, nil, 422)
	}
	request(t, alice, "GET", server.URL+"/api/v1/search?q=hello&cursor=broken", nil, 400)
}
