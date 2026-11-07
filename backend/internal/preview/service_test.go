package preview

import (
	"context"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func resolver() Resolver {
	return resolverFunc(func(ctx context.Context, network, host string) ([]netip.Addr, error) {
		switch host {
		case "private.example", "127.0.0.1", "metadata.google.internal":
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		case "rebind.example":
			return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("10.0.0.1")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	})
}
func TestMetadataParsingCacheAndSSRF(t *testing.T) {
	calls := 0
	service := New(Options{Resolver: resolver(), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credentials leaked")
		}
		header := http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}
		body := `<head><title>Fallback</title><meta property="og:title" content="A &amp; B"><meta name="description" content="A useful page"><meta property="og:image" content="/cover.png"></head><script>alert('never executed')</script>`
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}})
	result, err := service.Fetch(context.Background(), "https://public.example/article")
	if err != nil || result.Title != "A & B" || result.Image != "https://public.example/cover.png" || result.Description != "A useful page" {
		t.Fatal(result, err)
	}
	if _, err = service.Fetch(context.Background(), "https://public.example/article#fragment"); err != nil || calls != 1 {
		t.Fatal("metadata cache did not coalesce URLs", err, calls)
	}
	for _, raw := range []string{"http://public.example", "file:///etc/passwd", "https://user:secret@public.example/", "https://private.example/", "https://127.0.0.1/", "https://metadata.google.internal/", "https://rebind.example/"} {
		before := calls
		if _, err = service.Fetch(context.Background(), raw); err == nil || calls != before {
			t.Fatalf("unsafe URL reached HTTP: %s", raw)
		}
	}
}
func TestRedirectAndMetadataBounds(t *testing.T) {
	calls := 0
	service := New(Options{Resolver: resolver(), Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://private.example/secret"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}})
	if _, err := service.Fetch(context.Background(), "https://public.example/redirect"); err == nil || calls != 1 {
		t.Fatal("private redirect followed")
	}
	base, _ := url.Parse("https://public.example/")
	if _, err := parse(strings.NewReader(`<head><meta content="`+strings.Repeat("x", 70<<10)), base); err == nil {
		t.Fatal("unbounded HTML token accepted")
	}
	for _, ip := range []string{"0.0.0.0", "10.0.0.1", "100.64.0.1", "127.0.0.1", "169.254.169.254", "198.18.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "64:ff9b::7f00:1", "2002:7f00:1::"} {
		if public(netip.MustParseAddr(ip)) {
			t.Fatal("reserved address accepted", ip)
		}
	}
}
