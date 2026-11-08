package preview

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// oembedPage is an ordinary page with Open Graph tags that also advertises oEmbed documents.
func oembedPage(links string) string {
	return `<!DOCTYPE html><html><head><title>Page title</title>
<meta property="og:title" content="Open Graph title">
<meta property="og:description" content="Open Graph description">
<meta property="og:site_name" content="Example Site">
<meta property="og:image" content="https://public.example/og.png">
` + links + `</head><body><script>never()</script></body></html>`
}

const jsonLink = `<link rel="alternate" type="application/json+oembed" href="https://public.example/oembed?format=json&amp;url=https%3A%2F%2Fpublic.example%2Fpost" title="Post">`
const xmlLink = `<link rel="alternate" type="text/xml+oembed" href="/oembed?format=xml">`

// A real-shaped rich document: its html snippet must never reach the card.
const richDocument = `{"version":"1.0","type":"rich","title":"oEmbed title","author_name":"Ada","author_url":"https://public.example/ada","provider_name":"Example Provider","thumbnail_url":"https://public.example/thumb.jpg","thumbnail_width":480,"thumbnail_height":"360","html":"<iframe src=\"https://evil.example/\"></iframe><script>steal()</script>"}`

func TestOEmbedJSONFillsTheCard(t *testing.T) {
	service, seen := page(t, map[string]string{
		"https://public.example/post": oembedPage(jsonLink),
		"https://public.example/oembed?format=json&url=https%3A%2F%2Fpublic.example%2Fpost": richDocument,
	}, nil)
	value, err := service.Fetch(context.Background(), "https://public.example/post")
	if err != nil {
		t.Fatal(err)
	}
	if value.Title != "oEmbed title" || value.Author != "Ada" || value.SiteName != "Example Provider" || value.Description != "Open Graph description" {
		t.Fatalf("card not filled from oEmbed: %+v", value)
	}
	if value.Image != "https://public.example/thumb.jpg" || value.ImageWidth != 480 || value.ImageHeight != 360 {
		t.Fatalf("thumbnail not used: %+v", value)
	}
	encoded, _ := json.Marshal(value)
	if strings.Contains(string(encoded), "iframe") || strings.Contains(string(encoded), "steal") || strings.Contains(string(encoded), "evil.example") {
		t.Fatalf("oEmbed html reached the card: %s", encoded)
	}
	if len(*seen) != 2 {
		t.Fatalf("expected page and oEmbed requests, got %v", *seen)
	}
	if _, err := service.Fetch(context.Background(), "https://public.example/post"); err != nil || len(*seen) != 2 {
		t.Fatalf("oEmbed card was not cached with the page: %v %v", err, *seen)
	}
}

func TestOEmbedXMLIsAcceptedAndJSONPreferred(t *testing.T) {
	document := `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<oembed><version>1.0</version><type>video</type><title>XML &amp; title</title><author_name>Grace</author_name><provider_name>XML Provider</provider_name><thumbnail_url>/thumb.png</thumbnail_url><thumbnail_width>640</thumbnail_width><thumbnail_height>480</thumbnail_height><html>&lt;iframe&gt;&lt;/iframe&gt;</html></oembed>`
	service, seen := page(t, map[string]string{
		"https://public.example/xml":               oembedPage(xmlLink),
		"https://public.example/oembed?format=xml": document,
	}, nil)
	value, err := service.Fetch(context.Background(), "https://public.example/xml")
	if err != nil || value.Title != "XML & title" || value.Author != "Grace" || value.SiteName != "XML Provider" || value.Kind != "video" {
		t.Fatalf("XML oEmbed not used: %+v %v", value, err)
	}
	if value.Image != "https://public.example/thumb.png" || value.ImageWidth != 640 || value.ImageHeight != 480 {
		t.Fatalf("relative XML thumbnail not resolved: %+v", value)
	}
	if (*seen)[1] != "https://public.example/oembed?format=xml" {
		t.Fatalf("unexpected requests %v", *seen)
	}

	// With both advertised, in either order, only the JSON document is fetched.
	for _, links := range []string{xmlLink + jsonLink, jsonLink + xmlLink} {
		service, seen = page(t, map[string]string{
			"https://public.example/both": oembedPage(links),
			"https://public.example/oembed?format=json&url=https%3A%2F%2Fpublic.example%2Fpost": richDocument,
			"https://public.example/oembed?format=xml":                                          document,
		}, nil)
		if value, err = service.Fetch(context.Background(), "https://public.example/both"); err != nil || value.Title != "oEmbed title" {
			t.Fatalf("JSON not preferred: %+v %v", value, err)
		}
		for _, request := range *seen {
			if strings.Contains(request, "format=xml") {
				t.Fatalf("XML fetched although JSON was advertised: %v", *seen)
			}
		}
	}
}

func TestPhotoOEmbedUsesItsPicture(t *testing.T) {
	service, _ := page(t, map[string]string{
		"https://public.example/photo":      oembedPage(`<link rel="alternate" type="application/json+oembed" href="/photo.json">`),
		"https://public.example/photo.json": `{"type":"photo","version":"1.0","title":"Sunset","url":"https://public.example/sunset.jpg","width":"1200","height":800,"thumbnail_url":"https://public.example/small.jpg"}`,
	}, nil)
	value, err := service.Fetch(context.Background(), "https://public.example/photo")
	if err != nil || value.Image != "https://public.example/sunset.jpg" || value.ImageWidth != 1200 || value.ImageHeight != 800 || value.Title != "Sunset" {
		t.Fatalf("photo document not used: %+v %v", value, err)
	}
}

func TestUnusableOEmbedFallsBackToOpenGraph(t *testing.T) {
	for name, document := range map[string]string{
		"malformed JSON": `{"title": "broken`,
		"not JSON":       `<html>not oembed</html>`,
		"empty":          `{}`,
	} {
		service, _ := page(t, map[string]string{
			"https://public.example/post": oembedPage(jsonLink),
			"https://public.example/oembed?format=json&url=https%3A%2F%2Fpublic.example%2Fpost": document,
		}, nil)
		value, err := service.Fetch(context.Background(), "https://public.example/post")
		if err != nil || value.Title != "Open Graph title" || value.SiteName != "Example Site" || value.Image != "https://public.example/og.png" || value.Author != "" {
			t.Fatalf("%s: Open Graph card not kept: %+v %v", name, value, err)
		}
	}
	// A missing document (HTTP 404 from the fixture) and malformed XML fall back the same way.
	service, _ := page(t, map[string]string{"https://public.example/post": oembedPage(jsonLink)}, nil)
	if value, err := service.Fetch(context.Background(), "https://public.example/post"); err != nil || value.Title != "Open Graph title" {
		t.Fatalf("missing oEmbed: %+v %v", value, err)
	}
	service, _ = page(t, map[string]string{
		"https://public.example/xml":               oembedPage(xmlLink),
		"https://public.example/oembed?format=xml": `<oembed><title>unterminated`,
	}, nil)
	if value, err := service.Fetch(context.Background(), "https://public.example/xml"); err != nil || value.Title != "Open Graph title" {
		t.Fatalf("malformed XML: %+v %v", value, err)
	}
}

func TestOEmbedAddressesFollowThePageRules(t *testing.T) {
	for name, link := range map[string]string{
		"private host":  `<link rel="alternate" type="application/json+oembed" href="https://private.example/oembed">`,
		"loopback":      `<link rel="alternate" type="application/json+oembed" href="https://127.0.0.1/oembed">`,
		"metadata host": `<link rel="alternate" type="application/json+oembed" href="https://metadata.google.internal/computeMetadata/v1/">`,
		"plain HTTP":    `<link rel="alternate" type="application/json+oembed" href="http://public.example/oembed">`,
		"credentials":   `<link rel="alternate" type="application/json+oembed" href="https://user:secret@public.example/oembed">`,
	} {
		service, seen := page(t, map[string]string{"https://public.example/post": oembedPage(link)}, nil)
		value, err := service.Fetch(context.Background(), "https://public.example/post")
		if err != nil || value.Title != "Open Graph title" {
			t.Fatalf("%s: Open Graph card not kept: %+v %v", name, value, err)
		}
		if len(*seen) != 1 {
			t.Fatalf("%s: refused oEmbed address was requested: %v", name, *seen)
		}
	}
	// A public oEmbed address that redirects to a private host is refused at the redirect.
	service, seen := page(t, map[string]string{"https://public.example/post": oembedPage(`<link rel="alternate" type="application/json+oembed" href="/oembed">`)},
		map[string]string{"https://public.example/oembed": "https://private.example/oembed"})
	if value, err := service.Fetch(context.Background(), "https://public.example/post"); err != nil || value.Title != "Open Graph title" {
		t.Fatalf("redirect: %+v %v", value, err)
	}
	for _, request := range *seen {
		if strings.Contains(request, "private.example") {
			t.Fatalf("private oEmbed redirect followed: %v", *seen)
		}
	}
	// A thumbnail that is refused (a private host, plain HTTP) leaves the Open Graph image.
	for _, thumbnail := range []string{"https://private.example/thumb.png", "http://public.example/thumb.png"} {
		service, _ = page(t, map[string]string{
			"https://public.example/post": oembedPage(`<link rel="alternate" type="application/json+oembed" href="/doc">`),
			"https://public.example/doc":  `{"type":"link","title":"Doc","thumbnail_url":"` + thumbnail + `","thumbnail_width":10,"thumbnail_height":10}`,
		}, nil)
		value, err := service.Fetch(context.Background(), "https://public.example/post")
		if err != nil || value.Title != "Doc" || value.Image != "https://public.example/og.png" || value.ImageWidth != 0 {
			t.Fatalf("%s: Open Graph image not kept: %+v %v", thumbnail, value, err)
		}
	}
}

func TestOversizedOEmbedFallsBackToOpenGraph(t *testing.T) {
	service, _ := page(t, map[string]string{
		"https://public.example/post": oembedPage(`<link rel="alternate" type="application/json+oembed" href="/doc">`),
		"https://public.example/doc":  `{"title":"` + strings.Repeat("x", maxOEmbed) + `"}`,
	}, nil)
	if value, err := service.Fetch(context.Background(), "https://public.example/post"); err != nil || value.Title != "Open Graph title" {
		t.Fatalf("oversized oEmbed document read: %q %v", value.Title, err)
	}
}

// slowOEmbed serves the page but holds every oEmbed request until its context ends; its
// resolver honours the context like a real DNS lookup.
func slowOEmbed(calls *int) *Service {
	return New(Options{
		Resolver: resolverFunc(func(ctx context.Context, network, host string) ([]netip.Addr, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
		}),
		Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			*calls++
			if r.URL.Path == "/doc" {
				<-r.Context().Done()
				return nil, r.Context().Err()
			}
			body := oembedPage(`<link rel="alternate" type="application/json+oembed" href="/doc">`)
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		})},
	})
}

func TestSlowOEmbedKeepsThePageBudget(t *testing.T) {
	previous := oembedBudget
	oembedBudget = 50 * time.Millisecond
	defer func() { oembedBudget = previous }()
	calls := 0
	service := slowOEmbed(&calls)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	value, err := service.Fetch(ctx, "https://public.example/post")
	if err != nil || value.Title != "Open Graph title" || value.Image != "https://public.example/og.png" {
		t.Fatalf("slow oEmbed cost the card its image: %+v %v", value, err)
	}

	// When the caller's own deadline runs out during the lookup, the degraded card is returned
	// but not cached: the next fetch reads the page again.
	oembedBudget = time.Minute
	calls = 0
	service = slowOEmbed(&calls)
	short, cancelShort := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelShort()
	if _, err = service.Fetch(short, "https://public.example/post"); err != nil {
		t.Fatal(err)
	}
	oembedBudget = 50 * time.Millisecond
	value, err = service.Fetch(context.Background(), "https://public.example/post")
	if err != nil || calls != 4 || value.Image != "https://public.example/og.png" {
		t.Fatalf("card built after the deadline was cached: %+v %v calls=%d", value, err, calls)
	}
}

func TestRefreshStubThatAdvertisesOEmbedIsFollowed(t *testing.T) {
	service, seen := page(t, map[string]string{
		"https://public.example/stub": `<html><head><link rel="alternate" type="application/json+oembed" href="/missing"><meta http-equiv="refresh" content="0;url=/real"></head></html>`,
		"https://public.example/real": oembedPage(""),
	}, nil)
	value, err := service.Fetch(context.Background(), "https://public.example/stub")
	if err != nil || value.Title != "Open Graph title" {
		t.Fatalf("refresh not followed: %+v %v %v", value, err, *seen)
	}
	// Metadata in the body is still read when the head names only an oEmbed document.
	service, _ = page(t, map[string]string{
		"https://public.example/body": `<html><head><link rel="alternate" type="application/json+oembed" href="/missing"></head><body><meta property="og:title" content="Body title"></body></html>`,
	}, nil)
	if value, err = service.Fetch(context.Background(), "https://public.example/body"); err != nil || value.Title != "Body title" {
		t.Fatalf("body metadata skipped: %+v %v", value, err)
	}
}

func TestOEmbedOnlyPageAndUnrelatedLinks(t *testing.T) {
	// A page that names nothing but an oEmbed document is previewed from it.
	service, _ := page(t, map[string]string{
		"https://public.example/bare": `<html><head>` + `<link rel="alternate" type="application/json+oembed" href="/doc"></head></html>`,
		"https://public.example/doc":  `{"type":"link","version":"1.0","title":"Only oEmbed"}`,
	}, nil)
	if value, err := service.Fetch(context.Background(), "https://public.example/bare"); err != nil || value.Title != "Only oEmbed" {
		t.Fatalf("oEmbed-only page: %+v %v", value, err)
	}
	// When that document is unusable the page still has no preview.
	service, _ = page(t, map[string]string{"https://public.example/bare": `<html><head><link rel="alternate" type="application/json+oembed" href="/doc"></head></html>`}, nil)
	if _, err := service.Fetch(context.Background(), "https://public.example/bare"); err == nil {
		t.Fatal("metadata-less page previewed")
	}
	// Feeds, stylesheets and links without rel=alternate are not oEmbed.
	service, seen := page(t, map[string]string{"https://public.example/post": oembedPage(`<link rel="alternate" type="application/rss+xml" href="/feed"><link rel="stylesheet" type="application/json+oembed" href="/doc"><link type="application/json+oembed" href="/doc">`)}, nil)
	if value, err := service.Fetch(context.Background(), "https://public.example/post"); err != nil || value.Title != "Open Graph title" || len(*seen) != 1 {
		t.Fatalf("unrelated links followed: %+v %v %v", value, err, *seen)
	}
}

func TestGIFSharePagesKeepTheirCard(t *testing.T) {
	withOEmbed := strings.Replace(tenorPage, "</head>", `<link rel="alternate" type="application/json+oembed" href="https://tenor.com/oembed?url=x"></head>`, 1)
	service, seen := page(t, map[string]string{"https://tenor.com/view/mmgg-gif-123": withOEmbed}, nil)
	value, err := service.Fetch(context.Background(), "https://tenor.com/view/mmgg-gif-123")
	if err != nil || value.Media == nil || len(*seen) != 1 {
		t.Fatalf("GIF share page changed: %+v %v %v", value, err, *seen)
	}
}
