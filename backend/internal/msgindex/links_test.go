package msgindex

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func linkURLs(list []Link) []string {
	out := []string{}
	for _, l := range list {
		out = append(out, l.URL)
	}
	return out
}

func TestLinksFindsLinksInMessageText(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"several links", "read https://a.test/one and http://b.test/two?x=1 now", []string{"https://a.test/one", "http://b.test/two?x=1"}},
		{"sentence punctuation", "see https://a.test/page. Or https://b.test/x, (https://c.test/y)!", []string{"https://a.test/page", "https://b.test/x", "https://c.test/y"}},
		{"balanced parentheses stay", "https://en.wikipedia.org/wiki/Go_(programming_language).", []string{"https://en.wikipedia.org/wiki/Go_(programming_language)"}},
		{"unicode path", "→ https://例え.テスト/パス?q=猫 ok", []string{"https://例え.テスト/パス?q=猫"}},
		{"IRC formatting ends a link", "\x02https://a.test/bold\x02 and \x0304https://b.test/red\x03", []string{"https://a.test/bold", "https://b.test/red"}},
		{"no link inside code", "\x11https://b.test/mono\x11\n```\nhttps://c.test/fenced\n```\nhttps://d.test/after", []string{"https://d.test/after"}},
		{"credentials are never indexed", "https://user:secret@a.test/x https://b.test/", []string{"https://b.test/"}},
		{"not a link", "ftp://a.test/x https:// https://:80/ mailto:a@b.test", []string{}},
		// Received IRC text has no inline code; a composer's `code` arrives as monospace.
		{"stray backticks", "`https://a.test/x`", []string{"https://a.test/x"}},
		{"the same URL twice counts once", "https://a.test/x https://a.test/x", []string{"https://a.test/x"}},
	} {
		if got := linkURLs(Links(tc.text)); strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestLinksAreBounded(t *testing.T) {
	var b strings.Builder
	for i := range MaxLinksPerMessage + 5 {
		b.WriteString("https://a.test/" + strings.Repeat("x", i+1) + " ")
	}
	b.WriteString("https://a.test/" + strings.Repeat("y", MaxLinkLength))
	if got := Links(b.String()); len(got) != MaxLinksPerMessage {
		t.Fatalf("links: %d", len(got))
	}
	if got := Links("https://a.test/" + strings.Repeat("y", MaxLinkLength)); len(got) != 0 {
		t.Fatalf("an overlong URL was indexed: %d", len(got))
	}
}

func TestIdentityGroupsOnlyTheSameResource(t *testing.T) {
	id := func(raw string) string {
		got := Links(raw)
		if len(got) != 1 {
			t.Fatalf("%q: %v", raw, got)
		}
		return got[0].Identity
	}
	same := [][2]string{
		{"https://A.Test/Path?q=1", "https://a.test/Path?q=1"},
		{"https://a.test:443/x", "https://a.test/x"},
		{"https://a.test/x#section", "https://a.test/x"},
		{"https://a.test", "https://a.test/"},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "https://youtu.be/dQw4w9WgXcQ?t=30"},
	}
	for _, pair := range same {
		if id(pair[0]) != id(pair[1]) {
			t.Errorf("%q and %q should group", pair[0], pair[1])
		}
	}
	different := [][2]string{
		{"https://a.test/article/1", "https://a.test/article/2"},
		{"https://a.test/x?id=1", "https://a.test/x?id=2"},
		{"https://a.test/file?X-Amz-Signature=abc", "https://a.test/file?X-Amz-Signature=def"},
		{"https://a.test/x?a=1&b=2", "https://a.test/x?b=2&a=1"},
		{"https://a.test/Path", "https://a.test/path"},
		{"https://a.test/x", "http://a.test/x"},
		{"https://a.test:8443/x", "https://a.test/x"},
		{"https://news.test/", "https://blog.news.test/"},
	}
	for _, pair := range different {
		if id(pair[0]) == id(pair[1]) {
			t.Errorf("%q and %q must not group", pair[0], pair[1])
		}
	}
	// The original link, query and fragment included, is what is kept and opened.
	if got := Links("https://a.test/x?sig=1#frag")[0].URL; got != "https://a.test/x?sig=1#frag" {
		t.Fatalf("original URL: %q", got)
	}
}

func TestKindsMatchTheTimelinePreviews(t *testing.T) {
	for raw, want := range map[string]string{
		"https://a.test/page":                        KindLink,
		"https://i.imgur.com/cat.JPG":                KindImage,
		"https://a.test/cat.webp":                    KindImage,
		"https://media.tenor.com/x/cat.webp":         KindGIF,
		"https://a.test/dance.gif":                   KindGIF,
		"https://a.test/clip.mp4?token=1":            KindVideo,
		"https://a.test/song.flac":                   KindAudio,
		"https://www.youtube.com/shorts/dQw4w9WgXcQ": KindYouTube,
		"http://www.youtube.com/watch?v=dQw4w9WgXcQ": KindLink,
		"https://www.youtube.com/watch?v=tooshort":   KindLink,
	} {
		if got := Links(raw); len(got) != 1 || got[0].Kind != want {
			t.Errorf("%s: %+v, want %s", raw, got, want)
		}
	}
}

func TestHostsAreNormalizedToASCII(t *testing.T) {
	a, b := Links("https://Bücher.example/a")[0], Links("https://xn--bcher-kva.example/a")[0]
	if a.Host != "xn--bcher-kva.example" || a.Host != b.Host || a.Identity != b.Identity {
		t.Fatalf("unicode %+v, punycode %+v", a, b)
	}
	if got := NormalizeHost("WWW.Example.COM."); got != "www.example.com" {
		t.Fatalf("normalized: %q", got)
	}
}

func TestCapitalSchemesAreNotLinks(t *testing.T) {
	// The timeline links only lowercase http:// and https:// (MessageContent.vue).
	if got := Links("HTTPS://A.TEST/x Http://b.test/"); len(got) != 0 {
		t.Fatalf("links: %+v", got)
	}
}

// The timeline, server previews and the catalog share one definition of code.
func TestLinksSkipCodeLikeTheFrontend(t *testing.T) {
	raw, err := os.ReadFile("testdata/code-parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Text string
		URLs       []string
	}
	if err = json.Unmarshal(raw, &cases); err != nil || len(cases) == 0 {
		t.Fatalf("parity fixture: %v", err)
	}
	for _, tc := range cases {
		want := map[string]bool{}
		for _, u := range tc.URLs {
			want[trimTrailing(u)] = true
		}
		for _, l := range Links(tc.Text) {
			if strings.HasPrefix(l.URL, "https://") && !want[l.URL] {
				t.Errorf("%s: indexed %q, which the timeline shows as code", tc.Name, l.URL)
			}
		}
	}
}
