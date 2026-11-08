package linkmedia

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/iframely"
)

type connectorFunc func(context.Context, string) (Result, error)

func (f connectorFunc) Resolve(ctx context.Context, raw string) (Result, error) { return f(ctx, raw) }

const pngFixture = "\x89PNG\r\n\x1a\nimage fixture bytes"

func TestTikTokBlockedCDNFallsBackToOfficialPlayer(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"meta":{"title":"TikTok - Make Your Day","site":"TikTok"}}`))
	}))
	defer provider.Close()
	client, _ := iframely.New(iframely.Options{BaseURL: provider.URL})
	r := NewResolver(ResolverOptions{Iframely: client, Downloader: connectorFunc(func(context.Context, string) (Result, error) {
		return Result{Provider: "tiktok", Assets: []Asset{{URL: "https://video.example/movie.mp4", Kind: "video"}}}, nil
	}), HTTPClient: &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) { return response(403, "text/html", "blocked"), nil })}})
	out, err := r.ResolveSource(context.Background(), "https://www.tiktok.com/@creator/video/123?is_from_webapp=1")
	if err != nil || out.Embed == nil || !strings.Contains(out.Embed.HTML, `https://www.tiktok.com/player/v1/123?autoplay=0`) || len(out.Assets) != 0 {
		t.Fatalf("blocked TikTok became a homepage card: %+v %v", out, err)
	}
}

func TestResolverSourcePreferenceFallbackAndSpotify(t *testing.T) {
	lookups, probes, downloads := 0, 0, 0
	iframe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lookups++
		_, _ = w.Write([]byte(`{"meta":{"title":"Fallback"},"html":"<iframe src=\"https://provider.example/embed\"></iframe>"}`))
	}))
	defer iframe.Close()
	client, _ := iframely.New(iframely.Options{BaseURL: iframe.URL})
	httpClient := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		probes++
		if r.Header.Get("Range") != "bytes=0-65535" {
			t.Error("source probe attempted an unbounded download")
		}
		if r.URL.Path == "/blocked.png" {
			return response(200, "image/png", "<html>Please log in</html>"), nil
		}
		return response(200, "image/png", pngFixture), nil
	})}
	direct := connectorFunc(func(context.Context, string) (Result, error) {
		return Result{Provider: "reddit", Assets: []Asset{{URL: "https://i.redd.it/source.png", Kind: "image"}}}, nil
	})
	downloader := connectorFunc(func(context.Context, string) (Result, error) { downloads++; return Result{}, ErrUnavailable })
	r := NewResolver(ResolverOptions{Reddit: direct, Downloader: downloader, Iframely: client, HTTPClient: httpClient})
	out, err := r.Resolve(context.Background(), "https://reddit.com/comments/abc")
	if err != nil || out.Embed != nil || len(out.Assets) != 1 || out.Assets[0].ObjectKey != "" || lookups != 0 || downloads != 0 {
		t.Fatalf("direct delivery: %+v %v", out, err)
	}
	out, err = r.Resolve(context.Background(), "https://open.spotify.com/track/example")
	if err != nil || out.Embed == nil || out.Embed.HTML == "" || lookups != 1 || downloads != 0 || probes != 1 {
		t.Fatalf("Spotify fallback: %+v %v", out, err)
	}
	r.options.Reddit = connectorFunc(func(context.Context, string) (Result, error) { return Result{}, ErrUnavailable })
	out, err = r.Resolve(context.Background(), "https://reddit.com/comments/abc")
	if err != nil || out.Embed == nil || lookups != 2 || downloads != 1 {
		t.Fatalf("unavailable post fallback: %+v %v", out, err)
	}
	out, err = r.Resolve(context.Background(), "https://source.example/blocked.png")
	if err != nil || out.Embed == nil || lookups != 3 {
		t.Fatalf("login page fallback: %+v %v", out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = r.Resolve(ctx, "https://reddit.com/comments/abc"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation did not stop fallback", err)
	}
}

func TestCDNCopyOnlyWhenSourceNeedsHeadersAndBoundsDownloads(t *testing.T) {
	uploads := 0
	var stored []byte
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected storage operation %s", r.Method)
			w.WriteHeader(400)
			return
		}
		uploads++
		stored, _ = io.ReadAll(r.Body)
		w.Header().Set("ETag", `"fixture"`)
	}))
	defer storage.Close()
	cdnClient, err := cdn.New(cdn.Options{Endpoint: storage.URL, Bucket: "fixture", Region: "us-east-1", Prefix: "media", PublicURL: "https://cdn.example.test", AccessKey: "test-access", SecretKey: "test-secret", Transport: storage.Client().Transport})
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/blocked.png" && r.Header.Get("Referer") == "" {
			return response(403, "text/plain", "blocked"), nil
		}
		return response(200, "image/png", pngFixture), nil
	})}
	r := NewResolver(ResolverOptions{CDN: cdnClient, HTTPClient: client, MaxFileBytes: 1024, Reddit: connectorFunc(func(context.Context, string) (Result, error) {
		return Result{Provider: "reddit", Assets: []Asset{{URL: "https://source.example/public.png", Kind: "image"}}}, nil
	})})
	public, err := r.Resolve(context.Background(), "https://reddit.com/comments/abc")
	if err != nil || public.Assets[0].URL != "https://source.example/public.png" || uploads != 0 {
		t.Fatalf("public source copied unnecessarily: %+v %v", public, err)
	}
	r.options.Instagram = connectorFunc(func(context.Context, string) (Result, error) {
		return Result{Provider: "instagram", Assets: []Asset{{URL: "https://source.example/blocked.png", Kind: "image", Headers: http.Header{"Referer": {"https://instagram.com/"}}}}}, nil
	})
	out, err := r.Resolve(context.Background(), "https://instagram.com/p/Photo/")
	if err != nil || len(out.Assets) != 1 || out.Assets[0].ObjectKey == "" || !strings.HasPrefix(out.Assets[0].URL, "https://cdn.example.test/media/") || out.Assets[0].Headers != nil || string(stored) != pngFixture || uploads != 1 {
		t.Fatalf("header-dependent source: %+v %v uploads=%d bytes=%d", out, err, uploads, len(stored))
	}
	r.options.MaxFileBytes = 4
	if _, err := r.Store(context.Background(), Asset{URL: "https://source.example/public.png", Kind: "image"}); !errors.Is(err, ErrLimit) || uploads != 1 {
		t.Fatalf("oversized download uploaded: %v", err)
	}
	// Read sizes are kept distinct from partial response lengths.
	r.http = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		res := response(206, "image/png", pngFixture)
		res.Header.Set("Content-Range", "bytes 0-"+strconv.Itoa(len(pngFixture)-1)+"/9000")
		return res, nil
	})}
	_, size, err := r.probe(context.Background(), "https://source.example/public.png")
	if err != nil || size != 9000 {
		t.Fatalf("partial size: %d %v", size, err)
	}
}

func TestStoreStripsSourceCredentialsOnRedirect(t *testing.T) {
	// No storage request should occur: the destination answers with a login page.
	c, _ := cdn.New(cdn.Options{Endpoint: "https://storage.example.test", Bucket: "fixture", Region: "us-east-1", PublicURL: "https://cdn.example.test"})
	requests := 0
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			res := response(302, "text/plain", "")
			res.Header.Set("Location", "https://other.example.test/file.png")
			return res, nil
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Referer") != "" {
			t.Error("source headers crossed origins")
		}
		return response(200, "text/html", "<html>Log in</html>"), nil
	})}
	r := NewResolver(ResolverOptions{CDN: c, HTTPClient: client})
	_, err := r.Store(context.Background(), Asset{URL: "https://source.example.test/file.png", Kind: "image", Headers: http.Header{"Authorization": {"Bearer test-secret"}, "Cookie": {"test-secret"}, "Referer": {"https://source.example.test/"}}})
	if !errors.Is(err, ErrUnavailable) || requests != 2 {
		t.Fatalf("redirect: requests=%d err=%v", requests, err)
	}
}

func TestFailedGalleryCleansUpOnlyItsUploads(t *testing.T) {
	puts, deletes := 0, 0
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			puts++
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("ETag", `"fixture"`)
			return
		}
		if r.Method == http.MethodDelete {
			deletes++
			w.WriteHeader(204)
			return
		}
		w.WriteHeader(400)
	}))
	defer storage.Close()
	c, _ := cdn.New(cdn.Options{Endpoint: storage.URL, Bucket: "fixture", Region: "us-east-1", Prefix: "media", PublicURL: "https://cdn.example.test", Transport: storage.Client().Transport})
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/missing.png" {
			return response(404, "text/plain", "gone"), nil
		}
		if r.Header.Get("Referer") == "" {
			return response(403, "text/plain", "blocked"), nil
		}
		return response(200, "image/png", pngFixture), nil
	})}
	r := NewResolver(ResolverOptions{CDN: c, HTTPClient: client})
	_, err := r.deliverAll(context.Background(), Result{Assets: []Asset{
		{URL: "https://source.example.test/first.png", Kind: "image", Headers: http.Header{"Referer": {"https://instagram.com/"}}},
		{URL: "https://source.example.test/missing.png", Kind: "image"},
	}})
	if err == nil || puts != 1 || deletes != 1 {
		t.Fatalf("failed gallery cleanup: uploads=%d deletes=%d err=%v", puts, deletes, err)
	}
}

func TestNonMediaLinksStayWidgets(t *testing.T) {
	widgets := 0
	iframe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		widgets++
		_, _ = w.Write([]byte(`{"meta":{"title":"Widget"},"html":"<iframe></iframe>","links":{"image":[{"href":"https://source.example/original.png","type":"image/png"}],"player":[{"href":"https://source.example/original.mp4","type":"video/mp4"}]}}`))
	}))
	defer iframe.Close()
	client, _ := iframely.New(iframely.Options{BaseURL: iframe.URL})
	unexpected := connectorFunc(func(context.Context, string) (Result, error) {
		t.Error("generic link reached custom extraction")
		return Result{}, ErrUnsupported
	})
	r := NewResolver(ResolverOptions{Reddit: unexpected, Instagram: unexpected, Downloader: unexpected, Iframely: client, HTTPClient: &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		t.Error("widget link triggered media fetching")
		return nil, ErrUnavailable
	})}})
	for _, raw := range []string{"https://open.spotify.com/track/example", "https://example.com/article"} {
		out, err := r.Resolve(context.Background(), raw)
		if err != nil || out.Embed == nil || out.Embed.HTML == "" || len(out.Assets) != 0 {
			t.Fatalf("generic link did not stay a widget: %+v %v", out, err)
		}
	}
	if widgets != 2 {
		t.Fatalf("widget calls: %d", widgets)
	}
}

func TestDirectFilesUseBoundedSourcePreview(t *testing.T) {
	gif, _ := base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==")
	for _, tc := range []struct{ path, mime, body, kind string }{
		{"animation.gif?signature=kept", "image/gif", string(gif), "image"},
		{"photo.png", "image/png", pngFixture, "image"},
		{"movie.mp4", "video/mp4", strings.Repeat("\x00", 64), "video"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			calls := 0
			resolver := NewResolver(ResolverOptions{HTTPClient: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Header.Get("Range") != "bytes=0-65535" {
					t.Error("unbounded file fetch")
				}
				return response(200, tc.mime, tc.body), nil
			})}})
			raw := "https://media.example/" + tc.path
			out, err := resolver.ResolveSource(context.Background(), raw)
			if err != nil || out.Provider != "media" || out.Embed != nil || len(out.Assets) != 1 || calls != 1 {
				t.Fatalf("direct file failed: calls=%d error=%v", calls, err)
			}
			asset := out.Assets[0]
			if asset.URL != raw || asset.Kind != tc.kind || asset.ContentType != tc.mime || asset.ObjectKey != "" {
				t.Fatal("source URL or media type lost")
			}
			if tc.mime == "image/gif" && (asset.Width != 1 || asset.Height != 1) {
				t.Fatal("GIF dimensions not reserved")
			}
		})
	}
}
