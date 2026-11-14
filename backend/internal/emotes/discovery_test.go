package emotes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func discoveryFixture(t *testing.T) (*Service, *fixture) {
	t.Helper()
	f := newFixture(t)
	s, _, _ := newService(t, f)
	s.options.Endpoints.EmojiGGWeb = f.server.URL
	s.options.Endpoints.EmojiGGCDN = f.server.URL + "/cdn/"
	return s, f
}

func TestOwnerPersistenceAndProtection(t *testing.T) {
	s, store, dir := newService(t, nil)
	ctx := context.Background()
	entry, err := s.Upload(ctx, New{Shortcode: "owned", AddedBy: "private-account-id"}, pngImage(t, 8, 8), nil)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := s.Upload(ctx, New{Shortcode: "seeded", AddedBy: "system"}, pngImage(t, 8, 8), nil)
	if err != nil {
		t.Fatal(err)
	}
	restarted := openService(t, store, dir, nil)
	if _, err = restarted.UpdateFor(entry.ID, EmotePatch{}, "different", false); !errors.Is(err, ErrForbidden) {
		t.Fatal("foreign account could edit")
	}
	if err = restarted.DeleteFor(ctx, seed.ID, "private-account-id", false); !errors.Is(err, ErrForbidden) {
		t.Fatal("account could remove seed")
	}
	disabled := false
	if _, err = restarted.UpdateFor(entry.ID, EmotePatch{Enabled: &disabled}, "private-account-id", false); err != nil {
		t.Fatal(err)
	}
	owner := restarted.CatalogFor("private-account-id", false)
	stranger := restarted.CatalogFor("different", false)
	if len(owner.Items) != 2 || len(stranger.Items) != 1 {
		t.Fatalf("visibility owner=%+v stranger=%+v", owner.Items, stranger.Items)
	}
	raw, _ := json.Marshal(owner)
	if strings.Contains(string(raw), "private-account-id") || strings.Contains(string(raw), "addedBy") {
		t.Fatal("owner leaked")
	}
	if err = restarted.DeleteFor(ctx, entry.ID, "private-account-id", false); err != nil {
		t.Fatal(err)
	}
	if err = restarted.DeleteFor(ctx, seed.ID, "different", true); err != nil {
		t.Fatal(err)
	}
}

func seedPreviewEntry(s *Service, id, rawURL string) {
	s.website.mu.Lock()
	defer s.website.mu.Unlock()
	if s.website.entries == nil {
		s.website.entries = map[string]websiteCachedEntry{}
	}
	s.website.entries[id] = websiteCachedEntry{entry: discoveryEntry{meta: ProviderEmote{ImageURL: rawURL}}, expires: time.Now().Add(time.Hour)}
}

func TestPreviewQueueCoalescingCancellationAndStatic(t *testing.T) {
	s, f := discoveryFixture(t)
	gate := make(chan struct{})
	var active, maximum, downloads atomic.Int32
	data := pngImage(t, 8, 8)
	for i := 10; i < 75; i++ {
		id := fmt.Sprintf("%d_fixture", i)
		path := "/cdn/" + id + ".png"
		seedPreviewEntry(s, id, f.server.URL+path)
		f.handle(path, func(w http.ResponseWriter, r *http.Request) {
			downloads.Add(1)
			n := active.Add(1)
			defer active.Add(-1)
			for old := maximum.Load(); n > old && !maximum.CompareAndSwap(old, n); old = maximum.Load() {
			}
			select {
			case <-gate:
				_, _ = w.Write(data)
			case <-r.Context().Done():
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for i := 10; i < 40; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if _, err := s.Preview(ctx, "emoji.gg", id); err != nil {
				t.Error(err)
			}
		}(fmt.Sprintf("%d_fixture", i))
	}
	waitFor := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !check() {
			if time.Now().After(deadline) {
				t.Fatal("preview wait timed out")
			}
			time.Sleep(time.Millisecond)
		}
	}
	waitFor(func() bool {
		s.discovery.mu.Lock()
		defer s.discovery.mu.Unlock()
		return len(s.discovery.pending) == 30
	})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := s.Preview(ctx, "emoji.gg", "10_fixture"); err != nil {
			t.Error(err)
		}
	}()
	waitFor(func() bool {
		s.discovery.mu.Lock()
		defer s.discovery.mu.Unlock()
		return s.discovery.pending["10_fixture"].waiters == 2
	})
	close(gate)
	wg.Wait()
	if downloads.Load() != 30 || maximum.Load() > 2 {
		t.Fatalf("downloads %d max %d", downloads.Load(), maximum.Load())
	}
	seedPreviewEntry(s, "2_dance", f.server.URL+"/cdn/2_dance.gif")
	f.set("/cdn/2_dance.gif", 200, animatedGIF(t))
	still, err := s.PreviewStatic(context.Background(), "emoji.gg", "2_dance")
	if err != nil || still.Animated || still.ContentType != "image/png" {
		t.Fatalf("static %+v %v", still, err)
	}
	gate2 := make(chan struct{})
	f.handle("/cdn/74_fixture.png", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-gate2:
			_, _ = w.Write(data)
		case <-r.Context().Done():
		}
	})
	first, stop := context.WithCancel(context.Background())
	a, b := make(chan error, 1), make(chan error, 1)
	go func() { _, err := s.Preview(first, "emoji.gg", "74_fixture"); a <- err }()
	waitFor(func() bool {
		s.discovery.mu.Lock()
		defer s.discovery.mu.Unlock()
		return s.discovery.pending["74_fixture"] != nil
	})
	go func() { _, err := s.Preview(context.Background(), "emoji.gg", "74_fixture"); b <- err }()
	waitFor(func() bool {
		s.discovery.mu.Lock()
		defer s.discovery.mu.Unlock()
		return s.discovery.pending["74_fixture"] != nil && s.discovery.pending["74_fixture"].waiters == 2
	})
	stop()
	if err := <-a; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel %v", err)
	}
	close(gate2)
	if err := <-b; err != nil {
		t.Fatal(err)
	}
}

func TestPreviewAdmissionAndContentBounds(t *testing.T) {
	s, f := discoveryFixture(t)
	gate := make(chan struct{})
	defer close(gate)
	for i := 10; i < 75; i++ {
		id := fmt.Sprintf("%d_blocked", i)
		path := "/cdn/" + id + ".png"
		seedPreviewEntry(s, id, f.server.URL+path)
		f.handle(path, func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-gate:
			case <-r.Context().Done():
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := 10; i < 74; i++ {
		wg.Add(1)
		go func(id string) { defer wg.Done(); _, _ = s.Preview(ctx, "emoji.gg", id) }(fmt.Sprintf("%d_blocked", i))
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.discovery.mu.Lock()
		n := len(s.discovery.pending)
		s.discovery.mu.Unlock()
		if n == 64 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("queue did not fill")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := s.Preview(context.Background(), "emoji.gg", "74_blocked"); !errors.Is(err, ErrProvider) {
		t.Fatalf("overload %v", err)
	}
	cancel()
	wg.Wait()
	s.discovery.mu.Lock()
	remaining := len(s.discovery.pending)
	s.discovery.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("abandoned queue %d", remaining)
	}
	for i, body := range [][]byte{[]byte("not an image"), make([]byte, (64<<10)+1)} {
		id := fmt.Sprintf("%d_invalid", i+1)
		path := "/cdn/" + id + ".png"
		seedPreviewEntry(s, id, f.server.URL+path)
		f.set(path, 200, body)
		if _, err := s.Preview(context.Background(), "emoji.gg", id); err == nil {
			t.Fatal("invalid image accepted")
		}
	}
}
