package cdn

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStorageRoundTripAndRanges(t *testing.T) {
	data := []byte("a small CDN object")
	var stored []byte
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fixture/media/images/example.png" {
			t.Errorf("unexpected key %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("storage request was not signed")
		}
		switch r.Method {
		case http.MethodPut:
			stored, _ = io.ReadAll(r.Body)
			w.Header().Set("ETag", `"fixture-etag"`)
		case http.MethodHead, http.MethodGet:
			if stored == nil {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("ETag", `"fixture-etag"`)
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			body := stored
			status := 200
			if r.Method == http.MethodGet && r.Header.Get("Range") != "" {
				start, end := 0, len(stored)-1
				_, _ = fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
				body = stored[start : end+1]
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(stored)))
				status = 206
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.WriteHeader(status)
			if r.Method == http.MethodGet {
				_, _ = w.Write(body)
			}
		case http.MethodDelete:
			stored = nil
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected method %s", r.Method)
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	c, err := New(Options{Endpoint: server.URL, Bucket: "fixture", Region: "us-east-1", Prefix: "media", PublicURL: "https://cdn.example.test/assets", AccessKey: "test-access", SecretKey: "test-secret", Transport: server.Client().Transport})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	object, err := c.Put(ctx, "images/example.png", bytes.NewReader(data), int64(len(data)), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, data) || object.URL != "https://cdn.example.test/assets/media/images/example.png" || object.Size != int64(len(data)) {
		t.Fatalf("unexpected upload: %+v bytes=%q", object, stored)
	}
	for _, read := range []ReadOptions{{}, {Offset: 2, Length: 4}, {Length: 1}, {Offset: 5}} {
		body, info, err := c.Open(ctx, "images/example.png", read)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(body)
		body.Close()
		end := int64(len(data))
		if read.Length > 0 {
			end = read.Offset + read.Length
		}
		if err != nil || !bytes.Equal(got, data[read.Offset:end]) || info.ContentType != "image/png" {
			t.Fatalf("range %+v: %q, %+v, %v", read, got, info, err)
		}
	}
	info, err := c.Stat(ctx, "images/example.png")
	if err != nil || info.Size != int64(len(data)) {
		t.Fatalf("stat: %+v %v", info, err)
	}
	if err := c.Delete(ctx, "images/example.png"); err != nil {
		t.Fatal(err)
	}
	if body, _, err := c.Open(ctx, "images/example.png", ReadOptions{}); err == nil {
		body.Close()
		t.Fatal("missing object returned a reader")
	}
	for _, key := range []string{"../other", "a/../b", "/absolute", "a%2fb", "a?token=x", "."} {
		if _, err := c.Put(ctx, key, strings.NewReader("x"), 1, "text/plain"); err == nil {
			t.Fatalf("accepted key %q", key)
		}
	}
}

func TestLiveStorage(t *testing.T) {
	endpoint := os.Getenv("IRCORD_TEST_CDN_ENDPOINT")
	if endpoint == "" {
		t.Skip("set IRCORD_TEST_CDN_ENDPOINT and bucket/credentials for an opt-in storage probe")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	prefix := os.Getenv("IRCORD_TEST_CDN_PREFIX")
	if prefix == "" {
		prefix = "media"
	}
	c, err := New(Options{Endpoint: endpoint, Bucket: os.Getenv("IRCORD_TEST_CDN_BUCKET"), Region: "us-east-1", Prefix: prefix, PublicURL: os.Getenv("IRCORD_TEST_CDN_PUBLIC_URL"), AccessKey: os.Getenv("IRCORD_TEST_CDN_ACCESS_KEY"), SecretKey: os.Getenv("IRCORD_TEST_CDN_SECRET_KEY")})
	if err != nil {
		t.Fatal(err)
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	key := hex.EncodeToString(nonce[:])
	data := append([]byte("\x89PNG\r\n\x1a\nIRCord disposable storage probe"), nonce[:]...)
	if _, err := c.Put(ctx, key, bytes.NewReader(data), int64(len(data)), "image/png"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.Delete(cleanup, key); err != nil {
			t.Error(err)
		}
	})
	body, _, err := c.Open(ctx, key, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("stored bytes differ", err)
	}
	if c.URL(key) != "" {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.URL(key), nil)
		req.Header.Set("Range", "bytes=0-7")
		req.Header.Set("Origin", "http://127.0.0.1:3000")
		req.Header.Set("User-Agent", "Mozilla/5.0")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal("public CDN read failed", err)
		}
		public, err := io.ReadAll(io.LimitReader(res.Body, 512))
		res.Body.Close()
		if err != nil || res.StatusCode != 206 || !bytes.Equal(public, data[:8]) || res.Header.Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("public CDN range/CORS: HTTP %d bytes=%d err=%v", res.StatusCode, len(public), err)
		}
	}
}
