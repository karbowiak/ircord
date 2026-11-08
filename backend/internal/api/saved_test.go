package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

// Saved messages are account-scoped: no session means no access, and an account
// cannot bookmark into a conversation that is not its own.
func TestSavedMessageRoutesAreAccountScoped(t *testing.T) {
	_, server, _, _ := authenticatedApp(t)
	anonymous := cookieClient()
	request(t, anonymous, "GET", server.URL+"/api/v1/saved-messages", nil, http.StatusUnauthorized)
	request(t, anonymous, "PUT", server.URL+"/api/v1/saved-messages/m1", map[string]string{"conversationId": "c1"}, http.StatusUnauthorized)

	client := cookieClient()
	register(t, client, server.URL)
	var page irc.SavedPage
	if err := json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/saved-messages?limit=10", nil, http.StatusOK), &page); err != nil || len(page.Items) != 0 || page.NextCursor != nil {
		t.Fatalf("empty list: %+v %v", page, err)
	}
	request(t, client, "PUT", server.URL+"/api/v1/saved-messages/m1", map[string]string{"conversationId": "someone-elses"}, http.StatusNotFound)
	request(t, client, "DELETE", server.URL+"/api/v1/saved-messages/m1", nil, http.StatusNoContent)
	request(t, client, "GET", server.URL+"/api/v1/saved-messages?cursor=forged", nil, http.StatusBadRequest)
	request(t, client, "GET", server.URL+"/api/v1/saved-messages?limit=500", nil, http.StatusUnprocessableEntity)
	var refs Items[irc.SavedMessage]
	if err := json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/saved-message-ids", nil, http.StatusOK), &refs); err != nil || refs.Items == nil || len(refs.Items) != 0 {
		t.Fatalf("refs: %+v %v", refs, err)
	}
}

// One account's real conversation and message cannot be bookmarked, unbookmarked
// or listed by another account; the owner can, and only by canonical ID.
func TestSavedMessagesStayWithTheirOwner(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	alice, bob := cookieClient(), cookieClient()
	a := register(t, alice, server.URL)
	register(t, bob, server.URL)
	// Seed alice's stored workspace before her first workspace request loads it.
	conv := model.Conversation{ID: "net_dev", NetworkID: "net", Kind: "channel", Name: "dev", Target: "#dev", MemberIDs: []string{}, MemberModes: map[string]string{}}
	if _, err := store.Commit(a.Account.ID, "conversation.updated", conv); err != nil {
		t.Fatal(err)
	}
	msg := model.Message{ID: "net:abc", UpstreamID: "abc", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "alice's note", CreatedAt: time.Unix(1000, 0).UTC(), Reactions: []model.Reaction{}}
	if _, err := store.Commit(a.Account.ID, "message.created", msg); err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"conversationId": conv.ID}
	request(t, bob, "PUT", server.URL+"/api/v1/saved-messages/net:abc", body, http.StatusNotFound)
	request(t, alice, "PUT", server.URL+"/api/v1/saved-messages/abc", body, http.StatusNotFound)
	var item irc.SavedItem
	if err := json.Unmarshal(request(t, alice, "PUT", server.URL+"/api/v1/saved-messages/net:abc", body, http.StatusOK), &item); err != nil || item.Status != "available" || item.Message.Text != "alice's note" {
		t.Fatalf("alice's save: %+v %v", item, err)
	}
	request(t, bob, "DELETE", server.URL+"/api/v1/saved-messages/net:abc", nil, http.StatusNoContent)
	var page irc.SavedPage
	if err := json.Unmarshal(request(t, bob, "GET", server.URL+"/api/v1/saved-messages", nil, http.StatusOK), &page); err != nil || len(page.Items) != 0 {
		t.Fatalf("bob sees: %+v %v", page, err)
	}
	if err := json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/saved-messages", nil, http.StatusOK), &page); err != nil || len(page.Items) != 1 || page.Items[0].MessageID != "net:abc" {
		t.Fatalf("bob removed alice's bookmark: %+v %v", page, err)
	}
	// The history window around it is alice's alone too.
	request(t, bob, "GET", server.URL+"/api/v1/conversations/net_dev/messages?around=net:abc", nil, http.StatusNotFound)
	var window model.MessagePage
	if err := json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/conversations/net_dev/messages?around=net:abc&limit=10", nil, http.StatusOK), &window); err != nil || len(window.Items) != 1 || window.HasNewer {
		t.Fatalf("window: %+v %v", window, err)
	}
	request(t, alice, "GET", server.URL+"/api/v1/conversations/net_dev/messages?around=net:abc&before=x", nil, http.StatusUnprocessableEntity)
}
