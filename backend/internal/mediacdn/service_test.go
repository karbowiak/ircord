package mediacdn

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ircord/backend/internal/filehost"
	"ircord/backend/internal/preview"
	"ircord/backend/internal/storage"
)

type testResolver struct{}

func (testResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
}

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func pixel(t *testing.T, red uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: red, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func fixture(t *testing.T, transport testTransport) (*Service, *storage.Store, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err = db.CreateAccount("alice", "test-only"); err != nil {
		t.Fatal(err)
	}
	fetcher := preview.New(preview.Options{Resolver: testResolver{}, Client: &http.Client{Transport: transport}})
	service, err := New(Options{Store: db, Blob: filehost.Local{Root: filepath.Join(dir, "blobs")}, Fetcher: fetcher, PublicURL: "https://cdn.example/media", MaxTotalBytes: 1024, MaxFileBytes: 512})
	if err != nil {
		t.Fatal(err)
	}
	return service, db, dir
}
func TestDeduplicationConcurrencyReferencesAndDurableRemoval(t *testing.T) {
	data := pixel(t, 1)
	var calls atomic.Int32
	s, db, dir := fixture(t, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
	})
	ctx := context.Background()
	raw := "https://images.example/a.png"
	var wg sync.WaitGroup
	records := make(chan Record, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.Fetch(ctx, raw)
			if err != nil {
				t.Error(err)
				return
			}
			records <- r
		}()
	}
	wg.Wait()
	close(records)
	if calls.Load() != 1 {
		t.Fatal("same URL fetched more than once", calls.Load())
	}
	var hash string
	for r := range records {
		hash = r.Hash
		if len(hash) != 64 || r.Width != 2 || r.Height != 3 {
			t.Fatal(r)
		}
	}
	alias, err := s.Fetch(ctx, "https://images.example/alias.png")
	if err != nil || alias.Hash != hash {
		t.Fatal(alias, err)
	}
	files, err := os.ReadDir(filepath.Join(dir, "blobs"))
	if err != nil || len(files) != 1 {
		t.Fatal("duplicate bytes stored twice", files, err)
	}
	for _, message := range []string{"one", "two"} {
		if err = s.Track(ctx, "alice", "room", message, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.RemoveMessage(ctx, "alice", "room", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Fetch(ctx, raw); err != nil {
		t.Fatal("shared media removed too early", err)
	}
	if err = s.RemoveMessage(ctx, "alice", "room", "two"); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	if _, err = s.Fetch(ctx, raw); !errors.Is(err, ErrRemoved) {
		t.Fatal("removed source refetched", err)
	}
	if _, err = s.Fetch(ctx, "https://images.example/alias.png"); !errors.Is(err, ErrRemoved) {
		t.Fatal("removed content refetched through known alias", err)
	}
	if calls.Load() != before {
		t.Fatal("removal still contacted source")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	s.options.Store = reopened
	if _, err = s.Fetch(ctx, raw); !errors.Is(err, ErrRemoved) || calls.Load() != before {
		t.Fatal("tombstone lost at restart", err)
	}
}
func TestRemovalRacingDownloadAndQuota(t *testing.T) {
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	data := pixel(t, 2)
	s, db, _ := fixture(t, func(r *http.Request) (*http.Response, error) {
		close(started)
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
	})
	raw := "https://images.example/race.png"
	if err := s.Track(ctx, "alice", "room", "one", raw); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, err := s.Fetch(ctx, raw); result <- err }()
	<-started
	if err := s.RemoveMessage(ctx, "alice", "room", "one"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-result; !errors.Is(err, ErrRemoved) {
		t.Fatal("in-flight removal resurrected content", err)
	}
	var n int
	_ = db.DB.QueryRow("SELECT count(*) FROM media_objects").Scan(&n)
	if n != 0 {
		t.Fatal("removed pending object was stored")
	}
	s.options.MaxTotalBytes = int64(len(data) - 1)
	s.options.Fetcher = preview.New(preview.Options{Resolver: testResolver{}, Client: &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
	})}})
	if _, err := s.Fetch(ctx, "https://images.example/full.png"); !errors.Is(err, ErrLimit) {
		t.Fatal("total cap ignored", err)
	}
}
func TestRetentionAndContentTombstone(t *testing.T) {
	data := pixel(t, 3)
	var calls int
	s, db, _ := fixture(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
	})
	r, err := s.Fetch(context.Background(), "https://images.example/old.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.DB.Exec("UPDATE media_objects SET created_ns=?", time.Now().Add(-31*24*time.Hour).UnixNano())
	if err = s.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Fetch(context.Background(), "https://images.example/old.png"); !errors.Is(err, ErrRemoved) || calls != 1 {
		t.Fatal("expired source was fetched again", err, calls)
	}
	if reader, _, err := s.Open(context.Background(), r.Hash); err == nil {
		reader.Close()
		t.Fatal("expired object is served")
	}
}

func testFetcher(data []byte) *preview.Service {
	return preview.New(preview.Options{Resolver: testResolver{}, Client: &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
	})}})
}

func TestFailedMetadataCommitRemovesUploadedBytes(t *testing.T) {
	data := pixel(t, 4)
	s, db, dir := fixture(t, func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
	})
	// Fail after the blob has been uploaded and the object row has been staged.
	if _, err := db.DB.Exec(`CREATE TRIGGER fail_media_source BEFORE INSERT ON media_sources BEGIN SELECT RAISE(ABORT, 'injected source failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Fetch(context.Background(), "https://images.example/fail.png"); err == nil {
		t.Fatal("metadata failure was hidden")
	}
	files, err := os.ReadDir(filepath.Join(dir, "blobs"))
	if err != nil || len(files) != 0 {
		t.Fatal("uncommitted object left behind", len(files), err)
	}
	var count int
	if err = db.DB.QueryRow("SELECT count(*) FROM media_objects").Scan(&count); err != nil || count != 0 {
		t.Fatal("partial metadata committed", count, err)
	}
}
