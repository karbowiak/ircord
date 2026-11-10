package linkmedia

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

const b4CNE = "https://player.cnevids.com/embed/5670377e94c05f43e9000000/51097beb8ef9aff9f5000008"
const b4CNEFrame = "https://player-backend.cnevids.com/iframe/video/5670377e94c05f43e9000000?autoplay=0"
const b4Issuu = "https://issuu.com/visitfaroeislands/docs/tg25-uk-web_compressed"
const b4IssuuFrame = "https://e.issuu.com/embed.html?d=tg25-uk-web_compressed&u=visitfaroeislands"
const b4Carto = "https://rpa.carto.com/viz/9a23d690-6e00-4c87-8d06-608d8bf7ca18/embed_map"
const b4Widgetic = "https://widgetic.com/embed/555b5e6509c7e29e718b4569?bp=top-right"

func ioBody(value string) io.ReadCloser { return io.NopCloser(strings.NewReader(value)) }

func TestOEmbedAppsControlledPlayers(t *testing.T) {
	for _, tc := range []struct {
		source, candidate, expected string
		ratio                       float64
	}{
		{b4CNE, strings.TrimSuffix(b4CNEFrame, "?autoplay=0") + "?autoplay=1", b4CNEFrame, 16.0 / 9},
		{strings.Replace(b4CNE, "player.", "player-backend.", 1), b4CNEFrame, b4CNEFrame, 16.0 / 9},
		{"https://player.cnevids.com/iframe/video/5670377e94c05f43e9000000", b4CNEFrame, b4CNEFrame, 16.0 / 9},
		{b4Issuu, b4IssuuFrame, b4IssuuFrame, 4.0 / 3},
		{b4Carto, b4Carto, b4Carto, 4.0 / 3},
		{strings.Replace(b4Carto, "/embed_map", "/public_map", 1), b4Carto, b4Carto, 4.0 / 3},
		{b4Widgetic, b4Widgetic, b4Widgetic, 1},
	} {
		frame, _, ratio, ok := ControlledPlayer(tc.source, tc.candidate, "https://chat.example.test")
		if !ok || frame != tc.expected || ratio != tc.ratio {
			t.Errorf("wrong app frame: %q %v %t", frame, ratio, ok)
		}
		if _, _, _, ok := ControlledPlayer(tc.source, tc.candidate, tc.candidate); ok {
			t.Error("application origin gained player authority")
		}
	}
}

func TestOEmbedAppsIdentityAndCapabilityBoundaries(t *testing.T) {
	for _, tc := range []struct{ source, frame string }{
		{b4CNE, strings.Replace(b4CNEFrame, "5670377e94c05f43e9000000", "54c5640161646d294c080000", 1)},
		{b4CNE, b4CNE}, // No evidence for a two-ID frame, even with the same tuple.
		{b4CNE, strings.Replace(b4CNEFrame, "player-backend", "player-other", 1)},
		{b4CNE, b4CNEFrame + "&autoplay=1"},
		{b4Issuu, b4IssuuFrame + "&u=other"},
		{b4Issuu, strings.Replace(b4IssuuFrame, "visitfaroeislands", "other", 1)},
		{b4Issuu, strings.Replace(b4IssuuFrame, "tg25-uk-web_compressed", "other", 1)},
		{b4Issuu, "https://e.issuu.com/embed.html#123/456"},
		{b4Issuu, b4IssuuFrame + "&pageNumber=99"},
		{b4Issuu + "/extra", b4IssuuFrame},
		{b4Issuu + "?=private", b4IssuuFrame},
		{b4Issuu + "?", b4IssuuFrame},
		{b4Carto, strings.Replace(b4Carto, "rpa.carto", "other.carto", 1)},
		{b4Carto, strings.Replace(b4Carto, "/viz/", "/u/other/viz/", 1)},
		{strings.Replace(b4Carto, "/viz/", "/u/rpa/viz/", 1), b4Carto},
		{b4Carto, strings.Replace(b4Carto, "9a23d690", "9a23d691", 1)},
		{strings.Replace(b4Carto, ".carto.com", ".cartodb.com", 1), b4Carto},
		{b4Carto + "?use-external-access-token", b4Carto},
		{b4Carto + "?=private", b4Carto},
		{b4Widgetic, strings.Replace(b4Widgetic, "555b5e6509c7e29e718b4569", "555b5e6509c7e29e718b4568", 1)},
		{b4Widgetic + "&bp=top-right", b4Widgetic},
		{strings.Replace(b4Widgetic, "top-right", "none", 1), b4Widgetic},
		{"https://widgetic.com/widgets/photo/mosaic-gallery/", b4Widgetic},
		{"https://app.widgetic.com/editor", b4Widgetic},
		{"https://www.kickstarter.com/projects/creator/project", "https://www.kickstarter.com/projects/creator/project/widget/video.html"},
	} {
		if _, _, _, ok := ControlledPlayer(tc.source, tc.frame, ""); ok {
			t.Errorf("unreviewed identity accepted: %s -> %s", tc.source, tc.frame)
		}
	}
	for _, tc := range [][2]string{{b4CNE, b4CNEFrame}, {b4Issuu, b4IssuuFrame}, {b4Carto, b4Carto}, {b4Widgetic, b4Widgetic}} {
		for _, mutate := range []func(string) string{
			func(s string) string { return strings.Replace(s, "https:", "http:", 1) },
			func(s string) string { return strings.Replace(s, "https://", "https://user:pass@", 1) },
			func(s string) string { return s + "&access_token=private" },
			func(s string) string { return strings.Replace(s, ".com", ".com:444", 1) },
			func(s string) string { return strings.Replace(s, ".com", ".com.evil.example", 1) },
		} {
			if _, _, _, ok := ControlledPlayer(mutate(tc[0]), tc[1], ""); ok {
				t.Error("unsafe source accepted")
			}
			if _, _, _, ok := ControlledPlayer(tc[0], mutate(tc[1]), ""); ok {
				t.Error("unsafe frame accepted")
			}
		}
	}
}

func TestOEmbedAppsMarkupRebuild(t *testing.T) {
	valid := `<div style="height:9999px"><iframe src="` + html.EscapeString(b4IssuuFrame) + `" allow="allow-top-navigation" onload="evil()" width="1"></iframe></div>`
	want := `<iframe src="` + html.EscapeString(b4IssuuFrame) + `"></iframe>`
	if got := providerMarkup(b4Issuu, valid); got != want {
		t.Fatalf("untrusted dimensions/attributes retained: %q", got)
	}
	for _, value := range []string{
		valid + valid,
		valid + `<script>evil()</script>`,
		strings.Replace(valid, `<iframe`, `<iframe srcdoc=""`, 1),
		strings.Replace(valid, `<iframe`, `<iframe src="https://evil.example"`, 1),
		`<script src="https://issuu.com/player.js"></script>`,
		strings.Repeat("x", maxWidgetBytes+1),
	} {
		if got := providerMarkup(b4Issuu, value); got != "" {
			t.Error("mixed, duplicate or script-only output gained frame authority")
		}
	}
}

func b4PublicPage(raw string) string {
	return `<html><head><title>Public source</title><meta property="og:title" content="Public source"><link rel="canonical" href="` + html.EscapeString(raw) + `"></head><body></body></html>`
}

func b4Resolver(t *testing.T, change func(*http.Request, *http.Response)) (*Resolver, *int) {
	t.Helper()
	r := NewResolver(ResolverOptions{})
	calls := 0
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" || req.URL.Scheme != "https" {
			t.Error("app request inherited credentials or insecure transport")
		}
		body, mime := b4PublicPage(req.URL.String()), "text/html"
		if oembedAppEndpoint(req.URL) {
			source := req.URL.Query().Get("url")
			frame := b4CNEFrame
			if req.URL.Hostname() == "issuu.com" {
				u, _ := url.Parse(source)
				p := strings.Split(u.Path, "/")
				frame = "https://e.issuu.com/embed.html?" + url.Values{"u": {p[1]}, "d": {p[3]}}.Encode()
			} else {
				u, _ := url.Parse(source)
				frame = "https://player-backend.cnevids.com/iframe/video/" + cneVideoID(u)
			}
			data, _ := json.Marshal(map[string]any{"type": "rich", "title": "Public item", "provider_name": "Public provider", "width": 1, "height": 9999, "html": `<iframe src="` + html.EscapeString(frame) + `"></iframe>`})
			body, mime = string(data), "application/json"
		}
		if req.URL.Hostname() == "widgetic.com" {
			body = `<html><head><title>Widgetic</title></head><body><script>data: '/api/v2/compositions/` + strings.TrimPrefix(req.URL.Path, "/embed/") + `/embed/data'</script></body></html>`
		}
		if strings.HasSuffix(req.URL.Path, "/public_map") {
			body = b4PublicPage(req.URL.String()) + `<a href="` + strings.TrimSuffix(req.URL.String(), "/public_map") + `/embed_map">Embed</a>`
		}
		res := response(200, mime, body)
		res.Request = req
		if change != nil {
			change(req, res)
		}
		return res, nil
	})}
	return r, &calls
}

func TestOEmbedAppsPinnedFixtures(t *testing.T) {
	data, err := os.ReadFile("../../../frontend/shared/embed-fixtures/iframely.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct{ Provider, URL string }
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, fixture := range catalog.Fixtures {
		if fixture.Provider != "cnevids.com" && fixture.Provider != "issuu.com" && fixture.Provider != "cartodb.com" {
			continue
		}
		count++
		t.Run(fixture.URL, func(t *testing.T) {
			r, calls := b4Resolver(t, nil)
			value, err := r.ResolveSource(context.Background(), fixture.URL)
			wantWidget := strings.HasPrefix(fixture.URL, "https:")
			if err != nil || value.Preview == nil || (value.Preview.Kind == "widget") != wantWidget || len(value.Assets) > 0 {
				t.Fatalf("incorrect fixture result: %+v %v", value, err)
			}
			if *calls > 3 || !wantWidget && *calls != 0 {
				t.Fatalf("unexpected provider calls: %d", *calls)
			}
		})
	}
	if count != 11 {
		t.Fatalf("fixture inventory changed: %d", count)
	}
	r, calls := b4Resolver(t, nil)
	value, err := r.ResolveSource(context.Background(), b4Widgetic)
	if err != nil || value.Preview == nil || value.Preview.Kind != "widget" || *calls != 1 {
		t.Fatalf("documented composition failed: %+v %v", value, err)
	}
}

func TestOEmbedAppsPublicEligibility(t *testing.T) {
	for _, tc := range []struct {
		name, source, reason string
		change               func(*http.Request, *http.Response)
	}{
		{"source-forbidden", b4CNE, "restricted", func(req *http.Request, res *http.Response) {
			if req.URL.String() == b4CNE {
				res.StatusCode = 403
			}
		}},
		{"source-login-redirect", b4Issuu, "restricted", func(req *http.Request, res *http.Response) {
			if req.URL.String() == b4Issuu {
				u, _ := url.Parse("https://issuu.com/signin")
				res.Request = &http.Request{URL: u}
			}
		}},
		{"publisher-canonical-mismatch", b4Issuu, "restricted", func(req *http.Request, res *http.Response) {
			if req.URL.String() == b4Issuu {
				res.Body = ioBody(b4PublicPage(strings.Replace(b4Issuu, "visitfaroeislands", "other", 1)))
			}
		}},
		{"official-endpoint-redirect", b4CNE, "restricted", func(req *http.Request, res *http.Response) {
			if oembedAppEndpoint(req.URL) {
				u, _ := url.Parse("https://evil.example/oembed")
				res.Request = &http.Request{URL: u}
			}
		}},
		{"script-only", b4CNE, "unsupported_type", func(req *http.Request, res *http.Response) {
			if oembedAppEndpoint(req.URL) {
				res.Body = ioBody(`{"type":"rich","title":"Public item","html":"<script src='https://evil.example/player.js'></script>"}`)
			}
		}},
		{"wrong-content-oembed", b4CNE, "unsupported_type", func(req *http.Request, res *http.Response) {
			if oembedAppEndpoint(req.URL) {
				res.Body = ioBody(`{"type":"rich","title":"Other item","html":"<iframe src='https://player-backend.cnevids.com/iframe/video/54c5640161646d294c080000'></iframe>"}`)
			}
		}},
		{"oembed-forbidden", b4Issuu, "restricted", func(req *http.Request, res *http.Response) {
			if oembedAppEndpoint(req.URL) {
				res.StatusCode = 403
			}
		}},
		{"frame-forbidden", b4CNE, "restricted", func(req *http.Request, res *http.Response) {
			if req.URL.String() == b4CNEFrame {
				res.StatusCode = 403
			}
		}},
		{"frame-ancestor-policy", b4Issuu, "restricted", func(req *http.Request, res *http.Response) {
			if req.URL.Hostname() == "e.issuu.com" {
				res.Header.Set("Content-Security-Policy", "frame-ancestors 'self'")
			}
		}},
		{"frame-wrong-mime", b4Issuu, "fetch_failed", func(req *http.Request, res *http.Response) {
			if req.URL.Hostname() == "e.issuu.com" {
				res.Header.Set("Content-Type", "application/json")
			}
		}},
		{"frame-content-redirect", b4Issuu, "restricted", func(req *http.Request, res *http.Response) {
			if req.URL.Hostname() == "e.issuu.com" {
				u, _ := url.Parse("https://e.issuu.com/embed.html?u=other&d=other")
				res.Request = &http.Request{URL: u}
			}
		}},
		{"oversize-page", b4CNE, "fetch_failed", func(req *http.Request, res *http.Response) {
			if req.URL.String() == b4CNE {
				res.Body = ioBody(strings.Repeat("x", maxPageBytes+1))
			}
		}},
		{"carto-tenant-migration", b4Carto, "restricted", func(req *http.Request, res *http.Response) {
			res.Body = ioBody(b4PublicPage(strings.Replace(b4Carto, "rpa.carto", "other.carto", 1)))
		}},
		{"carto-public-page-without-embed-link", strings.Replace(b4Carto, "/embed_map", "/public_map", 1), "unsupported_type", func(req *http.Request, res *http.Response) {
			res.Body = ioBody(b4PublicPage(req.URL.String()))
		}},
		{"direct-frame-restriction", b4Widgetic, "restricted", func(req *http.Request, res *http.Response) {
			res.Header.Set("X-Frame-Options", "SAMEORIGIN")
		}},
		{"widgetic-generic-shell", b4Widgetic, "unsupported_type", func(req *http.Request, res *http.Response) { res.Body = ioBody(b4PublicPage(b4Widgetic)) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := b4Resolver(t, tc.change)
			value, err := r.ResolveSource(context.Background(), tc.source)
			if err != nil || value.Preview == nil || value.Preview.Kind != "card" || value.Preview.Reason != tc.reason || value.Preview.HTML != "" {
				t.Fatalf("restricted source gained authority: %+v %v", value, err)
			}
		})
	}
}

func TestOEmbedAppsUnsupportedFamiliesDoNotGuessFrames(t *testing.T) {
	for _, raw := range []string{
		"https://www.kickstarter.com/projects/sparkdevices/spark-electron-cellular-dev-kit-with-a-simple-data",
		"https://app.widgetic.com/compositions/555b5e6509c7e29e718b4569",
		"https://widgetic.com/widgets/photo/mosaic-gallery/",
		"https://clausa.app.carto.com/map/ff76a0cd-fd9c-4893-8c2b-d9c587f2d699",
		strings.Replace(b4Carto, ".carto.com", ".cartodb.com", 1),
		strings.Replace(b4CNE, "https:", "http:", 1),
		b4Issuu + "?token=private",
	} {
		r, calls := b4Resolver(t, nil)
		value, err := r.ResolveSource(context.Background(), raw)
		if err != nil || value.Preview == nil || value.Preview.Kind != "card" || value.Preview.Reason != "unsupported_type" || *calls != 0 {
			t.Errorf("unsupported family was fetched or rendered: %+v %v calls=%d", value, err, *calls)
		}
	}
}
