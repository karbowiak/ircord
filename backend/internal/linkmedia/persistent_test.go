package linkmedia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/storage"
)

// memoryStore is an object store that keeps sizes and types, so stored
// copies can be checked and reused.
type memoryStore struct {
	mu                   sync.Mutex
	objects              map[string][2]string
	puts, heads, deletes int
}

func newMemoryCacheResolver(t *testing.T) (*Resolver, *memoryStore) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := &memoryStore{objects: map[string][2]string{}}
	client, err := cdn.New(cdn.Options{Endpoint: "https://storage.example.test", Bucket: "fixture", Region: "us-east-1", PublicURL: "https://cdn.example.test", Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		res := response(http.StatusOK, "application/xml", "")
		res.Header.Set("ETag", `"fixture"`)
		switch req.Method {
		case http.MethodPut:
			m.puts++
			n, _ := io.Copy(io.Discard, req.Body)
			m.objects[req.URL.Path] = [2]string{strconv.FormatInt(n, 10), req.Header.Get("Content-Type")}
		case http.MethodHead:
			m.heads++
			object, ok := m.objects[req.URL.Path]
			if !ok {
				return response(http.StatusNotFound, "application/xml", ""), nil
			}
			res.ContentLength, _ = strconv.ParseInt(object[0], 10, 64)
			res.Header.Set("Content-Length", object[0])
			res.Header.Set("Content-Type", object[1])
			res.Header.Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		case http.MethodDelete:
			m.deletes++
			delete(m.objects, req.URL.Path)
			res.StatusCode = http.StatusNoContent
		default:
			t.Errorf("unexpected storage method: %s", req.Method)
		}
		return res, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	r := NewResolver(ResolverOptions{CDN: client})
	r.persistent = &publicCache{db: store.DB, namespace: client.Namespace()}
	r.http = noSourceNetwork(t)
	return r, m
}

func (m *memoryStore) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.objects)
}

// passDay moves every stored deadline one day closer, as if a day had passed,
// then runs the background cleanup.
func passDay(t *testing.T, r *Resolver) {
	t.Helper()
	day := (24 * time.Hour).Nanoseconds()
	if _, err := r.persistent.db.Exec(`UPDATE public_media_results SET expires_ns=expires_ns-?,used_ns=used_ns-?;
		UPDATE public_media_objects SET expires_ns=expires_ns-?,limit_ns=limit_ns-?,used_ns=used_ns-?`, day, day, day, day, day); err != nil {
		t.Fatal(err)
	}
	r.cache = &resolutionCache{}
	if err := r.cleanupPublicCache(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func storedResultCount(t *testing.T, r *Resolver) int {
	t.Helper()
	var n int
	if err := r.persistent.db.QueryRow(`SELECT count(*) FROM public_media_results`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDailyPreviewRefreshReusesThumbnail(t *testing.T) {
	r, m := newMemoryCacheResolver(t)
	png := decodablePNG(t)
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/thumb.png" {
			return response(http.StatusOK, "image/png", png), nil
		}
		res := response(http.StatusOK, "text/html; charset=utf-8", `<html><head><title>An article</title><meta property="og:image" content="https://example.com/thumb.png"></head></html>`)
		res.Request = req
		return res, nil
	})}
	const days = 30
	var first string
	for day := 0; day < days; day++ {
		out, err := r.ResolveSource(context.Background(), "https://example.com/article")
		if err != nil || out.Preview == nil || out.Preview.ImageAsset == nil {
			t.Fatalf("day %d: preview without thumbnail: %+v %v", day, out, err)
		}
		if first == "" {
			first = out.Preview.ImageAsset.ObjectKey
		} else if out.Preview.ImageAsset.ObjectKey != first {
			t.Fatalf("day %d: refresh stored another copy of the thumbnail", day)
		}
		if day == 0 {
			// The object follows the one-day result, not the 90-day limit.
			var expires, limit int64
			if err := r.persistent.db.QueryRow(`SELECT expires_ns,limit_ns FROM public_media_objects`).Scan(&expires, &limit); err != nil {
				t.Fatal(err)
			}
			if deadline := time.Unix(0, expires); deadline.After(time.Now().Add(2*24*time.Hour)) || time.Unix(0, limit).Before(time.Now().Add(MediaCacheTTL-time.Hour)) {
				t.Fatalf("object deadline %v not tied to its result", deadline)
			}
		}
		passDay(t, r)
	}
	t.Logf("%d daily refreshes: uploads=%d stored objects=%d", days, m.puts, m.count())
	if m.puts != 1 || m.count() != 1 {
		t.Fatalf("daily refresh stored %d uploads, %d objects", m.puts, m.count())
	}
	// Once the page is no longer viewed, the thumbnail goes after the grace period.
	passDay(t, r)
	passDay(t, r)
	if m.count() != 0 || m.deletes != 1 {
		t.Fatalf("unreferenced thumbnail kept: objects=%d deletes=%d", m.count(), m.deletes)
	}
}

func TestFullResultTableEvictsLeastRecentlyUsed(t *testing.T) {
	r, _ := newMemoryCacheResolver(t)
	r.persistent.maxResults = 2
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		// Distinct bytes per path keep each file its own object.
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.RGBA{R: req.URL.Path[1], A: 255})
		var out bytes.Buffer
		if err := png.Encode(&out, img); err != nil {
			t.Error(err)
		}
		return response(http.StatusOK, "image/png", out.String()), nil
	})}
	links := []string{"https://media.example/a.png", "https://media.example/b.png", "https://media.example/c.png"}
	for i, raw := range links[:2] {
		if _, err := r.ResolveSource(context.Background(), raw); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		// Keep the use order distinct within the clock resolution.
		if _, err := r.persistent.db.Exec(`UPDATE public_media_results SET used_ns=? WHERE used_ns>?`, i+1, 1000); err != nil {
			t.Fatal(err)
		}
	}
	// Reading the first entry again makes the second the least recently used.
	r.cache = &resolutionCache{}
	if _, ok := r.loadPublicResult(context.Background(), links[0]); !ok {
		t.Fatal("stored result not read back")
	}
	if _, err := r.ResolveSource(context.Background(), links[2]); err != nil {
		t.Fatalf("full table refused a new result: %v", err)
	}
	if storedResultCount(t, r) != 2 {
		t.Fatal("result table grew past its limit")
	}
	for i, want := range []bool{true, false, true} {
		if _, ok := r.loadPublicResult(context.Background(), links[i]); ok != want {
			t.Fatalf("%s kept=%t, want %t", links[i], ok, want)
		}
	}
}

func TestFullObjectBudgetsEvictLeastRecentlyUsed(t *testing.T) {
	key := func(n int) string { return "link-media/" + strings.Repeat(strconv.Itoa(n), 64) }
	limit := time.Now().Add(MediaCacheTTL)
	live := func(r *Resolver) []string {
		rows, err := r.persistent.db.Query(`SELECT object_key FROM public_media_objects WHERE expires_ns>? ORDER BY object_key`, time.Now().UnixNano())
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var keys []string
		for rows.Next() {
			var k string
			_ = rows.Scan(&k)
			keys = append(keys, k)
		}
		return keys
	}
	t.Run("count", func(t *testing.T) {
		r, m := newMemoryCacheResolver(t)
		r.persistent.maxObjects = 2
		for n := 1; n <= 3; n++ {
			if err := r.reserveObject(context.Background(), key(n), 10, "", limit); err != nil {
				t.Fatalf("reserve %d: %v", n, err)
			}
		}
		if got := live(r); strings.Join(got, ",") != key(2)+","+key(3) {
			t.Fatalf("live objects after eviction: %v", got)
		}
		if err := r.cleanupPublicCache(context.Background()); err != nil || m.deletes != 1 {
			t.Fatalf("evicted object not deleted: deletes=%d %v", m.deletes, err)
		}
	})
	t.Run("bytes", func(t *testing.T) {
		r, _ := newMemoryCacheResolver(t)
		r.persistent.maxBytes = 100
		for n, size := range []int64{40, 40, 40} {
			if err := r.reserveObject(context.Background(), key(n+1), size, "", limit); err != nil {
				t.Fatalf("reserve %d: %v", n+1, err)
			}
		}
		if got := live(r); strings.Join(got, ",") != key(2)+","+key(3) {
			t.Fatalf("byte budget kept: %v", got)
		}
		if err := r.reserveObject(context.Background(), key(4), 101, "", limit); !errors.Is(err, ErrLimit) {
			t.Fatalf("file larger than the whole budget accepted: %v", err)
		}
	})
	t.Run("large upload over many small objects", func(t *testing.T) {
		r, _ := newMemoryCacheResolver(t)
		r.persistent.maxBytes = 2000
		tx, err := r.persistent.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		for n := 0; n < 1500; n++ {
			if _, err := tx.Exec(`INSERT INTO public_media_objects(namespace,object_key,expires_ns,retry_ns,size_bytes,limit_ns,used_ns) VALUES(?,?,?,0,1,?,?)`,
				r.persistent.namespace, fmt.Sprintf("link-media/%064x", n), limit.UnixNano(), limit.UnixNano(), n); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		// Room needs 1,100 victims, more than one eviction batch.
		if err := r.reserveObject(context.Background(), key(9), 1600, "", limit); err != nil {
			t.Fatalf("large upload refused while room was evictable: %v", err)
		}
		var kept int
		var bytes int64
		if err := r.persistent.db.QueryRow(`SELECT count(*),sum(size_bytes) FROM public_media_objects WHERE expires_ns>?`, time.Now().UnixNano()).Scan(&kept, &bytes); err != nil || kept != 401 || bytes != 2000 {
			t.Fatalf("live objects=%d bytes=%d %v", kept, bytes, err)
		}
	})
	t.Run("failed deletions", func(t *testing.T) {
		r, _ := newMemoryCacheResolver(t)
		r.persistent.maxObjects = 1
		if err := r.reserveObject(context.Background(), key(1), 10, "", limit); err != nil {
			t.Fatal(err)
		}
		// Evicts the first object; its deletion has not run yet.
		if err := r.reserveObject(context.Background(), key(2), 10, "", limit); err != nil {
			t.Fatal(err)
		}
		if err := r.reserveObject(context.Background(), key(3), 10, "", limit); !errors.Is(err, ErrBusy) {
			t.Fatalf("deletion backlog did not stop new uploads: %v", err)
		}
	})
}

func TestUploadingCopyStaysReusable(t *testing.T) {
	r, m := newMemoryCacheResolver(t)
	key := "link-media/" + strings.Repeat("1", 64)
	// Reserved but not yet uploaded, as while another request's Put runs.
	if err := r.reserveObject(context.Background(), key, 10, "content", time.Now().Add(MediaCacheTTL)); err != nil {
		t.Fatal(err)
	}
	var before int64
	_ = r.persistent.db.QueryRow(`SELECT expires_ns FROM public_media_objects`).Scan(&before)
	if _, _, ok := r.reuseObject(context.Background(), "content", 10, "image/png"); ok || m.heads != 1 {
		t.Fatalf("missing bytes reused: heads=%d", m.heads)
	}
	var contentKey string
	var after int64
	if err := r.persistent.db.QueryRow(`SELECT content_key,expires_ns FROM public_media_objects`).Scan(&contentKey, &after); err != nil || contentKey != "content" || after != before {
		t.Fatalf("failed check changed the copy: key=%q deadline moved=%t %v", contentKey, after != before, err)
	}
	m.mu.Lock()
	m.objects["/fixture/"+key] = [2]string{"10", "image/png"}
	m.mu.Unlock()
	if got, _, ok := r.reuseObject(context.Background(), "content", 10, "image/png"); !ok || got != key {
		t.Fatal("uploaded copy not reused")
	}
}

func TestEvictedObjectCannotBackNewResult(t *testing.T) {
	r, _ := newMemoryCacheResolver(t)
	r.persistent.maxObjects = 1
	limit := time.Now().Add(MediaCacheTTL)
	first := "link-media/" + strings.Repeat("1", 64)
	if err := r.reserveObject(context.Background(), first, 10, "", limit); err != nil {
		t.Fatal(err)
	}
	if err := r.reserveObject(context.Background(), "link-media/"+strings.Repeat("2", 64), 10, "", limit); err != nil {
		t.Fatal(err)
	}
	value := Result{Provider: "media", Assets: []Asset{{Kind: "image", ContentType: "image/png", URL: r.options.CDN.URL(first), ObjectKey: first, Size: 10, ExpiresAt: limit}}}
	if err := r.savePublicResult(context.Background(), "https://media.example/evicted.png", value); !errors.Is(err, ErrBusy) || storedResultCount(t, r) != 0 {
		t.Fatalf("result saved for an evicted object: %v", err)
	}
}
