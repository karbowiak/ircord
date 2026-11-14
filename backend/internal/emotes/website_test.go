package emotes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func websiteCard(base, id, name, creator, ext string) string {
	return fmt.Sprintf(`<div class="card asset-card"><img data-src="%s/cdn/%s.%s"><a href="%s/emoji/%s">View</a><div class="asset-metadata"><a href="%s/emoji/%s">%s</a><a href="%s/user/123">%s</a></div></div>`, base, id, ext, base, id, base, id, name, base, creator)
}
func websitePageHTML(cards, next string) string {
	return `<html><body><div id="items">` + cards + next + `</div></body></html>`
}
func websiteDetailHTML(base, id, name, creator, ext, license string) string {
	return fmt.Sprintf(`<html><head><link rel="canonical" href="%s/emoji/%s"></head><body><img id="emojiPreview" src="%s/cdn/%s.%s" alt="%s Emoji"><a href="%s/user/123">%s</a><h5><i></i>%s</h5></body></html>`, base, id, base, id, ext, name, base, creator, license)
}

func TestWebsiteSearchPagingPreviewImportAndIdentity(t *testing.T) {
	s, f := discoveryFixture(t)
	base := f.server.URL
	s.options.Endpoints.EmojiGGWeb = base
	const first = "887554-hyperkekw"
	const second = "773784-kekw"
	f.handle("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "kekw" {
			http.Redirect(w, r, base+"/emojis/kekw", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(websitePageHTML(websiteCard(base, "907939-okdog", "okdog", "masaru_86", "png"), "")))
	})
	f.set("/emojis/kekw", 200, []byte(websitePageHTML(websiteCard(base, first, "hyperkekw", "xchasex", "gif")+websiteCard(base, second, "kekw", "artist", "png"), `<a class="next-page" href="`+base+`/emojis/kekw&amp;page=1&amp;sort=recent">Next</a>`)))
	f.set("/emojis/kekw&page=1&sort=recent", 200, []byte(websitePageHTML(websiteCard(base, "55252-nazunakekw", "nazunakekw", "creator", "png"), "")))
	f.set("/emoji/"+first, 200, []byte(websiteDetailHTML(base, first, "hyperkekw", "xchasex", "gif", "Basic License")))
	f.set("/cdn/"+first+".gif", 200, animatedGIF(t))
	ctx := context.Background()
	one, err := s.Discover(ctx, DiscoveryQuery{Query: "kekw", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Items) != 1 || one.Items[0].ProviderID != first || one.Items[0].Creator != "xchasex" || len(one.Filters.Categories) != 0 || one.NextCursor == "" {
		t.Fatalf("first page %+v", one)
	}
	secondPage, err := s.Discover(ctx, DiscoveryQuery{Query: "kekw", Limit: 1, Cursor: one.NextCursor})
	if err != nil || len(secondPage.Items) != 1 || secondPage.Items[0].ProviderID != second || secondPage.NextCursor == "" {
		t.Fatalf("second page %+v %v", secondPage, err)
	}
	third, err := s.Discover(ctx, DiscoveryQuery{Query: "kekw", Limit: 1, Cursor: secondPage.NextCursor})
	if err != nil || len(third.Items) != 1 || third.Items[0].ProviderID != "55252-nazunakekw" {
		t.Fatalf("third page %+v %v", third, err)
	}
	filtered, err := s.Discover(ctx, DiscoveryQuery{Query: "kekw", Animation: "animated"})
	if err != nil || len(filtered.Items) != 1 || filtered.NextCursor == "" {
		t.Fatalf("filtered pagination %+v %v", filtered, err)
	}
	last, err := s.Discover(ctx, DiscoveryQuery{Query: "kekw", Animation: "animated", Cursor: filtered.NextCursor})
	if err != nil || len(last.Items) != 0 {
		t.Fatalf("filtered next %+v %v", last, err)
	}
	other, err := s.Discover(ctx, DiscoveryQuery{Query: "dog"})
	if err != nil || len(other.Items) != 1 || other.Items[0].ProviderID != "907939-okdog" {
		t.Fatalf("other query %+v %v", other, err)
	}
	if _, err = s.Discover(ctx, DiscoveryQuery{Query: "kekw", Category: "3"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unsupported category %v", err)
	}
	if _, err = s.Discover(ctx, DiscoveryQuery{Query: "dog", Cursor: one.NextCursor, Limit: 1}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mismatched cursor %v", err)
	}
	still, err := s.PreviewStatic(ctx, "emoji.gg", first)
	if err != nil || still.Animated || still.ContentType != "image/png" {
		t.Fatalf("preview %+v %v", still, err)
	}
	imported, err := s.Import(ctx, "emoji.gg", first, New{Shortcode: "hyperkekw", AddedBy: "owner"})
	if err != nil || !imported.Animated || imported.License != "Basic License" || imported.Creator != "xchasex" || imported.StaticURL == imported.URL {
		t.Fatalf("import %+v %v", imported, err)
	}
	installed, err := s.Discover(ctx, DiscoveryQuery{Query: "kekw"})
	if err != nil || !installed.Items[0].Installed {
		t.Fatal("installed status missing")
	}
	if _, err = s.Import(ctx, "emoji.gg", first, New{Shortcode: "another", AddedBy: "owner"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate import %v", err)
	}
	s.website.mu.Lock()
	delete(s.website.entries, first)
	s.website.mu.Unlock()
	if _, err = s.discoverEntry(ctx, first); err != nil {
		t.Fatalf("detail fallback %v", err)
	}
	// A cursor cannot survive eviction or snapshot replacement.
	s.website.mu.Lock()
	delete(s.website.pages, websiteKey("kekw", "kekw", 0))
	s.website.mu.Unlock()
	if _, err = s.Discover(ctx, DiscoveryQuery{Query: "kekw", Limit: 1, Cursor: one.NextCursor}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expired cursor %v", err)
	}
}

func TestWebsiteFeaturedBrowse(t *testing.T) {
	s, f := discoveryFixture(t)
	base := f.server.URL
	f.handle("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(websitePageHTML(websiteCard(base, "33_second", "second", "artist", "png"), "")))
			return
		}
		_, _ = w.Write([]byte(websitePageHTML(websiteCard(base, "22_first", "first", "artist", "gif"), `<a class="next-page" href="`+base+`/?page=1&amp;sort=featured">Next</a>`)))
	})
	first, err := s.Discover(context.Background(), DiscoveryQuery{})
	if err != nil || len(first.Items) != 1 || len(first.Filters.Categories) != 0 || first.NextCursor == "" {
		t.Fatalf("featured %+v %v", first, err)
	}
	next, err := s.Discover(context.Background(), DiscoveryQuery{Cursor: first.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].ProviderID != "33_second" {
		t.Fatalf("featured next %+v %v", next, err)
	}
	filtered, err := s.Discover(context.Background(), DiscoveryQuery{Animation: "static"})
	if err != nil || len(filtered.Items) != 0 || filtered.NextCursor == "" {
		t.Fatalf("empty filtered page %+v %v", filtered, err)
	}
	filteredNext, err := s.Discover(context.Background(), DiscoveryQuery{Animation: "static", Cursor: filtered.NextCursor})
	if err != nil || len(filteredNext.Items) != 1 || filteredNext.Items[0].ProviderID != "33_second" {
		t.Fatalf("filtered next page %+v %v", filteredNext, err)
	}
	if _, err = s.Discover(context.Background(), DiscoveryQuery{Category: "3"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unsupported category %v", err)
	}
	if _, err = s.Import(context.Background(), "emoji.gg", "12345", New{Shortcode: "invalid"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("numeric emoji ID accepted: %v", err)
	}
}

func TestWebsiteSearchRejectsCorruptionAndUnsafeURLs(t *testing.T) {
	for _, body := range []string{`<html>broken</html>`, strings.Repeat("x", (1<<20)+1), websitePageHTML(websiteCard("https://evil.example", "1_bad", "bad", "bad", "png"), "")} {
		s, f := discoveryFixture(t)
		s.options.Endpoints.EmojiGGWeb = f.server.URL
		f.set("/search", 200, []byte(body))
		if _, err := s.Discover(context.Background(), DiscoveryQuery{Query: "bad"}); !errors.Is(err, ErrProvider) {
			t.Fatalf("accepted corrupt page: %v", err)
		}
	}
	s, f := discoveryFixture(t)
	s.options.Endpoints.EmojiGGWeb = f.server.URL
	f.handle("/search", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.example/emojis/bad", http.StatusFound)
	})
	if _, err := s.Discover(context.Background(), DiscoveryQuery{Query: "bad"}); !errors.Is(err, ErrProvider) {
		t.Fatalf("accepted redirect: %v", err)
	}
	f.set("/emoji/1_bad", 200, []byte(websiteDetailHTML("https://evil.example", "1_bad", "bad", "artist", "png", "Basic License")))
	if _, err := s.websiteDetail(context.Background(), "1_bad"); !errors.Is(err, ErrProvider) {
		t.Fatalf("accepted unsafe detail: %v", err)
	}
	if _, err := s.discoverEntry(context.Background(), "../../internal"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("accepted unsafe ID: %v", err)
	}
}

func TestWebsiteSearchCoalescesAndRetainsSafeStale(t *testing.T) {
	s, f := discoveryFixture(t)
	base := f.server.URL
	s.options.Endpoints.EmojiGGWeb = base
	gate := make(chan struct{})
	f.handle("/search", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-gate:
			_, _ = w.Write([]byte(websitePageHTML(websiteCard(base, "907939-okdog", "okdog", "artist", "png"), "")))
		case <-r.Context().Done():
		}
	})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Discover(context.Background(), DiscoveryQuery{Query: "dog"}); err != nil {
				t.Error(err)
			}
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.website.mu.Lock()
		p := s.website.pending[websiteKey("dog", "", 0)]
		count := 0
		if p != nil {
			count = p.waiters
		}
		s.website.mu.Unlock()
		if count == 12 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("lookups not coalesced")
		}
		time.Sleep(time.Millisecond)
	}
	close(gate)
	wg.Wait()
	if f.requests.Load() != 1 {
		t.Fatalf("search requests %d", f.requests.Load())
	}
	s.website.mu.Lock()
	key := websiteKey("dog", "", 0)
	old := s.website.pages[key]
	old.fetched = time.Now().Add(-10 * time.Minute)
	s.website.pages[key] = old
	s.website.mu.Unlock()
	f.set("/search", 200, []byte("<html>corrupt</html>"))
	stale, err := s.Discover(context.Background(), DiscoveryQuery{Query: "dog"})
	if err != nil || !stale.Stale || len(stale.Items) != 1 {
		t.Fatalf("stale search %+v %v", stale, err)
	}
	requests := f.requests.Load()
	if _, err = s.Discover(context.Background(), DiscoveryQuery{Query: "dog"}); err != nil || f.requests.Load() != requests {
		t.Fatal("failed refresh was not throttled")
	}
	s.website.mu.Lock()
	old = s.website.pages[key]
	old.fetched = time.Now().Add(-2 * time.Hour)
	s.website.pages[key] = old
	s.website.mu.Unlock()
	f.set("/search", 503, nil)
	if _, err = s.Discover(context.Background(), DiscoveryQuery{Query: "dog"}); !errors.Is(err, ErrProvider) {
		t.Fatalf("expired stale search %v", err)
	}
}

func TestWebsiteSearchCanceledWaiterDoesNotCancelSharedFetch(t *testing.T) {
	s, f := discoveryFixture(t)
	base := f.server.URL
	s.options.Endpoints.EmojiGGWeb = base
	gate := make(chan struct{})
	f.handle("/search", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-gate:
			_, _ = w.Write([]byte(websitePageHTML(websiteCard(base, "907939-okdog", "okdog", "artist", "png"), "")))
		case <-r.Context().Done():
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { _, err := s.Discover(ctx, DiscoveryQuery{Query: "dog"}); first <- err }()
	wait := func(n int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			s.website.mu.Lock()
			p := s.website.pending[websiteKey("dog", "", 0)]
			count := 0
			if p != nil {
				count = p.waiters
			}
			s.website.mu.Unlock()
			if count == n {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("waiting for shared website request")
			}
			time.Sleep(time.Millisecond)
		}
	}
	wait(1)
	go func() { _, err := s.Discover(context.Background(), DiscoveryQuery{Query: "dog"}); second <- err }()
	wait(2)
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	close(gate)
	if err := <-second; err != nil {
		t.Fatalf("other waiter: %v", err)
	}
}

// This opt-in check fetches provider search metadata only and never downloads images.
func TestWebsiteLiveSearch(t *testing.T) {
	if os.Getenv("IRCORD_TEST_LIVE_EMOJI") != "1" {
		t.Skip("live provider check is opt-in")
	}
	s, _, _ := newService(t, nil)
	for _, q := range []string{"", "kekw", "dog", "happy cat"} {
		result, err := s.Discover(context.Background(), DiscoveryQuery{Query: q})
		if err != nil || len(result.Items) == 0 {
			t.Fatalf("%s: %+v %v", q, result, err)
		}
		t.Logf("%s: %d items, first %s, more=%t", q, len(result.Items), result.Items[0].ProviderID, result.NextCursor != "")
		if q == "kekw" {
			meta, err := s.websiteDetail(context.Background(), result.Items[0].ProviderID)
			if err != nil || meta.License == "" || meta.Creator == "" {
				t.Fatalf("detail: %+v %v", meta, err)
			}
			t.Logf("detail attribution=%q license=%q", meta.Creator, meta.License)
		}
		if result.NextCursor != "" {
			next, err := s.Discover(context.Background(), DiscoveryQuery{Query: q, Cursor: result.NextCursor})
			if err != nil {
				t.Fatalf("%s next page: %v", q, err)
			}
			t.Logf("%s next page: %d items", q, len(next.Items))
		}
	}
}
