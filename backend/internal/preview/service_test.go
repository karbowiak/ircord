package preview

import (
	"context"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"ircord/backend/internal/model"
)

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func resolver() Resolver {
	return resolverFunc(func(ctx context.Context, network, host string) ([]netip.Addr, error) {
		switch host {
		case "private.example", "127.0.0.1", "metadata.google.internal":
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		case "rebind.example":
			return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("10.0.0.1")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	})
}
func TestMetadataParsingCacheAndSSRF(t *testing.T) {
	calls := 0
	service := New(Options{Resolver: resolver(), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credentials leaked")
		}
		header := http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}
		body := `<head><title>Fallback</title><meta property="og:title" content="A &amp; B"><meta name="description" content="A useful page"><meta property="og:image" content="/cover.png"></head><script>alert('never executed')</script>`
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}})
	result, err := service.Fetch(context.Background(), "https://public.example/article")
	if err != nil || result.Title != "A & B" || result.Image != "https://public.example/cover.png" || result.Description != "A useful page" {
		t.Fatal(result, err)
	}
	if _, err = service.Fetch(context.Background(), "https://public.example/article#fragment"); err != nil || calls != 1 {
		t.Fatal("metadata cache did not coalesce URLs", err, calls)
	}
	for _, raw := range []string{"http://public.example", "file:///etc/passwd", "https://user:secret@public.example/", "https://private.example/", "https://127.0.0.1/", "https://metadata.google.internal/", "https://rebind.example/"} {
		before := calls
		if _, err = service.Fetch(context.Background(), raw); err == nil || calls != before {
			t.Fatalf("unsafe URL reached HTTP: %s", raw)
		}
	}
}
func TestRedirectAndMetadataBounds(t *testing.T) {
	calls := 0
	service := New(Options{Resolver: resolver(), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://private.example/secret"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}})
	if _, err := service.Fetch(context.Background(), "https://public.example/redirect"); err == nil || calls != 1 {
		t.Fatal("private redirect followed")
	}
	base, _ := url.Parse("https://public.example/")
	if _, _, err := parse(strings.NewReader(`<head><meta content="`+strings.Repeat("x", maxPage+1024)), base); err == nil {
		t.Fatal("unbounded HTML token accepted")
	}
	long, _, err := parse(strings.NewReader(`<head><meta property="og:title" content="`+strings.Repeat("t", 70<<10)+`"><meta name="description" content="`+strings.Repeat("d", 70<<10)+`"></head>`), base)
	if err != nil || len(long.Title) != 200 || len(long.Description) != 500 {
		t.Fatal("long metadata was not bounded", len(long.Title), len(long.Description), err)
	}
	for _, ip := range []string{"0.0.0.0", "10.0.0.1", "100.64.0.1", "127.0.0.1", "169.254.169.254", "198.18.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "64:ff9b::7f00:1", "2002:7f00:1::"} {
		if public(netip.MustParseAddr(ip)) {
			t.Fatal("reserved address accepted", ip)
		}
	}
}

// page serves fixed HTML per URL and records every request, so fixtures never touch the network.
func page(t *testing.T, pages map[string]string, redirects map[string]string) (*Service, *[]string) {
	var seen []string
	service := New(Options{Resolver: resolver(), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, r.URL.String())
		if to, ok := redirects[r.URL.String()]; ok {
			return &http.Response{StatusCode: 301, Header: http.Header{"Location": []string{to}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		body, ok := pages[r.URL.String()]
		if !ok {
			return &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}})
	return service, &seen
}

// Trimmed from a real tenor.com share page: the GIF and an MP4 rendition are both named.
const tenorPage = `<!DOCTYPE html><html><head><title>Mmgg GIF - Mmgg - Discover &amp; Share GIFs</title>
<meta name="twitter:card" content="player">
<meta name="twitter:image" content="https://media1.tenor.com/m/QgagS8CozjoAAAAC/mmgg.gif">
<meta name="twitter:player:stream" content="https://media.tenor.com/QgagS8CozjoAAAPo/mmgg.mp4">
<meta property="og:site_name" content="Tenor">
<meta property="og:title" content="Mmgg GIF - Mmgg - Discover &amp; Share GIFs">
<meta property="og:description" content="Click to view the GIF">
<meta property="og:type" content="video.other">
<meta property="og:image" content="https://media1.tenor.com/m/QgagS8CozjoAAAAC/mmgg.gif">
<meta property="og:image:type" content="image/gif">
<meta property="og:image:width" content="498">
<meta property="og:image:height" content="493">
<meta property="og:video" content="https://media.tenor.com/QgagS8CozjoAAAPo/mmgg.mp4">
</head><body><script>never()</script></body></html>`

// What giphy.com serves with HTTP 200 for every non-canonical share URL (bare id, stale slug):
// no metadata, only an immediate refresh to the canonical page.
const giphyStub = `<html><head><meta charSet="utf-8"/><meta id="__next-page-redirect" http-equiv="refresh" content="0;url=https://giphy.com/gifs/cat-working-JIX9t2j0ZTN9S"/></head><body></body></html>`

// Trimmed from a real giphy.com share page: a WebP still comes before the GIF.
const giphyPage = `<html><head><title>Cat Working GIF - Find &amp; Share on GIPHY</title>
<meta property="og:title" content="Cat Working GIF - Find &amp; Share on GIPHY">
<meta property="og:image" content="https://media4.giphy.com/media/JIX9t2j0ZTN9S/giphy.webp"><meta property="og:image:type" content="image/webp">
<meta property="og:image" content="https://media4.giphy.com/media/JIX9t2j0ZTN9S/giphy.gif"><meta property="og:image:type" content="image/gif"><meta property="og:image:width" content="480"><meta property="og:image:height" content="480">
</head></html>`

func TestShareLinksResolveToInlineGIFs(t *testing.T) {
	service, seen := page(t, map[string]string{
		"https://tenor.com/view/mmgg-gif-20186311":         tenorPage,
		"https://giphy.com/gifs/JIX9t2j0ZTN9S":             giphyStub,
		"https://giphy.com/gifs/cat-working-JIX9t2j0ZTN9S": giphyPage,
	}, map[string]string{"https://tenor.com/bwRxP.gif": "https://tenor.com/view/mmgg-gif-20186311"})
	result, err := service.Fetch(context.Background(), "https://tenor.com/bwRxP.gif")
	if err != nil || result.Media == nil {
		t.Fatal("tenor short link did not resolve", result, err)
	}
	if *result.Media != (model.PreviewMedia{URL: "https://media1.tenor.com/m/QgagS8CozjoAAAAC/mmgg.gif", Kind: "gif", Width: 498, Height: 493}) || result.URL != "https://tenor.com/view/mmgg-gif-20186311" || result.SiteName != "Tenor" {
		t.Fatal("unexpected tenor media", result, *result.Media)
	}
	if len(*seen) != 2 {
		t.Fatal("expected one redirect and one page fetch", *seen)
	}
	*seen = nil
	result, err = service.Fetch(context.Background(), "https://giphy.com/gifs/JIX9t2j0ZTN9S")
	if err != nil || result.Media == nil || result.Media.URL != "https://media4.giphy.com/media/JIX9t2j0ZTN9S/giphy.gif" || result.Media.Width != 480 {
		t.Fatal("giphy GIF not chosen over its still", result, err)
	}
	if want := []string{"https://giphy.com/gifs/JIX9t2j0ZTN9S", "https://giphy.com/gifs/cat-working-JIX9t2j0ZTN9S"}; strings.Join(*seen, " ") != strings.Join(want, " ") {
		t.Fatal("giphy stub refresh was not followed to the canonical page", *seen)
	}
}
func TestMetaRefreshIsBoundedLikeRedirects(t *testing.T) {
	stub := func(to string) string {
		return `<head><meta http-equiv="Refresh" content="0; URL='` + to + `'"></head>`
	}
	service, seen := page(t, map[string]string{
		"https://giphy.com/private":   stub("https://private.example/secret"),
		"https://giphy.com/elsewhere": stub("https://other.example/page"),
		"https://giphy.com/plaintext": stub("http://giphy.com/gifs/x"),
		"https://giphy.com/delayed":   `<head><meta http-equiv="refresh" content="5;url=https://giphy.com/gifs/x"></head>`,
		"https://giphy.com/loop":      stub("https://giphy.com/loop"),
	}, nil)
	for _, raw := range []string{"https://giphy.com/private", "https://giphy.com/elsewhere", "https://giphy.com/plaintext", "https://giphy.com/delayed"} {
		*seen = nil
		if _, err := service.Fetch(context.Background(), raw); err == nil || len(*seen) != 1 {
			t.Fatal("unsafe or non-immediate refresh was followed", raw, *seen, err)
		}
	}
	*seen = nil
	if _, err := service.Fetch(context.Background(), "https://giphy.com/loop"); err == nil || len(*seen) != 4 {
		t.Fatal("refresh loop escaped the redirect budget", *seen, err)
	}
}
func TestMediaOnlyPageGetsATitle(t *testing.T) {
	service, _ := page(t, map[string]string{
		"https://tenor.com/view/bare": `<head><meta property="og:image" content="https://media.tenor.com/a.gif"></head>`,
	}, nil)
	result, err := service.Fetch(context.Background(), "https://tenor.com/view/bare")
	if err != nil || result.Media == nil || result.Title != "tenor.com GIF" {
		t.Fatal("media-only page must stay renderable", result, err)
	}
}
func TestImagePropertiesAttachToTheirImage(t *testing.T) {
	base, _ := url.Parse("https://giphy.com/gifs/x")
	aliased, _, err := parse(strings.NewReader(`<head><title>T</title><meta property="og:image" content="https://media.giphy.com/still.png"><meta property="og:image:type" content="image/png"><meta property="og:image:url" content="https://media.giphy.com/anim.gif"><meta property="og:image:type" content="image/gif"><meta property="og:image:width" content="320"></head>`), base)
	if err != nil || aliased.Media == nil || aliased.Media.URL != "https://media.giphy.com/anim.gif" || aliased.Media.Width != 320 {
		t.Fatal("og:image:url entry lost its properties", aliased, err)
	}
	page := `<head><title>T</title>` + strings.Repeat(`<meta property="og:image" content="https://media.giphy.com/s.png"><meta property="og:image:type" content="image/png">`, 16) +
		`<meta property="og:image" content="https://media.giphy.com/late.gif"><meta property="og:image:type" content="image/gif"></head>`
	capped, _, err := parse(strings.NewReader(page), base)
	if err != nil || capped.Media != nil {
		t.Fatal("properties of an image past the cap relabelled an earlier one", capped.Media, err)
	}
}
func TestInlineMediaIsLimitedToSupportedProviders(t *testing.T) {
	gif := `<head><title>T</title><meta property="og:image" content="%s"><meta property="og:image:type" content="image/gif"></head>`
	service, _ := page(t, map[string]string{
		"https://news.example/story":     strings.ReplaceAll(gif, "%s", "https://news.example/a.gif"),
		"https://tenor.com/view/offsite": strings.ReplaceAll(gif, "%s", "https://cdn.example/a.gif"),
		"https://tenor.com/view/plain":   strings.ReplaceAll(gif, "%s", "http://media.tenor.com/a.gif"),
		"https://tenor.com/view/still":   `<head><title>Still</title><meta property="og:image" content="https://media.tenor.com/a.png"></head>`,
		"https://tenor.com/view/none":    `<head><title>Search results</title></head>`,
	}, nil)
	for _, raw := range []string{"https://news.example/story", "https://tenor.com/view/offsite", "https://tenor.com/view/plain", "https://tenor.com/view/still", "https://tenor.com/view/none"} {
		result, err := service.Fetch(context.Background(), raw)
		if err != nil || result.Media != nil || result.Title == "" {
			t.Fatal("expected an ordinary card without inline media", raw, result, err)
		}
	}
}
func TestPrivateMediaHostIsDropped(t *testing.T) {
	service := New(Options{Resolver: resolverFunc(func(ctx context.Context, network, host string) ([]netip.Addr, error) {
		if host == "media.tenor.com" {
			return []netip.Addr{netip.MustParseAddr("10.0.0.5")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	}), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		body := `<head><title>T</title><meta property="og:image" content="https://media.tenor.com/a.gif"></head>`
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}})
	result, err := service.Fetch(context.Background(), "https://tenor.com/view/x")
	if err != nil || result.Media != nil || result.Image != "" || result.Title != "T" {
		t.Fatal("private media destination survived", result, err)
	}
}
func TestOrdinarySitesWithLargeInlineScripts(t *testing.T) {
	// News front pages (dr.dk, bbc.com) and video sites inline far more than 64 KiB of script
	// before or around their metadata; the card must still come through.
	script := "<script>" + strings.Repeat("var x=1;", 40<<10) + "</script>"
	service, _ := page(t, map[string]string{
		"https://www.dr.dk/": `<!DOCTYPE html><html lang="da"><head>` + script + `<title>DR: Nyheder - Breaking - TV - Radio</title>
<meta name="description" content="Dit Nyhedsoverblik"><meta property="og:site_name" content="DR">
<meta property="og:image" content="https://asset.dr.dk/drdk/shareimage.png"></head><body>` + script + `</body></html>`,
		"https://late.example/": `<html><head>` + strings.Repeat(script, 2) + `<meta property="og:title" content="Late"></head>`,
	}, nil)
	result, err := service.Fetch(context.Background(), "https://www.dr.dk/")
	if err != nil || result.Title != "DR: Nyheder - Breaking - TV - Radio" || result.SiteName != "DR" || result.Image != "https://asset.dr.dk/drdk/shareimage.png" || result.Media != nil {
		t.Fatal("ordinary site card", result, err)
	}
	if result, err = service.Fetch(context.Background(), "https://late.example/"); err != nil || result.Title != "Late" {
		t.Fatal("metadata after large scripts", result, err)
	}
}
func TestUnavailableProviderFallsBack(t *testing.T) {
	service, seen := page(t, map[string]string{}, map[string]string{"https://tenor.com/abc.gif": "https://private.example/view/x"})
	if _, err := service.Fetch(context.Background(), "https://tenor.com/view/missing"); err == nil {
		t.Fatal("missing share page produced a preview")
	}
	*seen = nil
	_, err := service.Fetch(context.Background(), "https://tenor.com/abc.gif")
	if err == nil || !strings.Contains(err.Error(), "private or reserved") || len(*seen) != 1 {
		t.Fatal("redirect to a private address was followed", *seen, err)
	}
}

// githubIssuePage is shaped like a GitHub issue page: a long head of styles and scripts before
// the Open Graph tags, the share image on opengraph.githubassets.com, and a large body.
func githubIssuePage() string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html lang="en" data-color-mode="auto"><head><meta charset="utf-8">`)
	for i := 0; i < 400; i++ {
		b.WriteString(`<link crossorigin="anonymous" media="all" rel="stylesheet" href="https://github.githubassets.com/assets/primer-` + strings.Repeat("a", 40) + `.css" />`)
	}
	b.WriteString(`<script type="application/json" id="client-env">` + strings.Repeat(`{"x":"y"},`, 20000) + `</script>`)
	b.WriteString(`<title>Let channel operators add trusted media hosts for their channel · Issue #140 · karbowiak/ircord · GitHub</title>
<meta name="description" content="Under the default Trusted media setting, only a fixed list of well-known media hosts is embedded (trustedMediaHosts in the client). A channel where a bot posts...">
<meta property="og:image" content="https://opengraph.githubassets.com/0123abcd/karbowiak/ircord/issues/140" /><meta property="og:image:alt" content="Under the default Trusted media setting..." /><meta property="og:image:width" content="1200" /><meta property="og:image:height" content="600" /><meta property="og:site_name" content="GitHub" /><meta property="og:type" content="object" /><meta property="og:title" content="Let channel operators add trusted media hosts for their channel · Issue #140 · karbowiak/ircord" /><meta property="og:url" content="https://github.com/karbowiak/ircord/issues/140" /><meta property="og:description" content="Under the default Trusted media setting, only a fixed list of well-known media hosts is embedded (trustedMediaHosts in the client). A channel where a bot posts..." />
<meta name="twitter:image" content="https://opengraph.githubassets.com/0123abcd/karbowiak/ircord/issues/140" /><meta name="twitter:card" content="summary_large_image" />
</head><body>`)
	b.WriteString(strings.Repeat(`<div class="comment">text</div>`, 30000))
	b.WriteString(`</body></html>`)
	return b.String()
}

func TestGitHubIssuePageGetsOrdinaryCard(t *testing.T) {
	page := githubIssuePage()
	if len(page) <= maxPage {
		t.Fatal("fixture should exceed the page bound", len(page))
	}
	service := New(Options{Resolver: resolver(), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "github.com" || r.Header.Get("User-Agent") != "IRCord-LinkPreview/1" {
			t.Errorf("unexpected request %s %q", r.URL, r.Header.Get("User-Agent"))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(page)), Request: r}, nil
	})}})
	value, err := service.Fetch(context.Background(), "https://github.com/karbowiak/ircord/issues/140")
	if err != nil {
		t.Fatal(err)
	}
	if value.SiteName != "GitHub" || value.Title != "Let channel operators add trusted media hosts for their channel · Issue #140 · karbowiak/ircord" || value.Kind != "" || value.Images != nil {
		t.Fatalf("%+v", value)
	}
	if !strings.HasPrefix(value.Description, "Under the default Trusted media setting") || value.Image != "https://opengraph.githubassets.com/0123abcd/karbowiak/ircord/issues/140" || value.ImageWidth != 1200 || value.ImageHeight != 600 {
		t.Fatalf("%+v", value)
	}
}

func TestGitHubRenamedRepositoryAndCancelledRequest(t *testing.T) {
	page := githubIssuePage()
	calls := 0
	service := New(Options{Resolver: resolver(), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if err := r.Context().Err(); err != nil {
			return nil, err
		}
		switch r.URL.Path {
		case "/old-owner/old-name/issues/140":
			// A renamed or transferred repository answers with a permanent redirect.
			return &http.Response{StatusCode: 301, Header: http.Header{"Location": []string{"https://github.com/karbowiak/ircord/issues/140"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		case "/karbowiak/ircord/issues/140":
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(page)), Request: r}, nil
		}
		t.Errorf("unexpected request %s", r.URL)
		return nil, io.EOF
	})}})
	// One caller giving up fails only its own request; nothing is cached or paused for others.
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Fetch(cancelled, "https://github.com/old-owner/old-name/issues/140"); err == nil {
		t.Fatal("cancelled request produced a card")
	}
	value, err := service.Fetch(context.Background(), "https://github.com/old-owner/old-name/issues/140")
	if err != nil || value.SiteName != "GitHub" || !strings.HasSuffix(value.Title, "Issue #140 · karbowiak/ircord") || value.URL != "https://github.com/karbowiak/ircord/issues/140" {
		t.Fatal(value, err)
	}
}
