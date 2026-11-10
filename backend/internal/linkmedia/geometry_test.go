package linkmedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/storage"
)

func TestCachedImageGeometryIsBoundedReadOnlyMetadata(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	noNetwork := roundTrip(func(*http.Request) (*http.Response, error) {
		t.Error("geometry lookup performed network or CDN I/O")
		return nil, ErrUnavailable
	})
	client, err := cdn.New(cdn.Options{Endpoint: "https://storage.example.test", Bucket: "fixture", Region: "us-east-1", PublicURL: "https://cdn.example.test", Transport: noNetwork})
	if err != nil {
		t.Fatal(err)
	}
	r := NewResolver(ResolverOptions{CDN: client})
	r.http = &http.Client{Transport: noNetwork}
	r.persistent = &publicCache{db: store.DB, namespace: client.Namespace()}
	base := Result{Provider: "media", Assets: []Asset{{URL: client.URL("link-media/" + strings.Repeat("a", 64)), ObjectKey: "link-media/" + strings.Repeat("a", 64), Kind: "image", ContentType: "image/png", Width: 600, Height: 400, ExpiresAt: time.Now().Add(time.Hour)}}}
	seed := func(raw, namespace, key string, expires time.Time, change func(*Result)) {
		t.Helper()
		value := base
		value.URL = raw
		value.Assets = append([]Asset(nil), base.Assets...)
		if change != nil {
			change(&value)
		}
		payload, _ := json.Marshal(value)
		if _, err := store.DB.Exec(`INSERT INTO public_media_results(namespace,cache_key,payload,expires_ns) VALUES(?,?,?,?)`, namespace, key, payload, expires.UnixNano()); err != nil {
			t.Fatal(err)
		}
	}
	var sources []string
	for _, tc := range []struct {
		name   string
		change func(*Result)
	}{
		{"valid", nil},
		{"expired-asset", func(v *Result) { v.Assets[0].ExpiresAt = time.Now().Add(-time.Second) }},
		{"wrong-source", func(v *Result) { v.URL = "https://other.example/image.png" }},
		{"multi", func(v *Result) { v.Assets = append(v.Assets, v.Assets[0]) }},
		{"zero", func(v *Result) { v.Assets[0].Width = 0 }},
		{"oversized", func(v *Result) { v.Assets[0].Width, v.Assets[0].Height = 8193, 10 }},
		{"pixels", func(v *Result) { v.Assets[0].Width, v.Assets[0].Height = 8000, 8000 }},
		{"foreign-cdn", func(v *Result) { v.Assets[0].URL = "https://foreign.example/image.png" }},
		{"video", func(v *Result) { v.Assets[0].Kind, v.Assets[0].ContentType = "video", "video/mp4" }},
		{"card", func(v *Result) { v.Preview = &Preview{Kind: "card"} }},
		{"large-manifest", func(v *Result) { v.Title = strings.Repeat("x", maxGeometryManifestBytes) }},
	} {
		raw := "https://source.example/" + tc.name + ".png"
		seed(raw, r.persistent.namespace, publicResultKey(raw), time.Now().Add(time.Hour), tc.change)
		sources = append(sources, raw)
	}
	for _, tc := range []struct {
		name, namespace, key string
		expires              time.Time
	}{
		{"namespace", "other-namespace", "", time.Now().Add(time.Hour)},
		{"version", r.persistent.namespace, "obsolete-key", time.Now().Add(time.Hour)},
		{"expired-result", r.persistent.namespace, "", time.Now().Add(-time.Second)},
	} {
		raw := "https://source.example/" + tc.name + ".png"
		key := tc.key
		if key == "" {
			key = publicResultKey(raw)
		}
		seed(raw, tc.namespace, key, tc.expires, nil)
		sources = append(sources, raw)
	}
	sources = append(sources, "https://source.example/missing.png", "https://source.example/valid.png?different=1", "http://127.0.0.1/private.png")
	var changesBefore, changesAfter int64
	if err := store.DB.QueryRow(`SELECT total_changes()`).Scan(&changesBefore); err != nil {
		t.Fatal(err)
	}
	got := r.CachedImageGeometry(context.Background(), sources)
	if err := store.DB.QueryRow(`SELECT total_changes()`).Scan(&changesAfter); err != nil {
		t.Fatal(err)
	}
	if changesBefore != changesAfter {
		t.Fatal("geometry lookup mutated cache state")
	}
	if len(got) != 1 || got[sources[0]] != (ImageGeometry{Width: 600, Height: 400}) {
		t.Fatalf("cache lookup granted invalid geometry: %+v", got)
	}
	for i := 0; i < MaxGeometryURLs+5; i++ {
		raw := fmt.Sprintf("https://source.example/bounded-%d.png", i)
		seed(raw, r.persistent.namespace, publicResultKey(raw), time.Now().Add(time.Hour), nil)
		sources = append(sources, raw)
	}
	got = r.CachedImageGeometry(context.Background(), sources)
	if len(got) > MaxGeometryURLs || got[sources[len(sources)-1]].Width != 0 {
		t.Fatal("lookup exceeded its per-batch source budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if len(r.CachedImageGeometry(ctx, sources)) != 0 {
		t.Fatal("canceled optional lookup returned hints")
	}
	if _, err := store.DB.Exec(`DROP TABLE public_media_results`); err != nil {
		t.Fatal(err)
	}
	if len(r.CachedImageGeometry(context.Background(), sources)) != 0 {
		t.Fatal("cache failure did not degrade to missing hints")
	}
}
