package api

import "testing"

func TestGenericProviderFramePolicy(t *testing.T) {
	markup := `<iframe src="https://www.loom.com/embed/abc" height="9999" width="10"></iframe>`
	_, frame, height, ratio, safe := prepareGenericWidget(markup, "https://www.loom.com/share/abc", "https://chat.ircord.com", func(string) bool { return false })
	if !safe || frame == "" || height != 270 || ratio != 16.0/9 {
		t.Fatal("controlled provider frame rejected or trusted provider dimensions")
	}
	for _, tc := range []struct{ markup, source string }{
		{markup, "https://unrelated.example/post"},
		{markup, "https://www.youtube.com/watch?v=abc"},
		{`<iframe src="https://www.loom.com/embed/abc" srcdoc="<script>evil()</script>"></iframe>`, "https://www.loom.com/share/abc"},
		{markup + `<script>alert(1)</script>`, "https://www.loom.com/share/abc"},
		{markup + markup, "https://www.loom.com/share/abc"},
	} {
		if _, _, _, _, safe := prepareGenericWidget(tc.markup, tc.source, "https://chat.ircord.com", func(string) bool { return false }); safe {
			t.Fatal("untrusted generic widget gained iframe authority")
		}
	}
}

func TestGenericProviderFinalFramePolicies(t *testing.T) {
	for _, tc := range []struct {
		name, source, frame string
		height              int
		ratio               float64
	}{
		{"art19", "https://art19.com/shows/example", "https://art19.com/shows/example/embed?type=artwork", 200, 0},
		{"libsyn", "https://directory.libsyn.com/episode/index/id/3252958", "https://player.libsyn.com/embed/episode/id/3252958/theme/standard/height/9999", 96, 0},
		{"simplecast", "https://example.simplecast.com/episodes/public-example", "https://player.simplecast.com/abcdef12?dark=true", 200, 0},
		{"geogebra", "https://www.geogebra.org/classic/hs52mgmq", "https://www.geogebra.org/material/iframe/id/hs52mgmq", 360, 4.0 / 3},
		{"documentcloud", "https://www.documentcloud.org/documents/73991-day-three-documents", "https://embed.documentcloud.org/documents/73991", 640, 3.0 / 4},
		{"prezi", "https://prezi.com/hvsanqexuoza/designthinking-vs-leanstartup/", "https://prezi.com/embed/hvsanqexuoza/", 400, 11.0 / 8},
		{"scribd", "https://www.scribd.com/doc/116154615/title", "https://www.scribd.com/embeds/116154615/content", 640, 3.0 / 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			markup := `<div style="height:9999px"><iframe width="1" height="9999" src="` + tc.frame + `"></iframe></div>`
			_, frame, height, ratio, ok := prepareGenericWidget(markup, tc.source, "https://chat.ircord.com", func(string) bool { return false })
			if !ok || frame == "" || height != tc.height || ratio != tc.ratio {
				t.Fatalf("provider did not keep app geometry: %q %d %v %t", frame, height, ratio, ok)
			}
			for _, source := range []string{"https://unrelated.example/post", "https://www.loom.com/share/abc"} {
				if _, _, _, _, ok := prepareGenericWidget(markup, source, "https://chat.ircord.com", func(string) bool { return false }); ok {
					t.Fatal("foreign source gained provider iframe authority")
				}
			}
		})
	}
}

func TestGenericProviderRejectsPrivateFrames(t *testing.T) {
	for _, frame := range []string{
		"https://www.loom.com/embed/abc?access_token=private",
		"https://www.loom.com/embed/abc?redirect=http://127.0.0.1/admin",
		"https://user:pass@www.loom.com/embed/abc",
		"https://www.loom.com/embed/other",
		"https://www.loom.com/embed/abc/extra",
		"https://www.loom.com/embed/abc%2fextra",
		"https://www.youtube.com/embed/abc",
	} {
		markup := `<iframe src="` + frame + `"></iframe>`
		if _, _, _, _, ok := prepareGenericWidget(markup, "https://www.loom.com/share/abc", "https://chat.ircord.com", func(string) bool { return false }); ok {
			t.Fatal("unsafe player gained iframe authority")
		}
	}
	if _, _, _, _, ok := prepareGenericWidget(`<iframe src="https://www.loom.com/embed/abc"></iframe>`, "https://www.loom.com/share/abc", "https://www.loom.com", func(string) bool { return false }); ok {
		t.Fatal("application origin gained provider iframe authority")
	}
}

func TestGenericProviderNativeImageRouting(t *testing.T) {
	image := []LinkMediaAsset{{Kind: "image", URL: "https://cdn.example.test/image", ContentType: "image/png"}}
	for _, raw := range []string{"https://giphy.com/gifs/public-example-abc", "https://xkcd.com/123/", "https://www.flickr.com/photos/example/123"} {
		if !nativePageMedia(raw, image) {
			t.Fatal("source with reserved image geometry was demoted to a card")
		}
		for _, assets := range [][]LinkMediaAsset{nil, {image[0], image[0]}, {{Kind: "video"}}, {{Kind: "audio"}}, {{Kind: "file"}}} {
			if nativePageMedia(raw, assets) {
				t.Fatal("unsupported native result escaped the card policy")
			}
		}
	}
	for _, raw := range []string{"https://unknown.example/photo/123", "https://giphy.com/videos/example", "https://xkcd.com/archive", "https://flickr.com/photos/example/sets/123"} {
		if nativePageMedia(raw, image) {
			t.Fatal("unknown oEmbed source gained native image geometry")
		}
	}
}

func TestNativeProviderPageMediaRouting(t *testing.T) {
	for _, fixture := range []struct{ url, kind string }{
		{"https://d.pr/i/9jB7", "image"},
		{"https://d.pr/v/6xCyV3", "video"},
		{"https://imageshack.com/i/eyn2zNjEj", "image"},
		{"https://kalyr.smugmug.com/Music/2016/i-95w2R4W", "image"},
		{"https://giphy.com/clips/example-kBMPSpmc4vbIc3h7Zo", "video"},
	} {
		assets := []LinkMediaAsset{{Kind: fixture.kind}}
		if !nativePageMedia(fixture.url, assets) {
			t.Fatalf("reviewed %s route lost its native renderer", fixture.kind)
		}
		for _, wrong := range []string{"image", "video", "audio", "file"} {
			if wrong != fixture.kind && nativePageMedia(fixture.url, []LinkMediaAsset{{Kind: wrong}}) {
				t.Fatalf("%s route acquired a %s renderer", fixture.kind, wrong)
			}
		}
		if nativePageMedia(fixture.url, append(assets, assets...)) {
			t.Fatal("single-media route acquired gallery authority")
		}
	}
}

func TestNativeSocialMediaRouting(t *testing.T) {
	image := LinkMediaAsset{Kind: "image"}
	video := LinkMediaAsset{Kind: "video"}
	gallery := []LinkMediaAsset{image, video, image}
	for _, fixture := range []struct{ source, provider string }{
		{"https://www.instagram.com/p/DfZcglGlDYM/?utm_source=ig_web_copy_link", "instagram"},
		{"https://www.instagram.com/reel/DfX9cZwS4DI/", "instagram"},
		{"https://www.tiktok.com/@zaylento.yt/video/7703024247081291040?is_from_webapp=1", "tiktok"},
	} {
		if !nativeSocialMedia(fixture.source, fixture.provider, gallery) {
			t.Fatal("reviewed social post lost its native gallery renderer")
		}
		for _, rejected := range [][]LinkMediaAsset{nil, make([]LinkMediaAsset, 31), {image, {Kind: "audio"}}, {{Kind: "file"}}} {
			if nativeSocialMedia(fixture.source, fixture.provider, rejected) {
				t.Fatal("invalid social collection gained native media authority")
			}
		}
	}
	for _, fixture := range []struct{ source, provider string }{
		{"https://www.instagram.com/p/DfZcglGlDYM/", "tiktok"},
		{"https://www.tiktok.com/@zaylento.yt/video/7703024247081291040", "instagram"},
		{"https://www.instagram.com/accounts/login/", "instagram"},
		{"https://www.instagram.com/p/DfZcglGlDYM/?sessionid=private", "instagram"},
		{"https://vm.tiktok.com/short/", "tiktok"},
		{"https://www.instagram.com.evil.example/p/DfZcglGlDYM/", "instagram"},
	} {
		if nativeSocialMedia(fixture.source, fixture.provider, gallery) {
			t.Fatal("unreviewed or mismatched social source gained native gallery authority")
		}
	}
}
