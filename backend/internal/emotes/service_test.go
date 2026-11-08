package emotes

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ircord/backend/internal/filehost"
	"ircord/backend/internal/storage"
)

func pngImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func animatedGIF(t *testing.T) []byte {
	t.Helper()
	frames := &gif.GIF{}
	for i := 0; i < 2; i++ {
		frame := image.NewPaletted(image.Rect(0, 0, 8, 8), palette.Plan9)
		frame.SetColorIndex(i, i, uint8(10+i))
		frames.Image = append(frames.Image, frame)
		frames.Delay = append(frames.Delay, 5)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, frames); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// webp builds an extended WebP header; flag 0x02 marks it animated.
func webp(w, h int, animated bool) []byte {
	chunk := make([]byte, 10)
	if animated {
		chunk[0] = 0x02
	}
	put24 := func(b []byte, v int) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }
	put24(chunk[4:], w-1)
	put24(chunk[7:], h-1)
	body := append([]byte("WEBPVP8X"), binary.LittleEndian.AppendUint32(nil, 10)...)
	body = append(body, chunk...)
	return append(append([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, uint32(len(body)))...), body...)
}

type fixture struct {
	server   *httptest.Server
	requests atomic.Int64
	mu       sync.Mutex
	routes   map[string]func() (int, []byte)
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{routes: map[string]func() (int, []byte){}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		f.mu.Lock()
		route := f.routes[r.URL.Path]
		f.mu.Unlock()
		if route == nil {
			http.NotFound(w, r)
			return
		}
		status, body := route()
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(f.server.Close)
	return f
}
func (f *fixture) set(path string, status int, body []byte) {
	f.mu.Lock()
	f.routes[path] = func() (int, []byte) { return status, body }
	f.mu.Unlock()
}
func (f *fixture) endpoints() Endpoints {
	return Endpoints{SevenTVAPI: f.server.URL + "/7tv/v3", SevenTVCDN: f.server.URL + "/7tvcdn/emote/", FFZAPI: f.server.URL + "/ffz/v1", FFZCDN: f.server.URL + "/ffzcdn/emote/"}
}

// sevenTV serves a 7TV emote, using the provider's real response shape.
func (f *fixture) sevenTV(t *testing.T, id, name, owner string, animated bool) {
	host := f.server.URL + "/7tvcdn/emote/" + id
	f.set("/7tv/v3/emotes/"+id, 200, []byte(fmt.Sprintf(`{"id":%q,"name":%q,"animated":%t,"owner":{"display_name":%q},"host":{"url":%q,"files":[{"name":"1x.webp","static_name":"1x_static.webp","format":"WEBP"},{"name":"2x.webp","static_name":"2x_static.webp","format":"WEBP"},{"name":"2x.avif","static_name":"2x_static.avif","format":"AVIF"}]}}`, id, name, animated, owner, host)))
	f.set("/7tvcdn/emote/"+id+"/2x.webp", 200, webp(64, 64, animated))
	f.set("/7tvcdn/emote/"+id+"/2x_static.webp", 200, webp(64, 64, false))
}
func (f *fixture) ffz(t *testing.T, id, name, owner string) {
	f.set("/ffz/v1/emote/"+id, 200, []byte(fmt.Sprintf(`{"emote":{"id":%s,"name":%q,"owner":{"_id":1,"name":%q,"display_name":%q},"urls":{"1":"%s/ffzcdn/emote/%s/1","2":"%s/ffzcdn/emote/%s/2"},"animated":null}}`, id, name, owner, owner, f.server.URL, id, f.server.URL, id)))
	f.set("/ffzcdn/emote/"+id+"/2", 200, pngImage(t, 27, 32))
}

func openService(t *testing.T, store *storage.Store, dir string, f *fixture) *Service {
	t.Helper()
	options := Options{Store: store, Blob: filehost.Local{Root: filepath.Join(dir, "emotes")}, PublicURL: "https://chat.example", MaxBytes: 64 << 10}
	if f != nil {
		options.Endpoints = f.endpoints()
	}
	s, err := Open(options)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func newService(t *testing.T, f *fixture) (*Service, *storage.Store, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return openService(t, store, dir, f), store, dir
}
func find(c EmoteCatalog, shortcode string) *Emote {
	for i := range c.Items {
		if c.Items[i].Shortcode == shortcode {
			return &c.Items[i]
		}
	}
	return nil
}
func read(t *testing.T, s *Service, url string) (string, []byte) {
	t.Helper()
	key := strings.TrimSuffix(url[strings.LastIndex(url, "/")+1:], filepath.Ext(url))
	mime, body, err := s.Open(context.Background(), key)
	if err != nil {
		t.Fatalf("open %s: %v", url, err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	return mime, data
}

func TestSeedsImportPinnedProviderEmotesOnce(t *testing.T) {
	f := newFixture(t)
	f.sevenTV(t, "01GA29CZ2R000C36HNE7Z0DQXD", "KEKW", "moecoquito", false)
	f.sevenTV(t, "01HGXBVKPR00025ZBQF6KKY8YD", "OOOO", "ThePurpleAxel", true)
	f.ffz(t, "481082", "NotLikeThis", "battykatty")
	s, store, dir := newService(t, f)
	seeds := []Seed{DefaultSeeds[0], DefaultSeeds[3], DefaultSeeds[4]}
	if pending := s.SeedOnce(context.Background(), seeds); pending != 0 {
		t.Fatalf("pending seeds: %d", pending)
	}
	c := s.Catalog(false)
	kekw, oooo, nlt := find(c, "kekw"), find(c, "oooo"), find(c, "notlikethis")
	if kekw == nil || oooo == nil || nlt == nil {
		t.Fatalf("seeded catalog: %+v", c.Items)
	}
	if kekw.Name != "KEKW" || !slices.Contains(kekw.Aliases, "KEKW") || kekw.Source != "7tv" || kekw.ProviderID != "01GA29CZ2R000C36HNE7Z0DQXD" || kekw.Creator != "moecoquito" || kekw.SourceURL != "https://7tv.app/emotes/01GA29CZ2R000C36HNE7Z0DQXD" {
		t.Fatalf("kekw lost its canonical name or attribution: %+v", kekw)
	}
	if kekw.Animated || kekw.StaticURL != kekw.URL || !strings.HasPrefix(kekw.URL, "https://chat.example/emotes/") {
		t.Fatalf("still emote variants: %+v", kekw)
	}
	if !oooo.Animated || oooo.StaticURL == oooo.URL {
		t.Fatalf("animated emote needs a separate still frame: %+v", oooo)
	}
	if mime, data := read(t, s, oooo.StaticURL); mime != "image/webp" || data[20]&0x02 != 0 {
		t.Fatal("the stored still frame is animated")
	}
	if nlt.Name != "NotLikeThis" || nlt.Source != "ffz" || nlt.Creator != "battykatty" || !slices.Contains(nlt.Aliases, "NotLikeThis") {
		t.Fatalf("ffz attribution: %+v", nlt)
	}
	if mime, _ := read(t, s, nlt.URL); mime != "image/png" {
		t.Fatalf("ffz image type %s", mime)
	}
	// Seeds apply once: a removed seed stays removed, and nothing is fetched again.
	if err := s.Delete(context.Background(), kekw.ID); err != nil {
		t.Fatal(err)
	}
	before := f.requests.Load()
	if s.SeedOnce(context.Background(), seeds) != 0 || f.requests.Load() != before {
		t.Fatal("applied seeds were fetched again")
	}
	if find(s.Catalog(false), "kekw") != nil {
		t.Fatal("a deleted seed came back")
	}
	// The catalog, version and images survive a restart.
	version := s.Version()
	again := openService(t, store, dir, f)
	if again.Version() != version || len(again.Catalog(false).Items) != 2 {
		t.Fatalf("restart lost catalog state: %d/%d", again.Version(), len(again.Catalog(false).Items))
	}
	read(t, again, find(again.Catalog(false), "oooo").URL)
}

func TestProviderFailuresLeaveTheCatalogUsable(t *testing.T) {
	f := newFixture(t)
	s, _, _ := newService(t, f)
	if _, err := s.Upload(context.Background(), New{Shortcode: "local"}, pngImage(t, 32, 32), nil); err != nil {
		t.Fatal(err)
	}
	f.set("/7tv/v3/emotes/01GA29CZ2R000C36HNE7Z0DQXD", 503, []byte("down"))
	if _, err := s.Import(context.Background(), "7tv", "01GA29CZ2R000C36HNE7Z0DQXD", New{}); !errors.Is(err, ErrProvider) {
		t.Fatalf("outage: %v", err)
	}
	if s.SeedOnce(context.Background(), DefaultSeeds[:1]) != 1 {
		t.Fatal("a failed seed must be retried")
	}
	cases := map[string]string{
		"malformed JSON":  `{"id":`,
		"wrong ID":        `{"id":"01FBFNMS380002BEVDT2R9QCDR","name":"KEKL","host":{"url":"//cdn.7tv.app/emote/01FBFNMS380002BEVDT2R9QCDR","files":[{"name":"2x.webp"}]}}`,
		"foreign host":    `{"id":"01FKF8QHVR000B1FXDJGM9EDWQ","name":"KEKLEO","host":{"url":"//evil.example/emote/01FKF8QHVR000B1FXDJGM9EDWQ","files":[{"name":"2x.webp"}]}}`,
		"unsafe name":     `{"id":"01HGXBVKPR00025ZBQF6KKY8YD","name":"<b>","host":{"url":"//cdn.7tv.app/emote/01HGXBVKPR00025ZBQF6KKY8YD","files":[{"name":"2x.webp"}]}}`,
		"no still frame":  `{"id":"01FCY771D800007PQ2DF3GDTN6","name":"Rain","animated":true,"host":{"url":"//cdn.7tv.app/emote/01FCY771D800007PQ2DF3GDTN6","files":[{"name":"2x.webp"}]}}`,
		"no usable image": `{"id":"01FE3XY508000AA32JP519W2EW","name":"PETPET","host":{"url":"//cdn.7tv.app/emote/01FE3XY508000AA32JP519W2EW","files":[{"name":"2x.avif"}]}}`,
	}
	ids := map[string]string{"malformed JSON": "01GA29CZ2R000C36HNE7Z0DQX0", "wrong ID": "01GA29CZ2R000C36HNE7Z0DQX1", "foreign host": "01FKF8QHVR000B1FXDJGM9EDWQ", "unsafe name": "01HGXBVKPR00025ZBQF6KKY8YD", "no still frame": "01FCY771D800007PQ2DF3GDTN6", "no usable image": "01FE3XY508000AA32JP519W2EW"}
	for name, body := range cases {
		if name != "foreign host" {
			body = strings.ReplaceAll(body, "//cdn.7tv.app/emote/", f.server.URL+"/7tvcdn/emote/")
		}
		f.set("/7tv/v3/emotes/"+ids[name], 200, []byte(body))
		if _, err := s.Import(context.Background(), "7tv", ids[name], New{}); !errors.Is(err, ErrProvider) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A provider image that is not an image, or too large, is rejected.
	f.sevenTV(t, "01GA29CZ2R000C36HNE7Z0DQX2", "Fake", "x", false)
	f.set("/7tvcdn/emote/01GA29CZ2R000C36HNE7Z0DQX2/2x.webp", 200, []byte("<html>not an image</html>"))
	if _, err := s.Import(context.Background(), "7tv", "01GA29CZ2R000C36HNE7Z0DQX2", New{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("non-image provider asset: %v", err)
	}
	f.sevenTV(t, "01GA29CZ2R000C36HNE7Z0DQX3", "Huge", "x", false)
	f.set("/7tvcdn/emote/01GA29CZ2R000C36HNE7Z0DQX3/2x.webp", 200, make([]byte, 65<<10))
	if _, err := s.Import(context.Background(), "7tv", "01GA29CZ2R000C36HNE7Z0DQX3", New{}); !errors.Is(err, ErrProvider) {
		t.Fatalf("oversized provider asset: %v", err)
	}
	for _, bad := range [][2]string{{"7tv", "../../etc"}, {"ffz", "12a"}, {"bttv", "1"}} {
		if _, err := s.Import(context.Background(), bad[0], bad[1], New{}); !errors.Is(err, ErrInvalid) {
			t.Errorf("%v: %v", bad, err)
		}
	}
	if c := s.Catalog(true); len(c.Items) != 1 || c.Items[0].Shortcode != "local" {
		t.Fatalf("failures changed the catalog: %+v", c.Items)
	}
}

func TestLookupsAreSharedAndCached(t *testing.T) {
	f := newFixture(t)
	f.sevenTV(t, "01GA29CZ2R000C36HNE7Z0DQXD", "KEKW", "moecoquito", false)
	s, _, _ := newService(t, f)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Lookup(context.Background(), "7tv", "01GA29CZ2R000C36HNE7Z0DQXD"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if _, err := s.Lookup(context.Background(), "7tv", "01GA29CZ2R000C36HNE7Z0DQXD"); err != nil || f.requests.Load() != 1 {
		t.Fatalf("lookups made %d requests (%v)", f.requests.Load(), err)
	}
	if _, err := s.Import(context.Background(), "7tv", "01GA29CZ2R000C36HNE7Z0DQXD", New{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import(context.Background(), "7tv", "01GA29CZ2R000C36HNE7Z0DQXD", New{Shortcode: "other"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("a provider emote imported twice: %v", err)
	}
}

func TestNamesAreUniqueAndRenamesKeepHistory(t *testing.T) {
	s, _, _ := newService(t, nil)
	ctx := context.Background()
	kekw, err := s.Upload(ctx, New{Shortcode: ":KEKW:"}, pngImage(t, 32, 32), nil)
	if err != nil {
		t.Fatal(err)
	}
	if kekw.Shortcode != "kekw" || kekw.Name != "KEKW" || !slices.Equal(kekw.Aliases, []string{"KEKW"}) {
		t.Fatalf("names: %+v", kekw)
	}
	for _, in := range []New{{Shortcode: "kekw"}, {Shortcode: "other", Aliases: []string{"Kekw"}}, {Name: "kEkW"}} {
		if _, err := s.Upload(ctx, in, pngImage(t, 32, 32), nil); !errors.Is(err, ErrConflict) {
			t.Fatalf("%+v must conflict: %v", in, err)
		}
	}
	for _, in := range []New{{Shortcode: "a"}, {Shortcode: "has space"}, {Shortcode: "ok", Aliases: []string{"<x>"}}} {
		if _, err := s.Upload(ctx, in, pngImage(t, 32, 32), nil); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%+v must be invalid: %v", in, err)
		}
	}
	renamed := "kekwide"
	updated, err := s.Update(kekw.ID, EmotePatch{Shortcode: &renamed, Name: &renamed})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Shortcode != "kekwide" || !slices.Contains(updated.Retired, "kekw") || !slices.Contains(updated.Retired, "KEKW") {
		t.Fatalf("rename must retire the old names: %+v", updated)
	}
	// A retired name is free for a new emote, and then (in any case) no longer refers to the old one.
	fresh, err := s.Upload(ctx, New{Shortcode: "kekw"}, pngImage(t, 32, 32), nil)
	if err != nil {
		t.Fatal(err)
	}
	old := find(s.Catalog(false), "kekwide")
	if len(old.Retired) != 0 || fresh.Shortcode != "kekw" {
		t.Fatalf("claiming a retired name: %+v", old)
	}
	// Disabled entries are hidden from other accounts but keep their names.
	off := false
	if _, err = s.Update(fresh.ID, EmotePatch{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	if find(s.Catalog(false), "kekw") != nil || find(s.Catalog(true), "kekw") == nil {
		t.Fatal("disabled emotes are visible only to managers")
	}
	if _, err := s.Upload(ctx, New{Shortcode: "kekw"}, pngImage(t, 32, 32), nil); !errors.Is(err, ErrConflict) {
		t.Fatal("a disabled emote keeps its names")
	}
	if _, err := s.Update("missing", EmotePatch{Enabled: &off}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestUploadsAreBoundedAndKeepStillFrames(t *testing.T) {
	s, _, _ := newService(t, nil)
	ctx := context.Background()
	for name, data := range map[string][]byte{"text": []byte("hello"), "too large": make([]byte, 65<<10), "too wide": pngImage(t, 1025, 1), "empty": nil} {
		if _, err := s.Upload(ctx, New{Shortcode: "x_" + strings.ReplaceAll(name, " ", "")}, data, nil); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	g, err := s.Upload(ctx, New{Shortcode: "dance"}, animatedGIF(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if mime, _ := read(t, s, g.StaticURL); !g.Animated || mime != "image/png" || g.StaticURL == g.URL {
		t.Fatalf("animated GIF still frame: %+v %s", g, mime)
	}
	if _, err := s.Upload(ctx, New{Shortcode: "spin"}, webp(32, 32, true), nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("animated WebP needs a still frame: %v", err)
	}
	if _, err := s.Upload(ctx, New{Shortcode: "spin"}, webp(32, 32, true), webp(32, 32, true)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an animated still frame: %v", err)
	}
	spin, err := s.Upload(ctx, New{Shortcode: "spin"}, webp(32, 32, true), pngImage(t, 32, 32))
	if err != nil || !spin.Animated || spin.StaticURL == spin.URL {
		t.Fatalf("animated WebP with still: %+v %v", spin, err)
	}
	// Deleting removes the stored images too.
	key := strings.TrimSuffix(spin.URL[strings.LastIndex(spin.URL, "/")+1:], ".webp")
	if err := s.Delete(ctx, spin.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted emote image is still served")
	}
}

func TestChangesNotifySubscribersWithTheNewVersion(t *testing.T) {
	s, _, _ := newService(t, nil)
	updates, stop := s.Subscribe()
	defer stop()
	e, err := s.Upload(context.Background(), New{Shortcode: "wave"}, pngImage(t, 16, 16), nil)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case v := <-updates:
		if v != s.Version() || v != 1 {
			t.Fatalf("version %d", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no catalog notification")
	}
	if err := s.Delete(context.Background(), e.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case v := <-updates:
		if v != 2 {
			t.Fatalf("version %d", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no catalog notification")
	}
}
