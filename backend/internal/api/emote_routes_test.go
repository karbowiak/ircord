package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/emotes"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/storage"
)

// emoteApp is an authenticated backend whose emote catalog lets managers (usernames) change it.
func emoteApp(t *testing.T, managers func(*Options)) (*httptest.Server, *emotes.Service) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	root := t.TempDir()
	base := "http://" + server.Listener.Addr().String()
	files, err := filehost.New(filehost.Options{Store: store, Blob: filehost.Local{Root: filepath.Join(root, "files")}, AvatarBlob: filehost.Local{Root: filepath.Join(root, "avatars")}, PublicURL: base})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := emotes.Open(emotes.Options{Store: store, Blob: filehost.Local{Root: filepath.Join(root, "files")}, PublicURL: base})
	if err != nil {
		t.Fatal(err)
	}
	manager := irc.New(irc.Options{Store: store, Files: files, AllowPrivate: true})
	service := auth.New(store, auth.Options{Registration: true, BeforeDelete: func(owner string) error {
		if err := manager.DropOwner(owner); err != nil {
			return err
		}
		return files.DeleteAccount(context.Background(), owner)
	}})
	options := Options{Store: store, Files: files, Auth: service, Emotes: catalog}
	managers(&options)
	server.Config.Handler = New(manager, options).Handler
	server.Start()
	t.Cleanup(func() { service.Close(); manager.Close(); server.Close(); store.Close() })
	return server, catalog
}

func uploadEmote(t *testing.T, client *http.Client, base string, fields map[string]string, status int) emotes.Emote {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", "wave.png")
	_ = png.Encode(part, image.NewNRGBA(image.Rect(0, 0, 24, 24)))
	for k, v := range fields {
		_ = form.WriteField(k, v)
	}
	form.Close()
	req, _ := http.NewRequest("POST", base+"/api/v1/emotes", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != status {
		t.Fatalf("upload: %d %s", res.StatusCode, data)
	}
	var e emotes.Emote
	_ = json.Unmarshal(data, &e)
	return e
}
func catalogOf(t *testing.T, client *http.Client, base string) emotes.EmoteCatalog {
	t.Helper()
	var c emotes.EmoteCatalog
	if err := json.Unmarshal(request(t, client, "GET", base+"/api/v1/emotes", nil, 200), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEmoteManagementIsPermittedAndSharedAcrossAccounts(t *testing.T) {
	adminClient, memberClient := cookieClient(), cookieClient()
	server, _ := emoteApp(t, func(o *Options) { o.EmoteManagers = []string{"emoteboss"} })
	admin := testAccount{Username: "EmoteBoss", Password: "correct-horse-battery"}
	if err := json.Unmarshal(request(t, adminClient, "POST", server.URL+"/api/v1/auth/signup", admin.credentials(), 201), &admin.LoginResult); err != nil {
		t.Fatal(err)
	}
	register(t, memberClient, server.URL)

	if c := catalogOf(t, memberClient, server.URL); c.CanManage || len(c.Items) != 0 {
		t.Fatalf("member catalog: %+v", c)
	}
	if !catalogOf(t, adminClient, server.URL).CanManage {
		t.Fatal("the configured manager cannot manage")
	}
	request(t, cookieClient(), "GET", server.URL+"/api/v1/emotes", nil, 401)
	uploadEmote(t, memberClient, server.URL, map[string]string{"shortcode": "nope"}, 403)
	request(t, memberClient, "POST", server.URL+"/api/v1/emotes/imports", map[string]string{"provider": "7tv", "providerId": "01GA29CZ2R000C36HNE7Z0DQXD"}, 403)

	wave := uploadEmote(t, adminClient, server.URL, map[string]string{"shortcode": "Wave", "aliases": "hello, hi", "creator": "Our artist"}, 201)
	if wave.Shortcode != "wave" || wave.Name != "Wave" || strings.Join(wave.Aliases, ",") != "Wave,hello,hi" || wave.Creator != "Our artist" {
		t.Fatalf("uploaded emote: %+v", wave)
	}
	uploadEmote(t, adminClient, server.URL, map[string]string{"shortcode": "HELLO"}, 409)
	request(t, memberClient, "PATCH", server.URL+"/api/v1/emotes/"+wave.ID, map[string]any{"enabled": false}, 403)
	request(t, memberClient, "DELETE", server.URL+"/api/v1/emotes/"+wave.ID, nil, 403)

	// Every account sees the entry and can load its image without a session.
	shared := catalogOf(t, memberClient, server.URL)
	if len(shared.Items) != 1 || shared.Items[0].ID != wave.ID {
		t.Fatalf("member does not see the shared emote: %+v", shared.Items)
	}
	raw, _ := json.Marshal(shared)
	if strings.Contains(string(raw), admin.Account.ID) || strings.Contains(string(raw), admin.Username) {
		t.Fatal("the catalog exposes the uploader's account")
	}
	res, err := http.Get(wave.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/png" || !strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
		t.Fatalf("emote image: %d %v", res.StatusCode, res.Header)
	}
	for _, bad := range []string{"/emotes/../files", "/emotes/0123456789abcdef0123456789abcdef.png", "/emotes/xyz"} {
		if res, _ := http.Get(server.URL + bad); res == nil || res.StatusCode != 404 {
			t.Fatalf("%s should not be served", bad)
		}
	}

	// A live member socket learns about changes; a reconnect learns the current version.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	socket, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/events", &websocket.DialOptions{HTTPClient: memberClient, HTTPHeader: http.Header{"Origin": []string{server.URL}}})
	if err != nil {
		t.Fatal(err)
	}
	defer socket.CloseNow()
	var ready struct {
		Type         string `json:"type"`
		EmoteVersion int64  `json:"emoteVersion"`
	}
	_, frame, err := socket.Read(ctx)
	if err != nil || json.Unmarshal(frame, &ready) != nil || ready.Type != "session.ready" || ready.EmoteVersion != shared.Version {
		t.Fatalf("session.ready: %s %v", frame, err)
	}
	_ = socket.Write(ctx, websocket.MessageText, []byte(`{"v":1,"type":"session.resume","after":0}`))
	renamed := request(t, adminClient, "PATCH", server.URL+"/api/v1/emotes/"+wave.ID, map[string]any{"shortcode": "waving", "name": "waving"}, 200)
	var updated emotes.Emote
	_ = json.Unmarshal(renamed, &updated)
	if len(updated.Retired) != 2 || !slices.Contains(updated.Retired, "wave") || !slices.Contains(updated.Retired, "Wave") {
		t.Fatalf("rename keeps old names for history: %+v", updated)
	}
	for {
		_, frame, err = socket.Read(ctx)
		if err != nil {
			t.Fatal("no emotes.updated event")
		}
		var event struct {
			Type string `json:"type"`
			Data struct {
				Version int64 `json:"version"`
			} `json:"data"`
		}
		_ = json.Unmarshal(frame, &event)
		if event.Type == "emotes.updated" {
			if event.Data.Version != shared.Version+1 {
				t.Fatalf("event version %d", event.Data.Version)
			}
			break
		}
	}

	// Deleting the uploader's account leaves the shared emote and its image in place.
	request(t, adminClient, "DELETE", server.URL+"/api/v1/auth/account", map[string]string{"password": admin.Password}, 204)
	if c := catalogOf(t, memberClient, server.URL); len(c.Items) != 1 || c.Items[0].Shortcode != "waving" {
		t.Fatalf("account deletion removed a shared emote: %+v", c.Items)
	}
	if res, err := http.Get(wave.URL); err != nil || res.StatusCode != 200 {
		t.Fatal("account deletion removed a shared emote image")
	}
}

func TestEveryAccountMayManageWithWildcard(t *testing.T) {
	server, _ := emoteApp(t, func(o *Options) { o.EmoteManagers = []string{"*"} })
	client := cookieClient()
	register(t, client, server.URL)
	e := uploadEmote(t, client, server.URL, map[string]string{"shortcode": "party"}, 201)
	request(t, client, "PATCH", server.URL+"/api/v1/emotes/"+e.ID, map[string]any{"shortcode": "x"}, 422)
	request(t, client, "DELETE", server.URL+"/api/v1/emotes/"+e.ID, nil, 204)
	request(t, client, "DELETE", server.URL+"/api/v1/emotes/"+e.ID, nil, 404)
	request(t, client, "POST", server.URL+"/api/v1/emotes/imports", map[string]string{"provider": "bttv", "providerId": "1"}, 422)
}
