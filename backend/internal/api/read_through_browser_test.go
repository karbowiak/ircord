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

// The browser uses a disposable account, backend, frontend, and retained messages.
func TestReadThroughBrowser(t *testing.T) {
	if os.Getenv("IRCORD_READ_THROUGH_BROWSER") != "true" {
		t.Skip("set IRCORD_READ_THROUGH_BROWSER=true with an isolated frontend build")
	}
	frontend := os.Getenv("IRCORD_READ_THROUGH_FRONTEND")
	if frontend == "" {
		t.Fatal("IRCORD_READ_THROUGH_FRONTEND must name the isolated built frontend")
	}
	app, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	owner := account.Account.ID
	commit := func(kind string, data any) {
		t.Helper()
		if _, err := store.Commit(owner, kind, data); err != nil {
			t.Fatal(err)
		}
	}
	commit("network.updated", irc.Network{ID: "read-net", Name: "Read fixture", Status: "disconnected", Nick: "reader", Auth: model.NetworkAuth{Method: "none"}})
	for _, id := range []string{"read-room", "old-room", "dispose-room"} {
		commit("conversation.updated", irc.Conversation{ID: id, NetworkID: "read-net", Kind: "channel", Name: id, Target: "#" + id, Joined: true, MemberIDs: []string{owner}})
		commit("message.created", irc.Message{ID: id + "-a", ConversationID: id, SenderID: "peer", Kind: "message", Text: "Observed A", CreatedAt: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)})
	}
	app.Manager.Snapshot(owner)
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		msg := irc.Message{ID: "read-room-b", ConversationID: "read-room", SenderID: "peer", Kind: "message", Text: "Later B", CreatedAt: time.Date(2026, 11, 14, 12, 0, 1, 0, time.UTC)}
		if err := store.SaveHistory(owner, msg); err != nil {
			http.Error(w, "fixture insert failed", 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer control.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	ui := fmt.Sprintf("http://127.0.0.1:%d", port)
	app.options.Origins = []string{ui}
	fixture, err := json.Marshal(map[string]any{"api": server.URL, "ui": ui, "uiPort": port, "owner": owner, "username": account.Username, "password": account.Password, "control": control.URL, "frontend": frontend})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "bun", "scripts/verify-read-through-browser.mjs")
	command.Dir = filepath.Join("..", "..", "..", "frontend")
	command.Env = append(os.Environ(), "IRCORD_READ_THROUGH_TEST="+string(fixture))
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}
