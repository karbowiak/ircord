package msgindex

import (
	"fmt"
	"strings"
	"testing"

	"ircord/backend/internal/model"
)

func TestSearchKindsCoverAllLinksAndExcludeCode(t *testing.T) {
	var many strings.Builder
	for i := 0; i < 21; i++ {
		fmt.Fprintf(&many, "https://x.test/page%d ", i)
	}
	for _, tc := range []struct {
		text string
		want int
	}{
		{"\x11https://x.test/a.gif\x11\n```\nhttps://x.test/b.png\n```", 0},
		{many.String() + "https://x.test/last.png", SearchKind("link") | SearchKind("image")},
		{"https://tenor.com/view/cat", SearchKind("link") | SearchKind("gif")},
		{"https://giphy.com/gifs/cat", SearchKind("link") | SearchKind("gif")},
		{"https://vimeo.com/123", SearchKind("link") | SearchKind("video")},
	} {
		if got := SearchKinds(model.Message{Text: tc.text}); got != tc.want {
			t.Fatalf("%q: got %d, want %d", tc.text, got, tc.want)
		}
	}
}
