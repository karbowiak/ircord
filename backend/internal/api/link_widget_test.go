package api

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	"ircord/backend/internal/iframely"
)

func TestScriptWidgetFrames(t *testing.T) {
	const twitter = `<blockquote class="twitter-tweet">Post</blockquote><script src="https://platform.x.com/widgets.js"></script>`
	const bluesky = `<blockquote data-bluesky-uri="at://did:plc:abc/app.bsky.feed.post/3abc"></blockquote><script src="https://embed.bsky.app/static/embed.js"></script>`
	for _, tc := range []struct{ name, markup, source, host string }{
		{"twitter", twitter, "https://x.com/author/status/123?tracking=1", "platform.twitter.com"},
		{"legacy-twitter", twitter, "https://twitter.com/author/status/123", "platform.twitter.com"},
		{"bluesky", bluesky, "https://bsky.app/profile/author.bsky.social/post/3abc", "embed.bsky.app"},
		{"wrong-script", strings.ReplaceAll(twitter, "platform.x.com", "platform.x.com.evil.test"), "https://x.com/author/status/123", ""},
		{"wrong-source", twitter, "https://x.com.evil.test/author/status/123", ""},
		{"wrong-post", bluesky, "https://bsky.app/profile/author.bsky.social/post/different", ""},
		{"arbitrary-script", `<script>alert(1)</script>`, "https://example.com", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame, _, ratio := providerWidgetLayout(tc.markup, tc.source, "https://chat.ircord.com")
			u, _ := url.Parse(frame)
			if u.Hostname() != tc.host || ratio != 0 {
				t.Fatal("provider frame selection failed")
			}
			if tc.host != "" {
				frame, _, _ = providerWidgetLayout(tc.markup, tc.source, "https://"+tc.host)
				if frame != "" {
					t.Fatal("same-origin script frame allowed")
				}
			}
		})
	}
}

func TestLiveSocialIframelyWidgets(t *testing.T) {
	base := os.Getenv("IRCORD_TEST_IFRAMELY_URL")
	if base == "" || os.Getenv("IRCORD_TEST_X_WIDGET_CAPTURE") == "" || os.Getenv("IRCORD_TEST_BLUESKY_WIDGET_CAPTURE") == "" {
		t.Skip("opt-in real Iframely social widgets")
	}
	client, err := iframely.New(iframely.Options{BaseURL: base})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ source, host, path string }{
		{"https://x.com/Osinttechnical/status/2119345734642475100", "platform.twitter.com", os.Getenv("IRCORD_TEST_X_WIDGET_CAPTURE")},
		{"https://bsky.app/profile/chriso-wiki.bsky.social/post/3mxgtsqzqjs2t", "embed.bsky.app", os.Getenv("IRCORD_TEST_BLUESKY_WIDGET_CAPTURE")},
	} {
		data, err := client.Lookup(context.Background(), tc.source, iframely.RequestOptions{})
		if err != nil {
			t.Fatal(err)
		}
		frame, height, ratio := providerWidgetLayout(data.HTML, tc.source, "https://chat.ircord.com")
		u, _ := url.Parse(frame)
		if u.Hostname() != tc.host || ratio != 0 {
			t.Fatal("original social widget did not yield a provider frame")
		}
		raw, _ := json.Marshal(LinkMediaWidget{URL: frame, Height: height, External: true, Token: strings.Repeat("7", 48)})
		if err := os.WriteFile(tc.path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
