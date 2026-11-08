package linkmedia

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"ircord/backend/internal/iframely"
)

func TestHandlerRoutingAndIframelyFallback(t *testing.T) {
	var calls []string
	iframe, _ := iframely.New(iframely.Options{BaseURL: "https://iframely.example", HTTPClient: &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		calls = append(calls, "iframely")
		return response(200, "application/json", `{"meta":{"title":"Widget"},"html":"<div>A publisher widget</div>"}`), nil
	})}})
	primary := func(name string) Connector {
		return connectorFunc(func(context.Context, string) (Result, error) {
			calls = append(calls, name)
			return Result{}, ErrUnavailable
		})
	}
	options := ResolverOptions{
		Reddit: primary("reddit"), Instagram: primary("instagram"), Iframely: iframe,
		Downloader: connectorFunc(func(_ context.Context, raw string) (Result, error) {
			calls = append(calls, "downloader")
			return Result{URL: raw, Provider: "source", Assets: []Asset{{URL: "https://media.example/image.png", Kind: "image"}}}, nil
		}),
		HTTPClient: &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
			return response(200, "image/png", pngFixture), nil
		})},
	}
	resolver := NewResolver(options)
	for _, tc := range []struct {
		url, provider string
		calls         []string
	}{
		{"https://reddit.com/comments/abc", "source", []string{"reddit", "downloader"}},
		{"https://instagram.com/p/Example/", "source", []string{"instagram", "downloader"}},
		{"https://tiktok.com/@creator/video/123", "source", []string{"downloader"}},
		{"https://example.com/article", "iframely", []string{"iframely"}},
		{"https://instagram.com.evil.test/p/Example/", "iframely", []string{"iframely"}},
	} {
		calls = nil
		out, err := resolver.Resolve(context.Background(), tc.url)
		if err != nil || out.Provider != tc.provider || !slices.Equal(calls, tc.calls) {
			t.Fatalf("routing %s: provider=%s calls=%v err=%v", tc.url, out.Provider, calls, err)
		}
	}
	// Disabling extraction still leaves the Iframely fallback for handled hosts.
	options.Downloader = nil
	resolver = NewResolver(options)
	calls = nil
	out, err := resolver.Resolve(context.Background(), "https://instagram.com/p/Unavailable/")
	if err != nil || out.Embed == nil || !slices.Equal(calls, []string{"instagram", "iframely"}) {
		t.Fatalf("failed provider fallback: calls=%v err=%v", calls, err)
	}
}
