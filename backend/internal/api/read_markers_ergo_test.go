package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/irc"
)

// Runs two actual browser contexts against the Go API and a native girc client
// on the same disposable Ergo account. No production account or channel is used.
func TestErgoReadMarkerDevices(t *testing.T) {
	addr := os.Getenv("IRCORD_ERGO_ADDR")
	if addr == "" || os.Getenv("IRCORD_READ_MARKER_BROWSER") != "true" {
		t.Skip("run make test-read-markers")
	}
	host, portString, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(random[:6])
	nick, room, password := "reader"+suffix, "#read"+suffix, hex.EncodeToString(random[:])
	var mu sync.Mutex
	messages, markers := map[string]time.Time{}, map[string]time.Time{}
	registered := make(chan struct{}, 1)
	reader := markerPeer(t, girc.Config{Server: host, Port: port, Nick: nick, User: "reader", Name: "Disposable read-marker test", AllowFlood: true, SupportedCaps: map[string][]string{"server-time": nil, "draft/read-marker": nil}}, func(c *girc.Client) {
		c.Handlers.Add("NOTICE", func(_ *girc.Client, e girc.Event) {
			if strings.Contains(e.Last(), "Account created") {
				select {
				case registered <- struct{}{}:
				default:
				}
			}
		})
		c.Handlers.Add("PRIVMSG", func(_ *girc.Client, e girc.Event) {
			if at, err := time.Parse(time.RFC3339Nano, e.Tags["time"]); err == nil {
				mu.Lock()
				messages[e.Last()] = at
				mu.Unlock()
			}
		})
		c.Handlers.Add("MARKREAD", func(_ *girc.Client, e girc.Event) {
			if len(e.Params) != 2 {
				return
			}
			if at, err := time.Parse(time.RFC3339Nano, strings.TrimPrefix(e.Last(), "timestamp=")); err == nil {
				mu.Lock()
				markers[e.Params[0]] = at
				mu.Unlock()
			}
		})
	})
	reader.Send(&girc.Event{Command: "PRIVMSG", Params: []string{"NickServ", "REGISTER " + password}, Sensitive: true})
	select {
	case <-registered:
	case <-time.After(10 * time.Second):
		t.Fatal("disposable upstream registration failed")
	}
	reader.Cmd.Join(room)
	sender := markerPeer(t, girc.Config{Server: host, Port: port, Nick: "sender" + suffix, User: "sender", Name: "Disposable sender", AllowFlood: true}, nil)
	sender.Cmd.Join(room)
	app, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	other := cookieClient()
	register(t, other, server.URL)
	// Reserve a separate UI port without touching the user's running dev server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	uiPort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	ui := fmt.Sprintf("http://127.0.0.1:%d", uiPort)
	app.options.Origins = []string{ui}
	var network irc.Network
	if err := json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/networks", irc.NetworkConfig{ID: "read-test", Name: "Read marker fixture", Host: host, Port: port, Nick: nick, SASLUser: nick, SASLPassword: password, AutoJoin: []string{room}}, 200), &network); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	if err = app.Manager.WaitConnected(ctx, account.Account.ID, network.ID); err != nil {
		t.Fatal(err)
	}
	markerWait(t, func() bool {
		for _, c := range app.Manager.Snapshot(account.Account.ID).Conversations {
			if c.Target == room && c.Joined {
				return true
			}
		}
		return false
	})
	// Native read commands are emitted only with real timestamps observed on IRC.
	// This loopback-only control plane is part of the test, not the shipped API.
	token := hex.EncodeToString(random[:])
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", 401)
			return
		}
		var in struct{ Text, Target string }
		if r.Method == "POST" {
			if json.NewDecoder(r.Body).Decode(&in) != nil {
				http.Error(w, "invalid input", 400)
				return
			}
		}
		target := room
		if in.Target == "direct" {
			target = sender.GetNick()
		}
		switch r.URL.Path {
		case "/send":
			destination := room
			if in.Target == "direct" {
				destination = nick
			}
			sender.Cmd.Message(destination, in.Text)
		case "/read":
			mu.Lock()
			at, ok := messages[in.Text]
			mu.Unlock()
			if !ok {
				http.Error(w, "message not observed on native client", 409)
				return
			}
			reader.Send(&girc.Event{Command: "MARKREAD", Params: []string{target, "timestamp=" + at.UTC().Format("2006-01-02T15:04:05.000Z")}})
		case "/state":
			mu.Lock()
			defer mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"messages": messages, "markers": markers})
			return
		default:
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer control.Close()
	config := map[string]any{"api": server.URL, "ui": ui, "uiPort": uiPort, "username": account.Username, "password": account.Password, "accountId": account.Account.ID, "networkId": network.ID, "room": room, "nick": nick, "sender": sender.GetNick(), "control": control.URL, "token": token}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Abs("../../../frontend")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "node", "scripts/verify-read-markers.mjs")
	command.Dir = dir
	command.Env = append(os.Environ(), "IRCORD_READ_TEST="+string(raw))
	output, err := command.CombinedOutput()
	safe := strings.ReplaceAll(string(output), account.Password, "[disposable password]")
	safe = strings.ReplaceAll(safe, token, "[disposable token]")
	t.Log(safe)
	if err != nil {
		lines, _ := app.Manager.ServerLog(account.Account.ID, network.ID, "", 100, true)
		for _, line := range lines.Items {
			if line.Command == "CHATHISTORY" || line.Command == "BATCH" || line.Command == "FAIL" || line.Command == "MARKREAD" {
				t.Log(line.Command, line.Params)
			}
		}
		t.Fatal("browser/native verification failed:", err)
	}
	// The other IRCord account has no visibility of read state or conversations.
	if strings.Contains(string(request(t, other, "GET", server.URL+"/api/v1/bootstrap", nil, 200)), room) {
		t.Fatal("read-test workspace leaked to another account")
	}
}

func markerPeer(t *testing.T, cfg girc.Config, handlers func(*girc.Client)) *girc.Client {
	t.Helper()
	c := girc.New(cfg)
	if handlers != nil {
		handlers(c)
	}
	ready, done := make(chan struct{}), make(chan struct{})
	c.Handlers.Add(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) { close(ready) })
	go func() { defer close(done); _ = c.Connect() }()
	t.Cleanup(func() { c.Close(); <-done })
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("native IRC client did not connect")
	}
	return c
}

func markerWait(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("read-marker fixture did not become ready")
}
