package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/mediacdn"
	"ircord/backend/internal/preview"
)

// This test normally uses local objects and the isolated Ergo fixture. Supplying
// IRCORD_TEST_CDN_* additionally exercises the real cluster bucket and CDN.
func TestErgoStoredMediaHTTPEventsAndDeletion(t *testing.T) {
	address := os.Getenv("IRCORD_ERGO_ADDR")
	if address == "" {
		t.Skip("set IRCORD_ERGO_ADDR for real IRC validation")
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 80, G: 140, B: 200, A: 255})
	var encoded bytes.Buffer
	_ = png.Encode(&encoded, img)
	data := append(encoded.Bytes(), []byte(suffix)...)
	var fetches atomic.Int32
	var media *mediacdn.Service
	var objectBlob filehost.Blob
	app, server, store, _ := authenticatedApp(t, func(options *irc.Options) {
		objectBlob = filehost.Local{Root: filepath.Join(t.TempDir(), "media")}
		publicURL := "https://cdn.example/media"
		if endpoint := os.Getenv("IRCORD_TEST_CDN_ENDPOINT"); endpoint != "" {
			b, e := filehost.NewS3(endpoint, "ircord-media", "us-east-1", os.Getenv("IRCORD_TEST_CDN_ACCESS_KEY"), os.Getenv("IRCORD_TEST_CDN_SECRET_KEY"), false)
			if e != nil {
				t.Fatal(e)
			}
			b.Prefix = "media/"
			objectBlob = b
			publicURL = os.Getenv("IRCORD_TEST_CDN_PUBLIC_URL")
		}
		media, err = mediacdn.New(mediacdn.Options{Store: options.Store, Blob: objectBlob, PublicURL: publicURL, Fetcher: testMediaFetcher(data, &fetches)})
		if err != nil {
			t.Fatal(err)
		}
		options.Media = media
	})
	user, outsider := cookieClient(), cookieClient()
	account := register(t, user, server.URL)
	other := register(t, outsider, server.URL)
	var bootstrap struct {
		Capabilities struct {
			MediaCDN bool `json:"mediaCDN"`
		}
	}
	json.Unmarshal(request(t, user, "GET", server.URL+"/api/v1/bootstrap", nil, 200), &bootstrap)
	if !bootstrap.Capabilities.MediaCDN {
		t.Fatal("client capability missing")
	}
	var network irc.Network
	json.Unmarshal(request(t, user, "POST", server.URL+"/api/v1/networks", map[string]any{"name": "CDN test", "host": host, "port": port, "tls": false, "nick": "cdn" + suffix[len(suffix)-8:]}, 200), &network)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err = app.Manager.WaitConnected(ctx, account.Account.ID, network.ID); err != nil {
		t.Fatal(err)
	}
	var conv irc.Conversation
	json.Unmarshal(request(t, user, "POST", server.URL+"/api/v1/conversations", map[string]string{"networkId": network.ID, "name": "cdn" + suffix}, 200), &conv)
	waitAPI(t, func() bool {
		for _, c := range app.Manager.Snapshot(account.Account.ID).Conversations {
			if c.ID == conv.ID {
				return c.Joined
			}
		}
		return false
	})
	cursor := app.Manager.Snapshot(account.Account.ID).Cursor
	otherCursor := app.Manager.Snapshot(other.Account.ID).Cursor
	raw := "https://fixture.example/" + suffix + ".png"
	request(t, user, "POST", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", map[string]string{"clientId": "store-media", "text": raw}, 202)
	var msg irc.Message
	waitAPI(t, func() bool {
		command, e := store.Command(account.Account.ID, "store-media")
		if e != nil || command.MessageID == "" {
			return false
		}
		msg, e = store.Message(account.Account.ID, conv.ID, command.MessageID)
		return e == nil && len(msg.Media) == 1 && msg.Media[0].Status == "ready"
	})
	defer objectBlob.Delete(context.Background(), msg.Media[0].Hash)
	events, _, _, stop := app.Manager.Subscribe(account.Account.ID, cursor)
	stop()
	loading, ready := false, false
	for _, event := range events {
		var value irc.Message
		if json.Unmarshal(event.Data, &value) != nil || len(value.Media) == 0 {
			continue
		}
		loading = loading || value.Media[0].Status == "loading"
		ready = ready || value.Media[0].Status == "ready"
	}
	if !loading || !ready {
		t.Fatal("private durable media progress missing", loading, ready)
	}
	private, _, _, stop := app.Manager.Subscribe(other.Account.ID, otherCursor)
	stop()
	if len(private) != 0 {
		t.Fatal("media leaked outside the account")
	}
	if fetches.Load() != 1 {
		t.Fatal("source fetched more than once", fetches.Load())
	}
	if os.Getenv("IRCORD_TEST_CDN_PUBLIC_URL") != "" {
		response, e := http.Get(msg.Media[0].URL)
		if e != nil {
			t.Fatal(e)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || !bytes.Equal(body, data) {
			t.Fatal("real CDN copy did not match", response.StatusCode)
		}
	}
	// The creator is an operator in this disposable channel, which Ergo permits
	// to redact. Both the IRC deletion and object cleanup must finish.
	request(t, user, "DELETE", server.URL+"/api/v1/conversations/"+conv.ID+"/messages/"+msg.ID, nil, 200)
	waitAPI(t, func() bool { _, e := media.Lookup(context.Background(), raw); return errors.Is(e, mediacdn.ErrRemoved) })
	request(t, user, "POST", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", map[string]string{"clientId": "removed-media", "text": raw}, 202)
	waitAPI(t, func() bool {
		command, e := store.Command(account.Account.ID, "removed-media")
		if e != nil || command.MessageID == "" {
			return false
		}
		again, e := store.Message(account.Account.ID, conv.ID, command.MessageID)
		return e == nil && len(again.Media) == 1 && again.Media[0].Status == "removed"
	})
	if fetches.Load() != 1 {
		t.Fatal("removed link was automatically fetched again", fetches.Load())
	}
}

func testMediaFetcher(data []byte, counter *atomic.Int32) *preview.Service {
	return preview.New(preview.Options{Resolver: publicResolver{}, Client: &http.Client{Transport: previewTransport(func(r *http.Request) (*http.Response, error) {
		counter.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
	})}})
}
