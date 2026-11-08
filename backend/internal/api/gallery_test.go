package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

// A gallery lists the caller's own stored images of a conversation they have
// open; another account gets nothing, not even a count.
func TestGalleryIsAccountScoped(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	request(t, cookieClient(), "GET", server.URL+"/api/v1/conversations/net_dev/gallery", nil, http.StatusUnauthorized)
	alice, bob := cookieClient(), cookieClient()
	a := register(t, alice, server.URL)
	register(t, bob, server.URL)
	conv := model.Conversation{ID: "net_dev", NetworkID: "net", Kind: "channel", Name: "dev", Target: "#dev", MemberIDs: []string{}, MemberModes: map[string]string{}}
	if _, err := store.Commit(a.Account.ID, "conversation.updated", conv); err != nil {
		t.Fatal(err)
	}
	for i, text := range []string{"first https://i.imgur.com/one.png", "two at once https://i.imgur.com/two.jpg https://media.tenor.com/three.gif", "no images here"} {
		msg := model.Message{ID: "net:" + string(rune('a'+i)), ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: text, CreatedAt: time.Unix(int64(1000+i), 0).UTC(), Reactions: []model.Reaction{}}
		if _, err := store.Commit(a.Account.ID, "message.created", msg); err != nil {
			t.Fatal(err)
		}
	}
	var page irc.GalleryPage
	if err := json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/conversations/net_dev/gallery?limit=2", nil, http.StatusOK), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Items) != 2 || page.NextCursor == nil || page.Items[0].URL != "https://i.imgur.com/two.jpg" || page.Items[1].Kind != "gif" || page.Items[0].Excerpt == "" || page.Items[0].NetworkID != "net" {
		t.Fatalf("first page: %+v", page)
	}
	var rest irc.GalleryPage
	if err := json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/conversations/net_dev/gallery?limit=2&cursor="+*page.NextCursor, nil, http.StatusOK), &rest); err != nil || len(rest.Items) != 1 || rest.Items[0].MessageID != "net:a" || rest.NextCursor != nil {
		t.Fatalf("second page: %+v %v", rest, err)
	}
	request(t, bob, "GET", server.URL+"/api/v1/conversations/net_dev/gallery", nil, http.StatusNotFound)
	request(t, alice, "GET", server.URL+"/api/v1/conversations/net_dev/gallery?cursor=forged", nil, http.StatusBadRequest)
	request(t, alice, "GET", server.URL+"/api/v1/conversations/net_dev/gallery?limit=500", nil, http.StatusUnprocessableEntity)
}
