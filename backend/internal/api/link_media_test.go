package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ircord/backend/internal/iframely"
	"ircord/backend/internal/linkmedia"
	"ircord/backend/internal/model"
)

type clientMediaConnector func(context.Context, string) (linkmedia.Result, error)

func (f clientMediaConnector) Resolve(ctx context.Context, raw string) (linkmedia.Result, error) {
	return f(ctx, raw)
}

type mediaTransport func(*http.Request) (*http.Response, error)

func (f mediaTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMessageLinkRenderingIsAuthorizedCachedAndIsolated(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	alice, bob := cookieClient(), cookieClient()
	a := register(t, alice, server.URL)
	register(t, bob, server.URL)
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"meta":{"title":"Widget"},"html":"<div>Provider content</div><script>window.widget=true</script>"}`))
	}))
	defer provider.Close()
	iframe, _ := iframely.New(iframely.Options{BaseURL: provider.URL})
	native := clientMediaConnector(func(context.Context, string) (linkmedia.Result, error) {
		return linkmedia.Result{URL: "https://reddit.com/comments/abc", Provider: "reddit", Title: "Photos", Assets: []linkmedia.Asset{{URL: "https://i.redd.it/photo.png", Kind: "image", Headers: http.Header{"Referer": {"https://reddit.com/"}}}}}, nil
	})
	app.options.Media = linkmedia.NewResolver(linkmedia.ResolverOptions{Reddit: native, Iframely: iframe, HTTPClient: &http.Client{Transport: mediaTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(strings.NewReader("\x89PNG\r\n\x1a\nfixture"))}, nil
	})}})
	conv := model.Conversation{ID: "net_dev", NetworkID: "net", Kind: "channel", Name: "dev", Target: "#dev", MemberIDs: []string{}, MemberModes: map[string]string{}}
	_, _ = store.Commit(a.Account.ID, "conversation.updated", conv)
	msg := model.Message{ID: "net:abc", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "https://reddit.com/comments/abc https://example.com/article", CreatedAt: time.Now(), Reactions: []model.Reaction{}}
	_, _ = store.Commit(a.Account.ID, "message.created", msg)
	path := "/api/v1/conversations/net_dev/messages/net:abc/link-media"
	request(t, bob, "POST", server.URL+path, map[string]any{"url": "https://example.com/article"}, 404)
	request(t, alice, "POST", server.URL+path, map[string]any{"url": "https://example.com/unrelated"}, 404)
	request(t, alice, "POST", server.URL+path, map[string]any{"url": "https://example.com/art"}, 404)
	if calls.Load() != 0 {
		t.Fatal("unauthorized URL fetched")
	}
	var out LinkMediaResponse
	raw := request(t, alice, "POST", server.URL+path, map[string]any{"url": "https://reddit.com/comments/abc"}, 200)
	if json.Unmarshal(raw, &out) != nil || len(out.Assets) != 1 || out.Widget != nil || strings.Contains(string(raw), "Referer") {
		t.Fatalf("native response: %s", raw)
	}
	for range 2 {
		raw = request(t, alice, "POST", server.URL+path, map[string]any{"url": "https://example.com/article"}, 200)
		if json.Unmarshal(raw, &out) != nil || out.Widget == nil || out.Widget.External || strings.Contains(string(raw), "window.widget") {
			t.Fatalf("widget response exposed markup: %s", raw)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate resolution was not cached")
	}
	secondMsg := msg
	secondMsg.ID = "net:second"
	_, _ = store.Commit(a.Account.ID, "message.created", secondMsg)
	secondRaw := request(t, alice, "POST", server.URL+"/api/v1/conversations/net_dev/messages/net:second/link-media", map[string]any{"url": "https://example.com/article"}, 200)
	var second LinkMediaResponse
	if json.Unmarshal(secondRaw, &second) != nil || second.Widget == nil || second.Widget.Token == out.Widget.Token || calls.Load() != 1 {
		t.Fatal("URL cache did not retain separate message authorization")
	}
	request(t, bob, "GET", server.URL+out.Widget.URL, nil, 404)
	res, err := alice.Get(server.URL + out.Widget.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	policy := res.Header.Get("Content-Security-Policy")
	if res.StatusCode != 200 || !strings.Contains(policy, "sandbox allow-scripts") || strings.Contains(policy, "allow-same-origin") || res.Header.Get("Cache-Control") != "private, no-store" || res.Header.Get("Referrer-Policy") != "strict-origin-when-cross-origin" || !strings.Contains(string(body), "ircord:widget-size") {
		t.Fatalf("widget isolation: status=%d policy=%s", res.StatusCode, policy)
	}
	msg.Text = "removed the link"
	_, _ = store.Commit(a.Account.ID, "message.updated", msg)
	request(t, alice, "GET", server.URL+out.Widget.URL, nil, 404)
	request(t, alice, "GET", server.URL+second.Widget.URL, nil, 200)
	request(t, alice, "POST", server.URL+path, map[string]any{"url": "https://example.com/article"}, 404)
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

func TestMessageLinkSummaryAndProviderFailures(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	alice := cookieClient()
	a := register(t, alice, server.URL)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("url"), "dr.dk") {
			_, _ = w.Write([]byte(`{"meta":{"title":"DR <Nyheder>","description":"News & radio","site":"DR"},"links":[{"href":"https://asset.dr.dk/photo.png","type":"image/png","rel":["twitter","thumbnail"]}]}`))
		} else if strings.Contains(r.URL.Query().Get("url"), "edition.cnn.com") {
			_, _ = w.Write([]byte(`{"meta":{"title":"CNN video"},"html":"<div style=\"padding-bottom:56.25%\"><iframe src=\"https://fave.api.cnn.io/player\"></iframe></div>","links":[{"href":"https://media.cnn.com/poster.jpg","type":"image/jpg","rel":["thumbnail"]}]}`))
		} else {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"error":{"code":403,"message":"secret diagnostics"}}`))
		}
	}))
	defer provider.Close()
	iframe, _ := iframely.New(iframely.Options{BaseURL: provider.URL})
	app.options.Media = linkmedia.NewResolver(linkmedia.ResolverOptions{Iframely: iframe, Downloader: clientMediaConnector(func(context.Context, string) (linkmedia.Result, error) {
		return linkmedia.Result{}, linkmedia.ErrRestricted
	})})
	_, _ = store.Commit(a.Account.ID, "conversation.updated", model.Conversation{ID: "room", Kind: "channel"})
	_, _ = store.Commit(a.Account.ID, "message.created", model.Message{ID: "post", ConversationID: "room", Kind: "message", Text: "https://www.dr.dk https://eve-kill.com/kill/139603827 https://instagram.com/reel/Restricted/ https://edition.cnn.com/story", CreatedAt: time.Now()})
	path := server.URL + "/api/v1/conversations/room/messages/post/link-media"
	var out LinkMediaResponse
	raw := request(t, alice, "POST", path, map[string]any{"url": "https://www.dr.dk"}, 200)
	if json.Unmarshal(raw, &out) != nil || out.Widget == nil || out.Widget.External {
		t.Fatalf("metadata-only page rejected: %s", raw)
	}
	res, err := alice.Get(server.URL + out.Widget.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(body), "DR &lt;Nyheder&gt;") || !strings.Contains(string(body), `src="https://asset.dr.dk/photo.png"`) || !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox allow-scripts") {
		t.Fatal("summary metadata/isolation lost")
	}
	for target, reason := range map[string]string{"https://eve-kill.com/kill/139603827": "site refused", "https://instagram.com/reel/Restricted/": "isn’t publicly accessible"} {
		raw = request(t, alice, "POST", path, map[string]any{"url": target}, 422)
		if !strings.Contains(string(raw), reason) || strings.Contains(string(raw), "secret diagnostics") {
			t.Fatalf("provider failure was hidden or leaked diagnostics: %s", raw)
		}
	}
	raw = request(t, alice, "POST", path, map[string]any{"url": "https://edition.cnn.com/story"}, 200)
	if json.Unmarshal(raw, &out) != nil || out.Widget == nil || !out.Widget.External || !out.Widget.ClickToLoad || out.Widget.AspectRatio != 16.0/9.0 || out.Widget.Thumbnail != "https://media.cnn.com/poster.jpg" {
		t.Fatalf("news player lost publisher isolation, sizing or activation: %s", raw)
	}
}

func TestSummaryWidgetEscapesProviderData(t *testing.T) {
	markup := summaryWidget(iframely.Response{Meta: iframely.Meta{Title: `<script>attack()</script>`, Description: `<img onerror="attack()">`, Site: `" onmouseover="attack()`}, Links: iframely.Links{"thumbnail": {{Type: "image/png", Href: "https://127.0.0.1/private", Rel: []string{"thumbnail"}}}}}, "https://example.com/?a=1&b=2")
	if strings.Contains(markup, "<script>attack") || strings.Contains(markup, `<img onerror="attack`) || strings.Contains(markup, "127.0.0.1") || !strings.Contains(markup, "a=1&amp;b=2") {
		t.Fatal("unsafe summary markup")
	}
}

func TestSummaryWidgetGenericThumbnailAndLargeImage(t *testing.T) {
	for _, tc := range []struct {
		kind, width, height string
		compact             bool
	}{
		{"image", "64", "64", true},
		{"image/png", "700", "394", false},
	} {
		data := iframely.Response{Meta: iframely.Meta{Title: "A post"}, Links: iframely.Links{"twitter": {{Type: tc.kind, Href: "https://eve-kill.com/images/ship", Rel: []string{"twitter", "thumbnail", "og"}, Media: map[string]json.RawMessage{"width": json.RawMessage(tc.width), "height": json.RawMessage(tc.height)}}}}}
		markup := summaryWidget(data, "https://eve-kill.com/kill/139603827")
		if !strings.Contains(markup, `src="https://eve-kill.com/images/ship"`) || strings.Contains(markup, `<article class="summary is-compact">`) != tc.compact {
			t.Fatalf("thumbnail %s %sx%s was missing or sized incorrectly", tc.kind, tc.width, tc.height)
		}
	}
}

func TestGitHubWidgetUsesRepositoryAndResourceContext(t *testing.T) {
	for _, tc := range []struct{ url, title, reference, wantTitle string }{
		{"https://github.com/karbowiak/ircord/issues/5", "Edit messages · Issue #5 · karbowiak/ircord", "Issue #5", "Edit messages"},
		{"https://github.com/karbowiak/ircord/pull/9", "Fix embeds · Pull Request #9 · karbowiak/ircord", "Pull request #9", "Fix embeds"},
		{"https://github.com/karbowiak/ircord", "GitHub - karbowiak/ircord: IRC client", "Repository", "ircord"},
	} {
		markup := summaryWidget(iframely.Response{Meta: iframely.Meta{Title: tc.title, Description: `<script>bad()</script>`}}, tc.url)
		if !strings.Contains(markup, `class="github-card"`) || !strings.Contains(markup, tc.reference) || !strings.Contains(markup, "karbowiak/ircord") || !strings.Contains(markup, "<h2>"+tc.wantTitle+"</h2>") || strings.Contains(markup, "<script>bad") {
			t.Fatalf("GitHub context/escaping failed for %s", tc.url)
		}
	}
	if markup := summaryWidget(iframely.Response{Meta: iframely.Meta{Title: "Impersonator"}}, "https://github.com.evil.test/karbowiak/ircord/issues/5"); strings.Contains(markup, `class="github-card"`) {
		t.Fatal("lookalike host acquired provider UI")
	}
}

func TestLiveIframelyMetadataWidget(t *testing.T) {
	base := os.Getenv("IRCORD_TEST_IFRAMELY_URL")
	if base == "" {
		t.Skip("opt-in real Iframely metadata/widget check")
	}
	client, err := iframely.New(iframely.Options{BaseURL: base})
	if err != nil {
		t.Fatal(err)
	}
	targets := []struct{ url, title, image, path, token string }{{"https://www.dr.dk", "DR: Nyheder", "https://asset.dr.dk/", os.Getenv("IRCORD_TEST_WIDGET_CAPTURE"), strings.Repeat("c", 48)}}
	if path := os.Getenv("IRCORD_TEST_EVE_WIDGET_CAPTURE"); path != "" {
		targets = append(targets, struct{ url, title, image, path, token string }{"https://eve-kill.com/kill/139603827", "Anhinga | Auroraqual", "https://eve-kill.com/images/types/89807/overlayrender", path, strings.Repeat("d", 48)})
	}
	if path := os.Getenv("IRCORD_TEST_GITHUB_WIDGET_CAPTURE"); path != "" {
		targets = append(targets, struct{ url, title, image, path, token string }{"https://github.com/karbowiak/ircord/issues/5", "Add message editing", "https://opengraph.githubassets.com/", path, strings.Repeat("e", 48)})
	}
	for _, target := range targets {
		data, err := client.Lookup(context.Background(), target.url, iframely.RequestOptions{})
		if err != nil {
			t.Fatal(err)
		}
		markup := summaryWidget(data, target.url)
		if markup == "" || !strings.Contains(markup, target.title) || !strings.Contains(markup, target.image) {
			t.Fatalf("real %s metadata or thumbnail did not render", target.url)
		}
		if target.path != "" {
			if err = os.WriteFile(target.path, []byte(widgetDocument(markup, target.url, target.token)), 0600); err != nil {
				t.Fatal(err)
			}
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
	thumbnail, _ := widgetImage(data)
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
