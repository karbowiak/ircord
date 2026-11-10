package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/linkmedia"
	"ircord/backend/internal/model"
)

func TestEmbedGeometryAuthorizedHistoryAndSocketReplay(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	first, other := cookieClient(), cookieClient()
	account := register(t, first, server.URL)
	register(t, other, server.URL)
	client, err := cdn.New(cdn.Options{Endpoint: "https://storage.example.test", Bucket: "fixture", Region: "us-east-1", PublicURL: "https://cdn.example.test", Transport: mediaTransport(func(*http.Request) (*http.Response, error) {
		t.Error("message serialization called CDN storage")
		return nil, linkmedia.ErrUnavailable
	})})
	if err != nil {
		t.Fatal(err)
	}
	app.options.Media = linkmedia.NewResolver(linkmedia.ResolverOptions{CDN: client})
	ctx, cancel := context.WithCancel(context.Background())
	done := app.options.Media.StartCache(ctx, store.DB)
	t.Cleanup(func() { cancel(); <-done })
	const source = "https://source.example/known.png"
	key := "link-media/" + strings.Repeat("a", 64)
	manifest := linkmedia.Result{URL: source, Provider: "media", Assets: []linkmedia.Asset{{URL: client.URL(key), ObjectKey: key, ContentType: "image/png", Kind: "image", Width: 900, Height: 600, ExpiresAt: time.Now().Add(time.Hour)}}}
	payload, _ := json.Marshal(manifest)
	hash := sha256.Sum256([]byte("native-v1\x00" + source))
	if _, err := store.DB.Exec(`INSERT INTO public_media_results(namespace,cache_key,payload,expires_ns) VALUES(?,?,?,?)`, client.Namespace(), "native-v1:"+hex.EncodeToString(hash[:]), payload, time.Now().Add(time.Hour).UnixNano()); err != nil {
		t.Fatal(err)
	}
	conv := model.Conversation{ID: "geometry-room", NetworkID: "offline", Kind: "channel", Name: "geometry", Target: "#geometry"}
	if _, err := store.Commit(account.Account.ID, "conversation.updated", conv); err != nil {
		t.Fatal(err)
	}
	for i, tc := range []struct {
		id, text string
		deleted  bool
	}{
		{"visible", source, false},
		{"different", source + "?different=1", false},
		{"fenced", "```\n" + source + "\n```", false},
		{"mono", "\x11" + source + "\x11", false},
		{"deleted", source, true},
		{"edited", "removed the link", false},
	} {
		msg := model.Message{ID: tc.id, ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: tc.text, Deleted: tc.deleted, CreatedAt: time.Now().Add(time.Duration(i) * time.Second), Reactions: []model.Reaction{}}
		if _, err := store.Commit(account.Account.ID, "message.created", msg); err != nil {
			t.Fatal(err)
		}
	}
	check := func(msg model.Message) {
		t.Helper()
		if msg.ID == "visible" {
			if len(msg.EmbedGeometry) != 1 || msg.EmbedGeometry[0] != (model.EmbedGeometry{URL: source, Width: 900, Height: 600}) {
				t.Fatalf("authorized initial message lacks cached dimensions: %+v", msg.EmbedGeometry)
			}
		} else if len(msg.EmbedGeometry) != 0 {
			t.Fatalf("deleted, code, edited, or different URL received hints: %s", msg.ID)
		}
	}
	for _, suffix := range []string{"", "?around=visible"} {
		var page model.MessagePage
		body := request(t, first, "GET", server.URL+"/api/v1/conversations/"+conv.ID+"/messages"+suffix, nil, 200)
		if err := json.Unmarshal(body, &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 6 {
			t.Fatalf("history changed: %d items", len(page.Items))
		}
		for _, msg := range page.Items {
			check(msg)
		}
		if bytes.Contains(body, []byte("cdn.example.test")) || bytes.Contains(body, []byte("objectKey")) {
			t.Fatal("geometry leaked delivery authority")
		}
	}
	request(t, other, "GET", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", nil, 404)
	if _, _, err := store.BeginCommand(account.Account.ID, model.SendInput{ClientID: "geometry-command", ConversationID: conv.ID, Text: source}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`UPDATE outbox SET state='sent',message_id='visible' WHERE owner=? AND client_id='geometry-command'`, account.Account.ID); err != nil {
		t.Fatal(err)
	}
	var command CommandResponse
	if json.Unmarshal(request(t, first, "GET", server.URL+"/api/v1/commands/geometry-command", nil, 200), &command) != nil || command.Message == nil {
		t.Fatal("acknowledged message disappeared")
	}
	check(*command.Message)
	request(t, other, "GET", server.URL+"/api/v1/commands/geometry-command", nil, 404)
	var saved irc.SavedItem
	if json.Unmarshal(request(t, first, "PUT", server.URL+"/api/v1/saved-messages/visible", map[string]string{"conversationId": conv.ID}, 200), &saved) != nil || saved.Message == nil {
		t.Fatal("saved message disappeared")
	}
	check(*saved.Message)
	var savedPage irc.SavedPage
	if json.Unmarshal(request(t, first, "GET", server.URL+"/api/v1/saved-messages", nil, 200), &savedPage) != nil || len(savedPage.Items) != 1 || savedPage.Items[0].Message == nil {
		t.Fatal("saved page changed")
	}
	check(*savedPage.Items[0].Message)
	var search irc.SearchPage
	if json.Unmarshal(request(t, first, "GET", server.URL+"/api/v1/search?q=known", nil, 200), &search) != nil || len(search.Items) == 0 {
		t.Fatal("search did not find the fixture")
	}
	for _, item := range search.Items {
		check(item.Message)
	}
	var offline irc.OfflineSnapshot
	if json.Unmarshal(request(t, first, "GET", server.URL+"/api/v1/offline/snapshot", nil, 200), &offline) != nil || len(offline.Items) == 0 {
		t.Fatal("offline page changed")
	}
	for _, msg := range offline.Items {
		check(msg)
	}
	var replay struct {
		Items []irc.Event `json:"items"`
		Valid bool        `json:"valid"`
	}
	if err := json.Unmarshal(request(t, first, "GET", server.URL+"/api/v1/events/replay?after=0", nil, 200), &replay); err != nil {
		t.Fatal(err)
	}
	if !replay.Valid {
		t.Fatal("authorized replay became invalid")
	}
	for _, event := range replay.Items {
		if event.Type == "message.created" {
			var msg model.Message
			if json.Unmarshal(event.Data, &msg) != nil {
				t.Fatal("invalid replay")
			}
			check(msg)
		}
	}
	if raw := request(t, other, "GET", server.URL+"/api/v1/events/replay?after=0", nil, 200); bytes.Contains(raw, []byte(source)) {
		t.Fatal("private geometry crossed accounts")
	}
	socket := accountSocket(t, first, server.URL, 0)
	defer socket.CloseNow()
	readCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	for _, expected := range replay.Items {
		_, raw, err := socket.Read(readCtx)
		if err != nil {
			t.Fatal(err)
		}
		var event irc.Event
		if json.Unmarshal(raw, &event) != nil || event.Seq != expected.Seq || event.Type != expected.Type {
			t.Fatal("geometry changed socket ordering")
		}
		if event.Type == "message.created" {
			var msg model.Message
			_ = json.Unmarshal(event.Data, &msg)
			check(msg)
		}
	}
	stored, err := store.Message(account.Account.ID, conv.ID, "visible")
	if err != nil || len(stored.EmbedGeometry) != 0 {
		t.Fatal("serialization mutated stored message")
	}
	events, _, err := store.Replay(account.Account.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if bytes.Contains(event.Data, []byte("embedGeometry")) {
			t.Fatal("serialization mutated durable events")
		}
	}
}

func TestEmbedGeometryClonesAndBindsUpdatedMessages(t *testing.T) {
	app := &Server{}
	message := model.Message{ID: "message", Text: "removed", EmbedGeometry: []model.EmbedGeometry{{URL: "https://unrelated.example/image.png", Width: 1, Height: 1}}}
	data, _ := json.Marshal(message)
	original := []irc.Event{{V: 1, Seq: 12, Type: "message.updated", Data: data}, {V: 1, Seq: 13, Type: "profile.updated", Data: json.RawMessage(`{"name":"unmodified"}`)}}
	out := app.eventsWithGeometry(context.Background(), original)
	var updated model.Message
	_ = json.Unmarshal(out[0].Data, &updated)
	if len(updated.EmbedGeometry) != 0 || out[0].Seq != 12 || !bytes.Equal(out[1].Data, original[1].Data) || !bytes.Contains(original[0].Data, []byte("embedGeometry")) {
		t.Fatal("decoration mutated shared payloads or retained stale hints")
	}
	if len(message.EmbedGeometry) != 1 {
		t.Fatal("message input was mutated")
	}
	if len(app.messagesWithGeometry(context.Background(), []model.Message{message})[0].EmbedGeometry) != 0 {
		t.Fatal("missing cache retained untrusted geometry")
	}
}
