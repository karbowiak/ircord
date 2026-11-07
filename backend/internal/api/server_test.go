package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

func request(t *testing.T, client *http.Client, method, url string, body any, status int) []byte {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	data, _ := io.ReadAll(r.Body)
	if r.StatusCode != status {
		t.Fatalf("%s %s: %d: %s", method, url, r.StatusCode, data)
	}
	return data
}
func TestAPIValidationAndOriginIsolation(t *testing.T) {
	m := irc.New(irc.Options{})
	defer m.Close()
	app := New(m, Options{Identity: DevelopmentSessions()})
	server := httptest.NewServer(app.Handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	request(t, client, "GET", server.URL+"/api/v1/bootstrap", nil, 200)
	req, _ := http.NewRequest("PATCH", server.URL+"/api/v1/preferences", strings.NewReader(`{"theme":"light"}`))
	req.Header.Set("Origin", "https://attacker.invalid")
	r, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatal("cross-origin mutation accepted")
	}
	request(t, client, "POST", server.URL+"/api/v1/networks", map[string]any{"name": "n", "host": "127.0.0.1", "port": 0, "nick": "a", "tls": false}, 400)
	data := request(t, client, "GET", server.URL+"/openapi.json", nil, 200)
	if !bytes.Contains(data, []byte(`"/api/v1/networks"`)) {
		t.Fatal("Huma schema missing network operation")
	}
	a := request(t, client, "PATCH", server.URL+"/api/v1/profile", map[string]any{"displayName": "Alice", "bio": "owner A"}, 200)
	if !bytes.Contains(a, []byte("Alice")) {
		t.Fatal("profile not saved")
	}
	secondJar, _ := cookiejar.New(nil)
	b := request(t, &http.Client{Jar: secondJar}, "GET", server.URL+"/api/v1/bootstrap", nil, 200)
	if bytes.Contains(b, []byte("Alice")) {
		t.Fatal("profile leaked between sessions")
	}
	header := http.Header{"Origin": []string{"https://attacker.invalid"}}
	if socket, resp, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/events", &websocket.DialOptions{HTTPHeader: header, HTTPClient: client}); err == nil {
		socket.CloseNow()
		t.Fatal("cross-origin websocket accepted")
	} else if resp.StatusCode != 403 {
		t.Fatal(resp.StatusCode)
	}
}
func TestErgoAPIWebSocketFlow(t *testing.T) {
	addr := os.Getenv("IRCORD_ERGO_ADDR")
	if addr == "" {
		t.Skip("run make test-integration")
	}
	host, portText, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portText)
	m := irc.New(irc.Options{AllowPrivate: true})
	defer m.Close()
	app := New(m, Options{Identity: DevelopmentSessions()})
	server := httptest.NewServer(app.Handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	var bootstrap irc.Snapshot
	json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/bootstrap", nil, 200), &bootstrap)
	socket, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/events", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{server.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	defer socket.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, _, err = socket.Read(ctx); err != nil {
		t.Fatal(err)
	}
	send := func(v any) {
		body, _ := json.Marshal(v)
		if err := socket.Write(ctx, websocket.MessageText, body); err != nil {
			t.Fatal(err)
		}
	}
	send(map[string]any{"v": 1, "type": "session.resume", "after": bootstrap.Cursor})
	suffix := fmt.Sprint(time.Now().UnixNano())
	var n irc.Network
	json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/networks", irc.NetworkConfig{ID: "real", Name: "Ergo", Host: host, Port: port, Nick: "api" + suffix[len(suffix)-8:]}, 200), &n)
	wait := func(kind string, predicate func(irc.Event) bool) irc.Event {
		for {
			_, raw, err := socket.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var e irc.Event
			if json.Unmarshal(raw, &e) != nil {
				t.Fatal("invalid event")
			}
			if e.Type == "command.error" {
				t.Fatalf("command rejected: %s", raw)
			}
			if e.Type == kind && predicate(e) {
				return e
			}
		}
	}
	wait("network.updated", func(e irc.Event) bool {
		var net irc.Network
		json.Unmarshal(e.Data, &net)
		return net.Status == "connected"
	})
	var conv irc.Conversation
	json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/conversations", map[string]string{"networkId": n.ID, "name": "api" + suffix[len(suffix)-8:]}, 200), &conv)
	wait("conversation.updated", func(e irc.Event) bool {
		var v irc.Conversation
		json.Unmarshal(e.Data, &v)
		return v.ID == conv.ID && v.Joined
	})
	send(map[string]any{"v": 1, "type": "message.send", "clientId": "browser-hi", "conversationId": conv.ID, "text": "hi from browser"})
	event := wait("message.created", func(e irc.Event) bool {
		var msg irc.Message
		json.Unmarshal(e.Data, &msg)
		return msg.ClientID == "browser-hi"
	})
	var msg irc.Message
	json.Unmarshal(event.Data, &msg)
	if msg.Text != "hi from browser" || msg.UpstreamID == "" {
		t.Fatal("not a real server echo")
	}
	data := request(t, client, "GET", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", nil, 200)
	if !bytes.Contains(data, []byte("hi from browser")) {
		t.Fatal("history mismatch")
	}
	otherJar, _ := cookiejar.New(nil)
	request(t, &http.Client{Jar: otherJar}, "GET", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", nil, 404)
	redact := server.URL + "/api/v1/conversations/" + url.PathEscape(conv.ID) + "/messages/" + url.PathEscape(msg.ID)
	request(t, &http.Client{Jar: otherJar}, "DELETE", redact, nil, 404)
	// This nick created the channel, so Ergo (which lets channel operators redact)
	// accepts the REDACT; the response and the private stream carry the change.
	var deleted RedactResponse
	json.Unmarshal(request(t, client, "DELETE", redact, nil, 200), &deleted)
	if !deleted.Message.Deleted || deleted.Message.Text != "Message deleted" || deleted.AttachmentRetained {
		t.Fatalf("redaction response: %+v", deleted)
	}
	wait("message.updated", func(e irc.Event) bool {
		var v irc.Message
		json.Unmarshal(e.Data, &v)
		return v.ID == msg.ID && v.Deleted
	})
	request(t, client, "DELETE", redact, nil, 409)
	if data = request(t, client, "GET", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", nil, 200); bytes.Contains(data, []byte("hi from browser")) {
		t.Fatal("deleted text is still in history")
	}
	// Same client ID produces the same upstream message, not a second PRIVMSG.
	send(map[string]any{"v": 1, "type": "message.send", "clientId": "browser-hi", "conversationId": conv.ID, "text": "hi from browser"})
	// A second device resumes the same owner's private stream, with no new
	// upstream connection or separate history, and receives the canonical ID.
	device, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/events", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{server.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	defer device.CloseNow()
	if _, _, err = device.Read(ctx); err != nil {
		t.Fatal(err)
	}
	resume, _ := json.Marshal(map[string]any{"v": 1, "type": "session.resume", "after": event.Seq - 1})
	if err = device.Write(ctx, websocket.MessageText, resume); err != nil {
		t.Fatal(err)
	}
	_, replayed, err := device.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var replay irc.Event
	json.Unmarshal(replayed, &replay)
	if replay.Type != "message.created" || !bytes.Contains(replay.Data, []byte(msg.ID)) {
		t.Fatal("device replay lost canonical identity")
	}
}

func TestProfileNameIsOptionalAndClearable(t *testing.T) {
	m := irc.New(irc.Options{})
	defer m.Close()
	app := New(m, Options{Identity: DevelopmentSessions()})
	server := httptest.NewServer(app.Handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	user := func(body []byte) model.Member {
		var v struct {
			User model.Member `json:"user"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatal(err)
		}
		return v.User
	}
	profile := func(fields map[string]any) model.Member {
		var v model.Member
		if err := json.Unmarshal(request(t, client, "PATCH", server.URL+"/api/v1/profile", fields, 200), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	if v := user(request(t, client, "GET", server.URL+"/api/v1/bootstrap", nil, 200)); v.NameSource == "profile" || v.DisplayName == "You" {
		t.Fatal("fresh account has a profile name", v)
	}
	if v := profile(map[string]any{"bio": "only the bio"}); v.NameSource == "profile" {
		t.Fatal("unrelated save created a profile name", v)
	}
	if v := profile(map[string]any{"displayName": "Alice", "bio": ""}); v.DisplayName != "Alice" || v.NameSource != "profile" {
		t.Fatal("explicit name not saved", v)
	}
	if v := profile(map[string]any{"bio": "kept"}); v.DisplayName != "Alice" {
		t.Fatal("omitted name cleared the profile", v)
	}
	for _, cleared := range []any{"", nil} {
		profile(map[string]any{"displayName": "Alice", "bio": ""})
		if v := profile(map[string]any{"displayName": cleared, "bio": ""}); v.NameSource == "profile" || v.DisplayName != v.Nick {
			t.Fatal("clearing did not restore the nickname", cleared, v)
		}
	}
	if v := user(request(t, client, "GET", server.URL+"/api/v1/bootstrap", nil, 200)); v.NameSource == "profile" {
		t.Fatal("cleared name came back on reload", v)
	}
}
