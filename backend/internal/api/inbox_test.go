package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

// The inbox is account-scoped: no session means no access, and one account
// can neither list nor dismiss another account's items.
func TestInboxRoutesAreAccountScoped(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	anonymous := cookieClient()
	request(t, anonymous, "GET", server.URL+"/api/v1/inbox", nil, http.StatusUnauthorized)
	request(t, anonymous, "GET", server.URL+"/api/v1/inbox/summary", nil, http.StatusUnauthorized)
	request(t, anonymous, "PUT", server.URL+"/api/v1/inbox/m1/dismissed", nil, http.StatusUnauthorized)

	alice, bob := cookieClient(), cookieClient()
	a := register(t, alice, server.URL)
	register(t, bob, server.URL)
	conv := model.Conversation{ID: "net_dev", NetworkID: "net", Kind: "channel", Name: "dev", Target: "#dev", MemberIDs: []string{}, MemberModes: map[string]string{}}
	msg := model.Message{ID: "net:abc", UpstreamID: "abc", ConversationID: conv.ID, SenderID: "net:peer", Kind: "message", Text: "alice, a question", CreatedAt: time.Now().UTC().Add(-time.Minute), Reactions: []model.Reaction{}}
	for kind, data := range map[string]any{"conversation.updated": conv, "message.created": msg} {
		if _, err := store.Commit(a.Account.ID, kind, data); err != nil {
			t.Fatal(err)
		}
	}
	entry := model.InboxEntry{MessageID: msg.ID, ConversationID: conv.ID, NetworkID: "net", SenderID: msg.SenderID, Reasons: []string{"mention"}, CreatedAt: msg.CreatedAt}
	if _, err := store.Commit(a.Account.ID, "inbox.added", entry); err != nil {
		t.Fatal(err)
	}

	var page irc.InboxPage
	if err := json.Unmarshal(request(t, bob, "GET", server.URL+"/api/v1/inbox", nil, http.StatusOK), &page); err != nil || len(page.Items) != 0 || page.Unread != 0 {
		t.Fatalf("bob sees: %+v %v", page, err)
	}
	request(t, bob, "PUT", server.URL+"/api/v1/inbox/net:abc/dismissed", nil, http.StatusNotFound)
	if err := json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/inbox?filter=mentions", nil, http.StatusOK), &page); err != nil || len(page.Items) != 1 || page.Items[0].Excerpt != "alice, a question" || page.Unread != 1 {
		t.Fatalf("alice's inbox: %+v %v", page, err)
	}
	request(t, alice, "PUT", server.URL+"/api/v1/inbox/net:abc/dismissed", nil, http.StatusNoContent)
	var summary irc.InboxSummary
	if err := json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/inbox/summary", nil, http.StatusOK), &summary); err != nil || summary.Unread != 0 {
		t.Fatalf("summary after dismiss: %+v %v", summary, err)
	}
	request(t, alice, "DELETE", server.URL+"/api/v1/inbox/net:abc/dismissed", nil, http.StatusNoContent)
	if err := json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/inbox/summary", nil, http.StatusOK), &summary); err != nil || summary.Unread != 1 {
		t.Fatalf("summary after restore: %+v %v", summary, err)
	}
	request(t, alice, "GET", server.URL+"/api/v1/inbox?cursor=forged", nil, http.StatusBadRequest)
	request(t, alice, "GET", server.URL+"/api/v1/inbox?filter=everything", nil, http.StatusUnprocessableEntity)
	request(t, alice, "GET", server.URL+"/api/v1/inbox?limit=500", nil, http.StatusUnprocessableEntity)
}
