package linkmedia

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/storage"
)

const redditGIFPostURL = "https://www.reddit.com/r/AsianCuties/comments/1xs482i/bay_area_babe/"
const redditGIFPreviewURL = "https://v.redd.it/gb1r0e5jmn0i1/CMAF_480.mp4"

// Distilled public metadata for 1xs482i, inspected on 2026-11-10. Both url and
// url_overridden_by_dest point to redgifs.com/watch/scalysomearabianoryx; there
// is no crosspost parent. media/secure_media contain an iframe and poster, but
// preview.reddit_video_preview already offers this completed native MP4.
const redditGIFPostFixture = `{
	"id":"1xs482i","title":"Public GIF post","author":"fixture-author",
	"url":"https://www.redgifs.com/watch/scalysomearabianoryx",
	"url_overridden_by_dest":"https://www.redgifs.com/watch/scalysomearabianoryx",
	"is_video":false,"is_self":false,"over_18":true,
	"secure_media":{"type":"redgifs.com","oembed":{"type":"video","html":"<iframe src=\"https://www.redgifs.com/ifr/scalysomearabianoryx\"></iframe>","thumbnail_url":"https://media.redgifs.com/ScalySomeArabianoryx-poster.jpg"}},
	"preview":{"enabled":false,"images":[{"source":{"url":"https://external-preview.redd.it/fixture.jpeg","width":480,"height":854}}],
		"reddit_video_preview":{"fallback_url":"https://v.redd.it/gb1r0e5jmn0i1/CMAF_480.mp4","has_audio":false,"height":854,"width":480,"duration":13,"is_gif":true,"transcoding_status":"completed"}}
}`

func redditGIFTestPost(t *testing.T) redditPost {
	t.Helper()
	var post redditPost
	if err := json.Unmarshal([]byte(redditGIFPostFixture), &post); err != nil {
		t.Fatal(err)
	}
	return post
}

func redditPostConnector(t *testing.T, post redditPost) *Reddit {
	t.Helper()
	encoded, err := json.Marshal(post)
	if err != nil {
		t.Fatal(err)
	}
	return NewReddit(RedditOptions{ClientID: "fixture-client", ClientSecret: "fixture-secret", HTTPClient: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "www.reddit.com":
			return response(http.StatusOK, "application/json", `{"access_token":"fixture-token","expires_in":3600}`), nil
		case "oauth.reddit.com":
			return response(http.StatusOK, "application/json", `[{"data":{"children":[{"data":`+string(encoded)+`}]}}]`), nil
		default:
			t.Errorf("connector fetched an unrelated source: %s", req.URL.Redacted())
			return nil, ErrUnavailable
		}
	})}})
}

func newRedditPreviewTestResolver(t *testing.T) (*Resolver, *int) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	uploads := new(int)
	objects := map[string]struct {
		size int64
		mime string
	}{}
	client, err := cdn.New(cdn.Options{Endpoint: "https://storage.example.test", Bucket: "fixture", Region: "us-east-1", PublicURL: "https://cdn.example.test", Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		res := response(http.StatusOK, "application/xml", "")
		res.Header.Set("ETag", `"fixture"`)
		switch req.Method {
		case http.MethodPut:
			*uploads++
			size, err := io.Copy(io.Discard, req.Body)
			if err != nil {
				t.Error(err)
			}
			objects[req.URL.Path] = struct {
				size int64
				mime string
			}{size, req.Header.Get("Content-Type")}
		case http.MethodHead:
			object, ok := objects[req.URL.Path]
			if !ok {
				return response(http.StatusNotFound, "application/xml", ""), nil
			}
			res.ContentLength = object.size
			res.Header.Set("Content-Length", strconv.FormatInt(object.size, 10))
			res.Header.Set("Content-Type", object.mime)
			res.Header.Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		default:
			t.Errorf("unexpected storage method: %s", req.Method)
		}
		return res, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	r := NewResolver(ResolverOptions{CDN: client})
	r.persistent = &publicCache{db: store.DB, namespace: client.Namespace()}
	return r, uploads
}

func TestRedditGIFPreviewPrecedesStillImage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*redditPost)
		kind   string
	}{
		{name: "public-redgifs", kind: "video"},
		{name: "ordinary-still", change: func(p *redditPost) { p.Preview.Video = redditVideoPreview{} }, kind: "image"},
		{name: "incomplete", change: func(p *redditPost) { p.Preview.Video.Status = "processing" }, kind: "image"},
		{name: "not-a-gif", change: func(p *redditPost) { p.Preview.Video.IsGIF = false }, kind: "image"},
		{name: "has-audio", change: func(p *redditPost) { audio := true; p.Preview.Video.HasAudio = &audio }, kind: "image"},
		{name: "unknown-audio", change: func(p *redditPost) { p.Preview.Video.HasAudio = nil }, kind: "image"},
		{name: "unknown-width", change: func(p *redditPost) { p.Preview.Video.Width = 0 }, kind: "image"},
		{name: "excessive-height", change: func(p *redditPost) { p.Preview.Video.Height = maxImageDimension + 1 }, kind: "image"},
		{name: "unknown-duration", change: func(p *redditPost) { p.Preview.Video.Duration = 0 }, kind: "image"},
		{name: "manifest", change: func(p *redditPost) { p.Preview.Video.URL = "https://v.redd.it/gb1r0e5jmn0i1/HLSPlaylist.m3u8" }, kind: "image"},
		{name: "external-mp4", change: func(p *redditPost) { p.Preview.Video.URL = "https://media.redgifs.com/ScalySomeArabianoryx.mp4" }, kind: "image"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			post := redditGIFTestPost(t)
			if tc.change != nil {
				tc.change(&post)
			}
			raw := redditGIFPostURL + "?utm_source=share"
			value, err := redditPostConnector(t, post).Resolve(context.Background(), raw)
			if err != nil || len(value.Assets) != 1 || value.Assets[0].Kind != tc.kind || value.URL != raw || value.Provider != "reddit" || value.ID != "1xs482i" {
				t.Fatalf("wrong native preview selection: %+v %v", value, err)
			}
			asset := value.Assets[0]
			if tc.kind == "video" && (asset.URL != redditGIFPreviewURL || asset.Width != 480 || asset.Height != 854 || asset.Duration != 13 || asset.ContentType != "video/mp4") {
				t.Fatalf("motion preview lost its source or metadata: %+v", asset)
			}
			if tc.kind == "image" && asset.URL != "https://external-preview.redd.it/fixture.jpeg" {
				t.Fatal("ordinary still fallback changed", asset.URL)
			}
		})
	}
	post := redditGIFTestPost(t)
	post.IsVideo = true
	value, err := redditPostConnector(t, post).Resolve(context.Background(), redditGIFPostURL)
	if !errors.Is(err, ErrUnavailable) || len(value.Assets) != 0 {
		t.Fatal("a native Reddit video became a silent preview", err)
	}
}

func TestRedditPreviewVideoURLIsNarrow(t *testing.T) {
	for _, raw := range []string{redditGIFPreviewURL, "https://v.redd.it/abc/DASH_480.mp4?source=fallback"} {
		if !redditPreviewVideoURL(raw) || redditMediaURL(raw) {
			t.Fatalf("video source unsupported or promoted to image: %s", raw)
		}
	}
	for _, raw := range []string{
		"http://v.redd.it/abc/CMAF_480.mp4", "https://v.redd.it.evil.test/abc/CMAF_480.mp4",
		"https://user:secret@v.redd.it/abc/CMAF_480.mp4", "https://v.redd.it:444/abc/CMAF_480.mp4",
		"https://127.0.0.1/abc/CMAF_480.mp4", "https://v.redd.it/abc/CMAF_480.mp4?token=secret",
		"https://v.redd.it/abc/CMAF_480.mp4?source=fallback&source=other", "https://v.redd.it/abc/CMAF_480.mp4#fragment",
		"https://v.redd.it/abc%2fother/CMAF_480.mp4", "https://v.redd.it/abc/../CMAF_480.mp4",
		"https://v.redd.it/abc/movie.mp4", "https://v.redd.it/abc/HLSPlaylist.m3u8",
	} {
		if redditPreviewVideoURL(raw) {
			t.Errorf("unreviewed video source accepted: %s", raw)
		}
	}
}

func TestRedditGIFPreviewIngestAndCacheEpoch(t *testing.T) {
	r, uploads := newRedditPreviewTestResolver(t)
	r.options.Reddit = redditPostConnector(t, redditGIFTestPost(t))
	data := nativeFixtureBytes(t, "video")
	fetches := 0
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		fetches++
		if req.URL.String() != redditGIFPreviewURL || req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
			t.Error("preview changed source or inherited credentials")
		}
		return response(http.StatusOK, "application/octet-stream", string(data)), nil
	})}
	// Keep a previous card and negative result under the old epoch. Only
	// this provider's version changes; unrelated social/cache keys are stable.
	legacyHash := sha256.Sum256([]byte("native-reddit-v2\x00" + redditGIFPostURL))
	legacyKey := "native-reddit-v2:" + hex.EncodeToString(legacyHash[:])
	legacy := infoCard(redditGIFPostURL, "Previous still preview", "", "", time.Hour)
	encoded, _ := json.Marshal(legacy)
	if _, err := r.persistent.db.Exec(`INSERT INTO public_media_results(namespace,cache_key,payload,expires_ns) VALUES(?,?,?,?)`, r.persistent.namespace, legacyKey, encoded, time.Now().Add(time.Hour).UnixNano()); err != nil {
		t.Fatal(err)
	}
	settled := make(chan struct{})
	close(settled)
	r.cache.entries = map[string]*resolutionEntry{"native-reddit-v2:" + redditGIFPostURL: {value: legacy, err: ErrUnavailable, expires: time.Now().Add(time.Hour), done: settled}}
	value, err := r.ResolveSource(context.Background(), redditGIFPostURL)
	if err != nil || len(value.Assets) != 1 || value.URL != redditGIFPostURL || *uploads != 1 || fetches != 1 || value.Preview != nil {
		t.Fatalf("native preview was not ingested once: %+v uploads=%d fetches=%d error=%v", value, *uploads, fetches, err)
	}
	asset := value.Assets[0]
	if !r.IsDeliveryURL(asset.URL) || asset.SourceURL != redditGIFPreviewURL || asset.Kind != "video" || asset.ContentType != "video/mp4" || asset.Width != 480 || asset.Height != 854 || asset.Duration != 13 || asset.Size != int64(len(data)) || asset.SHA256 == "" || time.Until(asset.ExpiresAt) < 89*24*time.Hour {
		t.Fatalf("preview lost validation, metadata or 90-day lifecycle: %+v", asset)
	}
	// A new resolver simulates a process restart and must reuse the durable
	// video manifest instead of the old card, without fetching media again.
	fresh := NewResolver(r.options)
	fresh.persistent = r.persistent
	reused, err := fresh.ResolveSource(context.Background(), redditGIFPostURL)
	if err != nil || len(reused.Assets) != 1 || reused.Assets[0].ObjectKey != asset.ObjectKey || reused.Assets[0].Duration != 13 || *uploads != 1 || fetches != 1 {
		t.Fatal("new epoch did not retain the native result", err)
	}
	var rows int
	if err := r.persistent.db.QueryRow(`SELECT count(*) FROM public_media_results`).Scan(&rows); err != nil || rows != 2 || !strings.HasPrefix(publicResultKey(redditGIFPostURL), "native-reddit-v3:") || resolutionVersion("https://www.instagram.com/p/Fixture/") != "native-instagram-v2" {
		t.Fatalf("cache migration changed unrelated entries or epochs: rows=%d error=%v", rows, err)
	}
}

func TestRedditGIFPreviewRejectsInvalidMedia(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		limit      int64
		redirect   string
	}{
		{name: "html", body: "<html><body>Log in</body></html>", status: http.StatusOK},
		{name: "image-disguised-as-video", body: string(nativeFixtureBytes(t, "image")), status: http.StatusOK},
		{name: "restricted", status: http.StatusForbidden},
		{name: "too-large", body: string(nativeFixtureBytes(t, "video")), status: http.StatusOK, limit: 16},
		{name: "private-redirect", status: http.StatusFound, redirect: "http://127.0.0.1/private.mp4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, uploads := newRedditPreviewTestResolver(t)
			r.options.Reddit = redditPostConnector(t, redditGIFTestPost(t))
			if tc.limit > 0 {
				r.options.MaxFileBytes = tc.limit
			}
			// Keep production redirect validation while replacing network I/O.
			r.http.Transport = roundTrip(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() == redditGIFPreviewURL {
					res := response(tc.status, "video/mp4", tc.body)
					res.Request = req
					if tc.redirect != "" {
						res.Header.Set("Location", tc.redirect)
					}
					return res, nil
				}
				if req.URL.Host == "127.0.0.1" {
					t.Error("private media redirect was fetched")
					return nil, ErrRestricted
				}
				res := response(http.StatusOK, "text/html", "<html><head><title>Public post</title></head></html>")
				res.Request = req
				return res, nil
			})
			value, err := r.ResolveSource(context.Background(), redditGIFPostURL)
			if len(value.Assets) != 0 || *uploads != 0 || value.URL != redditGIFPostURL {
				t.Fatalf("unaccepted video became native media: %+v uploads=%d error=%v", value, *uploads, err)
			}
		})
	}
}

func TestLiveRedditGIFPreviewDelivery(t *testing.T) {
	if os.Getenv("IRCORD_TEST_REDDIT_GIF_PREVIEW") != "true" {
		t.Skip("set IRCORD_TEST_REDDIT_GIF_PREVIEW=true for one anonymous MP4 fetch with disposable storage")
	}
	r, uploads := newRedditPreviewTestResolver(t)
	// Source metadata is the distilled public fixture above; only its MP4 is
	// fetched live, through the production public-only transport and ingest.
	r.options.Reddit = redditPostConnector(t, redditGIFTestPost(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	value, err := r.ResolveSource(ctx, redditGIFPostURL)
	if err != nil || len(value.Assets) != 1 || *uploads != 1 {
		t.Fatalf("public preview ingest failed: %v", err)
	}
	asset := value.Assets[0]
	if asset.Kind != "video" || asset.ContentType != "video/mp4" || asset.Size <= sniffBytes || asset.Size > r.options.MaxFileBytes || !r.IsDeliveryURL(asset.URL) || asset.SourceURL != redditGIFPreviewURL {
		t.Fatal("public source was not validated as native video")
	}
	t.Logf("source=%s observedMIME=%s bytes=%d metadataDimensions=%dx%d metadataDuration=%gs retainedURL=%s", asset.SourceURL, asset.ContentType, asset.Size, asset.Width, asset.Height, asset.Duration, value.URL)
}
