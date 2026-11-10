package linkmedia

import (
	"context"
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
	get := func(key string) string { return values[key] }
	r, err := fromEnv(get)
	if err != nil || r.options.Reddit == nil || r.options.Downloader == nil || r.options.Instagram == nil || r.options.Iframely != nil || r.options.CDN == nil {
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
	if strings.Contains(raw, "open.spotify.com") && (out.Embed == nil || !strings.Contains(out.Embed.HTML, "<iframe")) {
		t.Fatal("Spotify did not return an iframe embed")
	}
	if len(out.Assets) == 0 && (out.Embed == nil || out.Embed.HTML == "") && out.Description == "" {
		t.Fatal("resolver returned neither media, post text, nor embed markup")
	}
}
