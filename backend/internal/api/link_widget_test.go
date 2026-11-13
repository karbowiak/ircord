package api

import (
	"net/url"
	"strings"
	"testing"
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
