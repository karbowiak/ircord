package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

// Measures the shipped frontend against isolated, durable account state. No
// production credentials, messages, listeners, or upstream servers are used.
func TestWorkspaceSyncBrowser(t *testing.T) {
	if os.Getenv("IRCORD_SYNC_BROWSER") != "true" {
		t.Skip("set IRCORD_SYNC_BROWSER=true after building the frontend")
	}
	workspaceBrowser(t, false)
}

func TestOfflineArchiveBrowser(t *testing.T) {
	if os.Getenv("IRCORD_OFFLINE_BROWSER") != "true" {
		t.Skip("run make test-offline")
	}
	workspaceBrowser(t, true)
}

func workspaceBrowser(t *testing.T, offline bool) {
	app, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	owner := account.Account.ID
	baseTime := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	if offline {
		baseTime = time.Now().UTC().Add(-24 * time.Hour)
	}
	commit := func(kind string, data any) {
		t.Helper()
		if _, err := store.Commit(owner, kind, data); err != nil {
			t.Fatal(err)
		}
	}
	commit("member.updated", irc.Member{ID: owner, Nick: "sync-reader", DisplayName: "Sync reader", Status: "online", Role: "member"})
	commit("network.updated", irc.Network{ID: "sync-network", Name: "Sync fixture", Status: "disconnected", Nick: "sync-reader", Auth: model.NetworkAuth{Method: "none"}})
	members := []string{owner}
	for i := range 45 {
		id := fmt.Sprintf("sync-member-%d", i)
		members = append(members, id)
		commit("member.updated", irc.Member{ID: id, Nick: id, DisplayName: id, Status: "online", Role: "member"})
	}
	for i := range 3 {
		id := fmt.Sprintf("sync-room-%d", i)
		commit("conversation.updated", irc.Conversation{ID: id, NetworkID: "sync-network", Kind: "channel", Name: id, Target: "#" + id, Joined: true, MemberIDs: members, MemberModes: map[string]string{owner: "o"}, Topic: "Disposable synchronization fixture"})
		for j := range 200 {
			text := fmt.Sprintf("Fixture message %d in %s", j, id)
			if offline {
				text += " · København 東京"
			}
			commit("message.created", irc.Message{ID: fmt.Sprintf("%s-message-%d", id, j), ConversationID: id, SenderID: members[1+j%45], Kind: "message", Text: text, CreatedAt: baseTime.Add(time.Duration(j) * time.Second)})
		}
	}
	var photo irc.Message
	if offline {
		commit("conversation.updated", irc.Conversation{ID: "sync-dm", NetworkID: "sync-network", Kind: "direct", Name: members[1], MemberIDs: []string{owner, members[1]}, MemberModes: map[string]string{}})
		upload := offlinePNG(t, app, owner)
		photo = irc.Message{ID: "dm-photo", ConversationID: "sync-dm", SenderID: members[1], Kind: "message", Text: "Photo of my house", CreatedAt: time.Now().UTC().Add(-time.Hour), Attachment: &model.Attachment{Name: upload.Name, URL: upload.URL, ContentType: upload.ContentType, Width: upload.Width, Height: upload.Height, Size: upload.Size}}
		commit("message.created", photo)
		old := photo
		old.ID = "old-photo"
		old.Text = "Photo kept from last month"
		old.CreatedAt = time.Now().Add(-10 * 24 * time.Hour)
		commit("message.created", old)
		commit("message.created", irc.Message{ID: "outside-retention", ConversationID: "sync-room-0", SenderID: members[1], Kind: "message", Text: "Should not be archived", CreatedAt: time.Now().Add(-8 * 24 * time.Hour)})
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	ui := fmt.Sprintf("http://127.0.0.1:%d", port)
	app.options.Origins = []string{ui}
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("deletePhoto") == "true" {
			deleted := photo
			deleted.Deleted = true
			deleted.Attachment = nil
			deleted.Text = "Message deleted"
			if _, err := store.Commit(owner, "message.updated", deleted); err != nil {
				http.Error(w, "fixture deletion failed", 500)
				return
			}
		}
		if r.URL.Query().Get("remove") == "true" {
			if err := app.Manager.Part(owner, "sync-room-1"); err != nil {
				http.Error(w, "fixture removal failed", 500)
				return
			}
		}
		if r.URL.Query().Get("removeDM") == "true" {
			if err := app.Manager.Part(owner, "sync-dm"); err != nil {
				http.Error(w, "fixture removal failed", 500)
				return
			}
		}
		for i := range 25 {
			if _, err := app.Manager.SetProfile(owner, irc.Profile{Bio: fmt.Sprintf("resume-%s-%d", r.URL.Query().Get("run"), i)}); err != nil {
				http.Error(w, "fixture mutation failed", 500)
				return
			}
		}
		if r.URL.Query().Get("expire") == "true" {
			if _, err := store.DB.Exec("DELETE FROM events WHERE owner=? AND seq<?", owner, app.Manager.Cursor(owner)); err != nil {
				http.Error(w, "fixture expiration failed", 500)
				return
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"cursor": app.Manager.Snapshot(owner).Cursor})
	}))
	defer control.Close()
	label := os.Getenv("IRCORD_SYNC_LABEL")
	if label == "" {
		label = "current"
	}
	fixture, err := json.Marshal(map[string]any{"api": server.URL, "ui": ui, "uiPort": port, "owner": owner, "username": account.Username, "password": account.Password, "control": control.URL, "label": label})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	script := "scripts/verify-sync.mjs"
	if offline {
		script = "scripts/verify-offline.mjs"
	}
	command := exec.CommandContext(ctx, "bun", script)
	command.Dir = filepath.Join("..", "..", "..", "frontend")
	command.Env = append(os.Environ(), "IRCORD_SYNC_TEST="+string(fixture))
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}
