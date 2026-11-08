package preview

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
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

	// With both advertised (XML first), only the JSON document is fetched.
	service, seen = page(t, map[string]string{
		"https://public.example/both": oembedPage(xmlLink + jsonLink),
		"https://public.example/oembed?format=json&url=https%3A%2F%2Fpublic.example%2Fpost": richDocument,
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
	// A thumbnail on a private host is dropped like an Open Graph image.
	service, _ = page(t, map[string]string{
		"https://public.example/post": oembedPage(`<link rel="alternate" type="application/json+oembed" href="/doc">`),
		"https://public.example/doc":  `{"type":"link","title":"Doc","thumbnail_url":"https://private.example/thumb.png","thumbnail_width":10,"thumbnail_height":10}`,
	}, nil)
	if value, err := service.Fetch(context.Background(), "https://public.example/post"); err != nil || value.Image != "" || value.ImageWidth != 0 {
		t.Fatalf("private thumbnail kept: %+v %v", value, err)
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
