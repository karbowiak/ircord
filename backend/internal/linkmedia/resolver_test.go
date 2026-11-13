package linkmedia

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/storage"
)

type connectorFunc func(context.Context, string) (Result, error)

func (f connectorFunc) Resolve(ctx context.Context, raw string) (Result, error) { return f(ctx, raw) }

const pngFixture = "\x89PNG\r\n\x1a\nimage fixture bytes"

// noSourceNetwork fails every source request, so a test cannot reach the
// internet even when the machine running it has a network.
func noSourceNetwork(t *testing.T) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		t.Errorf("unexpected source request: %s", req.URL.Redacted())
		return nil, ErrUnavailable
	})}
}

func decodablePNG(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

type storedTestCounts struct{ puts, deletes int }

// newStoredTestResolver attaches an isolated object store and durable cache.
// Source requests fail until the test installs its own transport.
func newStoredTestResolver(t *testing.T, maxFileBytes int64) (*Resolver, *storedTestCounts) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	counts := &storedTestCounts{}
	client, err := cdn.New(cdn.Options{Endpoint: "https://storage.example.test", Bucket: "fixture", Region: "us-east-1", Prefix: "media", PublicURL: "https://cdn.example.test", AccessKey: "test-access", SecretKey: "test-secret", Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodPut:
			counts.puts++
			_, _ = io.Copy(io.Discard, req.Body)
			res := response(http.StatusOK, "application/xml", "")
			res.Header.Set("ETag", `"fixture"`)
			return res, nil
		case http.MethodDelete:
			counts.deletes++
			return response(http.StatusNoContent, "application/xml", ""), nil
		}
		t.Errorf("unexpected storage method: %s", req.Method)
		return response(http.StatusBadRequest, "application/xml", ""), nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	r := NewResolver(ResolverOptions{CDN: client, MaxFileBytes: maxFileBytes})
	r.persistent = &publicCache{db: store.DB, namespace: client.Namespace()}
	r.http = noSourceNetwork(t)
	return r, counts
}

func TestRedditFailureFallsBackToShortLivedPageCard(t *testing.T) {
	r := NewResolver(ResolverOptions{Reddit: connectorFunc(func(context.Context, string) (Result, error) {
		return Result{}, ErrUnavailable
	})})
	// The page answers with a normal card, which alone would be kept for a day,
	// or fails, which alone would be retried soon. Both must be retried soon.
	for _, pageAvailable := range []bool{true, false} {
		pages := 0
		r.cache = &resolutionCache{}
		r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			pages++
			if req.URL.Host != "reddit.com" {
				t.Errorf("fallback fetched another source: %s", req.URL.Redacted())
				return nil, ErrUnavailable
			}
			if !pageAvailable {
				return nil, ErrUnavailable
			}
			res := response(http.StatusOK, "text/html; charset=utf-8", `<html><head><title>A post</title><meta property="og:description" content="Summary"></head></html>`)
			res.Request = req
			return res, nil
		})}
		out, err := r.Resolve(context.Background(), "https://reddit.com/comments/abc")
		if err != nil || out.Preview == nil || len(out.Assets) != 0 || out.Embed != nil || pages != 1 {
			t.Fatalf("unavailable post fallback (page available: %t): %+v pages=%d %v", pageAvailable, out, pages, err)
		}
		if pageAvailable && out.Title != "A post" {
			t.Fatalf("fallback lost the page metadata: %+v", out)
		}
		// A temporary native failure must not hide behind a day-long generic card.
		if time.Until(out.CacheUntil) > time.Minute {
			t.Fatalf("fallback (page available: %t) cached until %v", pageAvailable, out.CacheUntil)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Resolve(ctx, "https://reddit.com/comments/other"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation did not stop fallback", err)
	}
}

func TestRedditSourceMediaNeedsValidatedStorage(t *testing.T) {
	// Without object storage, a source asset is never returned for direct use.
	r := NewResolver(ResolverOptions{Reddit: connectorFunc(func(context.Context, string) (Result, error) {
		return Result{Provider: "reddit", ID: "abc", Title: "Photos", Assets: []Asset{{URL: "https://i.redd.it/source.png", Kind: "image"}}}, nil
	})})
	png := decodablePNG(t)
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "i.redd.it" {
			return response(http.StatusOK, "image/png", png), nil
		}
		return nil, ErrUnavailable
	})}
	out, err := r.Resolve(context.Background(), "https://reddit.com/comments/abc")
	if err != nil || len(out.Assets) != 0 || out.Embed != nil || out.Title != "Photos" || out.Provider != "reddit" || out.ID != "abc" || time.Until(out.CacheUntil) > time.Minute {
		t.Fatalf("unstored Reddit media: %+v %v", out, err)
	}
}

func TestStoreBoundsDownloadsBeforeUpload(t *testing.T) {
	png := decodablePNG(t)
	r, counts := newStoredTestResolver(t, 1024)
	r.http = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, "image/png", png), nil
	})}
	stored, err := r.Store(context.Background(), Asset{URL: "https://source.example/public.png", Kind: "image"})
	if err != nil || stored.ObjectKey == "" || !r.IsDeliveryURL(stored.URL) || stored.SourceURL != "https://source.example/public.png" || counts.puts != 1 {
		t.Fatalf("validated copy: %+v puts=%d %v", stored, counts.puts, err)
	}
	// Large enough to identify the PNG signature, smaller than the file.
	r.options.MaxFileBytes = 20
	if _, err := r.Store(context.Background(), Asset{URL: "https://source.example/public.png", Kind: "image"}); !errors.Is(err, ErrLimit) || counts.puts != 1 {
		t.Fatalf("oversized download uploaded: puts=%d %v", counts.puts, err)
	}
}

func TestStoreStripsSourceCredentialsOnRedirect(t *testing.T) {
	r, counts := newStoredTestResolver(t, 1024)
	requests := 0
	r.http = nativeHTTPClient()
	r.http.Transport = roundTrip(func(req *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			res := response(http.StatusFound, "text/plain", "")
			res.Header.Set("Location", "https://other.example.test/file.png")
			return res, nil
		}
		if req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" || req.Header.Get("Referer") != "" {
			t.Error("source headers crossed origins")
		}
		return response(http.StatusOK, "text/html", "<html>Log in</html>"), nil
	})
	_, err := r.Store(context.Background(), Asset{URL: "https://source.example.test/file.png", Kind: "image", Headers: http.Header{"Authorization": {"Bearer test-secret"}, "Cookie": {"test-secret"}, "Referer": {"https://source.example.test/"}}})
	if err == nil || requests != 2 || counts.puts != 0 {
		t.Fatalf("redirect: requests=%d puts=%d err=%v", requests, counts.puts, err)
	}
}

func TestFailedGalleryCleansUpOnlyItsUploads(t *testing.T) {
	png := decodablePNG(t)
	r, counts := newStoredTestResolver(t, 1024)
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/missing.png" {
			return response(http.StatusNotFound, "text/plain", "gone"), nil
		}
		return response(http.StatusOK, "image/png", png), nil
	})}
	_, err := r.deliverAll(context.Background(), Result{Assets: []Asset{
		{URL: "https://source.example.test/first.png", Kind: "image"},
		{URL: "https://source.example.test/missing.png", Kind: "image"},
	}})
	if err == nil || counts.puts != 1 || counts.deletes != 1 {
		t.Fatalf("failed gallery cleanup: uploads=%d deletes=%d err=%v", counts.puts, counts.deletes, err)
	}
	var rows int
	if err := r.persistent.db.QueryRow(`SELECT count(*) FROM public_media_objects`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("discarded upload still tracked: rows=%d err=%v", rows, err)
	}
}

func TestGenericLinksNeverReachSocialConnectors(t *testing.T) {
	unexpected := connectorFunc(func(context.Context, string) (Result, error) {
		t.Error("generic link reached custom extraction")
		return Result{}, ErrUnsupported
	})
	r := NewResolver(ResolverOptions{Reddit: unexpected, Instagram: unexpected, Downloader: unexpected})
	pages := 0
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		pages++
		if req.URL.Path != "/article" {
			t.Errorf("page link fetched another source: %s", req.URL.Redacted())
			return nil, ErrUnavailable
		}
		res := response(http.StatusOK, "text/html; charset=utf-8", `<html><head><title>An article</title><meta property="og:description" content="Summary"></head></html>`)
		res.Request = req
		return res, nil
	})}
	out, err := r.Resolve(context.Background(), "https://example.com/article")
	if err != nil || out.Provider != "page" || out.Embed != nil || len(out.Assets) != 0 || out.Preview == nil || out.Title != "An article" || pages != 1 {
		t.Fatalf("generic link: %+v pages=%d %v", out, pages, err)
	}
}

func TestDirectFilesPublishValidatedCopies(t *testing.T) {
	gif, _ := base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==")
	for _, tc := range []struct{ path, mime, body string }{
		{"animation.gif?signature=kept", "image/gif", string(gif)},
		{"photo.png", "image/png", decodablePNG(t)},
	} {
		t.Run(tc.path, func(t *testing.T) {
			raw := "https://media.example/" + tc.path
			r, counts := newStoredTestResolver(t, 1<<20)
			fetches := 0
			r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				fetches++
				if req.URL.String() != raw {
					t.Errorf("unexpected source request: %s", req.URL.Redacted())
				}
				// Identification must use the bytes, not the upstream MIME header.
				return response(http.StatusOK, "application/octet-stream", tc.body), nil
			})}
			out, err := r.ResolveSource(context.Background(), raw)
			if err != nil || out.Provider != "media" || out.Embed != nil || len(out.Assets) != 1 || fetches != 1 || counts.puts != 1 {
				t.Fatalf("direct file failed: %+v fetches=%d puts=%d error=%v", out, fetches, counts.puts, err)
			}
			asset := out.Assets[0]
			if asset.Kind != "image" || asset.ContentType != tc.mime || asset.ObjectKey == "" || !r.IsDeliveryURL(asset.URL) || asset.SourceURL != raw {
				t.Fatalf("validated copy or media type lost: %+v", asset)
			}
			if tc.mime == "image/gif" && (asset.Width != 1 || asset.Height != 1) {
				t.Fatal("GIF dimensions not reserved")
			}
		})
	}
	// Without object storage a valid file still becomes a file card, never a
	// link to the unvalidated source.
	r := NewResolver(ResolverOptions{})
	r.http = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, "image/gif", string(gif)), nil
	})}
	out, err := r.ResolveSource(context.Background(), "https://media.example/no-storage.gif")
	if err != nil || out.Provider != "file" || len(out.Assets) != 0 || out.Preview == nil || out.Preview.Reason != "storage_unavailable" || out.Preview.File == nil || out.Preview.File.Asset.URL != "" {
		t.Fatalf("unstored file: %+v %v", out, err)
	}
	if !strings.HasPrefix(out.URL, "https://media.example/") {
		t.Fatalf("source URL lost: %s", out.URL)
	}
}
