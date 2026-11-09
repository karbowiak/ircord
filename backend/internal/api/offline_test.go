package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

func offlinePNG(t *testing.T, app *Server, owner string) filehost.Upload {
	t.Helper()
	var body bytes.Buffer
	photo := image.NewRGBA(image.Rect(0, 0, 96, 64))
	draw.Draw(photo, photo.Bounds(), image.NewUniform(color.RGBA{R: 83, G: 75, B: 121, A: 255}), image.Point{}, draw.Src)
	draw.Draw(photo, image.Rect(16, 16, 80, 48), image.NewUniform(color.RGBA{R: 167, G: 205, B: 189, A: 255}), image.Point{}, draw.Src)
	if err := png.Encode(&body, photo); err != nil {
		t.Fatal(err)
	}
	upload, err := app.options.Files.Upload(context.Background(), owner, "", "photo.png", body.Bytes(), app.options.Files.LocalProvider())
	if err != nil {
		t.Fatal(err)
	}
	return upload
}

func TestOfflineSnapshotPagesAreScopedAndRetained(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	alice, bob := cookieClient(), cookieClient()
	account := register(t, alice, server.URL)
	register(t, bob, server.URL)
	owner := account.Account.ID
	commit := func(kind string, data any) {
		t.Helper()
		if _, err := store.Commit(owner, kind, data); err != nil {
			t.Fatal(err)
		}
	}
	commit("conversation.updated", irc.Conversation{ID: "room", Kind: "channel", Name: "private", MemberIDs: []string{owner}})
	for i := range 525 {
		commit("message.created", irc.Message{ID: fmt.Sprintf("message-%04d", i), ConversationID: "room", SenderID: owner, Kind: "message", CreatedAt: time.Now().Add(-time.Hour), Text: "private history"})
	}
	commit("message.created", irc.Message{ID: "old", ConversationID: "room", SenderID: owner, Kind: "message", CreatedAt: time.Now().Add(-8 * 24 * time.Hour), Text: "old history"})
	before := ""
	seen := map[string]bool{}
	for {
		var page irc.OfflineSnapshot
		if err := json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/offline/snapshot?limit=200&before="+url.QueryEscape(before), nil, 200), &page); err != nil {
			t.Fatal(err)
		}
		if before == "" && (page.State == nil || len(page.State.Conversations) != 1) {
			t.Fatal("initial snapshot missing account state")
		}
		for _, message := range page.Items {
			if seen[message.ID] || message.ID == "old" {
				t.Fatal("duplicate or expired message exported")
			}
			seen[message.ID] = true
		}
		if page.NextCursor == nil {
			break
		}
		before = *page.NextCursor
	}
	if len(seen) != 525 {
		t.Fatalf("exported %d messages", len(seen))
	}
	var other irc.OfflineSnapshot
	json.Unmarshal(request(t, bob, "GET", server.URL+"/api/v1/offline/snapshot?before="+url.QueryEscape(before), nil, 200), &other)
	if len(other.Items) != 0 {
		t.Fatal("another account received history")
	}
	request(t, alice, "GET", server.URL+"/api/v1/offline/snapshot?before=invalid", nil, 400)
	var pinned irc.OfflinePinned
	json.Unmarshal(request(t, alice, "POST", server.URL+"/api/v1/offline/pinned", map[string]any{"items": []irc.OfflineMessageRef{{ConversationID: "room", MessageID: "old"}}}, 200), &pinned)
	if len(pinned.Items) != 1 {
		t.Fatal("old pinned message could not be revalidated")
	}
	if err := app.Manager.Part(owner, "room"); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(request(t, alice, "POST", server.URL+"/api/v1/offline/pinned", map[string]any{"items": []irc.OfflineMessageRef{{ConversationID: "room", MessageID: "old"}}}, 200), &pinned)
	if len(pinned.Items) != 0 {
		t.Fatal("closed conversation remained exportable")
	}
}

func TestOfflineMediaRequiresVisibleMessageAndSafeDestination(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	alice, bob := cookieClient(), cookieClient()
	account := register(t, alice, server.URL)
	register(t, bob, server.URL)
	upload := offlinePNG(t, app, account.Account.ID)
	for _, data := range []any{irc.Conversation{ID: "room", Kind: "direct", MemberIDs: []string{account.Account.ID}}, irc.Message{ID: "photo", ConversationID: "room", Kind: "message", CreatedAt: time.Now(), Attachment: &model.Attachment{URL: upload.URL, Name: upload.Name, ContentType: upload.ContentType}}} {
		kind := "conversation.updated"
		if _, ok := data.(irc.Message); ok {
			kind = "message.created"
		}
		if _, err := store.Commit(account.Account.ID, kind, data); err != nil {
			t.Fatal(err)
		}
	}
	path := server.URL + "/api/v1/offline/media?conversationId=room&messageId=photo&url=" + url.QueryEscape(upload.URL)
	response, err := alice.Get(path)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || len(body) == 0 || response.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("file not exported: %d", response.StatusCode)
	}
	request(t, bob, "GET", path, nil, 404)
	request(t, http.DefaultClient, "GET", path, nil, 401)
	request(t, alice, "GET", server.URL+"/api/v1/offline/media?conversationId=room&messageId=photo&url="+url.QueryEscape("http://127.0.0.1/private"), nil, 404)
	if _, err = store.Commit(account.Account.ID, "message.created", irc.Message{ID: "private-url", ConversationID: "room", Kind: "message", CreatedAt: time.Now(), Attachment: &model.Attachment{URL: "http://127.0.0.1/private", Name: "private.png"}}); err != nil {
		t.Fatal(err)
	}
	request(t, alice, "GET", server.URL+"/api/v1/offline/media?conversationId=room&messageId=private-url&url="+url.QueryEscape("http://127.0.0.1/private"), nil, 400)
	_, err = store.Commit(account.Account.ID, "message.updated", irc.Message{ID: "photo", ConversationID: "room", Kind: "message", CreatedAt: time.Now(), Deleted: true})
	if err != nil {
		t.Fatal(err)
	}
	request(t, alice, "GET", path, nil, 404)
}
