package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/emotes"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/storage"
)

func authenticatedApp(t *testing.T, customize ...func(*irc.Options)) (*Server, *httptest.Server, *storage.Store, *auth.Service) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	root := t.TempDir()
	files, err := filehost.New(filehost.Options{Store: store, Blob: filehost.Local{Root: filepath.Join(root, "files")}, AvatarBlob: filehost.Local{Root: filepath.Join(root, "avatars")}, PublicURL: "http://" + server.Listener.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := emotes.Open(emotes.Options{Store: store, Blob: filehost.Local{Root: filepath.Join(root, "emotes")}, PublicURL: "http://" + server.Listener.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	options := irc.Options{Store: store, Files: files, AllowPrivate: true}
	for _, change := range customize {
		change(&options)
	}
	manager := irc.New(options)
	var ended func(owner, session string)
	if options.Router != nil {
		ended = options.Router.EndSession
	}
	service := auth.New(store, auth.Options{Registration: true, SessionEnded: ended, BeforeDelete: func(owner string) error {
		if err := manager.DropOwner(owner); err != nil {
			return err
		}
		return files.DeleteAccount(context.Background(), owner)
	}})
	app := New(manager, Options{Media: options.Media, Store: store, Files: files, Emotes: catalog, Push: options.Push, Auth: service, Activity: options.Router})
	if options.Router != nil {
		options.Router.SetSource(manager)
	}
	server.Config.Handler = app.Handler
	server.Start()
	t.Cleanup(func() { service.Close(); manager.Close(); server.Close(); store.Close() })
	return app, server, store, service
}
func TestErgoAuthenticatedHTTPPostingAndDiagnostics(t *testing.T) {
	addr := os.Getenv("IRCORD_ERGO_ADDR")
	if addr == "" {
		t.Skip("run make test-integration")
	}
	host, portString, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portString)
	app, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	other := cookieClient()
	register(t, other, server.URL)
	socket := accountSocket(t, client, server.URL, 0)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	var network irc.Network
	json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/networks", irc.NetworkConfig{ID: "real", Name: "Ergo", Host: host, Port: port, Nick: "http" + suffix[len(suffix)-8:]}, 200), &network)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := app.Manager.WaitConnected(ctx, account.Account.ID, network.ID); err != nil {
		t.Fatal(err)
	}
	var conv irc.Conversation
	json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/conversations", map[string]string{"networkId": network.ID, "name": "http" + suffix[len(suffix)-8:]}, 200), &conv)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := app.Manager.Snapshot(account.Account.ID)
		joined := false
		for _, v := range snapshot.Conversations {
			if v.ID == conv.ID && v.Joined {
				joined = true
			}
		}
		if joined {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	body := map[string]any{"clientId": "http-send", "text": "authenticated HTTP message"}
	request(t, client, "POST", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", body, 202)
	var accepted irc.Message
	for {
		_, raw, err := socket.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var event irc.Event
		json.Unmarshal(raw, &event)
		if event.Type == "message.created" {
			json.Unmarshal(event.Data, &accepted)
			if accepted.ClientID == "http-send" {
				break
			}
		}
	}
	if accepted.UpstreamID == "" {
		t.Fatal("HTTP request did not reach real IRC")
	}
	var command CommandResponse
	json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/commands/http-send", nil, 200), &command)
	if command.State != "sent" || command.Message == nil {
		t.Fatal("durable command status missing")
	}
	request(t, client, "POST", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", body, 202)
	request(t, other, "GET", server.URL+"/api/v1/commands/http-send", nil, 404)
	request(t, other, "GET", server.URL+"/api/v1/networks/real/connection", nil, 404)
	var info irc.ConnectionInfo
	json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/networks/real/connection", nil, 200), &info)
	if info.State != "connected" || len(info.ISupport) == 0 || len(info.OfferedCapabilities) == 0 || info.ServerName == "" {
		t.Fatal("connection information incomplete", info)
	}
	var logs irc.ServerLogPage
	json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/networks/real/server-log", nil, 200), &logs)
	if len(logs.Items) == 0 {
		t.Fatal("missing server window log")
	}
	json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/networks/real/connection/trace", nil, 200), &logs)
	found := false
	for _, line := range logs.Items {
		if line.Command == "PRIVMSG" && strings.Contains(line.Text, "authenticated HTTP message") {
			found = true
		}
	}
	if !found {
		t.Fatal("raw diagnostic stream omitted conversation traffic")
	}
	var count int
	store.DB.QueryRow("SELECT count(*) FROM messages WHERE owner=? AND text='authenticated HTTP message'", account.Account.ID).Scan(&count)
	if count != 1 {
		t.Fatal("HTTP retry duplicated IRC delivery")
	}
}
func cookieClient() *http.Client { jar, _ := cookiejar.New(nil); return &http.Client{Jar: jar} }

// testAccount is a signed-up account and the credentials that sign it in.
type testAccount struct {
	auth.LoginResult
	Username, Password string
}

func (a testAccount) credentials() map[string]string {
	return map[string]string{"username": a.Username, "password": a.Password}
}

var accountCounter atomic.Int64

func register(t *testing.T, client *http.Client, base string) testAccount {
	t.Helper()
	a := testAccount{Username: fmt.Sprintf("tester%d_%d", time.Now().UnixNano()%1e6, accountCounter.Add(1)), Password: "correct-horse-battery"}
	if err := json.Unmarshal(request(t, client, "POST", base+"/api/v1/auth/signup", a.credentials(), 201), &a.LoginResult); err != nil {
		t.Fatal(err)
	}
	return a
}
func accountSocket(t *testing.T, client *http.Client, base string, after uint64) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	socket, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http")+"/api/v1/events", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{base}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = socket.Read(ctx); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"v": 1, "type": "session.resume", "after": after})
	if err = socket.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { socket.CloseNow() })
	return socket
}
func TestAnonymousAuthHTTPAndRevokedSockets(t *testing.T) {
	app, server, store, service := authenticatedApp(t)
	anonymous := cookieClient()
	request(t, anonymous, "GET", server.URL+"/api/v1/bootstrap", nil, 401)
	if body := request(t, anonymous, "GET", server.URL+"/api/v1/auth/session", nil, 200); !strings.Contains(string(body), `"account":null`) {
		t.Fatal("signed-out session contract mismatch")
	}
	first := cookieClient()
	account := register(t, first, server.URL)
	if account.Account.Username != account.Username || account.Account.ExpiresAt.Sub(account.Account.LastSeenAt) != storage.AccountRetention {
		t.Fatal("frontend account contract mismatch")
	}
	second := cookieClient()
	request(t, second, "POST", server.URL+"/api/v1/auth/login", map[string]string{"username": strings.ToUpper(account.Username), "password": account.Password}, 200)
	request(t, cookieClient(), "POST", server.URL+"/api/v1/auth/login", map[string]string{"username": account.Username, "password": "wrong-password"}, 401)
	other := cookieClient()
	otherAccount := register(t, other, server.URL)
	firstSocket := accountSocket(t, first, server.URL, 0)
	secondSocket := accountSocket(t, second, server.URL, 0)
	request(t, first, "PATCH", server.URL+"/api/v1/profile", map[string]any{"displayName": "Alice", "bio": "private profile"}, 200)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, socket := range []*websocket.Conn{firstSocket, secondSocket} {
		_, raw, err := socket.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "profile.updated") {
			t.Fatal("private event not delivered")
		}
	}
	if strings.Contains(string(request(t, other, "GET", server.URL+"/api/v1/bootstrap", nil, 200)), "private profile") {
		t.Fatal("cross-account data leakage")
	}
	request(t, first, "POST", server.URL+"/api/v1/auth/password", map[string]string{"current": "wrong-password", "password": "battery-staple-horse"}, 403)
	request(t, first, "POST", server.URL+"/api/v1/auth/password", map[string]string{"current": account.Password, "password": "battery-staple-horse"}, 204)
	if _, _, err := secondSocket.Read(ctx); err == nil {
		t.Fatal("password change left the other session connected")
	}
	request(t, second, "GET", server.URL+"/api/v1/bootstrap", nil, 401)
	request(t, cookieClient(), "POST", server.URL+"/api/v1/auth/login", account.credentials(), 401)
	account.Password = "battery-staple-horse"
	replacement := cookieClient()
	request(t, replacement, "POST", server.URL+"/api/v1/auth/login", account.credentials(), 200)
	if app.Manager.Snapshot(account.Account.ID).User.DisplayName != "Alice" {
		t.Fatal("password change changed account ownership")
	}
	raw, _ := json.Marshal(map[string]any{"v": 1, "type": "message.send", "clientId": "forbidden", "conversationId": "fake", "text": "no websocket commands"})
	if err := firstSocket.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
	// The password change and the new sign-in announced devices.updated first.
	_, raw, err := firstSocket.Read(ctx)
	for err == nil && strings.Contains(string(raw), `"devices.updated"`) {
		_, raw, err = firstSocket.Read(ctx)
	}
	if err != nil || !strings.Contains(string(raw), "command.error") {
		t.Fatal("authenticated websocket accepted a write command")
	}
	request(t, first, "POST", server.URL+"/api/v1/auth/logout", nil, 204)
	if _, _, err = firstSocket.Read(ctx); err == nil {
		t.Fatal("logout did not close session socket")
	}
	request(t, replacement, "DELETE", server.URL+"/api/v1/auth/account", nil, 204)
	var count int
	store.DB.QueryRow("SELECT count(*) FROM accounts WHERE id=?", account.Account.ID).Scan(&count)
	if count != 0 {
		t.Fatal("account deletion left account data")
	}
	store.DB.Exec("UPDATE accounts SET last_seen_ns=? WHERE id=?", time.Now().Add(-91*24*time.Hour).UnixNano(), otherAccount.Account.ID)
	if err = service.PruneExpired(); err != nil {
		t.Fatal(err)
	}
	request(t, other, "GET", server.URL+"/api/v1/auth/session", nil, 200)
}
