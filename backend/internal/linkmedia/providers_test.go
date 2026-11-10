package linkmedia

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/artyom/oembed"
	"ircord/backend/internal/cdn"
	"ircord/backend/internal/storage"
)

func TestControlledProviderPlayers(t *testing.T) {
	// Source forms and fixture IDs adapted from the pinned MIT upstream plugins.
	for _, tc := range []struct {
		name, source, frame string
		height              int
		ratio               float64
	}{
		{"youtube", "https://youtu.be/dQw4w9WgXcQ", "https://www.youtube.com/embed/dQw4w9WgXcQ", 270, 16.0 / 9},
		{"vimeo", "https://vimeo.com/76979871", "https://player.vimeo.com/video/76979871", 270, 16.0 / 9},
		{"spotify-track", "https://open.spotify.com/track/123", "https://open.spotify.com/embed/track/123", 152, 0},
		{"spotify-album", "https://open.spotify.com/album/123", "https://open.spotify.com/embed/album/123", 352, 0},
		{"soundcloud", "https://m.soundcloud.com/claude-debussy/clair-de-lune", "https://w.soundcloud.com/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Ftracks%2F282055227&visual=true&auto_play=true", 166, 0},
		{"soundcloud-set", "https://soundcloud.com/example/sets/music", "https://w.soundcloud.com/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Fplaylists%2F349557245", 450, 0},
		{"mixcloud", "https://www.mixcloud.com/sohoradio/dub-on-air-with-dennis-bovell-03032019/", "https://www.mixcloud.com/widget/iframe/?feed=%2Fother%2Fshow%2F", 120, 0},
		{"bandcamp", "https://decembersongs.bandcamp.com/album/an-east-nashville-christmas", "https://bandcamp.com/EmbeddedPlayer/album=123/size=large/bgcol=ffffff/linkcol=0687f5/", 120, 0},
		{"dailymotion", "https://www.dailymotion.com/video/xcv6dv", "https://geo.dailymotion.com/player/partner.html?video=xcv6dv", 270, 16.0 / 9},
		{"dailymotion-slug", "https://www.dailymotion.com/video/x10bix2_ircam-mani-feste-2013-du-29-mai-au-30-juin-2013_creation", "https://geo.dailymotion.com/player/partner.html?video=x10bix2", 270, 16.0 / 9},
		{"loom", "https://www.loom.com/share/0808e55f94a24992b2de488ac56d3815", "https://www.loom.com/embed/0808e55f94a24992b2de488ac56d3815", 270, 16.0 / 9},
		{"wistia", "https://appsumo.wistia.com/medias/fudkgyoejs", "https://fast.wistia.net/embed/iframe/fudkgyoejs", 270, 16.0 / 9},
		{"ted", "https://www.ted.com/talks/neha_narula_the_future_of_money", "https://embed.ted.com/talks/lang/zh-TW/neha_narula_the_future_of_money", 270, 16.0 / 9},
		{"twitch-video", "https://www.twitch.tv/videos/2687468093?parent=evil.example", "https://player.twitch.tv/?video=1&parent=evil.example", 270, 16.0 / 9},
		{"twitch-clip", "https://clips.twitch.tv/AwkwardHelplessSalamanderSwiftRage", "https://clips.twitch.tv/embed?clip=other", 270, 16.0 / 9},
		{"apple-album", "https://music.apple.com/jp/album/back-to-the-80s/1458246986", "https://embed.music.apple.com/jp/album/back-to-the-80s/1458246986", 450, 0},
		{"apple-song", "https://music.apple.com/us/album/eartha/1450438412?i=1450438420", "https://embed.music.apple.com/us/album/eartha/1450438412?i=1450438420", 175, 0},
		{"apple-playlist", "https://itunes.apple.com/us/playlist/the-a-list-country/idpl.87bb5b36a9bd49db8c975607452bfa2b?app=music", "https://embed.music.apple.com/us/playlist/the-a-list-country/pl.87bb5b36a9bd49db8c975607452bfa2b", 450, 0},
		{"apple-video", "https://music.apple.com/es/music-video/just-the-way-you-are/576670472", "https://embed.music.apple.com/es/music-video/just-the-way-you-are/576670472", 270, 16.0 / 9},
		{"codepen", "https://codepen.io/kevinjannis/pen/pyuix", "https://codepen.io/kevinjannis/embed/preview/pyuix?height=9999", 400, 0},
		{"jsfiddle", "https://jsfiddle.net/pborreli/pJgyu/", "https://jsfiddle.net/pborreli/pJgyu/embedded/", 400, 0},
		{"livecodes", "https://livecodes.io/?template=react", "https://livecodes.io/?template=react", 400, 0},
		{"speakerdeck", "https://speakerdeck.com/gr2m/rails-girls-zurich-keynote", "https://speakerdeck.com/player/abcdef123456", 360, 4.0 / 3},
		{"slideshare", "https://www.slideshare.net/cbo/dynamic-scoring-at-cbo-53071719", "https://www.slideshare.net/slideshow/embed_code/key/abcdef123456", 360, 4.0 / 3},
		{"deezer", "https://www.deezer.com/en/track/3135556", "https://widget.deezer.com/widget/auto/track/3135556", 300, 0},
		{"art19", "https://art19.com/shows/intercom-on-product-management/episodes/5801ff9e-51ba-4e15-a284-5069751c6bed", "https://art19.com/shows/intercom-on-product-management/episodes/5801ff9e-51ba-4e15-a284-5069751c6bed/embed?type=artwork&stretch=true", 200, 0},
		{"libsyn", "https://directory.libsyn.com/episode/index/id/3252958", "https://player.libsyn.com/embed/episode/id/3252958/theme/standard/height/9999", 96, 0},
		{"simplecast", "https://tgd.simplecast.com/episodes/dan-blackman-and-robyn-kanner-the-power-7fe152f4", "https://player.simplecast.com/abcdef12?dark=true", 200, 0},
		{"geogebra", "https://www.geogebra.org/classic/hs52mgmq", "https://www.geogebra.org/material/iframe/id/hs52mgmq?width=10&height=9999", 360, 4.0 / 3},
		{"documentcloud", "https://www.documentcloud.org/documents/73991-day-three-documents", "https://embed.documentcloud.org/documents/73991-day-three-documents/?title=0", 640, 3.0 / 4},
		{"prezi", "https://prezi.com/hvsanqexuoza/designthinking-vs-leanstartup/", "https://prezi.com/embed/hvsanqexuoza/?autoplay=true", 400, 11.0 / 8},
		{"scribd", "https://www.scribd.com/doc/116154615/Australia-Council-Arts-Funding-Guide-2013", "https://www.scribd.com/embeds/116154615/content?view_mode=slideshow", 640, 3.0 / 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame, height, ratio, ok := ControlledPlayer(tc.source, tc.frame, "https://chat.ircord.com")
			if !ok || frame == "" || height != tc.height || ratio != tc.ratio {
				t.Fatalf("player rejected or wrong geometry: %q %d %v %t", frame, height, ratio, ok)
			}
			if strings.Contains(frame, "evil.example") || strings.Contains(frame, "height=9999") || strings.Contains(frame, "auto_play=true") {
				t.Fatal("provider query controls escaped policy")
			}
			if strings.HasPrefix(tc.name, "twitch") && !strings.Contains(frame, "parent=chat.ircord.com") {
				t.Fatal("Twitch did not receive application parent")
			}
			if _, _, _, allowed := ControlledPlayer("https://unrelated.example/post", tc.frame, "https://chat.ircord.com"); allowed {
				t.Fatal("unrelated source gained a trusted frame")
			}
		})
	}
}

func TestControlledProviderBoundaries(t *testing.T) {
	for _, tc := range []struct{ source, frame string }{
		{"https://soundcloud.com/artist/song", "https://w.soundcloud.com/player/?url=http://127.0.0.1/admin"},
		{"https://soundcloud.com/artist/song", "https://w.soundcloud.com/player/?url=https://api.soundcloud.com/tracks/1?secret_token=s-private"},
		{"https://soundcloud.com/artist/song?secret_token=s-private", "https://w.soundcloud.com/player/?url=https://api.soundcloud.com/tracks/1"},
		{"https://www.loom.com/share/abc", "https://www.loom.com.evil.example/embed/abc"},
		{"https://www.loom.com/share/abc", "https://user:pass@www.loom.com/embed/abc"},
		{"https://www.loom.com/share/abc", "http://www.loom.com/embed/abc"},
		{"https://www.loom.com/share/abc", "https://www.loom.com:444/embed/abc"},
		{"https://www.loom.com/share/abc", "https://www.loom.com/embed/%2e%2e"},
		{"https://www.loom.com/share/abc", "https://www.youtube.com/embed/abc"},
		{"https://www.dailymotion.com/video/abc", "https://geo.dailymotion.com/player.html?video=abc"},
		{"https://www.dailymotion.com/video/abc", "https://geo.dailymotion.com/player/partner.html"},
		{"https://www.slideshare.net/secret/private", "https://www.slideshare.net/slideshow/embed_code/key/abc"},
		{"https://artist.bandcamp.com/album/title", "https://bandcamp.com/EmbeddedPlayer/size=large/"},
		{"https://soundcloud.com/artist/song/s-private", "https://w.soundcloud.com/player/?url=https://api.soundcloud.com/tracks/1"},
		{"https://soundcloud.com/artist/song", "https://w.soundcloud.com/player/?url=https://api.soundcloud.com/redirect"},
		{"https://soundcloud.com/artist/song", "https://w.soundcloud.com/player/?url=https://api.soundcloud.com:80/tracks/1"},
		{"https://soundcloud.com/artist/song", "https://w.soundcloud.com/player/?url=https://soundcloud.com/artist/song/s-private"},
		{"https://www.loom.com/share/abc?access_token=private", "https://www.loom.com/embed/abc"},
		{"https://www.loom.com/share/abc", "https://www.loom.com/embed/abc?token=private"},
		{"https://www.loom.com/share/abc", "https://www.loom.com/embed/abc?redirect=http://127.0.0.1/admin"},
		{"https://www.loom.com/share/abc", "https://www.loom.com/embed/abc?redirect=https://user:pass@public.example/admin"},
		{"https://user:pass@www.loom.com/share/abc", "https://www.loom.com/embed/abc"},
		{"https://127.0.0.1/share/abc", "https://www.loom.com/embed/abc"},
		{"https://www.loom.com/share/abc", "https://127.0.0.1/embed/abc"},
		{"https://www.loom.com/share/abc", "https://www.loom.com/embed/other"},
		{"https://www.youtube.com/watch?v=abc", "https://www.youtube.com/embed/other"},
		{"https://vimeo.com/123", "https://player.vimeo.com/video/456"},
		{"https://open.spotify.com/track/123", "https://open.spotify.com/embed/album/123"},
		{"https://www.geogebra.org/m/abc", "https://www.geogebra.org/material/iframe/id/other"},
		{"https://www.documentcloud.org/documents/123-title", "https://embed.documentcloud.org/documents/456-title"},
		{"https://www.scribd.com/doc/123/title", "https://www.scribd.com/embeds/456/content"},
		{"https://artist.bandcamp.com/album/title", "https://bandcamp.com/EmbeddedPlayer/album=123/track=456/?secret_token=private"},
		{"https://music.apple.com/us/album/title/123", "https://embed.music.apple.com/us/music-video/title/123"},
		{"https://music.apple.com/us/album/title/123", "https://embed.music.apple.com/us/album/title/456"},
		{"https://music.apple.com/us/album/title/123?i", "https://embed.music.apple.com/us/album/title/123"},
	} {
		if frame, _, _, ok := ControlledPlayer(tc.source, tc.frame, "https://chat.ircord.com"); ok {
			t.Errorf("unsafe player accepted: %q -> %q", tc.source, frame)
		}
	}
	if _, _, _, ok := ControlledPlayer("https://www.loom.com/share/abc", "https://www.loom.com/embed/abc", "https://www.loom.com"); ok {
		t.Fatal("application origin gained provider authority")
	}
	if _, _, _, ok := ControlledPlayer("https://www.twitch.tv/videos/123", "https://clips.twitch.tv/embed", "https://player.twitch.tv"); ok {
		t.Fatal("rewritten player gained application origin authority")
	}
}

func TestProviderPageAdaptations(t *testing.T) {
	for _, tc := range []struct{ source, expected string }{
		{"https://www.loom.com/share/0808e55f94a24992b2de488ac56d3815", "https://www.loom.com/embed/"},
		{"https://appsumo.wistia.com/medias/fudkgyoejs", "https://fast.wistia.net/embed/iframe/fudkgyoejs"},
		{"https://www.twitch.tv/videos/2687468093", "video=2687468093"},
		{"https://jsfiddle.net/pborreli/pJgyu/", "/pborreli/pJgyu/embedded/"},
		{"https://livecodes.io/?template=react", "embed=true"},
		{"https://music.apple.com/jp/album/back-to-the-80s/1458246986", "https://embed.music.apple.com/jp/album/back-to-the-80s/1458246986"},
		{"https://www.deezer.com/en/track/3135556", "https://widget.deezer.com/widget/auto/track/3135556"},
		{"https://art19.com/shows/intercom-on-product-management", "https://art19.com/shows/intercom-on-product-management/embed"},
		{"https://www.geogebra.org/student/mhs52mgmq", "https://www.geogebra.org/material/iframe/id/hs52mgmq"},
		{"https://www.documentcloud.org/documents/73991-day-three-documents", "https://embed.documentcloud.org/documents/73991"},
		{"https://prezi.com/hvsanqexuoza/designthinking-vs-leanstartup/", "https://prezi.com/embed/hvsanqexuoza/"},
		{"https://www.scribd.com/document/399637688/Prestons-Advert", "https://www.scribd.com/embeds/399637688/content"},
	} {
		if got := directProviderPlayer(tc.source, pageMetadata{}); !strings.Contains(got, tc.expected) {
			t.Errorf("provider adaptation failed: %s -> %s", tc.source, got)
		}
	}
	for _, raw := range []string{"https://example.com/post", "https://www.slideshare.net/secret/private", "https://www.dailymotion.com/video/abc", "https://www.geogebra.org/m/141300", "https://itunes.apple.com/album/id/1784748941", "https://soundcloud.com/artist/song/s-private"} {
		if got := directProviderPlayer(raw, pageMetadata{}); got != "" {
			t.Errorf("invented player for unsupported source %s", raw)
		}
	}
	endpoint, _ := url.Parse(providerEndpoint("https://www.ted.com/dubbing/neha_narula_the_future_of_money?language=fr"))
	if endpoint.Hostname() != "www.ted.com" || !strings.Contains(endpoint.Query().Get("url"), "/talks/") {
		t.Fatal("TED dubbing canonicalization lost")
	}
	metadata := readPageMetadata([]byte(`<html><head><meta property="og:title" content="Comic"><meta property="og:description" content="Description"><meta property="og:image" content="https://imgs.xkcd.com/comics/example.png"></head><body><div id="comic"><img src="//imgs.xkcd.com/comics/example.png" title="Explanation"></div></body></html>`))
	if metadata.title != "Comic" || metadata.comicTitle != "Explanation" || metadata.description != "Description" {
		t.Fatal("bounded metadata parser lost useful fields")
	}
}

func TestProviderFallbackPrefersControlledPagePlayer(t *testing.T) {
	for _, tc := range []struct{ source, metadata, frame string }{
		{"https://www.loom.com/share/abc", "", "https://www.loom.com/embed/abc"},
		{"https://tgd.simplecast.com/episodes/example", `<meta name="twitter:player" content="https://player.simplecast.com/abcdef12?dark=true">`, "https://player.simplecast.com/abcdef12"},
		{"https://thefeed.libsyn.com/example", `<iframe src="https://player.libsyn.com/embed/episode/id/3252958/theme/standard"></iframe>`, "https://html5-player.libsyn.com/embed/episode/id/3252958/"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			resolver := NewResolver(ResolverOptions{})
			resolver.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				var res *http.Response
				if req.URL.String() == tc.source {
					res = response(200, "text/html", `<html><head><title>Public example</title><link rel="alternate" type="application/json+oembed" href="https://provider.example/oembed">`+tc.metadata+`</head></html>`)
				} else {
					res = response(200, "application/json", `{"type":"rich","title":"Generic markup","html":"<script src=\"https://provider.example/widget.js\"></script>"}`)
				}
				res.Request = req
				return res, nil
			})}
			value, err := resolver.ResolveSource(context.Background(), tc.source)
			if err != nil || value.Preview == nil || !strings.Contains(value.Preview.HTML, tc.frame) || strings.Contains(value.Preview.HTML, "script") {
				t.Fatalf("generic oEmbed hid controlled page player: %+v %v", value.Preview, err)
			}
		})
	}
}

func TestProviderFallbackRetainsGenericMetadata(t *testing.T) {
	resolver := NewResolver(ResolverOptions{})
	resolver.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		var res *http.Response
		if req.URL.Hostname() == "unknown.example" {
			res = response(200, "text/html", `<html><head><link rel="alternate" type="application/json+oembed" href="https://provider.example/oembed"></head></html>`)
		} else {
			res = response(200, "application/json", `{"type":"rich","title":"Keep provider metadata","html":"<blockquote>Example</blockquote>"}`)
		}
		res.Request = req
		return res, nil
	})}
	value, err := resolver.ResolveSource(context.Background(), "https://unknown.example/post")
	if err != nil || value.Title != "Keep provider metadata" || value.Preview == nil || value.Preview.HTML != "<blockquote>Example</blockquote>" {
		t.Fatalf("generic fallback lost metadata: %+v %v", value, err)
	}
}

func TestProviderFallbackRetriesNativeImageFailure(t *testing.T) {
	resolver := NewResolver(ResolverOptions{})
	imageCalls := 0
	resolver.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		var res *http.Response
		switch req.URL.Hostname() {
		case "giphy.com":
			res = response(200, "text/html", `<html><head><meta property="og:image" content="https://media.giphy.com/media/abc/giphy.gif"><link rel="alternate" type="application/json+oembed" href="https://provider.example/oembed"></head></html>`)
		case "media.giphy.com":
			imageCalls++
			res = response(503, "text/plain", "Try again later")
		default:
			res = response(200, "application/json", `{"type":"rich","title":"Public GIF","html":"<iframe src=\"https://giphy.com/embed/abc\"></iframe>"}`)
		}
		res.Request = req
		return res, nil
	})}
	value, err := resolver.ResolveSource(context.Background(), "https://giphy.com/gifs/public-example-abc")
	if err != nil || value.Preview == nil || value.Title != "Public GIF" || imageCalls != 1 || time.Until(value.CacheUntil) > 61*time.Second || time.Until(value.Preview.ExpiresAt) > 61*time.Second {
		t.Fatalf("native failure retained day-long fallback: %+v %v imageCalls=%d", value, err, imageCalls)
	}
}

func TestProviderFallbackPrefersNativeImages(t *testing.T) {
	for _, tc := range []struct{ source, metadata, asset string }{
		{"https://giphy.com/gifs/public-example-abc", `<meta property="og:image" content="https://media.giphy.com/media/abc/giphy.gif">`, "https://media.giphy.com/media/abc/giphy.gif"},
		{"https://xkcd.com/123/", `</head><body><div id="comic"><img src="https://imgs.xkcd.com/comics/example.png" title="Comic explanation"></div></body>`, "https://imgs.xkcd.com/comics/example.png"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			store, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			uploads := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodPut {
					t.Errorf("unexpected storage request: %s", req.Method)
				}
				uploads++
				_, _ = io.Copy(io.Discard, req.Body)
				w.Header().Set("ETag", `"fixture"`)
			}))
			defer server.Close()
			client, err := cdn.New(cdn.Options{Endpoint: server.URL, Bucket: "fixture", Region: "us-east-1", PublicURL: "https://cdn.example.test", Transport: server.Client().Transport})
			if err != nil {
				t.Fatal(err)
			}
			resolver := NewResolver(ResolverOptions{CDN: client})
			resolver.persistent = &publicCache{db: store.DB, namespace: client.Namespace()}
			var imageBytes bytes.Buffer
			if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
				t.Fatal(err)
			}
			resolver.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
					t.Error("public provider request inherited account credentials")
				}
				var res *http.Response
				switch req.URL.String() {
				case tc.source:
					res = response(200, "text/html", `<html><head><meta property="og:title" content="Public image"><link rel="alternate" type="application/json+oembed" href="https://provider.example/oembed">`+tc.metadata+`</head></html>`)
				case tc.asset:
					res = response(200, "image/png", imageBytes.String())
				default:
					res = response(200, "application/json", `{"type":"rich","title":"Generic iframe","html":"<iframe src=\"https://giphy.com/embed/abc\"></iframe>"}`)
				}
				res.Request = req
				return res, nil
			})}
			value, err := resolver.ResolveSource(context.Background(), tc.source)
			if err != nil || len(value.Assets) != 1 || value.Preview != nil || uploads != 1 {
				t.Fatalf("rich oEmbed hid native image: %+v %v uploads=%d", value, err, uploads)
			}
			asset := value.Assets[0]
			if asset.SourceURL != tc.asset || asset.ContentType != "image/png" || asset.Width != 2 || asset.Height != 3 || !strings.HasPrefix(asset.URL, "https://cdn.example.test/") || time.Until(asset.ExpiresAt) < 89*24*time.Hour {
				t.Fatalf("native image skipped validated CDN ingest: %+v", asset)
			}
		})
	}
}

func TestProviderQueryPolicy(t *testing.T) {
	for _, tc := range []struct{ source, frame, expected string }{
		{"https://www.loom.com/share/abc", "https://www.loom.com/embed/abc?height=9999&redirect=https://public.example&autoplay=true", "https://www.loom.com/embed/abc"},
		{"https://livecodes.io/?template=react", "https://livecodes.io/?x=https://other.example/example&height=9999", "template=react"},
		{"https://music.apple.com/us/album/title/123?i=456", "https://embed.music.apple.com/us/album/title/123?i=999&height=9999", "?i=456"},
	} {
		frame, _, _, ok := ControlledPlayer(tc.source, tc.frame, "https://chat.ircord.com")
		if !ok || !strings.Contains(frame, tc.expected) || strings.Contains(frame, "height=9999") || strings.Contains(frame, "other.example") {
			t.Fatalf("provider query escaped policy: %q %t", frame, ok)
		}
	}
	for _, raw := range []string{"https://www.loom.com/share/abc?token=private", "https://soundcloud.com/artist/song?secret_token=s-private", "https://livecodes.io/?x=http://127.0.0.1/admin"} {
		if providerEndpoint(raw) != "" || directProviderPlayer(raw, pageMetadata{}) != "" {
			t.Fatal("private source gained provider authority")
		}
	}
}

func TestProviderResolutionCacheEpoch(t *testing.T) {
	for _, raw := range []string{"https://giphy.com/gifs/example-abc", "https://xkcd.com/123/", "https://www.loom.com/share/abc", "https://www.flickr.com/photos/example/123"} {
		if resolutionVersion(raw) != "native-providers-v3" || !strings.HasPrefix(publicResultKey(raw), "native-providers-v3:") {
			t.Fatal("provider results retained the previous cache epoch")
		}
	}
}

func TestProviderNativeImageSourcePolicy(t *testing.T) {
	for _, raw := range []string{
		"https://giphy.com/gifs/public-example-abc",
		"https://www.giphy.com/stickers/public-example-abc",
		"https://xkcd.com/123/",
		"https://www.flickr.com/photos/photographer/123/in/photostream/",
		"https://flic.kr/p/abcdef",
	} {
		if !NativeImageSource(raw) {
			t.Errorf("fixed native image adapter source rejected: %s", raw)
		}
	}
	for _, raw := range []string{
		"https://unknown.example/photos/photographer/123",
		"https://giphy.com.evil.example/gifs/example",
		"https://xkcd.com/archive/",
		"https://flickr.com/photos/photographer/albums/123",
		"https://flickr.com/photos/photographer/sets/123",
		"https://flic.kr/s/abcdef",
		"https://flickr.com/photos/photographer/123?access_token=private",
		"https://user:pass@flickr.com/photos/photographer/123",
		"https://127.0.0.1/photos/photographer/123",
	} {
		if NativeImageSource(raw) {
			t.Errorf("unreviewed native image source accepted: %s", raw)
		}
	}
}

func TestProviderOEmbedWrapperNormalization(t *testing.T) {
	resolver := NewResolver(ResolverOptions{})
	meta := &oembed.Metadata{Type: oembed.TypeRich, Title: "Sound", HTML: `<div><iframe height="9999" src="https://w.soundcloud.com/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Ftracks%2F123"></iframe></div><script src="https://evil.example/tracker.js"></script>`}
	value, err := resolver.oembedResult(context.Background(), "https://soundcloud.com/artist/track", meta, ResolutionCacheTTL)
	if err != nil || value.Preview == nil || strings.Contains(value.Preview.HTML, "script") || strings.Contains(value.Preview.HTML, "9999") || strings.Contains(value.Preview.HTML, "div") {
		t.Fatalf("provider wrapper normalization failed: %+v %v", value.Preview, err)
	}
}

func TestProviderFallbackRequiresSuccessfulPage(t *testing.T) {
	for _, status := range []int{401, 403, 404} {
		resolver := NewResolver(ResolverOptions{})
		resolver.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			res := response(status, "text/html", `<html><head><title>Unavailable</title></head></html>`)
			res.Request = req
			return res, nil
		})}
		value, err := resolver.ResolveSource(context.Background(), "https://www.loom.com/share/0808e55f94a24992b2de488ac56d3815")
		if err != nil || value.Preview == nil || value.Preview.Kind != "card" || value.Preview.HTML != "" {
			t.Fatalf("HTTP %d manufactured a player: %+v %v", status, value.Preview, err)
		}
	}
}
