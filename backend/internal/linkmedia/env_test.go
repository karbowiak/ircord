package linkmedia

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestEnvironmentConfiguration(t *testing.T) {
	values := map[string]string{
		"IRCORD_IFRAMELY_URL":     "http://127.0.0.1:18061/iframely",
		"IRCORD_REDDIT_CLIENT_ID": "test-client", "IRCORD_REDDIT_CLIENT_SECRET": "test-secret",
		"IRCORD_CDN_ENDPOINT": "http://127.0.0.1:19000", "IRCORD_CDN_BUCKET": "fixture",
		"IRCORD_CDN_PUBLIC_URL": "https://cdn.example.test", "IRCORD_CDN_ACCESS_KEY": "test-access", "IRCORD_CDN_SECRET_KEY": "test-secret",
	}
	get := func(key string) string {
		if strings.HasPrefix(key, "IRCORD_IFRAMELY_") {
			t.Error("inactive Iframely configuration was read")
		}
		return values[key]
	}
	r, err := fromEnv(get)
	if err != nil || r.options.Reddit == nil || r.options.Downloader == nil || r.options.Instagram == nil || r.options.CDN == nil {
		t.Fatalf("configured clients missing: %v", err)
	}
	if got := r.options.CDN.URL("example"); got != "https://cdn.example.test/media/example" {
		t.Fatalf("delivery mapping: %s", got)
	}
	delete(values, "IRCORD_CDN_SECRET_KEY")
	if _, err := fromEnv(get); err == nil || !strings.Contains(err.Error(), "IRCORD_CDN_SECRET_KEY") {
		t.Fatal("partial storage config accepted", err)
	}
	values = map[string]string{"IRCORD_LINKMEDIA_TIMEOUT": "invalid-private-value"}
	if _, err := fromEnv(get); err == nil || strings.Contains(err.Error(), "invalid-private-value") {
		t.Fatal("invalid duration accepted or exposed", err)
	}
	values = map[string]string{"IRCORD_YTDLP_BINARY": "disabled"}
	r, err = fromEnv(get)
	if err != nil || r.options.Downloader != nil || r.options.CDN != nil || r.options.Reddit != nil {
		t.Fatal("optional clients were not disabled", err)
	}
}

func TestLiveConfiguredResolver(t *testing.T) {
	raw := os.Getenv("IRCORD_TEST_LINKMEDIA_URL")
	if raw == "" {
		t.Skip("set IRCORD_TEST_LINKMEDIA_URL with the configured backend environment for an opt-in resolver probe")
	}
	r, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.ResolveSource(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("IRCORD_TEST_REQUIRE_NATIVE") == "true" {
		if len(out.Assets) == 0 || out.Embed != nil {
			t.Fatal("expected native source media, got widget fallback")
		}
		kinds := make([]string, len(out.Assets))
		for index, asset := range out.Assets {
			kinds[index] = asset.Kind
		}
		t.Logf("native assets=%d kinds=%s", len(out.Assets), strings.Join(kinds, ","))
		if want := os.Getenv("IRCORD_TEST_EXPECT_KINDS"); want != "" && strings.Join(kinds, ",") != want {
			t.Fatalf("gallery kinds/order: %s", strings.Join(kinds, ","))
		}
	}
	if strings.Contains(raw, "open.spotify.com") && !spotifyPreview(raw, out) {
		t.Fatal("Spotify did not return a controlled widget preview")
	}
	if !resolverHasContent(out) {
		t.Fatal("resolver returned neither media, post text, nor embed markup")
	}
}

func spotifyPreview(raw string, out Result) bool {
	return out.Preview != nil && out.Preview.Kind == "widget" && providerMarkup(raw, out.Preview.HTML) != ""
}

func resolverHasContent(out Result) bool {
	return len(out.Assets) > 0 || out.Description != "" || out.Preview != nil && (out.Preview.Description != "" || out.Preview.HTML != "")
}

func TestLegacyEmbedManifestRejected(t *testing.T) {
	for _, tc := range []struct {
		payload string
		stored  bool
	}{
		{`{"url":"https://example.com/post","preview":{"kind":"card"}}`, true},
		{`{"url":"https://example.com/post","preview":{"kind":"card"},"embed":null}`, true},
		{`{"url":"https://example.com/post","preview":{"kind":"card"},"embed":{"html":"<iframe></iframe>"}}`, false},
		{`{"url":"https://example.com/post","preview":{"kind":"card"},"embed":{}}`, false},
	} {
		var value Result
		if err := json.Unmarshal([]byte(tc.payload), &value); err != nil {
			t.Fatal(err)
		}
		if storedResult(value) != tc.stored {
			t.Fatalf("unexpected cache eligibility for %s", tc.payload)
		}
		encoded, err := json.Marshal(value)
		if err != nil || strings.Contains(string(encoded), `"embed":`) != !tc.stored {
			t.Fatalf("legacy field serialization: %s, %v", encoded, err)
		}
	}
}

func TestSpotifyNativePreview(t *testing.T) {
	const source = "https://open.spotify.com/track/123"
	r := NewResolver(ResolverOptions{})
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		var res *http.Response
		if req.URL.String() == source {
			res = response(200, "text/html", `<html><head><link rel="alternate" type="application/json+oembed" href="https://provider.example/oembed"></head></html>`)
		} else {
			res = response(200, "application/json", `{"type":"rich","title":"Track","html":"<iframe src=\"https://open.spotify.com/embed/track/123\"></iframe>"}`)
		}
		res.Request = req
		return res, nil
	})}
	got, err := r.ResolveSource(context.Background(), source)
	if err != nil || got.Embed != nil || !spotifyPreview(source, got) || !resolverHasContent(got) {
		t.Fatalf("native Spotify preview: %+v, %v", got, err)
	}
	for _, invalid := range []Result{
		{},
		{Preview: &Preview{Kind: "card", HTML: got.Preview.HTML}},
		{Preview: &Preview{Kind: "widget", HTML: `<iframe src="https://open.spotify.com/embed/track/other"></iframe>`}},
	} {
		if spotifyPreview(source, invalid) {
			t.Fatalf("invalid Spotify preview accepted: %+v", invalid)
		}
	}
	if resolverHasContent(Result{Preview: &Preview{Kind: "card"}}) {
		t.Fatal("empty fallback counted as content")
	}
}
