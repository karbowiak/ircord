package linkmedia

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, contentType, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
}

func TestRedditOAuthReuseRefreshAndOrderedGallery(t *testing.T) {
	tokens, posts := 0, 0
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "www.reddit.com" {
			tokens++
			id, secret, ok := r.BasicAuth()
			if !ok || id != "fixture-client" || secret != "fixture-secret" {
				t.Error("OAuth credentials missing")
			}
			return response(200, "application/json", `{"access_token":"fixture-token","expires_in":3600}`), nil
		}
		posts++
		if r.Header.Get("Authorization") != "Bearer fixture-token" || r.URL.Query().Get("raw_json") != "1" {
			t.Error("post authentication/options missing")
		}
		if posts == 3 {
			return response(401, "application/json", `{}`), nil
		}
		return response(200, "application/json", `[{"data":{"children":[{"data":{"id":"abc","title":"Gallery","author":"someone","permalink":"/r/test/comments/abc/gallery/","is_gallery":true,"gallery_data":{"items":[{"media_id":"second"},{"media_id":"first"}]},"media_metadata":{"first":{"status":"valid","m":"image/jpeg","s":{"u":"https://i.redd.it/first.jpg","x":20,"y":30}},"second":{"status":"valid","m":"image/png","s":{"u":"https://preview.redd.it/second.png?width=100&amp;format=png","x":100,"y":200}}}}}]}}]`), nil
	})}
	r := NewReddit(RedditOptions{ClientID: "fixture-client", ClientSecret: "fixture-secret", HTTPClient: client})
	for range 3 {
		out, err := r.Resolve(context.Background(), "https://redd.it/abc")
		if err != nil || len(out.Assets) != 2 || out.Assets[0].URL != "https://preview.redd.it/second.png?width=100&format=png" || out.Assets[1].Width != 20 || out.Title != "Gallery" {
			t.Fatalf("post: %+v %v", out, err)
		}
	}
	if tokens != 2 || posts != 4 {
		t.Fatalf("token reuse/401 refresh: tokens=%d posts=%d", tokens, posts)
	}
	for _, raw := range []string{"https://reddit.com.evil.test/comments/abc", "https://reddit.com/redirect?url=x", "https://reddit.com/comments/../secret"} {
		if _, err := r.Resolve(context.Background(), raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestInstagramPhotoAndMixedGallery(t *testing.T) {
	for _, fixture := range []struct {
		name, body string
		want       []string
	}{
		{"photo", `{"items":[{"code":"Photo","media_type":1,"user":{"username":"creator"},"caption":{"text":"A photo"},"image_versions2":{"candidates":[{"url":"https://scontent.cdninstagram.com/small.jpg","width":10,"height":20},{"url":"https://scontent.cdninstagram.com/full.jpg","width":100,"height":200}]}}]}`, []string{"https://scontent.cdninstagram.com/full.jpg"}},
		{"mixed", `<html><script type="application/json">{"data":{"xdt_shortcode_media":{"shortcode":"Photo","__typename":"XDTGraphSidecar","owner":{"username":"creator"},"edge_sidecar_to_children":{"edges":[{"node":{"__typename":"GraphImage","display_url":"https://scontent.cdninstagram.com/first.jpg"}},{"node":{"__typename":"GraphVideo","is_video":true,"video_url":"https://scontent.cdninstagram.com/second.mp4","display_url":"https://scontent.cdninstagram.com/thumbnail.jpg"}}]}}}}</script></html>`, []string{"https://scontent.cdninstagram.com/first.jpg", "https://scontent.cdninstagram.com/second.mp4"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			connector := NewInstagram(&http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				res := response(200, "text/html", fixture.body)
				res.Request = r
				return res, nil
			})})
			out, err := connector.Resolve(context.Background(), "https://instagram.com/p/Photo/?igsh=tracking")
			if err != nil || out.Author != "creator" || len(out.Assets) != len(fixture.want) {
				t.Fatalf("result: %+v %v", out, err)
			}
			for index, raw := range fixture.want {
				if out.Assets[index].URL != raw {
					t.Fatalf("gallery order: %+v", out.Assets)
				}
			}
		})
	}
	connector := NewInstagram(&http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		res := response(200, "text/html", `<meta property="og:image" content="https://cdn.example/thumbnail.jpg">Please log in`)
		res.Request = r
		return res, nil
	})})
	if _, err := connector.Resolve(context.Background(), "https://instagram.com/p/Photo/"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("thumbnail incorrectly treated as complete post", err)
	}
}

func fakeDownloader(t *testing.T, document string) *YTDLP {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "extractor")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$0.args\"\ncat \"$0.json\"\n"
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary+".json", []byte(document), 0600); err != nil {
		t.Fatal(err)
	}
	return NewYTDLP(YTDLPOptions{Binary: binary, Timeout: time.Second})
}

func TestYTDLPExtractsURLsWithoutDownloadsAndRejectsUnplayableResults(t *testing.T) {
	y := fakeDownloader(t, `{"id":"123","extractor_key":"TikTok","title":"A video","uploader":"creator","webpage_url":"https://www.tiktok.com/@creator/video/123","url":"https://v19-webapp-prime.tiktok.com/video/movie.mp4?expires=1999999999","ext":"mp4","protocol":"https","vcodec":"h264","acodec":"aac","width":720,"height":1280,"http_headers":{"Referer":"https://www.tiktok.com/","Cookie":"private-test-cookie"}}`)
	out, err := y.Resolve(context.Background(), "https://www.tiktok.com/@creator/video/123?sender_device=pc")
	if err != nil || len(out.Assets) != 1 || out.Assets[0].Kind != "video" || out.Assets[0].Width != 720 {
		t.Fatalf("extraction: %+v %v", out, err)
	}
	encoded, _ := json.Marshal(out)
	if strings.Contains(string(encoded), "private-test-cookie") {
		t.Fatal("source credentials leaked into JSON")
	}
	args, _ := os.ReadFile(y.options.Binary + ".args")
	for _, flag := range []string{"--simulate\n", "--skip-download\n", "--ignore-config\n", "--no-plugin-dirs\n", "--proxy\nhttp://127.0.0.1:", "--\nhttps://www.tiktok.com/@creator/video/123"} {
		if !strings.Contains(string(args), flag) {
			t.Fatalf("missing safe invocation argument %q", flag)
		}
	}
	for _, document := range []string{
		`{"ext":"mp4","thumbnail":"https://cdn.example.test/preview.jpg"}`,
		`{"url":"https://cdn.example.test/silent.mp4","ext":"mp4","vcodec":"h264","acodec":"none"}`,
		`{"url":"https://127.0.0.1/file.mp4","ext":"mp4"}`,
		`{"url":"https://cdn.example.test/manifest.m3u8","ext":"mp4","protocol":"m3u8_native"}`,
	} {
		if _, err := fakeDownloader(t, document).Resolve(context.Background(), "https://tiktok.com/@creator/video/123"); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("unplayable result accepted: %s: %v", document, err)
		}
	}
	for _, raw := range []string{"https://tiktok.com.evil.test/@a/video/123", "https://tiktok.com/redirect?url=http://localhost", "https://instagram.com/accounts/login/"} {
		if _, _, err := extractionURL(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestYTDLPCancellation(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "slow-extractor")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexec sleep 10\n"), 0700); err != nil {
		t.Fatal(err)
	}
	y := NewYTDLP(YTDLPOptions{Binary: binary, Timeout: 20 * time.Millisecond})
	start := time.Now()
	if _, err := y.Resolve(context.Background(), "https://tiktok.com/@creator/video/123"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("extractor did not stop promptly")
	}
}

func TestYTDLPRestrictionDiagnosticsStayPrivate(t *testing.T) {
	for diagnostic, want := range map[string]error{
		"Instagram sent an empty media response. Check access without being logged-in. Cookie=private-value": ErrRestricted,
		"This video is age-restricted. https://signed.example/?token=private-value":                          ErrAgeRestricted,
	} {
		y := fakeDownloader(t, diagnostic)
		if err := os.WriteFile(y.options.Binary, []byte("#!/bin/sh\ncat \"$0.json\" >&2\nexit 1\n"), 0700); err != nil {
			t.Fatal(err)
		}
		_, err := y.Resolve(context.Background(), "https://instagram.com/reel/Restricted/")
		if !errors.Is(err, want) || strings.Contains(err.Error(), "private-value") {
			t.Fatalf("restriction classification leaked/lost: %v", err)
		}
	}
}

func TestInstagramExtractorPhotoVariantsAndMixedGallery(t *testing.T) {
	photo := `{"id":"Photo","extractor_key":"Instagram","formats":[],"thumbnails":[{"url":"https://scontent.cdninstagram.com/small.jpg"},{"url":"https://scontent.cdninstagram.com/full.jpg"}],"thumbnail":"https://scontent.cdninstagram.com/full.jpg"}`
	video := `{"extractor_key":"Instagram","formats":[{"url":"https://scontent.cdninstagram.com/audio.m4a","ext":"m4a","protocol":"https","vcodec":"none","acodec":"aac"},{"url":"https://scontent.cdninstagram.com/video.mp4","ext":"mp4","protocol":"https"}]}`
	ctx := context.Background()
	out, err := fakeDownloader(t, photo).Resolve(ctx, "https://instagram.com/p/Photo/")
	if err != nil || len(out.Assets) != 1 || out.Assets[0].Kind != "image" || out.Assets[0].URL != "https://scontent.cdninstagram.com/full.jpg" {
		t.Fatalf("photo variants without dimensions: %+v %v", out, err)
	}
	out, err = fakeDownloader(t, `{"id":"Gallery","extractor_key":"Instagram","entries":[`+photo+`,`+video+`,`+photo+`]}`).Resolve(ctx, "https://instagram.com/p/Gallery/")
	if err != nil || len(out.Assets) != 3 || out.Assets[0].Kind != "image" || out.Assets[1].Kind != "video" || out.Assets[2].Kind != "image" {
		t.Fatalf("mixed gallery lost order or selected audio: %+v %v", out, err)
	}
	for _, document := range []string{
		`{"id":"Photo","extractor_key":"Instagram","formats":[],"duration":10,"thumbnails":[{"url":"https://scontent.cdninstagram.com/poster.jpg"}]}`,
		`{"id":"Photo","extractor_key":"Instagram","formats":[],"_has_drm":true,"thumbnails":[{"url":"https://scontent.cdninstagram.com/poster.jpg"}]}`,
		`{"id":"Photo","extractor_key":"TikTok","formats":[],"thumbnails":[{"url":"https://scontent.cdninstagram.com/poster.jpg"}]}`,
	} {
		if _, err := fakeDownloader(t, document).Resolve(ctx, "https://instagram.com/p/Photo/"); !errors.Is(err, ErrUnavailable) {
			t.Fatal("unavailable video became a still", err)
		}
	}
}

func TestLiveReddit(t *testing.T) {
	raw := os.Getenv("IRCORD_TEST_REDDIT_URL")
	if raw == "" {
		t.Skip("set IRCORD_TEST_REDDIT_URL and REDDIT_CLIENT_ID/REDDIT_CLIENT_SECRET for an opt-in API probe")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r := NewReddit(RedditOptions{ClientID: os.Getenv("REDDIT_CLIENT_ID"), ClientSecret: os.Getenv("REDDIT_CLIENT_SECRET"), UserAgent: os.Getenv("REDDIT_USER_AGENT")})
	out, err := r.Resolve(ctx, raw)
	if err != nil || out.Title == "" {
		t.Fatalf("Reddit API probe: %v", err)
	}
}

func TestLiveExtractor(t *testing.T) {
	raw := os.Getenv("IRCORD_TEST_EXTRACTOR_URL")
	if raw == "" {
		t.Skip("set IRCORD_TEST_EXTRACTOR_URL for an opt-in yt-dlp probe")
	}
	out, err := NewYTDLP(YTDLPOptions{Timeout: 20 * time.Second}).Resolve(context.Background(), raw)
	if err != nil || len(out.Assets) == 0 {
		t.Fatalf("yt-dlp probe: %v", err)
	}
}

func TestYTDLPCookiesReachOnlyTheirMediaHost(t *testing.T) {
	info := ytInfo{ExtractorKey: "TikTok", URL: "https://v19-webapp-prime.tiktok.com/video/1.mp4", Ext: "mp4", Protocol: "https", VCodec: "h264", ACodec: "aac",
		Cookies: `ttwid=1%7Cabc; Domain=.tiktok.com; Path=/; Secure; Expires=1825261466; other=x; Domain=.example.com; Path=/; tt_chain_token="pZ0==" ; Domain=.tiktok.com; Path=/; Secure`}
	asset, err := info.asset(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := asset.Headers.Get("Cookie"); got != "ttwid=1%7Cabc; tt_chain_token=pZ0==" {
		t.Fatalf("cookie header %q", got)
	}
}

func TestRedditLinkPostShowsItsPreview(t *testing.T) {
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "www.reddit.com" {
			return response(200, "application/json", `{"access_token":"fixture-token","expires_in":3600}`), nil
		}
		return response(200, "application/json", `[{"data":{"children":[{"data":{"id":"abc","title":"An article","author":"someone","permalink":"/r/test/comments/abc/article/","url":"https://news.example/story","preview":{"images":[{"source":{"url":"https://external-preview.redd.it/story.jpg?width=640&amp;s=sig","width":640,"height":336}}]}}}]}}]`), nil
	})}
	r := NewReddit(RedditOptions{ClientID: "fixture-client", ClientSecret: "fixture-secret", HTTPClient: client})
	out, err := r.Resolve(context.Background(), "https://redd.it/abc")
	if err != nil || len(out.Assets) != 1 || out.Assets[0].URL != "https://external-preview.redd.it/story.jpg?width=640&s=sig" || out.Assets[0].Width != 640 || out.Title != "An article" {
		t.Fatalf("link post: %+v %v", out, err)
	}
}
