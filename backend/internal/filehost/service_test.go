package filehost

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"ircord/backend/internal/storage"
)

func fixture(t *testing.T) (*storage.Store, string, []byte) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner); err != nil {
			t.Fatal(err)
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var data bytes.Buffer
	_ = png.Encode(&data, img)
	return store, dir, data.Bytes()
}
func TestLocalUploadPersistenceIsolationAndDeletion(t *testing.T) {
	store, dir, data := fixture(t)
	blob := Local{Root: filepath.Join(dir, "files")}
	service, err := New(Options{Store: store, Blob: blob, AvatarBlob: blob, PublicURL: "https://chat.example"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	upload, err := service.Upload(ctx, "alice", "network", "cat.png", data, service.LocalProvider())
	if err != nil {
		t.Fatal(err)
	}
	if upload.Width != 2 || upload.Height != 3 || upload.ContentType != "image/png" {
		t.Fatal("image metadata lost")
	}
	if _, err = service.Record("bob", upload.ID); err != ErrNotFound {
		t.Fatal("other owner read upload")
	}
	if _, err = service.Attachment("alice", "other-network", upload.ID); err != ErrNotFound {
		t.Fatal("upload escaped its network")
	}
	u, _ := url.Parse(upload.URL)
	parts := strings.Split(u.Path, "/")
	_, body, err := service.Download(ctx, parts[2], parts[3])
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := io.ReadAll(body)
	body.Close()
	if !bytes.Equal(actual, data) {
		t.Fatal("file changed in storage")
	}
	if _, _, err = service.Download(ctx, parts[2], "invalid"); err != ErrNotFound {
		t.Fatal("guessable download token")
	}
	record, _ := service.Record("alice", upload.ID)
	stat, err := os.Stat(filepath.Join(blob.Root, record.ObjectKey))
	if err != nil || stat.Mode().Perm() != 0600 {
		t.Fatal("upload permissions")
	}
	service, err = New(Options{Store: store, Blob: blob, PublicURL: "https://chat.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Record("alice", upload.ID); err != nil {
		t.Fatal("metadata not persistent")
	}
	if err = service.Delete(ctx, "bob", upload.ID); err != ErrNotFound {
		t.Fatal("cross-owner deletion")
	}
	if err = service.Delete(ctx, "alice", upload.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.Download(ctx, parts[2], parts[3]); err != ErrNotFound {
		t.Fatal("deleted link still works")
	}
	service.options.AvatarBlob = blob
	if _, err = service.Upload(ctx, "alice", "network", "x.svg", []byte("<svg xmlns='http://www.w3.org/2000/svg'><script>alert(1)</script></svg>"), service.AvatarProvider()); err == nil {
		t.Fatal("accepted active profile image content")
	}
}
func TestUpstreamDiscoveryAuthenticationAndFallback(t *testing.T) {
	store, _, data := fixture(t)
	posts := 0
	options := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			t.Error("application cookie forwarded")
		}
		if r.Method == "OPTIONS" {
			options++
			if r.Header.Get("Authorization") != "" {
				t.Error("discovery exposed credentials")
			}
			w.Header().Set("Accept-Post", "image/png")
			w.WriteHeader(204)
			return
		}
		if r.Method == "POST" {
			posts++
			user, password, ok := r.BasicAuth()
			if !ok || user != "upstream-account" || password != "upstream-secret" {
				t.Error("upstream SASL credentials missing")
			}
			body, _ := io.ReadAll(r.Body)
			if !bytes.Equal(body, data) || r.Header.Get("Content-Type") != "image/png" {
				t.Error("raw upload contract mismatch")
			}
			w.Header().Set("Location", "/download/example.png")
			w.WriteHeader(201)
			return
		}
		t.Error("unexpected upstream request")
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	service, err := New(Options{Store: store, Client: client})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	provider := service.Resolve(ctx, parsed.Hostname(), server.URL+"/upload", "upstream-account", "upstream-secret")
	if !provider.Available || provider.Provider != "upstream" {
		t.Fatal(provider)
	}
	upload, err := service.Upload(ctx, "alice", "network", "cat.png", data, provider)
	if err != nil || upload.URL != server.URL+"/download/example.png" || posts != 1 {
		t.Fatal("upstream upload failed", err)
	}
	if service.Resolve(ctx, "untrusted.example", server.URL, "upstream-account", "upstream-secret").Available {
		t.Fatal("credentials forwarded to an unapproved destination")
	}
	if service.Resolve(ctx, parsed.Hostname(), "http://example.org", "", "").Available {
		t.Fatal("unencrypted filehost accepted")
	}
	if service.Resolve(ctx, parsed.Hostname(), "", "", "").Available {
		t.Fatal("upload advertised without a provider")
	}
	service.options.Blob = Local{Root: t.TempDir()}
	before := options
	if service.Resolve(ctx, parsed.Hostname(), server.URL, "upstream-account", "upstream-secret").Provider != "local" || options != before {
		t.Fatal("upstream took priority over local storage")
	}
}
func TestS3RoundTrip(t *testing.T) {
	endpoint := os.Getenv("IRCORD_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set IRCORD_TEST_S3_ENDPOINT for the dedicated S3 integration service")
	}
	blob, err := NewS3(endpoint, "ircord-tests", "us-east-1", "GK00000000000000000000000000000001", "0000000000000000000000000000000000000000000000000000000000000002", false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	exists, err := blob.Client.BucketExists(ctx, blob.Bucket)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		if err = blob.Client.MakeBucket(ctx, blob.Bucket, minio.MakeBucketOptions{Region: "us-east-1"}); err != nil {
			t.Fatal(err)
		}
	}
	key := random(16)
	data := []byte("S3 storage round trip")
	if err = blob.Put(ctx, key, bytes.NewReader(data), int64(len(data)), "text/plain"); err != nil {
		t.Fatal(err)
	}
	defer blob.Delete(ctx, key)
	body, err := blob.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatal("S3 changed upload", err)
	}
	if err = blob.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if body, err = blob.Open(ctx, key); err == nil {
		body.Close()
		t.Fatal("S3 deletion failed")
	}
}

func TestUpstreamUploadNeverFollowsRedirects(t *testing.T) {
	store, _, data := fixture(t)
	followed := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed = true; w.WriteHeader(201) }))
	defer destination.Close()
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.Header().Set("Accept-Post", "image/png")
			w.WriteHeader(204)
			return
		}
		w.Header().Set("Location", destination.URL)
		w.WriteHeader(307)
	}))
	defer upstream.Close()
	service, err := New(Options{Store: store, Client: upstream.Client()})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(upstream.URL)
	provider := service.Resolve(context.Background(), u.Hostname(), upstream.URL, "irc-user", "irc-secret")
	if !provider.Available {
		t.Fatal("discovery failed")
	}
	if _, err = service.Upload(context.Background(), "alice", "network", "image.png", data, provider); err == nil || followed {
		t.Fatal("upstream redirect followed with a credential-bearing upload")
	}
}
