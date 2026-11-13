package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ircord/backend/internal/cdn"
	"ircord/backend/internal/iframely"
	"ircord/backend/internal/linkmedia"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

type clientMediaConnector func(context.Context, string) (linkmedia.Result, error)

func (f clientMediaConnector) Resolve(ctx context.Context, raw string) (linkmedia.Result, error) {
	return f(ctx, raw)
}

type mediaTransport func(*http.Request) (*http.Response, error)

func (f mediaTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// linkMediaTestApp attaches a resolver whose object store cannot be written.
// The api package cannot replace the resolver's source transport, so these
// tests only use paths that make no source request: connectors and seeded
// durable results.
func linkMediaTestApp(t *testing.T, options linkmedia.ResolverOptions) (*httptest.Server, *storage.Store, *cdn.Client) {
	t.Helper()
	app, server, store, _ := authenticatedApp(t)
	client, err := cdn.New(cdn.Options{Endpoint: "https://storage.example.test", Bucket: "fixture", Region: "us-east-1", PublicURL: "https://cdn.example.test", Transport: mediaTransport(func(req *http.Request) (*http.Response, error) {
		// Durable manifests are checked against their stored object first.
		if req.Method == http.MethodHead {
			res := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: http.NoBody, ContentLength: seededObjectSize, Request: req}
			res.Header.Set("Content-Length", strconv.Itoa(seededObjectSize))
			res.Header.Set("Content-Type", "image/png")
			res.Header.Set("ETag", `"fixture"`)
			res.Header.Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			return res, nil
		}
		t.Errorf("link media test wrote to object storage: %s", req.Method)
		return nil, linkmedia.ErrUnavailable
	})})
	if err != nil {
		t.Fatal(err)
	}
	options.CDN = client
	app.options.Media = linkmedia.NewResolver(options)
	ctx, cancel := context.WithCancel(context.Background())
	done := app.options.Media.StartCache(ctx, store.DB)
	t.Cleanup(func() { cancel(); <-done })
	return server, store, client
}

const seededObjectSize = 1234

// seedLinkResult stores a public resolver result under the cache version the
// resolver uses for that URL, as it would after resolving it.
func seedLinkResult(t *testing.T, store *storage.Store, client *cdn.Client, version, source string, result linkmedia.Result) {
	t.Helper()
	payload, _ := json.Marshal(result)
	hash := sha256.Sum256([]byte(version + "\x00" + source))
	if _, err := store.DB.Exec(`INSERT INTO public_media_results(namespace,cache_key,payload,expires_ns) VALUES(?,?,?,?)`, client.Namespace(), version+":"+hex.EncodeToString(hash[:]), payload, time.Now().Add(time.Hour).UnixNano()); err != nil {
		t.Fatal(err)
	}
}

// seedLinkManifest stores a validated media result as the resolver would after ingest.
func seedLinkManifest(t *testing.T, store *storage.Store, client *cdn.Client, source string) string {
	t.Helper()
	key := "link-media/" + strings.Repeat("b", 64)
	seedLinkResult(t, store, client, "native-v1", source, linkmedia.Result{URL: source, Provider: "media", Assets: []linkmedia.Asset{{URL: client.URL(key), ObjectKey: key, SourceURL: source, ContentType: "image/png", Size: seededObjectSize, Kind: "image", Width: 640, Height: 480, ExpiresAt: time.Now().Add(time.Hour), Headers: http.Header{"Referer": {"https://source.example/"}}}}})
	return client.URL(key)
}

// Reviewed provider pages use the "native-providers-v3" cache version.
const seededWidgetURL = "https://www.loom.com/share/abc"

func seedLinkWidget(t *testing.T, store *storage.Store, client *cdn.Client) {
	t.Helper()
	seedLinkResult(t, store, client, "native-providers-v3", seededWidgetURL, linkmedia.Result{URL: seededWidgetURL, Provider: "page", Title: "Loom",
		Preview: &linkmedia.Preview{Kind: "widget", Site: "Loom", Title: "Loom", HTML: `<iframe src="https://www.loom.com/embed/abc"></iframe>`}})
}

func TestMessageLinkRenderingIsAuthorizedAndCached(t *testing.T) {
	var calls atomic.Int32
	native := clientMediaConnector(func(context.Context, string) (linkmedia.Result, error) {
		calls.Add(1)
		return linkmedia.Result{URL: "https://reddit.com/comments/abc", Provider: "reddit", ID: "abc", Title: "Photos"}, nil
	})
	server, store, client := linkMediaTestApp(t, linkmedia.ResolverOptions{Reddit: native})
	alice, bob := cookieClient(), cookieClient()
	a := register(t, alice, server.URL)
	register(t, bob, server.URL)
	const file = "https://source.example/known.png"
	stored := seedLinkManifest(t, store, client, file)
	seedLinkWidget(t, store, client)
	conv := model.Conversation{ID: "net_dev", NetworkID: "net", Kind: "channel", Name: "dev", Target: "#dev", MemberIDs: []string{}, MemberModes: map[string]string{}}
	_, _ = store.Commit(a.Account.ID, "conversation.updated", conv)
	msg := model.Message{ID: "net:abc", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "https://reddit.com/comments/abc " + file + " " + seededWidgetURL, CreatedAt: time.Now(), Reactions: []model.Reaction{}}
	_, _ = store.Commit(a.Account.ID, "message.created", msg)
	path := "/api/v1/conversations/net_dev/messages/net:abc/link-media"
	request(t, bob, "POST", server.URL+path, map[string]any{"url": "https://reddit.com/comments/abc"}, 404)
	request(t, alice, "POST", server.URL+path, map[string]any{"url": "https://reddit.com/comments/unrelated"}, 404)
	request(t, alice, "POST", server.URL+path, map[string]any{"url": "https://reddit.com/comments/ab"}, 404)
	if calls.Load() != 0 {
		t.Fatal("unauthorized URL resolved")
	}
	var out LinkMediaResponse
	for range 2 {
		raw := request(t, alice, "POST", server.URL+path, map[string]any{"url": "https://reddit.com/comments/abc"}, 200)
		if json.Unmarshal(raw, &out) != nil || out.Kind != "card" || out.Provider != "reddit" || out.Card == nil || out.Card.Title != "Photos" || len(out.Assets) != 0 || out.Widget != nil {
			t.Fatalf("metadata card: %s", raw)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate resolution was not cached")
	}
	out = LinkMediaResponse{}
	raw := request(t, alice, "POST", server.URL+path, map[string]any{"url": file}, 200)
	// Source request headers never leave the resolver.
	if json.Unmarshal(raw, &out) != nil || out.Kind != "media" || len(out.Assets) != 1 || out.Assets[0].URL != stored || out.Card != nil || strings.Contains(string(raw), "Referer") {
		t.Fatalf("stored media response: %s", raw)
	}
	secondMsg := msg
	secondMsg.ID = "net:second"
	_, _ = store.Commit(a.Account.ID, "message.created", secondMsg)
	secondPath := "/api/v1/conversations/net_dev/messages/net:second/link-media"
	request(t, alice, "POST", server.URL+secondPath, map[string]any{"url": "https://reddit.com/comments/abc"}, 200)
	request(t, bob, "POST", server.URL+secondPath, map[string]any{"url": "https://reddit.com/comments/abc"}, 404)
	if calls.Load() != 1 {
		t.Fatal("public resolver result was not shared between messages")
	}
	// Each message gets its own widget document, readable only by its account.
	widget := func(path string) LinkMediaWidget {
		t.Helper()
		var out LinkMediaResponse
		raw := request(t, alice, "POST", server.URL+path, map[string]any{"url": seededWidgetURL}, 200)
		if json.Unmarshal(raw, &out) != nil || out.Kind != "widget" || out.Widget == nil || !out.Widget.External || out.Widget.Token == "" {
			t.Fatalf("widget response: %s", raw)
		}
		return *out.Widget
	}
	first, second := widget(path), widget(secondPath)
	if first.Token == second.Token {
		t.Fatal("widget token shared between messages")
	}
	request(t, bob, "POST", server.URL+path, map[string]any{"url": seededWidgetURL}, 404)
	document := "/api/v1/link-media/widgets/"
	request(t, bob, "GET", server.URL+document+first.Token, nil, 404)
	res, err := alice.Get(server.URL + document + first.Token)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	policy := res.Header.Get("Content-Security-Policy")
	if res.StatusCode != 200 || !strings.Contains(policy, "sandbox allow-scripts") || strings.Contains(policy, "allow-same-origin") || res.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("widget isolation: status=%d policy=%s", res.StatusCode, policy)
	}
	msg.Text = "removed the link"
	_, _ = store.Commit(a.Account.ID, "message.updated", msg)
	request(t, alice, "POST", server.URL+path, map[string]any{"url": "https://reddit.com/comments/abc"}, 404)
	request(t, alice, "POST", server.URL+path, map[string]any{"url": file}, 404)
	request(t, alice, "GET", server.URL+document+first.Token, nil, 404)
	request(t, alice, "GET", server.URL+document+second.Token, nil, 200)
}

func TestWidgetDocumentThemeIsSelectedAndEscaped(t *testing.T) {
	for _, theme := range []string{"dark", "light"} {
		if !strings.Contains(widgetDocument("hello", "https://example.com", "token", theme), "color-scheme:"+theme+"}") {
			t.Fatal("application scheme lost")
		}
	}
	if strings.Contains(widgetDocument("hello", "https://example.com", "token", `dark;}body{display:none`), "display:none") {
		t.Fatal("theme injected CSS")
	}
}

func TestWidgetFrameSelection(t *testing.T) {
	frame, height := externalWidget(`<iframe src="//open.spotify.com/embed/track/abc" height="352"></iframe>`, "https://chat.ircord.com")
	if frame != "https://open.spotify.com/embed/track/abc" || height != 352 {
		t.Fatalf("standalone frame: %q %d", frame, height)
	}
	for _, tc := range []struct {
		markup string
		height int
	}{
		{`<iframe src="https://open.spotify.com/embed/playlist/abc" style="border:0;width:100%;height:352px"></iframe>`, 352},
		{`<iframe src="https://player.example/embed" height="152" style="height:352px !important"></iframe>`, 352},
		{`<iframe src="https://player.example/embed" height="152" style="height:100%"></iframe>`, 152},
		{`<iframe src="https://player.example/embed" style="height:999999px"></iframe>`, 800},
	} {
		if _, height := externalWidget(tc.markup, "https://chat.ircord.com"); height != tc.height {
			t.Fatalf("provider height: %d, want %d", height, tc.height)
		}
	}
	for _, markup := range []string{
		`<iframe src="https://chat.ircord.com/api/v1/bootstrap"></iframe>`,
		`<iframe src="https://127.0.0.1/private"></iframe>`,
		`<iframe src="javascript:alert(1)"></iframe>`,
		`<iframe src="https://publisher.example/player"></iframe><script>initialize()</script>`,
	} {
		if frame, _ := externalWidget(markup, "https://chat.ircord.com"); frame != "" {
			t.Fatalf("unsafe/raw markup was treated as external: %s", markup)
		}
	}
	markup := `<div style="height:0;padding-bottom:56.25%;position:relative"><iframe src="https://fave.api.cnn.io/player" style="width:100%;height:100%;position:absolute"></iframe></div>`
	if frame, _, ratio := externalWidgetLayout(markup, "https://chat.ircord.com"); frame != "https://fave.api.cnn.io/player" || ratio != 16.0/9.0 {
		t.Fatalf("responsive publisher sizing lost: %s %f", frame, ratio)
	}
}

func TestMessageLinkProviderFailuresBecomeCards(t *testing.T) {
	downloader := clientMediaConnector(func(_ context.Context, raw string) (linkmedia.Result, error) {
		if strings.Contains(raw, "instagram.com") {
			return linkmedia.Result{}, errors.Join(linkmedia.ErrRestricted, errors.New("secret diagnostics"))
		}
		// An extractor result from another host is never published.
		return linkmedia.Result{URL: raw, Provider: "tiktok", ID: "123", Assets: []linkmedia.Asset{{URL: "https://media.example/movie.mp4", Kind: "video"}}}, nil
	})
	server, store, _ := linkMediaTestApp(t, linkmedia.ResolverOptions{Downloader: downloader})
	alice := cookieClient()
	a := register(t, alice, server.URL)
	_, _ = store.Commit(a.Account.ID, "conversation.updated", model.Conversation{ID: "room", Kind: "channel"})
	_, _ = store.Commit(a.Account.ID, "message.created", model.Message{ID: "post", ConversationID: "room", Kind: "message", Text: "https://instagram.com/reel/Restricted/ https://www.tiktok.com/@creator/video/123", CreatedAt: time.Now()})
	path := server.URL + "/api/v1/conversations/room/messages/post/link-media"
	for target, want := range map[string]struct{ provider, reason string }{
		"https://instagram.com/reel/Restricted/":    {"instagram", "restricted"},
		"https://www.tiktok.com/@creator/video/123": {"tiktok", "fetch_failed"},
	} {
		var out LinkMediaResponse
		raw := request(t, alice, "POST", path, map[string]any{"url": target}, 200)
		if json.Unmarshal(raw, &out) != nil || out.Kind != "card" || out.Provider != want.provider || out.Card == nil || out.Card.Reason != want.reason || len(out.Assets) != 0 || out.Widget != nil {
			t.Fatalf("provider failure for %s: %s", target, raw)
		}
		if strings.Contains(string(raw), "secret diagnostics") || strings.Contains(string(raw), "media.example") {
			t.Fatalf("provider failure leaked extractor details: %s", raw)
		}
	}
}

func TestLiveSpotifyWidgetHeight(t *testing.T) {
	base := os.Getenv("IRCORD_TEST_IFRAMELY_URL")
	if base == "" || os.Getenv("IRCORD_TEST_SPOTIFY_WIDGET_CAPTURE") == "" {
		t.Skip("opt-in real Spotify widget sizing")
	}
	client, err := iframely.New(iframely.Options{BaseURL: base})
	if err != nil {
		t.Fatal(err)
	}
	data, err := client.Lookup(context.Background(), "https://open.spotify.com/playlist/37i9dQZF1DX4sWSpwq3LiO", iframely.RequestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	frame, height := externalWidget(data.HTML, "https://chat.ircord.com")
	if !strings.HasPrefix(frame, "https://open.spotify.com/embed/playlist/") || height != 352 {
		t.Fatalf("Spotify frame height=%d, URL selected=%t", height, frame != "")
	}
	raw, _ := json.Marshal(LinkMediaWidget{URL: frame, Height: height, External: true, Token: strings.Repeat("f", 48)})
	if err = os.WriteFile(os.Getenv("IRCORD_TEST_SPOTIFY_WIDGET_CAPTURE"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLiveResponsiveIframelyWrapper(t *testing.T) {
	base, path := os.Getenv("IRCORD_TEST_IFRAMELY_URL"), os.Getenv("IRCORD_TEST_RESPONSIVE_WIDGET_CAPTURE")
	if base == "" || path == "" {
		t.Skip("opt-in responsive provider wrapper")
	}
	client, err := iframely.New(iframely.Options{BaseURL: base})
	if err != nil {
		t.Fatal(err)
	}
	const source = "https://edition.cnn.com/2026/11/08/politics/live-news/midterm-election-trump-congress"
	data, err := client.Lookup(context.Background(), source, iframely.RequestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	frame, height, ratio := externalWidgetLayout(data.HTML, "https://chat.ircord.com")
	if frame == "" || ratio != 16.0/9.0 {
		t.Fatal("responsive publisher frame or sizing lost")
	}
	thumbnail, _, _ := linkImage(data, "thumbnail", "image")
	raw, _ := json.Marshal(LinkMediaWidget{URL: frame, Height: height, AspectRatio: ratio, External: true, ClickToLoad: true, Thumbnail: thumbnail, Token: strings.Repeat("9", 48)})
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if target := os.Getenv("IRCORD_TEST_YOUTUBE_WIDGET_CAPTURE"); target != "" {
		youtube, err := client.Lookup(context.Background(), "https://youtu.be/R2EI1r9uqGQ", iframely.RequestOptions{})
		if err != nil {
			t.Fatal(err)
		}
		frame, height, ratio := externalWidgetLayout(youtube.HTML, "https://chat.ircord.com")
		if !strings.HasPrefix(frame, "https://www.youtube.com/embed/R2EI1r9uqGQ") || ratio <= 0 {
			t.Fatal("YouTube publisher frame/sizing lost")
		}
		raw, _ := json.Marshal(LinkMediaWidget{URL: frame, Height: height, AspectRatio: ratio, External: true, Token: strings.Repeat("8", 48)})
		if err = os.WriteFile(target, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSummaryCardCarriesSizedImageAndSafeLinks(t *testing.T) {
	data := iframely.Response{
		Meta: iframely.Meta{Title: "Add message editing", Description: "Users need to edit their own messages.", Site: "GitHub"},
		Links: iframely.Links{
			"thumbnail": {{Type: "image/png", Href: "https://opengraph.githubassets.com/card.png", Rel: []string{"thumbnail", "ssl"}, Media: map[string]json.RawMessage{"width": json.RawMessage("1200"), "height": json.RawMessage("600")}}},
			"icon":      {{Type: "image/png", Href: "http://insecure.example/favicon.png", Rel: []string{"icon"}}},
		},
	}
	card := summaryCard(data, "https://github.com/karbowiak/ircord/issues/5")
	if card == nil || card.Site != "GitHub" || card.Image != "https://opengraph.githubassets.com/card.png" || card.ImageWidth != 1200 || card.ImageHeight != 600 {
		t.Fatalf("card %+v", card)
	}
	if card.Icon != "" {
		t.Fatal("non-HTTPS icon kept")
	}
	if untitled := summaryCard(iframely.Response{Meta: iframely.Meta{Description: "Only text"}}, "https://www.example.com/a"); untitled.Title != "example.com" || untitled.Site != "example.com" {
		t.Fatalf("untitled card %+v", untitled)
	}
	if summaryCard(iframely.Response{}, "https://example.com") != nil {
		t.Fatal("empty metadata made a card")
	}
}
