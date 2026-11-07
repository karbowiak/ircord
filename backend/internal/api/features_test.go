package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
)

func uploadPNG(t *testing.T, client *http.Client, base, network string) filehost.Upload {
	t.Helper()
	var imageData bytes.Buffer
	_ = png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 3, 2)))
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "pixel.png")
	part.Write(imageData.Bytes())
	writer.Close()
	req, _ := http.NewRequest("POST", base+"/api/v1/uploads?networkId="+network, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("upload failed %d: %s", res.StatusCode, raw)
	}
	var upload filehost.Upload
	if err = json.NewDecoder(res.Body).Decode(&upload); err != nil {
		t.Fatal(err)
	}
	return upload
}
func TestAppDataAndFileOwnership(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	alice, bob := cookieClient(), cookieClient()
	account := register(t, alice, server.URL)
	register(t, bob, server.URL)
	var document AppDocument
	json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/app-data/library", nil, 200), &document)
	var notifications NotificationSettings
	json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/notifications/settings", nil, 200), &notifications)
	notifications.HighlightWords = []string{"private highlight"}
	request(t, alice, "PUT", server.URL+"/api/v1/notifications/settings", notifications, 200)
	if strings.Contains(string(request(t, bob, "GET", server.URL+"/api/v1/notifications/settings", nil, 200)), "private highlight") {
		t.Fatal("notification settings leaked")
	}
	old := document.Revision
	document.Data = json.RawMessage(`{ "gifs": [], "skinTone": "2", "title": "<hello>&" }`)
	json.Unmarshal(request(t, alice, "PUT", server.URL+"/api/v1/app-data/library", document, 200), &document)
	request(t, alice, "PUT", server.URL+"/api/v1/app-data/library", AppDocument{Revision: old, Data: json.RawMessage(`{}`)}, 409)
	document.Data = json.RawMessage(`{"skinTone":"3"}`)
	request(t, alice, "PUT", server.URL+"/api/v1/app-data/library", document, 200)
	var other AppDocument
	json.Unmarshal(request(t, bob, "GET", server.URL+"/api/v1/app-data/library", nil, 200), &other)
	if string(other.Data) != "null" {
		t.Fatal("app data leaked")
	}
	upload := uploadPNG(t, alice, server.URL, "")
	res, err := http.Get(upload.URL)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || res.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(res.Header.Get("Content-Type"), "image/png") {
		t.Fatal("public share link unusable")
	}
	res.Body.Close()
	rangeRequest, _ := http.NewRequest("GET", upload.URL, nil)
	rangeRequest.Header.Set("Range", "bytes=0-7")
	rangeResponse, err := http.DefaultClient.Do(rangeRequest)
	if err != nil {
		t.Fatal(err)
	}
	bytesRead, _ := io.ReadAll(rangeResponse.Body)
	rangeResponse.Body.Close()
	if rangeResponse.StatusCode != 206 || len(bytesRead) != 8 {
		t.Fatal("shareable file range requests failed")
	}
	request(t, bob, "DELETE", server.URL+"/api/v1/uploads/"+upload.ID, nil, 404)
	var imageData bytes.Buffer
	_ = png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData.Bytes())
	var profile irc.Member
	json.Unmarshal(request(t, alice, "PATCH", server.URL+"/api/v1/profile", map[string]any{"displayName": "Alice", "bio": "", "avatarDataUrl": dataURL}, 200), &profile)
	if profile.AvatarURL == "" {
		t.Fatal("profile image unavailable")
	}
	res, err = http.Get(profile.AvatarURL)
	if err != nil || res.StatusCode != 200 {
		t.Fatal("profile image is not shareable")
	}
	res.Body.Close()
	request(t, alice, "DELETE", server.URL+"/api/v1/auth/account", nil, 204)
	res, err = http.Get(upload.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatal("account deletion left shared files")
	}
	var n int
	store.DB.QueryRow("SELECT count(*) FROM app_data WHERE owner=?", account.Account.ID).Scan(&n)
	if n != 0 {
		t.Fatal("account deletion left app data")
	}
}
func TestErgoFrontendFeatureContracts(t *testing.T) {
	addr := os.Getenv("IRCORD_ERGO_ADDR")
	if addr == "" {
		t.Skip("run make test-integration")
	}
	host, portText, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portText)
	app, server, store, _ := authenticatedApp(t)
	client, other := cookieClient(), cookieClient()
	account := register(t, client, server.URL)
	register(t, other, server.URL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	suffix = suffix[len(suffix)-7:]
	room := "#features" + suffix
	nick := "feat" + suffix
	body := map[string]any{"name": "Features", "host": host, "port": port, "tls": false, "nick": nick, "auth": map[string]any{"method": "none"}, "autoJoin": []string{room}, "perform": []string{}}
	var network irc.Network
	json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/networks", body, 200), &network)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := app.Manager.WaitConnected(ctx, account.Account.ID, network.ID); err != nil {
		t.Fatal(err)
	}
	var conv irc.Conversation
	waitAPI(t, func() bool {
		for _, v := range app.Manager.Snapshot(account.Account.ID).Conversations {
			if v.NetworkID == network.ID && v.Joined && v.MemberModes[account.Account.ID] != "" {
				conv = v
				return true
			}
		}
		return false
	})
	for _, v := range app.Manager.Snapshot(account.Account.ID).Networks {
		if v.ID == network.ID {
			network = v
		}
	}
	if network.ISupport["PREFIX"] == "" || !network.Features.Setname || len(network.Features.Metadata) == 0 || network.Auth.Method != "none" {
		t.Fatal("frontend capability contract incomplete")
	}
	request(t, client, "POST", server.URL+"/api/v1/networks/"+network.ID+"/commands", map[string]string{"clientId": "topic", "line": "TOPIC " + room + " :contract topic"}, 202)
	waitAPI(t, func() bool {
		for _, v := range app.Manager.Snapshot(account.Account.ID).Conversations {
			if v.ID == conv.ID {
				return v.Topic == "contract topic"
			}
		}
		return false
	})
	for _, line := range []string{"QUIT", "CAP LS", "PRIVMSG #x :a\r\nQUIT", strings.Repeat("x", 511)} {
		request(t, client, "POST", server.URL+"/api/v1/networks/"+network.ID+"/commands", map[string]string{"clientId": "bad", "line": line}, func() int {
			if len(line) > 510 {
				return 422
			}
			return 400
		}())
	}
	request(t, client, "POST", server.URL+"/api/v1/networks/"+network.ID+"/commands", map[string]string{"clientId": "ban", "line": "MODE " + room + " +b trouble!*@*"}, 202)
	var bans struct {
		Items []irc.ChannelListEntry `json:"items"`
	}
	json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/conversations/"+conv.ID+"/lists/b", nil, 200), &bans)
	if len(bans.Items) != 1 || bans.Items[0].Mask != "trouble!*@*" {
		t.Fatal("channel mode list unavailable")
	}
	request(t, client, "PATCH", server.URL+"/api/v1/networks/"+network.ID+"/self", map[string]string{"realname": "Live realname", "away": "back soon"}, 200)
	waitAPI(t, func() bool {
		snapshot := app.Manager.Snapshot(account.Account.ID)
		for _, n := range snapshot.Networks {
			if n.ID == network.ID {
				return n.Realname == "Live realname" && snapshot.User.Status == "away"
			}
		}
		return false
	})
	var listing struct {
		Items []irc.ChannelListing `json:"items"`
	}
	json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/networks/"+network.ID+"/channels?q=features"+suffix, nil, 200), &listing)
	if len(listing.Items) != 1 || listing.Items[0].Name != room {
		t.Fatal("LIST contract failed")
	}
	request(t, client, "POST", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", map[string]any{"clientId": "action", "text": "waves", "kind": "action"}, 202)
	waitAPI(t, func() bool {
		var command CommandResponse
		json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/commands/action", nil, 200), &command)
		return command.Message != nil && command.Message.Kind == "action" && command.Message.Text == "waves"
	})
	upload := uploadPNG(t, client, server.URL, network.ID)
	request(t, other, "POST", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", map[string]any{"clientId": "stolen", "text": upload.URL, "attachments": []string{upload.ID}}, 404)
	request(t, client, "POST", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", map[string]any{"clientId": "file", "text": upload.URL, "attachments": []string{upload.ID}}, 202)
	waitAPI(t, func() bool {
		var command CommandResponse
		json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/commands/file", nil, 200), &command)
		return command.Message != nil && command.Message.Attachment != nil && command.Message.Attachment.Width == 3
	})
	// Deleting the message keeps the upload and reports it as deletable; another
	// sender's message in the same conversation is refused.
	var fileCommand CommandResponse
	json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/commands/file", nil, 200), &fileCommand)
	redactURL := server.URL + "/api/v1/conversations/" + url.PathEscape(conv.ID) + "/messages/" + url.PathEscape(fileCommand.Message.ID)
	request(t, other, "DELETE", redactURL, nil, 404)
	var redacted RedactResponse
	json.Unmarshal(request(t, client, "DELETE", redactURL, nil, 200), &redacted)
	if !redacted.Message.Deleted || !redacted.AttachmentRetained || redacted.UploadID != upload.ID {
		t.Fatalf("attachment retention not reported: %+v", redacted)
	}
	if res, err := http.Get(upload.URL); err != nil || res.StatusCode != 200 {
		t.Fatal("deleting the message deleted the file")
	} else {
		res.Body.Close()
	}
	peer := girc.New(girc.Config{Server: host, Port: port, Nick: "peer" + suffix, User: "peer", Name: "peer", AllowFlood: true})
	joined := make(chan struct{})
	peer.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) { c.Cmd.Join(room) })
	peer.Handlers.Add(girc.JOIN, func(c *girc.Client, e girc.Event) {
		if e.Source != nil && e.Source.Name == c.GetNick() {
			c.Cmd.Message(room, "from a peer")
			close(joined)
		}
	})
	go func() { _ = peer.Connect() }()
	defer peer.Close()
	select {
	case <-joined:
	case <-time.After(10 * time.Second):
		t.Fatal("peer did not join")
	}
	var foreign irc.Message
	waitAPI(t, func() bool {
		var page irc.MessagePage
		json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", nil, 200), &page)
		for _, m := range page.Items {
			if m.Text == "from a peer" {
				foreign = m
				return true
			}
		}
		return false
	})
	request(t, client, "DELETE", server.URL+"/api/v1/conversations/"+url.PathEscape(conv.ID)+"/messages/"+url.PathEscape(foreign.ID), nil, 403)
	var log irc.ServerLogPage
	json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/networks/"+network.ID+"/server-log", nil, 200), &log)
	found := false
	for _, line := range log.Items {
		if line.Kind == "outgoing" && line.Command == "TOPIC" {
			found = true
		}
	}
	if !found {
		t.Fatal("outgoing command missing from server window")
	}
	// Editing with an omitted password must preserve the write-only secret.
	password := "test-server-password"
	body["auth"] = map[string]any{"method": "server-password", "password": password}
	request(t, client, "PATCH", server.URL+"/api/v1/networks/"+network.ID, body, 200)
	body["auth"] = map[string]any{"method": "server-password"}
	raw := request(t, client, "PATCH", server.URL+"/api/v1/networks/"+network.ID, body, 200)
	if strings.Contains(string(raw), password) {
		t.Fatal("password returned to frontend")
	}
	saved, err := store.Networks(account.Account.ID)
	if err != nil || len(saved) != 1 || saved[0].Config.ServerPassword != password {
		t.Fatal("PATCH erased omitted password")
	}
	raw = request(t, client, "PATCH", server.URL+"/api/v1/networks/"+network.ID, map[string]any{"auth": map[string]any{"method": "server-password", "password": ""}}, 200)
	var cleared irc.Network
	json.Unmarshal(raw, &cleared)
	if cleared.Auth.PasswordSet || cleared.Status != "disconnected" {
		t.Fatal("clearing credentials did not disable the connection")
	}
	request(t, other, "DELETE", server.URL+"/api/v1/networks/"+network.ID, nil, 404)
	request(t, client, "DELETE", server.URL+"/api/v1/networks/"+network.ID, nil, 204)
	var count int
	store.DB.QueryRow("SELECT count(*) FROM messages WHERE owner=? AND conversation=?", account.Account.ID, conv.ID).Scan(&count)
	if count != 0 {
		t.Fatal("network deletion retained history")
	}
}
func waitAPI(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
