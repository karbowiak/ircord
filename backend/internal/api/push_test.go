package api

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/lrstanley/girc"
	"io"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/notify"
	"ircord/backend/internal/push"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type pushTransport func(*http.Request) (*http.Response, error)

func (f pushTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestErgoPushRulesQuietHoursAndIgnoredUnread(t *testing.T) {
	address := os.Getenv("IRCORD_ERGO_ADDR")
	if address == "" {
		t.Skip("run make test-integration")
	}
	host, portText, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(portText)
	var calls atomic.Int32
	var pushes *push.Service
	app, server, _, _ := authenticatedApp(t, func(options *irc.Options) {
		var err error
		pushes, err = push.New(push.Options{Store: options.Store, DataDir: t.TempDir(), Subject: "mailto:tests@example.invalid", Client: &http.Client{Transport: pushTransport(func(r *http.Request) (*http.Response, error) {
			if !strings.HasSuffix(r.URL.Path, "alice-device") {
				t.Error("wrong owner's endpoint received push")
			}
			if r.Header.Get("Content-Encoding") != "aes128gcm" {
				t.Error("push payload is not encrypted")
			}
			calls.Add(1)
			return &http.Response{StatusCode: 201, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
		})}})
		if err != nil {
			t.Fatal(err)
		}
		options.Push = pushes
	})
	defer pushes.Close()
	client, other := cookieClient(), cookieClient()
	account := register(t, client, server.URL)
	register(t, other, server.URL)
	key, _ := ecdh.P256().GenerateKey(rand.Reader)
	secret := make([]byte, 16)
	rand.Read(secret)
	subscription := map[string]any{"endpoint": "https://fcm.googleapis.com/fcm/send/alice-device", "expirationTime": nil, "keys": map[string]string{"p256dh": base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), "auth": base64.RawURLEncoding.EncodeToString(secret)}}
	request(t, client, "POST", server.URL+"/api/v1/push/subscriptions", map[string]any{"subscription": subscription, "device": "test browser"}, 204)
	request(t, client, "POST", server.URL+"/api/v1/push/test", nil, 204)
	if calls.Load() != 1 {
		t.Fatal("test push was not sent")
	}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	suffix = suffix[len(suffix)-7:]
	nick := "push" + suffix
	room := "#push" + suffix
	var network irc.Network
	json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/networks", map[string]any{"name": "Push", "host": host, "port": port, "tls": false, "nick": nick, "autoJoin": []string{room}}, 200), &network)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := app.Manager.WaitConnected(ctx, account.Account.ID, network.ID); err != nil {
		t.Fatal(err)
	}
	var conv irc.Conversation
	waitAPI(t, func() bool {
		for _, v := range app.Manager.Snapshot(account.Account.ID).Conversations {
			if v.NetworkID == network.ID && v.Joined {
				conv = v
				return true
			}
		}
		return false
	})
	peer := girc.New(girc.Config{Server: host, Port: port, Nick: "peer" + suffix, User: "test", Name: "test", AllowFlood: true})
	ready, done := make(chan struct{}), make(chan struct{})
	peer.Handlers.Add(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) { close(ready) })
	go func() { defer close(done); _ = peer.Connect() }()
	defer func() { peer.Close(); <-done }()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("peer did not connect")
	}
	peer.Cmd.Join(room)
	waitAPI(t, func() bool { return peer.LookupChannel(room) != nil })
	request(t, client, "POST", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", map[string]string{"clientId": "own", "text": "my own message"}, 202)
	waitAPI(t, func() bool {
		command, e := app.Manager.Command(account.Account.ID, "own")
		return e == nil && command.State == "sent"
	})
	if calls.Load() != 1 {
		t.Fatal("own message notified")
	}
	peer.Cmd.Message(room, "hey @"+nick)
	waitAPI(t, func() bool { return calls.Load() == 2 })
	t.Cleanup(func() {
		if t.Failed() {
			snapshot := app.Manager.Snapshot(account.Account.ID)
			t.Logf("conversations: %+v", snapshot.Conversations)
			t.Logf("user=%s calls=%d", snapshot.User.ID, calls.Load())
		}
	})
	rules := notify.Defaults()
	rules.ChannelDefault = "all"
	rules.QuietHours = notify.QuietHours{Enabled: true, Start: time.Now().UTC().Add(-time.Hour).Format("15:04"), End: time.Now().UTC().Add(time.Hour).Format("15:04"), TimeZone: "UTC"}
	request(t, client, "PUT", server.URL+"/api/v1/notifications/settings", rules, 200)
	peer.Cmd.Message(room, "quiet @"+nick)
	waitAPI(t, func() bool {
		for _, v := range app.Manager.Snapshot(account.Account.ID).Conversations {
			if v.ID == conv.ID {
				return v.UnreadCount == 2 && v.MentionCount == 2
			}
		}
		return false
	})
	if calls.Load() != 2 {
		t.Fatal("quiet hours sent a push")
	}
	rules.QuietHours.Enabled = false
	for _, member := range app.Manager.Snapshot(account.Account.ID).Members {
		if member.Nick == peer.GetNick() {
			if member.Username == "" || member.Host == "" {
				t.Fatal("IRC identity mask is missing")
			}
			rules.Ignore = []string{"*!" + member.Username + "@" + member.Host}
		}
	}
	request(t, client, "PUT", server.URL+"/api/v1/notifications/settings", rules, 200)
	waitAPI(t, func() bool {
		for _, v := range app.Manager.Snapshot(account.Account.ID).Conversations {
			if v.ID == conv.ID {
				return v.UnreadCount == 0 && v.MentionCount == 0
			}
		}
		return false
	})
	peer.Cmd.Message(room, "ignored @"+nick)
	waitAPI(t, func() bool {
		results, e := app.Manager.Search(account.Account.ID, "ignored @", conv.ID)
		return e == nil && len(results) == 1
	})
	if calls.Load() != 2 {
		t.Fatal("ignored sender pushed")
	}
	if strings.Contains(string(request(t, other, "GET", server.URL+"/api/v1/notifications/settings", nil, 200)), peer.GetNick()) {
		t.Fatal("notification policy leaked")
	}
	rules.Ignore = []string{}
	request(t, client, "PUT", server.URL+"/api/v1/notifications/settings", rules, 200)
	request(t, client, "POST", server.URL+"/api/v1/auth/logout", nil, 204)
	peer.Cmd.Message(room, "after logout @"+nick)
	waitAPI(t, func() bool {
		results, e := app.Manager.Search(account.Account.ID, "after logout @", conv.ID)
		return e == nil && len(results) == 1
	})
	time.Sleep(150 * time.Millisecond)
	if calls.Load() != 2 {
		t.Fatal("logged-out session received push")
	}
}
