package linkmedia

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

func TestNativeRoutingSelectsProviderConnectors(t *testing.T) {
	var calls []string
	failing := func(name string) Connector {
		return connectorFunc(func(context.Context, string) (Result, error) {
			calls = append(calls, name)
			return Result{}, ErrUnavailable
		})
	}
	options := ResolverOptions{Reddit: failing("reddit"), Instagram: failing("instagram"), Downloader: failing("downloader")}
	newResolver := func() *Resolver {
		r := NewResolver(options)
		// Page fallbacks fail without reaching the network.
		r.http = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) { return nil, ErrUnavailable })}
		return r
	}
	resolver := newResolver()
	for _, tc := range []struct {
		url, provider string
		calls         []string
	}{
		{"https://reddit.com/comments/abc", "page", []string{"reddit"}},
		{"https://instagram.com/p/Example/", "instagram", []string{"downloader", "instagram"}},
		{"https://www.tiktok.com/@creator/video/123", "tiktok", []string{"downloader"}},
		{"https://example.com/article", "page", nil},
		{"https://instagram.com.evil.test/p/Example/", "page", nil},
	} {
		calls = nil
		out, err := resolver.Resolve(context.Background(), tc.url)
		if err != nil || out.Provider != tc.provider || len(out.Assets) != 0 || out.Embed != nil || !slices.Equal(calls, tc.calls) {
			t.Fatalf("routing %s: provider=%s calls=%v err=%v", tc.url, out.Provider, calls, err)
		}
	}
	// Disabling extraction still leaves the public Instagram parser.
	options.Downloader = nil
	resolver = newResolver()
	calls = nil
	out, err := resolver.Resolve(context.Background(), "https://instagram.com/p/Unavailable/")
	if err != nil || out.Provider != "instagram" || out.Preview == nil || !slices.Equal(calls, []string{"instagram"}) {
		t.Fatalf("failed provider fallback: %+v calls=%v err=%v", out, calls, err)
	}
}
