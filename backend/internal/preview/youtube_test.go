package preview

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestYouTubeURLVariants(t *testing.T) {
	cases := map[string]YouTubeVideo{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":             {ID: "dQw4w9WgXcQ"},
		"https://youtube.com/watch?v=dQw4w9WgXcQ&t=1m30s":         {ID: "dQw4w9WgXcQ", Start: 90},
		"https://m.youtube.com/watch?feature=share&v=dQw4w9WgXcQ": {ID: "dQw4w9WgXcQ"},
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ":           {ID: "dQw4w9WgXcQ"},
		"https://youtu.be/dQw4w9WgXcQ?si=tracking&t=42":           {ID: "dQw4w9WgXcQ", Start: 42},
		"https://www.youtube.com/shorts/abcdefghijk":              {ID: "abcdefghijk", Short: true},
		"https://www.youtube.com/embed/dQw4w9WgXcQ?start=10":      {ID: "dQw4w9WgXcQ", Start: 10},
		"https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ":      {ID: "dQw4w9WgXcQ"},
		"https://www.youtube.com/live/dQw4w9WgXcQ":                {ID: "dQw4w9WgXcQ"},
		"https://youtu.be/dQw4w9WgXcQ?t=1h":                       {ID: "dQw4w9WgXcQ", Start: 3600},
		"https://youtu.be/dQw4w9WgXcQ?t=99999999":                 {ID: "dQw4w9WgXcQ"},
	}
	for raw, want := range cases {
		u, _ := url.Parse(raw)
		got, ok := ParseYouTube(u)
		if !ok || got != want {
			t.Errorf("%s: got %+v %v, want %+v", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"http://youtu.be/dQw4w9WgXcQ", "https://youtube.com/watch?v=short", "https://youtube.com/channel/UCabcdefghij", "https://youtube.com.evil.test/watch?v=dQw4w9WgXcQ", "https://youtu.be/dQw4w9WgXcQ%22%3E", "https://www.youtube.com/watch?v=dQw4w9WgXcQ/../x", "https://user@youtu.be/dQw4w9WgXcQ"} {
		u, _ := url.Parse(raw)
		if _, ok := ParseYouTube(u); ok {
			t.Errorf("accepted %s", raw)
		}
	}
}

func oembed(t *testing.T, status int, body string, calls *int) *Service {
	return New(Options{Resolver: resolver(), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		*calls++
		if r.URL.Host != "www.youtube.com" || r.URL.Path != "/oembed" || r.URL.Query().Get("url") != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
			t.Errorf("unexpected request %s", r.URL)
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}})
}

func TestYouTubeOEmbedStates(t *testing.T) {
	calls := 0
	service := oembed(t, 200, `{"title":"Never <b>Gonna</b>","author_name":"Rick","thumbnail_url":"https://evil.test/x.jpg","html":"<iframe src=\"https://evil.test\"></iframe>"}`, &calls)
	value, err := service.Fetch(context.Background(), "https://youtu.be/dQw4w9WgXcQ?t=5")
	if err != nil || value.Kind != "video" || value.Title != "Never <b>Gonna</b>" || value.Author != "Rick" || value.SiteName != "YouTube" {
		t.Fatal(value, err)
	}
	// The thumbnail is built from the validated ID; page/provider markup never passes through.
	if value.Image != "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg" || value.Embed == nil || value.Embed.ID != "dQw4w9WgXcQ" || value.Embed.Start != 5 || !value.Embed.Embeddable {
		t.Fatal(value.Image, value.Embed)
	}
	// Another variant of the same video is served from the cache with its own offset and URL.
	other, err := service.Fetch(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil || calls != 1 || other.Embed.Start != 0 || other.URL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" || value.Embed.Start != 5 {
		t.Fatal("variants not coalesced", calls, other, err)
	}

	calls = 0
	blocked, err := oembed(t, 401, `Unauthorized`, &calls).Fetch(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil || blocked.Embed == nil || blocked.Embed.Embeddable || blocked.Title != "YouTube video" {
		t.Fatal("embedding-disabled video", blocked, err)
	}
	if _, err = oembed(t, 404, `Not Found`, &calls).Fetch(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ"); err == nil {
		t.Fatal("unavailable video previewed")
	}
	if _, err = oembed(t, 200, `{"title":`, &calls).Fetch(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ"); err == nil {
		t.Fatal("malformed oEmbed accepted")
	}
	if _, err = oembed(t, 200, `{"title":"`+strings.Repeat("x", maxOEmbed)+`"}`, &calls).Fetch(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ"); err == nil {
		t.Fatal("oversized oEmbed accepted")
	}
}

func TestArticleAndGalleryPages(t *testing.T) {
	base, _ := url.Parse("https://news.example/story")
	article, _, err := parse(strings.NewReader(`<head><meta property="og:type" content="article"><meta property="og:title" content="Headline"><meta property="og:site_name" content="Daily"><meta property="og:image" content="https://news.example/lead.jpg"><meta property="og:image:width" content="1200"><meta property="og:image:height" content="630"><meta property="og:image" content="https://news.example/lead-small.jpg"></head>`), base)
	if err != nil || article.Kind != "article" || article.ImageWidth != 1200 || article.ImageHeight != 630 || article.Images != nil {
		t.Fatal("article", article, err)
	}
	bare, _, err := parse(strings.NewReader(`<head><meta property="article:published_time" content="2026-01-01"><title>Only a headline</title></head>`), base)
	if err != nil || bare.Kind != "article" || bare.Image != "" || bare.Description != "" {
		t.Fatal("article without image or description", bare, err)
	}
	var tags strings.Builder
	for i := 0; i < 14; i++ {
		tags.WriteString(`<meta property="og:image" content="/photo` + string(rune('a'+i)) + `.jpg"><meta property="og:image:width" content="400"><meta property="og:image:height" content="300">`)
	}
	tags.WriteString(`<meta property="og:image" content="/photoa.jpg"><meta property="og:image" content="http://plain.example/x.jpg">`)
	album, _, err := parse(strings.NewReader(`<head><meta property="og:title" content="Album">`+tags.String()+`</head>`), base)
	if err != nil || album.Kind != "" || len(album.Images) != maxGallery || album.Images[0].URL != "https://news.example/photoa.jpg" || album.Images[1].Width != 400 {
		t.Fatal("gallery", album.Images, err)
	}
	single, _, _ := parse(strings.NewReader(`<head><meta property="og:title" content="One"><meta property="og:image" content="/a.jpg"><meta property="og:image" content="/a.jpg"></head>`), base)
	if single.Images != nil {
		t.Fatal("one image became a gallery", single.Images)
	}
}

func TestGalleryImagesAreValidated(t *testing.T) {
	service := New(Options{Resolver: resolver(), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body := `<head><meta property="og:title" content="Album"><meta property="og:image" content="https://public.example/a.jpg"><meta property="og:image" content="https://private.example/b.jpg"><meta property="og:image" content="https://public.example/c.jpg"><meta property="og:image" content="https://127.0.0.1/d.jpg"></head>`
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}})
	value, err := service.Fetch(context.Background(), "https://public.example/album")
	if err != nil || len(value.Images) != 2 || value.Images[1].URL != "https://public.example/c.jpg" {
		t.Fatal(value.Images, err)
	}
}
