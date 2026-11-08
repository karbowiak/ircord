package iframely

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBothEndpointsAndResponseShapes(t *testing.T) {
	const target = "https://open.spotify.com/track/example?one=two&three=four"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("url") != target || r.URL.Query().Get("api_key") != "test-key" || r.URL.Query().Get("maxwidth") != "640" {
			t.Errorf("query encoding/options failed")
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/oembed") {
			_, _ = w.Write([]byte(`{"type":"rich","version":"1.0","provider_name":"Spotify","width":640,"html":"<iframe src=\"https://open.spotify.com/embed/track/example\"></iframe>"}`))
		} else {
			_, _ = w.Write([]byte(`{"url":"https://open.spotify.com/track/example","meta":{"title":"A track","site":"Spotify"},"html":"<iframe></iframe>","links":{"player":[{"href":"https://open.spotify.com/embed/track/example","type":"text/html","rel":["player"],"media":{"aspect-ratio":1.77}}]}}`))
		}
	}))
	defer server.Close()
	c, err := New(Options{BaseURL: server.URL + "/api/iframely", APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.Lookup(context.Background(), target, RequestOptions{MaxWidth: 640})
	if err != nil || out.Meta.Title != "A track" || len(out.Links["player"]) != 1 || out.HTML == "" {
		t.Fatalf("lookup: %+v %v", out, err)
	}
	oembed, err := c.OEmbed(context.Background(), target, RequestOptions{MaxWidth: 640})
	if err != nil || oembed.Type != "rich" || oembed.Width != 640 || oembed.HTML == "" {
		t.Fatalf("oembed: %+v %v", oembed, err)
	}
	var links Links
	if err := links.UnmarshalJSON([]byte(`[{"rel":["image"],"type":"image/jpeg","href":"https://source.example/p.jpg"}]`)); err != nil || len(links["image"]) != 1 {
		t.Fatal("flat media links failed", err)
	}
}

func TestFailuresLimitsCancellationAndKeyRedaction(t *testing.T) {
	var mode string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode {
		case "error":
			_, _ = w.Write([]byte(`{"error":404,"message":"contains-private-key"}`))
		case "nested":
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":{"source":"iframely","code":403,"message":"contains-private-key"}}`))
		case "redirect":
			http.Redirect(w, r, "/leak?api_key=contains-private-key", 302)
		case "large":
			_, _ = w.Write([]byte(strings.Repeat("x", 300)))
		default:
			_, _ = w.Write([]byte(`not JSON`))
		}
	}))
	defer server.Close()
	c, _ := New(Options{BaseURL: server.URL, APIKey: "contains-private-key", MaxResponseBytes: 256})
	for _, value := range []string{"error", "nested", "redirect", "large", "invalid"} {
		mode = value
		_, err := c.Lookup(context.Background(), "https://example.com", RequestOptions{})
		if err == nil || strings.Contains(err.Error(), "contains-private-key") {
			t.Fatalf("mode %s: %v", value, err)
		}
		if value == "nested" {
			var upstream *APIError
			if !errors.As(err, &upstream) || upstream.Code != "403" {
				t.Fatalf("nested code lost: %v", err)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Lookup(ctx, "https://example.com", RequestOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestRelativeWidgetUsesOriginalOEmbed(t *testing.T) {
	const original = `<blockquote data-bluesky-uri="at://did:plc:abc/app.bsky.feed.post/abc"></blockquote><script src="https://embed.bsky.app/static/embed.js"></script>`
	for _, available := range []bool{true, false} {
		t.Run(fmt.Sprint(available), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path == "/oembed" {
					if !available {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					_ = json.NewEncoder(w).Encode(OEmbed{HTML: original})
					return
				}
				broken := `<iframe src="/render?uri=example"></iframe>`
				_ = json.NewEncoder(w).Encode(Response{HTML: broken, Meta: Meta{Title: "A post"}, Links: Links{"app": {{HTML: broken}}}})
			}))
			defer server.Close()
			client, _ := New(Options{BaseURL: server.URL})
			result, err := client.Lookup(context.Background(), "https://bsky.app/profile/example/post/abc", RequestOptions{})
			if err != nil || calls != 2 || result.Meta.Title != "A post" || result.Links["app"][0].HTML != "" {
				t.Fatalf("fallback/metadata failed: calls=%d error=%v", calls, err)
			}
			if available && result.HTML != original || !available && result.HTML != "" {
				t.Fatal("relative frame survived or original widget lost")
			}
		})
	}
}

func TestLiveIframely(t *testing.T) {
	base := os.Getenv("IRCORD_TEST_IFRAMELY_URL")
	if base == "" {
		t.Skip("set IRCORD_TEST_IFRAMELY_URL for an opt-in service probe")
	}
	c, err := New(Options{BaseURL: base, APIKey: os.Getenv("IRCORD_TEST_IFRAMELY_KEY")})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	out, err := c.Lookup(ctx, "https://example.com", RequestOptions{})
	if err != nil || out.Meta.Title == "" {
		t.Fatalf("lookup has no metadata: %v", err)
	}
	if _, err := c.OEmbed(ctx, "https://example.com", RequestOptions{}); err != nil {
		t.Fatal(err)
	}
}
