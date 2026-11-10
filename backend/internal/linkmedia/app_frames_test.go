package linkmedia

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

type appFrameFixture struct {
	Provider, Source, URL string
}

func appFrameFixtures(t *testing.T) []appFrameFixture {
	t.Helper()
	data, err := os.ReadFile("../../../frontend/shared/embed-fixtures/iframely.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Fixtures []appFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	var out []appFrameFixture
	for _, item := range catalog.Fixtures {
		if appFrameFamily(item.Provider) != "" {
			out = append(out, item)
		}
	}
	if len(out) != 25 {
		t.Fatalf("unexpected real upstream fixture count: %d", len(out))
	}
	return out
}

func appFrameFamily(family string) string {
	return map[string]string{"dbdiagram.io": "dbdiagram", "slid.es": "slides", "quizlet.com": "quizlet", "openstreetmap.org": "openstreetmap", "maps.yandex.ru": "yandex-maps", "padlet.com": "padlet"}[family]
}

func TestAppFrameUpstreamFixtures(t *testing.T) {
	for _, fixture := range appFrameFixtures(t) {
		t.Run(fixture.Provider+"/"+fixture.URL, func(t *testing.T) {
			name := appFrameFamily(fixture.Provider)
			u, err := url.Parse(fixture.URL)
			if err != nil {
				t.Fatal(err)
			}
			target, eligible := appFrameRoute(name, u)
			blockedShape := strings.HasSuffix(u.Path, "/scatter") || strings.HasSuffix(u.Path, "/gravity") ||
				name == "padlet" && (strings.HasSuffix(u.Path, "/musique_et_publicite") || strings.HasSuffix(u.Path, "/rapaublues"))
			if eligible == blockedShape {
				t.Fatalf("route eligible=%t blockedShape=%t", eligible, blockedShape)
			}
			if !eligible {
				return
			}
			frame, height, ratio, ok := ControlledPlayer(fixture.URL, target.String(), "https://chat.example.test")
			if !ok || frame != target.String() {
				t.Fatalf("real route lost source binding: %q %t", frame, ok)
			}
			wantHeight, wantRatio := 360, 4.0/3
			switch name {
			case "dbdiagram":
				wantHeight, wantRatio = 270, 16.0/9
			case "slides":
				wantHeight, wantRatio = 350, 48.0/35
			case "quizlet":
				wantHeight, wantRatio = 500, 0
			case "padlet":
				wantHeight, wantRatio = 608, 0
			}
			if height != wantHeight || ratio != wantRatio || resolutionVersion(fixture.URL) != "native-providers-v3" {
				t.Fatalf("wrong geometry/cache epoch: %d %f", height, ratio)
			}
		})
	}
}

func TestAppFrameAnonymousFixtureResolution(t *testing.T) {
	for _, fixture := range appFrameFixtures(t) {
		t.Run(fixture.Provider+"/"+fixture.URL, func(t *testing.T) {
			name := appFrameFamily(fixture.Provider)
			u, _ := url.Parse(fixture.URL)
			target, eligible := appFrameRoute(name, u)
			r := NewResolver(ResolverOptions{})
			calls := 0
			r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" || req.URL.Query().Has("token") {
					t.Fatal("public frame inherited credentials")
				}
				if !eligible {
					t.Fatal("blocked source triggered a fetch")
				}
				body := `<html><head><meta property="og:title" content="Public fixture"><link rel="canonical" href="` + html.EscapeString(fixture.URL) + `"></head><body><script type="application/json">{"visibility":"all"}</script></body></html>`
				ct := "text/html"
				if name == "padlet" {
					id := strings.TrimPrefix(target.Path, "/embed/")
					if req.URL.Path == "/padlets/"+id+"/exports/feed.xml" {
						ct, body = "application/xml", `<?xml version="1.0"?><rss><channel><title>Public board</title></channel></rss>`
					} else {
						body = `<html><head><meta property="og:title" content="Public board"><link rel="alternate" href="https://padlet.com/padlets/` + id + `/exports/feed.xml?token=synthetic_discarded_token"></head></html>`
					}
				} else if name == "dbdiagram" {
					body = `<html><head><title>dbdiagram.io - Database Relationship Diagrams Design Tool</title></head><body>Generic app shell</body></html>`
				}
				res := response(200, ct, body)
				res.Request = req
				return res, nil
			})}
			result, err := r.ResolveSource(context.Background(), fixture.URL)
			wantWidget := eligible && name != "dbdiagram"
			if err != nil || result.Preview == nil || (result.Preview.Kind == "widget") != wantWidget || len(result.Assets) != 0 {
				t.Fatalf("incorrect anonymous result: %+v %v calls=%d", result.Preview, err, calls)
			}
			if wantWidget && !strings.Contains(result.Preview.HTML, html.EscapeString(target.String())) {
				t.Fatalf("result switched frame identity: %q", result.Preview.HTML)
			}
			if calls > 2 {
				t.Fatalf("unbounded provider requests: %d", calls)
			}
		})
	}
}

func TestAppFrameBoundaryPolicy(t *testing.T) {
	for _, tc := range []struct{ name, source, candidate string }{
		{"dbdiagram", "https://dbdiagram.io/d/6201630385022f4ee5518762", "https://dbdiagram.io/embed/61eeb8797cf3fc0e7c5d74aa"},
		{"slides", "https://slides.com/timkindberg/ui-router", "https://slides.com/other/deck/embed"},
		{"quizlet", "https://quizlet.com/43729824/physics", "https://quizlet.com/74274924/flashcards/embed"},
		{"quizlet", "https://quizlet.com/43729824/physics", "https://quizlet.com/43729824/learn/embed"},
		{"padlet", "https://padlet.com/embed/g44u46on3u5n", "https://padlet.com/embed/6x7j8v4yqrfp"},
		{"openstreetmap", "https://www.openstreetmap.org/#map=12/50.2598/28.6695", "https://www.openstreetmap.org/export/embed.html?bbox=1,2,3,4&layer=mapnik"},
		{"yandex-maps", "https://yandex.ru/maps/-/CWS~5QYp", "https://yandex.ru/map-widget/v1/-/CLu~B4po"},
	} {
		if _, _, _, ok := ControlledPlayer(tc.source, tc.candidate, "https://chat.example.test"); ok {
			t.Errorf("%s accepted a different content identity", tc.name)
		}
	}
	for _, raw := range []string{
		"https://slides.com/timkindberg/ui-router?token=private",
		"https://padlet.com/embed/g44u46on3u5n?password=private",
		"https://dbdiagram.io/d/6201630385022f4ee5518762?key=private",
		"https://yandex.ru/maps/?ll=NaN,55&z=14",
		"https://yandex.ru/maps/?ll=37,Infinity&z=14",
		"https://yandex.ru/maps/?ll=190,55&z=14",
		"https://yandex.ru/maps/?ll=37,55&z=99",
		"https://yandex.ru/maps/?ll=37,55&z=14&key=private",
		"https://yandex.ru/maps/?ll=37,55&z=14&poi%5Buri%5D=http://127.0.0.1/private",
		"https://www.openstreetmap.org/?lat=NaN&lon=16&zoom=15",
		"https://www.openstreetmap.org/?lat=90&lon=16&zoom=15",
		"https://www.openstreetmap.org/export/embed.html?bbox=4,3,2,1",
		"https://www.openstreetmap.org/export/embed.html?bbox=1,2,3,4&layer=cyclemap",
		"https://www.openstreetmap.org/#map=12/50/28&url=http://127.0.0.1/private",
		"https://user:pass@slides.com/timkindberg/ui-router",
		"https://slides.com:444/timkindberg/ui-router",
	} {
		u, _ := url.Parse(raw)
		if p := publicProviderFor(u); p != nil {
			t.Errorf("unsafe frame source accepted: %s", p.name)
		}
	}
}

func TestAppFrameFailedAndPrivatePages(t *testing.T) {
	source := "https://slides.com/timkindberg/ui-router"
	for _, tc := range []struct {
		name, final, body string
		status            int
	}{
		{"non-200", source, `<html><title>Unavailable</title></html>`, 403},
		{"login", "https://slides.com/users/sign_in", `<html><title>Sign in</title></html>`, 200},
		{"other-content", "https://slides.com/other/deck", `<html><title>Other deck</title></html>`, 200},
		{"private-metadata", source, `<html><meta property="og:title" content="Private"><link rel="canonical" href="` + source + `"><script>{"visibility":"private"}</script></html>`, 200},
		{"oversize", source, `<html>` + strings.Repeat("x", maxPageBytes) + `</html>`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewResolver(ResolverOptions{})
			r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				res := response(tc.status, "text/html", tc.body)
				final, _ := url.Parse(tc.final)
				res.Request = &http.Request{URL: final}
				return res, nil
			})}
			result, err := r.ResolveSource(context.Background(), source)
			if err != nil || result.Preview == nil || result.Preview.Kind != "card" || result.Preview.HTML != "" {
				t.Fatalf("restricted frame gained authority: %+v %v", result.Preview, err)
			}
		})
	}
	for _, value := range []string{"frame-ancestors 'none'", "frame-ancestors 'self'", "frame-ancestors https://private.example"} {
		if publicFrameHeaders(http.Header{"Content-Security-Policy": {value}}) {
			t.Fatal("restricted frame-ancestors accepted")
		}
	}
	if !publicFrameHeaders(http.Header{"Content-Security-Policy": {"default-src 'self'; frame-ancestors *"}}) {
		t.Fatal("public frame-ancestors rejected")
	}
}
