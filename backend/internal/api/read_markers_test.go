package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

func TestLocalReadMarkersSynchronizeAuthenticatedDevicesWithoutUpstream(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	first, second, other := cookieClient(), cookieClient(), cookieClient()
	account := register(t, first, server.URL)
	request(t, second, "POST", server.URL+"/api/v1/auth/login", account.credentials(), 200)
	register(t, other, server.URL)
	conv := irc.Conversation{ID: "local-room", NetworkID: "offline", Kind: "channel", Name: "test", Target: "#test", UnreadCount: 1, MentionCount: 1}
	if _, err := store.Commit(account.Account.ID, "conversation.updated", conv); err != nil {
		t.Fatal(err)
	}
	msg := irc.Message{ID: "unread", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Mention: true, CreatedAt: time.Now().UTC()}
	if err := store.SaveHistory(account.Account.ID, msg); err != nil {
		t.Fatal(err)
	}
	before := app.Manager.Snapshot(account.Account.ID)
	if len(before.Conversations) != 1 || before.Conversations[0].UnreadCount != 1 {
		t.Fatal("fixture did not begin unread")
	}
	a, b := accountSocket(t, first, server.URL, before.Cursor), accountSocket(t, second, server.URL, before.Cursor)
	request(t, first, "PATCH", server.URL+"/api/v1/conversations/"+conv.ID, map[string]bool{"read": true}, 200)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, socket := range []*websocket.Conn{a, b} {
		_, raw, err := socket.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event irc.Event
		if err = json.Unmarshal(raw, &event); err != nil {
			t.Fatal(err)
		}
		var saved model.ConversationCounts
		if err = json.Unmarshal(event.Data, &saved); err != nil {
			t.Fatal(err)
		}
		if event.Type != "conversation.counts" || event.Seq <= before.Cursor || saved.ID != conv.ID || saved.UnreadCount != 0 || saved.MentionCount != 0 {
			t.Fatal("device did not receive the canonical local read state", event.Type, saved)
		}
	}
	request(t, other, "PATCH", server.URL+"/api/v1/conversations/"+conv.ID, map[string]bool{"read": true}, 404)
	var reloaded struct {
		Conversations []irc.Conversation `json:"conversations"`
	}
	json.Unmarshal(request(t, second, "GET", server.URL+"/api/v1/bootstrap", nil, 200), &reloaded)
	if len(reloaded.Conversations) != 1 || reloaded.Conversations[0].UnreadCount != 0 || reloaded.Conversations[0].MentionCount != 0 {
		t.Fatal("reload lost local account read state")
	}
}
