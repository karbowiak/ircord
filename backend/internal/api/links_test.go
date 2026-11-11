package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

// The link catalog is account-scoped over HTTP, and its channel and kind
// filters accept several values by repeating the parameter.
func TestLinkRoutesAreAccountScopedAndFiltered(t *testing.T) {
	_, server, store, _ := authenticatedApp(t, func(options *irc.Options) {
		if err := options.Store.SaveSharedChannel("irc.example.test:6697/example", "#dev", []byte(`{"nsfw":true}`)); err != nil {
			t.Fatal(err)
		}
	})
	request(t, cookieClient(), "GET", server.URL+"/api/v1/links", nil, http.StatusUnauthorized)
	alice, bob := cookieClient(), cookieClient()
	a := register(t, alice, server.URL)
	register(t, bob, server.URL)
	for i, name := range []string{"dev", "ops", "art"} {
		conv := model.Conversation{ID: "net_" + name, NetworkID: "net", Kind: "channel", Name: name, Target: "#" + name, MemberIDs: []string{}, MemberModes: map[string]string{}}
		if name != "art" {
			conv.ChannelIdentity = &model.ChannelIdentity{Network: "irc.example.test:6697/example", Channel: "#" + name}
		}
		if _, err := store.Commit(a.Account.ID, "conversation.updated", conv); err != nil {
			t.Fatal(err)
		}
		text := "https://" + name + ".test/page"
		if name == "art" {
			text = "https://art.test/cat.png"
		}
		msg := model.Message{ID: "net:" + name, UpstreamID: name, ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: text, CreatedAt: time.Unix(int64(1000+i), 0).UTC(), Reactions: []model.Reaction{}}
		if _, err := store.Commit(a.Account.ID, "message.created", msg); err != nil {
			t.Fatal(err)
		}
	}
	list := func(client *http.Client, query string, want int) irc.LinkPage {
		t.Helper()
		var page irc.LinkPage
		if err := json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/links"+query, nil, http.StatusOK), &page); err != nil || len(page.Items) != want {
			t.Fatalf("%s: %+v %v", query, page, err)
		}
		return page
	}
	for _, card := range list(alice, "", 3).Items {
		source := card.Sources[0]
		if source.NSFW != (source.ConversationID == "net_dev") || source.NSFWKnown != (source.ConversationID != "net_art") {
			t.Fatalf("source classification over HTTP: %+v", source)
		}
	}
	list(alice, "?conversation=net_dev&conversation=net_ops", 2)
	list(alice, "?conversation=net_dev&conversation=net_art", 2)
	if page := list(alice, "?kind=image&kind=gif", 1); page.Items[0].Kind != "image" || page.Items[0].Sources[0].ConversationID != "net_art" {
		t.Fatalf("image card: %+v", page.Items[0])
	}
	list(alice, "?since=1970-01-01T00:16:41Z", 2)
	list(bob, "", 0)
	request(t, bob, "GET", server.URL+"/api/v1/links?conversation=net_dev", nil, http.StatusNotFound)
	request(t, alice, "GET", server.URL+"/api/v1/links?kind=script", nil, http.StatusUnprocessableEntity)
	request(t, alice, "GET", server.URL+"/api/v1/links?cursor=forged", nil, http.StatusBadRequest)
	request(t, alice, "GET", server.URL+"/api/v1/links?limit=500", nil, http.StatusUnprocessableEntity)
}
