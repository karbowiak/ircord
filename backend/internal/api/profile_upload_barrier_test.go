package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
)

type profileBarrierBlob struct {
	filehost.Local
	entered, release chan struct{}
	once             sync.Once
}

func (b *profileBarrierBlob) Put(ctx context.Context, key string, data io.Reader, size int64, mime string) error {
	close(b.entered)
	select {
	case <-b.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return b.Local.Put(ctx, key, data, size, mime)
}
func (b *profileBarrierBlob) unblock() { b.once.Do(func() { close(b.release) }) }

func TestProfileAvatarUploadCannotApplyToRecreatedNetwork(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	cfg := irc.NetworkConfig{ID: "one", Name: "one", Host: "127.0.0.1", Port: 1, Nick: "one"}
	if err := store.SaveNetwork(account.Account.ID, cfg, false); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Manager.NetworkProfile(account.Account.ID, "one"); err != nil {
		t.Fatal(err)
	}
	blob := &profileBarrierBlob{Local: filehost.Local{Root: t.TempDir()}, entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(blob.unblock)
	files, err := filehost.New(filehost.Options{Store: store, AvatarBlob: blob, PublicURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	app.options.Files = files
	var imageData bytes.Buffer
	if err = png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"bio": "old request", "avatarDataUrl": "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData.Bytes())})
	result := make(chan int, 1)
	go func() {
		req, _ := http.NewRequest("PATCH", server.URL+"/api/v1/networks/one/profile", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			result <- 0
			return
		}
		defer response.Body.Close()
		result <- response.StatusCode
	}()
	select {
	case <-blob.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not reach barrier")
	}
	if err = app.Manager.DeleteNetwork(account.Account.ID, "one"); err != nil {
		t.Fatal(err)
	}
	if _, err = app.Manager.Connect(account.Account.ID, cfg); err != nil {
		t.Fatal(err)
	}
	blob.unblock()
	select {
	case status := <-result:
		if status != http.StatusNotFound {
			t.Fatalf("stale edit returned %d", status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not finish")
	}
	profile, err := app.Manager.NetworkProfile(account.Account.ID, "one")
	if err != nil || profile.Bio != "" || profile.AvatarURL != "" {
		t.Fatal("stale edit applied", profile, err)
	}
	var count int
	if err = store.DB.QueryRow("SELECT count(*) FROM files WHERE owner=?", account.Account.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("unexpected orphan behavior", count, err)
	}
}

func TestProfileAvatarUploadMergesConcurrentColor(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	cfg := irc.NetworkConfig{ID: "one", Name: "one", Host: "one.invalid", Port: 6697, TLS: true, Nick: "one"}
	if err := store.SaveNetwork(account.Account.ID, cfg, false); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Manager.NetworkProfile(account.Account.ID, "one"); err != nil {
		t.Fatal(err)
	}
	blob := &profileBarrierBlob{Local: filehost.Local{Root: t.TempDir()}, entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(blob.unblock)
	files, err := filehost.New(filehost.Options{Store: store, AvatarBlob: blob, PublicURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	app.options.Files = files
	var imageData bytes.Buffer
	if err = png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"bio": "uploaded", "avatarDataUrl": "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData.Bytes())})
	result := make(chan int, 1)
	go func() {
		req, _ := http.NewRequest("PATCH", server.URL+"/api/v1/networks/one/profile", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			result <- 0
			return
		}
		defer response.Body.Close()
		result <- response.StatusCode
	}()
	select {
	case <-blob.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not reach barrier")
	}
	request(t, client, "PATCH", server.URL+"/api/v1/networks/one/profile", map[string]any{"nameColor": "#abcdef"}, 200)
	blob.unblock()
	select {
	case status := <-result:
		if status != 200 {
			t.Fatalf("upload status %d", status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not finish")
	}
	profile, err := app.Manager.NetworkProfile(account.Account.ID, "one")
	if err != nil || profile.Bio != "uploaded" || profile.NameColor != "#abcdef" || profile.AvatarURL == "" {
		t.Fatal("concurrent patch lost", profile, err)
	}
}
