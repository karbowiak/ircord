package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

func readOffline(t *testing.T, c *offlineCache, key string, fetch offlineFetch) string {
	t.Helper()
	reader, release, err := c.get(context.Background(), key, time.Minute, fetch)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestOfflineCacheSharesOneFetchBetweenConcurrentViewers(t *testing.T) {
	cache := newOfflineCache(1<<10, 1<<20, 8, time.Minute)
	var fetches atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	fetch := func(ctx context.Context, dst io.Writer) error {
		if fetches.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		_, err := io.WriteString(dst, "shared bytes")
		return err
	}
	var wg sync.WaitGroup
	results := make([]string, 5)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reader, done, err := cache.get(context.Background(), "https://example.com/a", time.Minute, fetch)
			if err != nil {
				return
			}
			defer done()
			data, _ := io.ReadAll(reader)
			results[i] = string(data)
		}()
	}
	<-started
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	for i, got := range results {
		if got != "shared bytes" {
			t.Fatalf("viewer %d read %q", i, got)
		}
	}
	if got := readOffline(t, cache, "https://example.com/a", fetch); got != "shared bytes" || fetches.Load() != 1 {
		t.Fatalf("later viewer refetched: %d fetches", fetches.Load())
	}
}

func TestOfflineCacheStopsAtTheSizeLimitWhileStreaming(t *testing.T) {
	cache := newOfflineCache(16, 1<<20, 8, time.Minute)
	var fetches atomic.Int32
	fetch := func(_ context.Context, dst io.Writer) error {
		fetches.Add(1)
		var err error
		for i := 0; i < 100 && err == nil; i++ { // an unbounded body of 4-byte writes
			_, err = dst.Write([]byte("abcd"))
		}
		return err
	}
	_, _, err := cache.get(context.Background(), "big", time.Minute, fetch)
	if !errors.Is(err, errOfflineTooLarge) {
		t.Fatalf("oversized body was not refused: %v", err)
	}
	// Failures are not remembered, so a smaller original can be fetched later.
	if _, _, err = cache.get(context.Background(), "big", time.Minute, fetch); !errors.Is(err, errOfflineTooLarge) || fetches.Load() != 2 {
		t.Fatalf("failed fetch was cached: %v after %d fetches", err, fetches.Load())
	}
	if len(cache.entries) != 0 || cache.total != 0 {
		t.Fatalf("failed fetches left entries behind: %d, %d bytes", len(cache.entries), cache.total)
	}
}

func TestOfflineCacheStaysWithinItsBoundsAndExpires(t *testing.T) {
	cache := newOfflineCache(16, 20, 8, time.Minute)
	now := time.Now()
	cache.now = func() time.Time { return now }
	var fetches atomic.Int32
	body := func(text string) offlineFetch {
		return func(_ context.Context, dst io.Writer) error {
			fetches.Add(1)
			_, err := io.WriteString(dst, text)
			return err
		}
	}
	readOffline(t, cache, "a", body("0123456789"))
	now = now.Add(time.Second)
	readOffline(t, cache, "b", body("0123456789"))
	now = now.Add(time.Second)
	readOffline(t, cache, "c", body("0123456789"))
	if cache.total > 20 || len(cache.entries) != 2 || cache.entries["a"] != nil {
		t.Fatalf("oldest original was not evicted: %d bytes in %d entries", cache.total, len(cache.entries))
	}
	before := fetches.Load()
	readOffline(t, cache, "c", body("0123456789"))
	if fetches.Load() != before {
		t.Fatal("a live copy was fetched again")
	}
	now = now.Add(2 * time.Minute)
	readOffline(t, cache, "c", body("0123456789"))
	if fetches.Load() != before+1 {
		t.Fatal("an expired copy was served")
	}
}

func TestOfflineCacheKeepsAReaderWorkingAfterEviction(t *testing.T) {
	cache := newOfflineCache(16, 10, 8, time.Minute)
	write := func(text string) offlineFetch {
		return func(_ context.Context, dst io.Writer) error { _, err := io.WriteString(dst, text); return err }
	}
	reader, release, err := cache.get(context.Background(), "a", time.Minute, write("0123456789"))
	if err != nil {
		t.Fatal(err)
	}
	readOffline(t, cache, "b", write("abcdefghij")) // evicts "a" while it is being read
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "0123456789" {
		t.Fatalf("evicted file broke an active reader: %q, %v", data, err)
	}
	release()
}

func TestOfflineCacheViewerLeavingDoesNotFailTheOthers(t *testing.T) {
	cache := newOfflineCache(1<<10, 1<<20, 8, time.Minute)
	release := make(chan struct{})
	fetch := func(ctx context.Context, dst io.Writer) error {
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		_, err := io.WriteString(dst, "kept")
		return err
	}
	gone, cancel := context.WithCancel(context.Background())
	left := make(chan error, 1)
	go func() {
		_, _, err := cache.get(gone, "a", time.Minute, fetch)
		left <- err
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := <-left; !errors.Is(err, context.Canceled) {
		t.Fatalf("leaving viewer got %v", err)
	}
	close(release)
	if got := readOffline(t, cache, "a", fetch); got != "kept" {
		t.Fatalf("remaining viewer read %q", got)
	}
}

type countingTransport struct {
	fetches atomic.Int32
	body    func() io.Reader
	length  int64
}

func (c *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.fetches.Add(1)
	return &http.Response{StatusCode: 200, ContentLength: c.length, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(c.body())}, nil
}

func TestOfflineMediaSharesDownloadsAndFixesTheContentType(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	alice, bob := cookieClient(), cookieClient()
	a, b := register(t, alice, server.URL), register(t, bob, server.URL)
	remote := "https://example.com/page.html"
	// Larger than the server's response buffer, so the length must be set by the handler.
	page := strings.Repeat("<script>alert(1)</script>", 4096)
	transport := &countingTransport{body: func() io.Reader { return strings.NewReader(page) }, length: -1}
	app.offlineHTTP = &http.Client{Transport: transport}
	for _, account := range []string{a.Account.ID, b.Account.ID} {
		for _, data := range []any{irc.Conversation{ID: "room", Kind: "direct", MemberIDs: []string{account}}, irc.Message{ID: "file", ConversationID: "room", Kind: "message", CreatedAt: time.Now(), Attachment: &model.Attachment{URL: remote, Name: "page.html"}}} {
			kind := "conversation.updated"
			if _, ok := data.(irc.Message); ok {
				kind = "message.created"
			}
			if _, err := store.Commit(account, kind, data); err != nil {
				t.Fatal(err)
			}
		}
	}
	path := server.URL + "/api/v1/offline/media?conversationId=room&messageId=file&url=" + url.QueryEscape(remote)
	for _, client := range []*http.Client{alice, bob} {
		response, err := client.Get(path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || string(body) != page {
			t.Fatalf("download failed: %d %q", response.StatusCode, body)
		}
		if got := response.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("upstream content type reached the browser: %q", got)
		}
		if got := response.Header.Get("Content-Disposition"); got != `attachment; filename="offline-file"` {
			t.Fatalf("download is not an attachment: %q", got)
		}
		if response.ContentLength != int64(len(page)) || len(response.TransferEncoding) != 0 {
			t.Fatalf("length not sent: %d %v", response.ContentLength, response.TransferEncoding)
		}
	}
	if transport.fetches.Load() != 1 {
		t.Fatalf("the original was fetched %d times for two viewers", transport.fetches.Load())
	}
}

func TestOfflineMediaRefusesOversizedBodiesWithoutALength(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	a := register(t, client, server.URL)
	remote := "https://example.com/huge.bin"
	app.offlineHTTP = &http.Client{Transport: &countingTransport{body: func() io.Reader { return io.LimitReader(zeroReader{}, offlineFileBytes+1) }, length: -1}}
	for _, data := range []any{irc.Conversation{ID: "room", Kind: "direct", MemberIDs: []string{a.Account.ID}}, irc.Message{ID: "file", ConversationID: "room", Kind: "message", CreatedAt: time.Now(), Attachment: &model.Attachment{URL: remote, Name: "huge.bin"}}} {
		kind := "conversation.updated"
		if _, ok := data.(irc.Message); ok {
			kind = "message.created"
		}
		if _, err := store.Commit(a.Account.ID, kind, data); err != nil {
			t.Fatal(err)
		}
	}
	request(t, client, "GET", server.URL+"/api/v1/offline/media?conversationId=room&messageId=file&url="+url.QueryEscape(remote), nil, 413)
}

func TestOfflineMediaKeepsOnlyDetectedMediaTypes(t *testing.T) {
	var png bytes.Buffer
	if err := pngEncodeTiny(&png); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		body []byte
		want string
	}{
		"https://example.com/photo":  {png.Bytes(), "image/png"}, // served upstream as text/html
		"https://example.com/vector": {[]byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`), "application/octet-stream"},
		"https://example.com/page":   {[]byte("<!doctype html><script>alert(1)</script>"), "application/octet-stream"},
	}
	app, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	a := register(t, client, server.URL)
	bodies := map[string][]byte{}
	for raw, tc := range cases {
		bodies[raw] = tc.body
	}
	app.offlineHTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: -1, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(bytes.NewReader(bodies[r.URL.String()]))}, nil
	})}
	if _, err := store.Commit(a.Account.ID, "conversation.updated", irc.Conversation{ID: "room", Kind: "direct", MemberIDs: []string{a.Account.ID}}); err != nil {
		t.Fatal(err)
	}
	for raw, tc := range cases {
		if _, err := store.Commit(a.Account.ID, "message.created", irc.Message{ID: "m-" + raw, ConversationID: "room", Kind: "message", Text: raw, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
		response, err := client.Get(server.URL + "/api/v1/offline/media?conversationId=room&messageId=" + url.QueryEscape("m-"+raw) + "&url=" + url.QueryEscape(raw))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if got := response.Header.Get("Content-Type"); response.StatusCode != 200 || got != tc.want {
			t.Fatalf("%s: %d %q, want %q", raw, response.StatusCode, got, tc.want)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func pngEncodeTiny(w io.Writer) error {
	return png.Encode(w, image.NewRGBA(image.Rect(0, 0, 4, 4)))
}

func TestOfflineCacheBoundsRunningFetchesAndSweepsExpiredCopies(t *testing.T) {
	cache := newOfflineCache(16, 1<<20, 64, time.Minute)
	now := time.Now()
	cache.now = func() time.Time { return now }
	release := make(chan struct{})
	block := func(ctx context.Context, dst io.Writer) error {
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		_, err := io.WriteString(dst, "x")
		return err
	}
	// Viewers that leave do not free their fetch's slot.
	for i := range offlineFillSlots {
		gone, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		_, _, err := cache.get(gone, fmt.Sprint("slow-", i), time.Minute, block)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("fetch %d: %v", i, err)
		}
	}
	if _, _, err := cache.get(context.Background(), "extra", time.Minute, block); !errors.Is(err, errOfflineBusy) {
		t.Fatalf("a fifth running fetch was allowed: %v", err)
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for readOfflineErr(cache, "extra", block) != nil {
		if time.Now().After(deadline) {
			t.Fatal("slots were never released")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// An expired copy is dropped when any other URL is requested.
	now = now.Add(2 * time.Minute)
	readOffline(t, cache, "other", block)
	cache.mu.Lock()
	left := len(cache.entries)
	cache.mu.Unlock()
	if left != 1 {
		t.Fatalf("expired copies still held: %d entries", left)
	}
}

func readOfflineErr(c *offlineCache, key string, fetch offlineFetch) error {
	_, release, err := c.get(context.Background(), key, time.Minute, fetch)
	if err == nil {
		release()
	}
	return err
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
