package linkmedia

import (
	"context"
	"encoding/json"
	"errors"
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

func TestSocialSourcePolicyAndCanonicalCache(t *testing.T) {
	for _, fixture := range socialReviewFixtures {
		canonical, provider, id, err := socialSource(fixture.URL)
		version := "native-social-v1"
		if provider == "instagram" {
			version = "native-instagram-v2"
		}
		if err != nil || id == "" || !NativeSocialSource(fixture.URL, provider) || strings.Contains(canonical, "?") || resolutionKey(fixture.URL) != canonical || resolutionVersion(fixture.URL) != version {
			t.Fatalf("authorized public fixture lost canonical identity: %s", fixture.Name)
		}
		if NativeSocialSource(fixture.URL, "reddit") {
			t.Fatal("social source granted another provider")
		}
	}
	for _, raw := range []string{
		"https://instagram.com/accounts/login/", "https://instagram.com/profile/", "https://vm.tiktok.com/AbCd/",
		"https://www.tiktok.com/@creator", "https://instagram.com.evil.example/p/Public/",
		"https://user:pass@instagram.com/p/Public/", "https://instagram.com:444/p/Public/",
		"https://instagram.com/p/Public/?sessionid=private", "https://instagram.com/p/Public/?access_token=private",
		"https://instagram.com/p/Public/?next=http://127.0.0.1/private", "https://www.tiktok.com/@creator/video/123?key=private",
	} {
		if _, _, _, err := socialSource(raw); err == nil {
			t.Fatal("restricted or unbound social source accepted")
		}
	}
}

func newSocialTestResolver(t *testing.T, puts, deletes *int) (*Resolver, *storage.Store) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	client, err := cdn.New(cdn.Options{Endpoint: "https://storage.example.test", Bucket: "fixture", Region: "us-east-1", PublicURL: "https://cdn.example.test", Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodPut:
			*puts++
			_, _ = io.Copy(io.Discard, req.Body)
		case http.MethodDelete:
			*deletes++
		default:
			t.Errorf("unexpected storage method: %s", req.Method)
		}
		res := response(200, "application/xml", "")
		res.Header.Set("ETag", `"fixture"`)
		return res, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	r := NewResolver(ResolverOptions{CDN: client})
	r.persistent = &publicCache{db: store.DB, namespace: client.Namespace()}
	return r, store
}

func TestSocialNativeCompleteGalleryAndPersistence(t *testing.T) {
	puts, deletes := 0, 0
	r, store := newSocialTestResolver(t, &puts, &deletes)
	raw := socialReviewFixtures[1].URL
	canonical, provider, id, _ := socialSource(raw)
	calls := 0
	r.options.Downloader = connectorFunc(func(_ context.Context, requested string) (Result, error) {
		calls++
		if requested != canonical {
			t.Error("extractor received tracking or another post")
		}
		return Result{URL: canonical, Provider: provider, ID: id, Title: "Public gallery", Assets: []Asset{
			{URL: "https://scontent.cdninstagram.com/first.png?expires=1999999999&sig=synthetic-source-value", Kind: "image"},
			{URL: "https://scontent.cdninstagram.com/second.mp4?expires=1999999999&sig=synthetic-source-value", Kind: "video"},
			{URL: "https://scontent.cdninstagram.com/third.png?expires=1999999999&sig=synthetic-source-value", Kind: "image"},
		}}, nil
	})
	r.options.Instagram = connectorFunc(func(context.Context, string) (Result, error) {
		t.Fatal("successful primary unnecessarily retried")
		return Result{}, nil
	})
	order := []string{}
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		order = append(order, req.URL.Path)
		kind := "image"
		if strings.HasSuffix(req.URL.Path, ".mp4") {
			kind = "video"
		}
		res := response(200, "application/octet-stream", string(nativeFixtureBytes(t, kind)))
		res.Request = req
		return res, nil
	})}
	value, err := r.ResolveSource(context.Background(), raw)
	if err != nil || len(value.Assets) != 3 || puts != 3 || deletes != 0 || calls != 1 || value.URL != raw || strings.Join(order, ",") != "/first.png,/second.mp4,/third.png" {
		t.Fatalf("complete ordered delivery failed: assets=%d puts=%d deletes=%d calls=%d err=%v", len(value.Assets), puts, deletes, calls, err)
	}
	for _, asset := range value.Assets {
		if !r.IsDeliveryURL(asset.URL) || asset.SourceURL != "" || asset.Headers != nil || !asset.SourceExpiresAt.IsZero() || time.Until(asset.ExpiresAt) < 89*24*time.Hour {
			t.Fatal("source data leaked or 90-day delivery lost")
		}
	}
	var payload []byte
	if err := store.DB.QueryRow(`SELECT payload FROM public_media_results WHERE cache_key=?`, publicResultKey(raw)).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "cdninstagram.com") || strings.Contains(string(payload), "synthetic-source-value") {
		t.Fatal("signed source URL persisted")
	}
	second := canonical + "?utm_source=different"
	value, err = r.ResolveSource(context.Background(), second)
	if err != nil || value.URL != second || calls != 1 {
		t.Fatal("tracking cache did not preserve each response URL")
	}
}

func TestSocialGalleryFailureRollsBackAndRetainsReason(t *testing.T) {
	puts, deletes := 0, 0
	r, _ := newSocialTestResolver(t, &puts, &deletes)
	canonical, provider, id, _ := socialSource(socialReviewFixtures[1].URL)
	r.options.Downloader = connectorFunc(func(context.Context, string) (Result, error) {
		return Result{URL: canonical, Provider: provider, ID: id, Assets: []Asset{{URL: "https://scontent.cdninstagram.com/first.png", Kind: "image"}, {URL: "https://scontent.cdninstagram.com/second.png", Kind: "image"}}}, nil
	})
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		res := response(403, "text/html", "restricted")
		if req.URL.Path == "/first.png" {
			res = response(200, "image/png", string(nativeFixtureBytes(t, "image")))
		}
		res.Request = req
		return res, nil
	})}
	value, err := r.ResolveSource(context.Background(), canonical)
	if err != nil || len(value.Assets) != 0 || puts != 1 || deletes != 1 || value.Preview == nil || value.Preview.Reason != "restricted" || value.Preview.Kind != "card" {
		t.Fatalf("partial gallery or hidden failure: assets=%d puts=%d deletes=%d reason=%v err=%v", len(value.Assets), puts, deletes, value.Preview, err)
	}
	noStorage := NewResolver(ResolverOptions{Downloader: r.options.Downloader})
	noStorage.http = r.http
	value, err = noStorage.ResolveSource(context.Background(), canonical)
	if err != nil || value.Preview == nil || value.Preview.Reason != "storage_unavailable" || value.Embed != nil {
		t.Fatal("storage failure became unsupported or widget fallback")
	}
}

func TestSocialExtractorBindingAndCompleteCollections(t *testing.T) {
	base := map[string]any{"id": "123", "extractor_key": "TikTok", "url": "https://v19-webapp-prime.tiktok.com/video/fixture.mp4", "ext": "mp4", "protocol": "https", "vcodec": "h264", "acodec": "aac"}
	for _, change := range []map[string]any{
		{"id": "other"}, {"webpage_url": "https://www.tiktok.com/@creator/video/999"}, {"extractor_key": "Generic"},
		{"playlist_count": 2}, {"n_entries": 2}, {"_has_drm": true}, {"age_limit": 18}, {"url": "https://unrelated.example/video.mp4"},
	} {
		item := map[string]any{}
		for key, value := range base {
			item[key] = value
		}
		for key, value := range change {
			item[key] = value
		}
		encoded, _ := json.Marshal(item)
		if _, err := fakeDownloader(t, string(encoded)).Resolve(context.Background(), "https://www.tiktok.com/@creator/video/123"); err == nil {
			t.Fatal("mismatched or incomplete extraction accepted")
		}
	}
	y := NewYTDLP(YTDLPOptions{Timeout: time.Hour, MaxItems: 100})
	if y.options.Timeout != 45*time.Second || y.options.MaxItems != 30 || cap(y.slots) != 2 {
		t.Fatal("extractor hard limits lost")
	}
	info := ytInfo{URL: "https://v19-webapp-prime.tiktok.com/video/hevc.mp4", Ext: "mp4", VCodec: "hevc", ACodec: "aac", Formats: []ytFormat{{URL: "https://v19-webapp-prime.tiktok.com/video/avc.mp4", Ext: "mp4", VCodec: "avc1", ACodec: "aac"}}}
	asset, err := info.asset(nil)
	if err != nil || !strings.HasSuffix(asset.URL, "/avc.mp4") {
		t.Fatal("H264 was not preferred when available")
	}
	videoOnly := `{"id":"Gallery","extractor_key":"Instagram","entries":[{"id":"Child","ext":"mp4","url":"https://scontent.cdninstagram.com/video.mp4","vcodec":"h264","acodec":"aac"}]}`
	if _, err := fakeDownloader(t, videoOnly).Resolve(context.Background(), "https://www.instagram.com/p/Gallery/"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("possibly filtered GraphSidecar videos were claimed as a full gallery")
	}
	large := Result{Assets: make([]Asset, maxNativeAssets+1)}
	if !errors.Is(validateSocialResult(large, "instagram", "Gallery"), ErrLimit) {
		t.Fatal("oversized gallery lost its limit reason")
	}
}

func TestSocialAnonymousHeadersAreOriginBound(t *testing.T) {
	info := ytInfo{ExtractorKey: "TikTok", URL: "https://v19-webapp-prime.tiktok.com/video/fixture.mp4", Ext: "mp4", VCodec: "h264", ACodec: "aac", Headers: map[string]string{"User-Agent": "PublicFixtureAgent", "Referer": "https://www.tiktok.com/", "Authorization": "private-auth"}, Cookies: `ttwid=public-session; Domain=.tiktok.com; Path=/; sessionid=private-account; Domain=.tiktok.com; Path=/`}
	asset, err := info.asset(nil)
	if err != nil {
		t.Fatal(err)
	}
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(asset.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: "account", Value: "private-jar"}})
	requests := 0
	r := NewResolver(ResolverOptions{})
	r.http = &http.Client{Jar: jar, Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Header.Get("Authorization") != "" || strings.Contains(req.Header.Get("Cookie"), "private") {
			t.Fatal("account authority forwarded")
		}
		res := response(200, "video/mp4", string(nativeFixtureBytes(t, "video")))
		if requests == 1 {
			if req.Header.Get("Cookie") != "ttwid=public-session" || req.Header.Get("User-Agent") != "PublicFixtureAgent" {
				t.Fatal("reviewed anonymous delivery headers missing")
			}
			res.StatusCode = http.StatusFound
			res.Header.Set("Location", "https://v20-webapp-prime.tiktok.com/video/fixture.mp4")
		} else if req.Header.Get("Cookie") != "" || req.Header.Get("User-Agent") != "" || req.Header.Get("Referer") != "" {
			t.Fatal("extractor authority crossed origins")
		}
		res.Request = req
		return res, nil
	})}
	res, err := r.fetchExtractionAsset(context.Background(), asset)
	if err != nil || requests != 2 {
		t.Fatalf("bounded redirect failed: %v", err)
	}
	res.Body.Close()
	if cookieHeader("ttwid=value-without-domain", asset.URL) != "" || cookieHeader(`ttwid=value; Domain=.tiktok.com`, "https://unrelated.example/media.mp4") != "" {
		t.Fatal("cookie escaped explicit media domain")
	}
}

func TestSocialInstagramPublicEmbedSecondary(t *testing.T) {
	for _, tc := range []struct {
		name, final, body string
		want              bool
	}{
		{"public-wrapper", "https://www.instagram.com/p/Photo/embed/captioned/", `<html><script>window.__additionalDataLoaded('extra', {"graphql":{"shortcode_media":{"shortcode":"Photo","__typename":"GraphImage","display_url":"https://scontent.cdninstagram.com/photo.jpg"}}});</script></html>`, true},
		{"wrong-post", "https://www.instagram.com/p/Other/embed/captioned/", `<html></html>`, false},
		{"login", "https://www.instagram.com/accounts/login/", `<html></html>`, false},
		{"partial-sidecar", "https://www.instagram.com/p/Photo/embed/captioned/", `{"shortcode":"Photo","__typename":"GraphSidecar","display_url":"https://scontent.cdninstagram.com/teaser.jpg"}`, false},
		{"partial-edges", "https://www.instagram.com/p/Photo/embed/captioned/", `{"shortcode":"Photo","__typename":"GraphSidecar","edge_sidecar_to_children":{"edges":[{"node":{"__typename":"GraphImage","display_url":"https://scontent.cdninstagram.com/first.jpg"}},null]}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			connector := NewInstagram(&http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://www.instagram.com/p/Photo/embed/captioned/" || req.Header.Get("Cookie") != "" {
					t.Fatal("secondary fetched a login/main page or account session")
				}
				res := response(200, "text/html", tc.body)
				final, _ := url.Parse(tc.final)
				res.Request = &http.Request{URL: final}
				return res, nil
			})})
			value, err := connector.Resolve(context.Background(), "https://www.instagram.com/reel/Photo/?utm_source=public")
			if (err == nil) != tc.want || tc.want && (value.ID != "Photo" || len(value.Assets) != 1) {
				t.Fatalf("public embed binding/completeness failed: %v", err)
			}
		})
	}
}

func TestSocialRestrictionsKeepHonestCards(t *testing.T) {
	r := NewResolver(ResolverOptions{Downloader: connectorFunc(func(context.Context, string) (Result, error) { return Result{}, ErrRestricted }), Instagram: connectorFunc(func(context.Context, string) (Result, error) { return Result{}, ErrUnavailable })})
	value, err := r.ResolveSource(context.Background(), socialReviewFixtures[0].URL)
	if err != nil || value.Preview == nil || value.Preview.Reason != "restricted" || value.Embed != nil || len(value.Assets) != 0 {
		t.Fatal("restriction hidden behind unsupported/widget")
	}
	r.options.Downloader = connectorFunc(func(context.Context, string) (Result, error) { return Result{}, context.Canceled })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.ResolveSource(ctx, socialReviewFixtures[0].URL); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost")
	}
}
