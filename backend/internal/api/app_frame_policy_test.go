package api

import (
	"strings"
	"testing"
)

func TestAppFrameGenericWidgetPolicy(t *testing.T) {
	for _, tc := range []struct {
		source, frame string
		height        int
		ratio         float64
	}{
		{"https://dbdiagram.io/d/6201630385022f4ee5518762", "https://dbdiagram.io/embed/6201630385022f4ee5518762", 270, 16.0 / 9},
		{"https://slides.com/timkindberg/ui-router", "https://slides.com/timkindberg/ui-router/embed", 350, 48.0 / 35},
		{"https://quizlet.com/43729824/physics", "https://quizlet.com/43729824/flashcards/embed", 500, 0},
		{"https://www.openstreetmap.org/export/embed.html?bbox=1,2,3,4&layer=mapnik", "https://www.openstreetmap.org/export/embed.html?bbox=1,2,3,4&layer=mapnik", 360, 4.0 / 3},
		{"https://yandex.ru/maps/-/CWS~5QYp", "https://yandex.ru/map-widget/v1/-/CWS~5QYp", 360, 4.0 / 3},
		{"https://padlet.com/embed/g44u46on3u5n", "https://padlet.com/embed/g44u46on3u5n", 608, 0},
	} {
		markup := `<div style="height:9999px"><iframe height="9999" width="1" src="` + tc.frame + `"></iframe></div>`
		_, frame, height, ratio, ok := prepareGenericWidget(markup, tc.source, "https://chat.example.test", func(string) bool { return false })
		if !ok || frame == "" || height != tc.height || ratio != tc.ratio || strings.Contains(frame, "9999") {
			t.Fatalf("controlled app lost fixed geometry: %q %d %f %t", frame, height, ratio, ok)
		}
		if _, _, _, _, ok := prepareGenericWidget(markup, "https://unrelated.example/item", "https://chat.example.test", func(string) bool { return false }); ok {
			t.Fatal("unrelated source gained app-frame authority")
		}
		if _, _, _, _, ok := prepareGenericWidget(markup+`<script>alert(1)</script>`, tc.source, "https://chat.example.test", func(string) bool { return false }); ok {
			t.Fatal("upstream script gained authority")
		}
	}
}
