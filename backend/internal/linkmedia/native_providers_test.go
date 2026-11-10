package linkmedia

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/storage"
)

// Public URLs and standard metadata below were reviewed anonymously against
// the pinned upstream plugin test URLs. Payload bytes are deliberately tiny
// synthetic fixtures; these tests never fetch those public URLs.
const nativeDroplrImage = "https://2code.d.pr/i/9jB7.png"
const nativeDroplrVideo = "https://cdn-std.droplr.net/files/acc_1148136/6xCyV3"
const nativeImageShackPhoto = "https://imagizer.imageshack.com/v2/1200x630q90/538/n2zNjE.jpg"
const nativeSmugMugPhoto = "https://kalyr.smugmug.com/Music/2016/Cambridge-Rock-Festival-2016/i-95w2R4W/0/KMVGrnH9KwfBRSXD6V2VmgfLVn56SCksWgW8G6WVS/600x399/DSC01206-600x399.jpg"
const nativeGiphyClip = "https://media0.giphy.com/media/v1.Y2lkPTc5MGI3NjExeHFjMnlyczNlMGZ3Z3J5anRrNW4wdnBhaWFtbDdvY3ZhZDNwYzY1aSZlcD12MV9pbnRlcm5hbF9naWZfYnlfaWQmY3Q9dg/kBMPSpmc4vbIc3h7Zo/giphy.mp4"

type nativeFixture struct {
	source, canonical, media, metadata, oembed, provider, kind string
	width, height                                              int
}

func nativeFixtures() []nativeFixture {
	return []nativeFixture{
		{"https://d.pr/i/9jB7", "https://2code.d.pr/i/9jB7", nativeDroplrImage,
			`<link rel="alternate" type="application/json+oembed" href="https://embed.droplr.com/oembed?url=https%3A%2F%2F2code.d.pr%2Fi%2F9jB7">`,
			`{"type":"photo","url":"` + nativeDroplrImage + `","width":4400,"height":2206}`, "droplr", "image", 2, 3},
		{"https://d.pr/v/6xCyV3", "https://d.pr/v/6xCyV3", nativeDroplrVideo,
			`<meta property="og:video" content="` + nativeDroplrVideo + `"><meta property="og:video:type" content="video/mp4"><link rel="alternate" type="application/json+oembed" href="https://embed.droplr.com/oembed?url=https%3A%2F%2Fd.pr%2Fv%2F6xCyV3">`,
			`{"type":"video","html":"<video controls></video>","width":1280,"height":720}`, "droplr", "video", 1280, 720},
		{"https://imageshack.com/i/eyn2zNjEj", "https://imageshack.com/i/eyn2zNjEj", nativeImageShackPhoto,
			`<meta property="og:image" content="` + nativeImageShackPhoto + `">`, "", "imageshack", "image", 2, 3},
		{"https://kalyr.smugmug.com/Music/2016/Cambridge-Rock-Festival-2016/i-95w2R4W", "https://kalyr.smugmug.com/Music/2016/Cambridge-Rock-Festival-2016/i-95w2R4W", nativeSmugMugPhoto,
			`<meta property="og:image" content="https://photos.smugmug.com/photos/i-95w2R4W/0/MLbngghGSFShQhjfZ8f4KmxTtPhp67PHqnMfKNVGD/XL/i-95w2R4W-XL.jpg">`,
			`{"type":"photo","url":"` + nativeSmugMugPhoto + `","width":600,"height":399}`, "smugmug", "image", 2, 3},
		{"https://giphy.com/videos/kehlani-open-passionate-kBMPSpmc4vbIc3h7Zo", "https://giphy.com/clips/kehlani-open-passionate-kBMPSpmc4vbIc3h7Zo", nativeGiphyClip,
			`<meta property="og:video" content="` + nativeGiphyClip + `"><meta property="og:video:type" content="video/mp4"><meta property="og:video:width" content="480"><meta property="og:video:height" content="270">`, "", "giphy-clips", "video", 480, 270},
	}
}

func nativeFixturePage(f nativeFixture) string {
	return `<html><head><meta property="og:title" content="Public fixture"><meta property="og:url" content="` + f.canonical + `">` + f.metadata + `</head><body></body></html>`
}

func newNativeTestResolver(t *testing.T) (*Resolver, *int) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	uploads := new(int)
	client, err := cdn.New(cdn.Options{Endpoint: "https://storage.example.test", Bucket: "fixture", Region: "us-east-1", PublicURL: "https://cdn.example.test", Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPut {
			t.Errorf("unexpected storage method: %s", req.Method)
		}
		*uploads++
		_, _ = io.Copy(io.Discard, req.Body)
		res := response(http.StatusOK, "application/xml", "")
		res.Header.Set("ETag", `"fixture"`)
		return res, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	r := NewResolver(ResolverOptions{CDN: client})
	r.persistent = &publicCache{db: store.DB, namespace: client.Namespace()}
	return r, uploads
}

func nativeFixtureBytes(t *testing.T, kind string) []byte {
	t.Helper()
	if kind == "video" {
		return []byte{0, 0, 0, 32, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 2, 0, 'i', 's', 'o', 'm', 'i', 's', 'o', '2', 'a', 'v', 'c', '1', 'm', 'p', '4', '1'}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestNativeProviderReviewedMediaUsesValidatedIngest(t *testing.T) {
	for _, fixture := range nativeFixtures() {
		t.Run(fixture.source, func(t *testing.T) {
			r, uploads := newNativeTestResolver(t)
			data := nativeFixtureBytes(t, fixture.kind)
			mediaCalls := 0
			r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" {
					t.Error("public metadata inherited credentials")
				}
				var res *http.Response
				switch {
				case req.URL.String() == fixture.source:
					res = response(http.StatusOK, "text/html", nativeFixturePage(fixture))
				case req.URL.String() == fixture.media:
					mediaCalls++
					res = response(http.StatusOK, "application/octet-stream", string(data))
				case req.URL.Host == "embed.droplr.com" || req.URL.Host == "api.smugmug.com":
					if req.URL.Query().Get("url") != fixture.canonical {
						t.Error("oEmbed changed the source identity")
					}
					res = response(http.StatusOK, "application/json", fixture.oembed)
				default:
					t.Errorf("unexpected request: %s", req.URL.Redacted())
					res = response(http.StatusNotFound, "text/plain", "unavailable")
				}
				res.Request = req
				return res, nil
			})}
			value, err := r.resolveNativeProvider(context.Background(), fixture.source)
			if err != nil || len(value.Assets) != 1 || value.Preview != nil || value.Provider != fixture.provider || *uploads != 1 || mediaCalls != 1 {
				t.Fatalf("native media not ingested once: provider=%s assets=%d uploads=%d fetches=%d error=%v", value.Provider, len(value.Assets), *uploads, mediaCalls, err)
			}
			asset := value.Assets[0]
			if asset.SourceURL != fixture.media || asset.Kind != fixture.kind || asset.Width != fixture.width || asset.Height != fixture.height || !r.IsDeliveryURL(asset.URL) || time.Until(asset.ExpiresAt) < 89*24*time.Hour {
				t.Fatalf("native media lost validation, source identity, dimensions or 90-day deadline: %+v", asset)
			}
			if fixture.provider == "smugmug" && strings.Contains(asset.SourceURL, "/X3/") {
				t.Fatal("invented a larger SmugMug source")
			}
		})
	}
}

func TestNativeProviderRejectsUnreviewedRoutesAndPrivateSources(t *testing.T) {
	for _, raw := range []string{
		"https://500px.com/photo/gqZ6Tykaj5", // No reviewed public metadata source.
		"https://d.pr/secret", "https://d.pr/i/9jB7?password=private", "https://evil.d.pr.evil.test/i/9jB7", "https://d.pr/i/9jB7?resourcekey=private",
		"https://imageshack.com/user/lynnseyography", "https://imageshack.com/i/eyn2zNjEj?token=private",
		"https://kalyr.smugmug.com/Music/2016/", "https://photos.smugmug.com/i-95w2R4W", "https://api.smugmug.com/i-95w2R4W",
		"https://giphy.com/gifs/idk-aXSLMy6fDsI4E", "https://giphy.com/clips/id?api_key=private", "https://giphy.com/clips/id%2Fother",
		"http://d.pr/i/9jB7", "https://user:pass@d.pr/i/9jB7", "https://d.pr:444/i/9jB7",
	} {
		if NativeProviderSource(raw) {
			t.Errorf("unreviewed or private route accepted: %s", raw)
		}
	}
	for _, f := range nativeFixtures() {
		if !NativeProviderSource(f.source) || NativeProviderImageSource(f.source) != (f.kind == "image") {
			t.Errorf("reviewed route or geometry missing: %s", f.source)
		}
		other := "image"
		if f.kind == "image" {
			other = "video"
		}
		if !NativeProviderMediaSource(f.source, f.kind) || NativeProviderMediaSource(f.source, other) {
			t.Errorf("source page changed its reserved media kind: %s", f.source)
		}
	}
}

func TestNativeProviderRejectsMediaFromAnotherContentOrOrigin(t *testing.T) {
	for _, f := range nativeFixtures() {
		source, _ := nativeProviderSource(f.source)
		if !nativeProviderMedia(source, f.media) {
			t.Fatalf("reviewed public media not accepted: %s", f.media)
		}
		for _, raw := range []string{
			"https://evil.example/" + source.id + ".jpg",
			strings.Replace(f.media, source.id, "OtherPublicContent", 1),
			strings.Replace(f.media, "https://", "http://", 1),
			"https://cdn.example/files/" + strings.Repeat("a", 32) + "/" + strings.Repeat("b", 48) + "/photo.png",
		} {
			// ImageShack short IDs wrap the media basename; substitute that
			// reviewed basename rather than an unrelated source-ID spelling.
			if source.name == "imageshack" && raw == f.media {
				raw = strings.Replace(raw, "n2zNjE", "OtherPublicContent", 1)
			}
			if nativeProviderMedia(source, raw) {
				t.Errorf("wrong media identity or origin accepted: provider=%s", source.name)
			}
		}
	}
}

func TestNativeProviderFailedOrPrivatePageDoesNotFetchAlternateOrMedia(t *testing.T) {
	fixture := nativeFixtures()[0]
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		r := NewResolver(ResolverOptions{})
		calls := 0
		r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			calls++
			res := response(status, "text/html", nativeFixturePage(fixture))
			res.Request = req
			return res, nil
		})}
		value, err := r.resolveNativeProvider(context.Background(), fixture.source)
		if err != nil || len(value.Assets) != 0 || value.Preview == nil || calls != 1 {
			t.Fatalf("unsuccessful public page triggered another fetch: status=%d calls=%d error=%v", status, calls, err)
		}
	}
	if m := readNativeProviderMetadata([]byte(`<html><head></head><body><meta property="og:url" content="https://d.pr/i/9jB7"><script>secret</script></body></html>`)); m.canonical != "" {
		t.Fatal("body/script metadata granted a media source")
	}
}

func TestNativeProviderRedirectPoliciesStayBoundedAndPublic(t *testing.T) {
	r := NewResolver(ResolverOptions{})
	source, _ := nativeProviderSource("https://d.pr/i/9jB7")
	client := r.nativeProviderClient(func(next string) bool { return nativeProviderMedia(source, next) })
	for _, raw := range []string{"https://127.0.0.1/i/9jB7.png", "https://evil.example/i/9jB7.png", "https://cdn-sec.droplr.net/files/acc_166565/OtherContent"} {
		u, _ := url.Parse(raw)
		if err := client.http.CheckRedirect(&http.Request{URL: u, Header: http.Header{}}, nil); err == nil {
			t.Error("redirect escaped its public provider/content policy")
		}
	}
	u, _ := url.Parse(nativeDroplrImage)
	if err := client.http.CheckRedirect(&http.Request{URL: u, Header: http.Header{}}, make([]*http.Request, 4)); err == nil {
		t.Fatal("redirect count became unbounded")
	}
	if !nativeProviderMedia(source, "https://cdn-sec.droplr.net/files/acc_166565/9jB7?Expires=1999999999&Key-Pair-Id=public-fixture&Signature=public-fixture") {
		t.Fatal("anonymous same-drop CDN redirect was rejected")
	}
	if nativeProviderMedia(source, nativeDroplrImage+"?auth=private") {
		t.Fatal("private media query was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.resolveNativeProvider(ctx, "https://d.pr/i/9jB7"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was hidden: %v", err)
	}
}

func TestNativeProviderAnonymousImageRedirectRetainsIdentity(t *testing.T) {
	r, uploads := newNativeTestResolver(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(nativeDroplrImage)
	jar.SetCookies(u, []*http.Cookie{{Name: "account", Value: "private-fixture"}})
	redirect := "https://cdn-sec.droplr.net/files/acc_166565/9jB7?Expires=1999999999&Key-Pair-Id=public-fixture&Signature=public-fixture"
	r.http = &http.Client{Jar: jar, Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" {
			t.Error("anonymous source/redirect inherited credentials")
		}
		var res *http.Response
		switch req.URL.String() {
		case nativeDroplrImage:
			res = response(http.StatusFound, "text/plain", "")
			res.Header.Set("Location", redirect)
		case redirect:
			res = response(http.StatusOK, "image/png", string(nativeFixtureBytes(t, "image")))
		default:
			t.Fatal("redirect escaped the same-drop source")
		}
		res.Request = req
		return res, nil
	})}
	source, _ := nativeProviderSource("https://d.pr/i/9jB7")
	client := r.nativeProviderClient(func(next string) bool { return nativeProviderMedia(source, next) })
	asset, err := client.ingest(context.Background(), Asset{URL: nativeDroplrImage, Kind: "image"}, "")
	if err != nil || *uploads != 1 || asset.SourceURL != nativeDroplrImage || strings.Contains(asset.SourceURL, "Signature") || !r.IsDeliveryURL(asset.URL) {
		t.Fatalf("public redirect lost isolation or canonical source: uploads=%d error=%v", *uploads, err)
	}
}

func TestNativeProviderSmugMugRichAndWrongPhotoStayUnavailable(t *testing.T) {
	f := nativeFixtures()[3]
	for _, payload := range []string{
		`{"type":"rich","html":"<script>untrusted()</script>"}`,
		`{"type":"photo","url":"https://evil.example/photo.jpg"}`,
		`{"type":"photo","url":"` + strings.Replace(nativeSmugMugPhoto, "i-95w2R4W", "i-OtherImage", 1) + `"}`,
	} {
		r, uploads := newNativeTestResolver(t)
		r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			var res *http.Response
			if req.URL.String() == f.source {
				res = response(http.StatusOK, "text/html", nativeFixturePage(f))
			} else if req.URL.Host == "api.smugmug.com" {
				res = response(http.StatusOK, "application/json", payload)
			} else {
				t.Fatal("unvalidated rich/wrong-photo metadata was fetched")
			}
			res.Request = req
			return res, nil
		})}
		value, err := r.resolveNativeProvider(context.Background(), f.source)
		if err != nil || len(value.Assets) != 0 || *uploads != 0 {
			t.Fatalf("rich/wrong-photo result became media: uploads=%d error=%v", *uploads, err)
		}
	}
}

func TestNativeProviderIngestRejectsActiveContentAndSize(t *testing.T) {
	fixture := nativeFixtures()[2]
	for _, tooLarge := range []bool{false, true} {
		r, uploads := newNativeTestResolver(t)
		if tooLarge {
			r.options.MaxFileBytes = 4
		}
		r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			body, ct := nativeFixturePage(fixture), "text/html"
			if req.URL.String() == fixture.media {
				body = `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`
				if tooLarge {
					body = string(nativeFixtureBytes(t, "image"))
				}
			}
			res := response(http.StatusOK, ct, body)
			res.Request = req
			return res, nil
		})}
		value, err := r.resolveNativeProvider(context.Background(), fixture.source)
		if err != nil || len(value.Assets) != 0 || *uploads != 0 {
			t.Fatalf("unvalidated bytes reached CDN: oversized=%v uploads=%d error=%v", tooLarge, *uploads, err)
		}
	}
}
