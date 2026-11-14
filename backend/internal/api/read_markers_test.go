package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
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

func TestReadThroughHTTPAndOwnership(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	client, other := cookieClient(), cookieClient()
	account := register(t, client, server.URL)
	register(t, other, server.URL)
	conv := irc.Conversation{ID: "room", NetworkID: "offline", Kind: "channel", Name: "test"}
	if _, err := store.Commit(account.Account.ID, "conversation.updated", conv); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	for _, id := range []string{"a", "b"} {
		if err := store.SaveHistory(account.Account.ID, irc.Message{ID: id, ConversationID: conv.ID, SenderID: "peer", Kind: "message", CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	app.Manager.Snapshot(account.Account.ID)
	path := server.URL + "/api/v1/conversations/room"
	request(t, client, "PATCH", path, map[string]any{"readThroughMessageId": "a"}, 200)
	if n, err := store.Unread(account.Account.ID, conv.ID); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	request(t, client, "PATCH", path, map[string]any{"readThroughMessageId": "missing"}, 404)
	request(t, client, "PATCH", path, map[string]any{"readThroughMessageId": "missing", "favorite": true}, 404)
	request(t, client, "PATCH", path, map[string]any{"readThroughMessageId": "", "favorite": true}, 422)
	request(t, client, "PATCH", path, map[string]any{"readThroughMessageId": "a", "read": false, "favorite": true}, 422)
	if got := app.Manager.Snapshot(account.Account.ID).Conversations[0]; got.Favorite || got.UnreadCount != 1 {
		t.Fatalf("invalid boundary changed favorite or read state: %+v", got)
	}
	request(t, other, "PATCH", path, map[string]any{"readThroughMessageId": "a"}, 404)
	request(t, client, "PATCH", path, map[string]any{"readThroughMessageId": "a", "read": false}, 422)
	request(t, client, "PATCH", path, map[string]any{"readThroughMessageId": ""}, 422)
	request(t, client, "PATCH", path, map[string]any{"unreadFrom": "a"}, 200)
	request(t, client, "PATCH", path, map[string]any{"readThroughMessageId": "b"}, 200)
	if id, err := store.UnreadFrom(account.Account.ID, conv.ID); err != nil || id != "a" {
		t.Fatal(id, err)
	}
	request(t, client, "PATCH", path, map[string]any{"read": true}, 200)
	if n, err := store.Unread(account.Account.ID, conv.ID); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}

// This reproduces the old route's schema using the same Huma configuration.
func TestOldConversationPatchRejectsUnknownReadThrough(t *testing.T) {
	router := chi.NewRouter()
	api := humachi.New(router, huma.DefaultConfig("legacy", "1"))
	called := false
	huma.Patch(api, "/conversation", func(ctx context.Context, in *struct {
		Body struct {
			Read       *bool   `json:"read,omitempty"`
			UnreadFrom *string `json:"unreadFrom,omitempty"`
			Favorite   *bool   `json:"favorite,omitempty"`
		}
	}) (*struct{}, error) {
		called = true
		return &struct{}{}, nil
	})
	req := httptest.NewRequest(http.MethodPatch, "/conversation", strings.NewReader(`{"readThroughMessageId":"a"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 422 || called {
		t.Fatalf("legacy handler status=%d called=%v", response.Code, called)
	}
}
