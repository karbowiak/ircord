package irc

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/netip"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"ircord/backend/internal/filehost"
	"ircord/backend/internal/mediacdn"
	"ircord/backend/internal/model"
	"ircord/backend/internal/preview"
	"ircord/backend/internal/storage"
)

type mediaResolver struct{}

func (mediaResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
}

type mediaTransport func(*http.Request) (*http.Response, error)

func (f mediaTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestMediaSourcesIgnoreDocumentsCodeAndIncludePreviewThumbnails(t *testing.T) {
	msg := Message{Text: "https://assets.example/a.jpg https://assets.example/a.png?x=1 https://assets.example/movie.mp4 https://assets.example/paper.pdf https://assets.example/a.doc\n```\nhttps://assets.example/code.png\n```\nhttps://youtu.be/dQw4w9WgXcQ", Preview: &model.LinkPreview{Image: "https://assets.example/no-extension", Images: []model.PreviewImage{{URL: "https://assets.example/album.webp"}}}}
	items := mediaSources(msg)
	if len(items) != 6 {
		t.Fatal(items)
	}
	for _, v := range items {
		if v.Status != "loading" || v.URL != "" {
			t.Fatal("original URL published for loading media", v)
		}
	}
}
func TestIncomingMediaLoadingReadyAndPrivateReplay(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner+"-test-only"); err != nil {
			t.Fatal(err)
		}
	}
	var body bytes.Buffer
	_ = png.Encode(&body, image.NewRGBA(image.Rect(0, 0, 2, 3)))
	gate := make(chan struct{})
	var calls atomic.Int32
	fetcher := preview.New(preview.Options{Resolver: mediaResolver{}, Client: &http.Client{Transport: mediaTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		select {
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-gate:
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body.Bytes())), Request: r}, nil
	})}})
	media, err := mediacdn.New(mediacdn.Options{Store: store, Blob: filehost.Local{Root: filepath.Join(dir, "media")}, PublicURL: "https://cdn.example/media", Fetcher: fetcher})
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store, Media: media, AllowPrivate: true})
	defer m.Close()
	port, peers := capabilityServer(t, "message-tags", "")
	if _, err = m.Connect("alice", NetworkConfig{ID: "one", Host: "127.0.0.1", Port: port, Nick: "alice", Name: "test"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = m.WaitConnected(ctx, "alice", "one"); err != nil {
		t.Fatal(err)
	}
	peer := <-peers
	conv, err := m.Join("alice", "one", "#media")
	if err != nil {
		t.Fatal(err)
	}
	peer.send(":alice!u@h JOIN #media")
	eventually(t, func() bool { return findConversation(t, m, "alice", "one", "#media").Joined })
	bobCursor := m.Snapshot("bob").Cursor
	aliceCursor := m.Snapshot("alice").Cursor
	peer.send("@msgid=media-one :peer!u@h PRIVMSG #media :https://images.example/test.mp4")
	eventually(t, func() bool {
		items := page(m, "alice", conv.ID)
		return len(items) == 1 && len(items[0].Media) == 1 && items[0].Media[0].Status == "loading"
	})
	if calls.Load() != 1 {
		eventually(t, func() bool { return calls.Load() == 1 })
	}
	// IRC delivery remains live while the remote host is blocked.
	peer.send("@msgid=media-two :peer!u@h PRIVMSG #media :still chatting")
	eventually(t, func() bool { return len(page(m, "alice", conv.ID)) == 2 })
	close(gate)
	eventually(t, func() bool {
		v := page(m, "alice", conv.ID)
		return len(v) == 2 && len(v[0].Media) == 1 && v[0].Media[0].Status == "ready"
	})
	items := page(m, "alice", conv.ID)
	ready := items[0].Media[0]
	if ready.Width != 2 || ready.URL == ready.Source || ready.ContentType != "image/png" || ready.Kind != "image" {
		t.Fatal(ready)
	}
	events, _, _, stop := m.Subscribe("alice", aliceCursor)
	stop()
	loading, done := false, false
	for _, e := range events {
		var msg Message
		if json.Unmarshal(e.Data, &msg) != nil || len(msg.Media) == 0 {
			continue
		}
		loading = loading || e.Type == "message.created" && msg.Media[0].Status == "loading"
		done = done || e.Type == "message.updated" && msg.Media[0].Status == "ready"
	}
	if !loading || !done {
		t.Fatal("durable loading/ready replay missing", loading, done)
	}
	private, _, _, stop := m.Subscribe("bob", bobCursor)
	stop()
	if len(private) != 0 {
		t.Fatal("media events leaked to another account", private)
	}
	if err = media.Remove(context.Background(), ready.Hash); err != nil {
		t.Fatal(err)
	}
	m.MediaRemoved(ready.Hash)
	if got := page(m, "alice", conv.ID)[0].Media[0]; got.Status != "removed" || got.URL != "" {
		t.Fatal("removal did not invalidate live media", got)
	}
	history, err := m.HistoryContext(context.Background(), "alice", conv.ID, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, msg := range history.Items {
		if msg.UpstreamID == "media-one" {
			found = len(msg.Media) == 1 && msg.Media[0].Status == "removed"
		}
	}
	if !found {
		t.Fatal("removal did not survive history", history.Items)
	}
}

type mediaPutBarrier struct {
	filehost.Blob
	written, release chan struct{}
}

func (b mediaPutBarrier) Put(ctx context.Context, key string, r io.Reader, size int64, mime string) error {
	if err := b.Blob.Put(ctx, key, r, size, mime); err != nil {
		return err
	}
	close(b.written)
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func TestMediaRemovalBeforeReadyPublicationDoesNotReviveCopy(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateAccount("alice", "race-test-only"); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	_ = png.Encode(&body, image.NewRGBA(image.Rect(0, 0, 2, 3)))
	blob := mediaPutBarrier{Blob: filehost.Local{Root: filepath.Join(dir, "media")}, written: make(chan struct{}), release: make(chan struct{})}
	fetcher := preview.New(preview.Options{Resolver: mediaResolver{}, Client: &http.Client{Transport: mediaTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body.Bytes())), Request: r}, nil
	})}})
	media, err := mediacdn.New(mediacdn.Options{Store: store, Blob: blob, PublicURL: "https://cdn.example/media", Fetcher: fetcher})
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store, Media: media})
	defer m.Close()
	w := m.space("alice")
	raw := "https://images.example/race.png"
	msg := Message{ID: "one", ConversationID: "room", Kind: "message", Media: []model.HostedMedia{{Source: raw, Status: "loading", Kind: "image"}}}
	w.mu.Lock()
	w.messages["room"] = []Message{msg}
	w.mu.Unlock()
	m.mediaJobs <- mediaJob{workspace: w, conversation: "room", message: "one", source: raw, key: "race"}
	select {
	case <-blob.written:
	case <-time.After(5 * time.Second):
		t.Fatal("download did not reach object storage")
	}
	func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		close(blob.release)
		var record mediacdn.Record
		eventually(t, func() bool { record, err = media.Lookup(context.Background(), raw); return err == nil })
		if err = media.Remove(context.Background(), record.Hash); err != nil {
			t.Fatal(err)
		}
	}()
	eventually(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		item := w.messages["room"][0].Media[0]
		return item.Status == "removed" && item.URL == ""
	})
}
