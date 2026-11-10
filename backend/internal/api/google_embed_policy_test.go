package api

import (
	"strings"
	"testing"
)

func TestGoogleGenericWidgetPolicy(t *testing.T) {
	for _, tc := range []struct {
		source, frame string
		height        int
		ratio         float64
	}{
		{"https://docs.google.com/document/d/public_policy_fixture/edit", "https://docs.google.com/document/d/public_policy_fixture/preview?sle=true", 640, 3.0 / 4},
		{"https://docs.google.com/presentation/d/e/public_policy_fixture/pub", "https://docs.google.com/presentation/d/e/public_policy_fixture/embed?start=true", 270, 16.0 / 9},
		{"https://docs.google.com/forms/d/e/public_policy_fixture/viewform", "https://docs.google.com/forms/d/e/public_policy_fixture/viewform", 640, 0},
		{"https://drive.google.com/file/d/public_policy_fixture/view", "https://drive.google.com/file/d/public_policy_fixture/preview", 640, 3.0 / 4},
		{"https://calendar.google.com/calendar/embed?src=public%40example.test", "https://calendar.google.com/calendar/embed?src=public%40example.test", 360, 4.0 / 3},
		{"https://www.google.com/maps/d/viewer?mid=public_policy_fixture", "https://www.google.com/maps/d/embed?mid=public_policy_fixture", 360, 4.0 / 3},
	} {
		markup := `<div style="height:9999px"><iframe width="9999" height="9999" src="` + tc.frame + `"></iframe></div>`
		_, frame, height, ratio, ok := prepareGenericWidget(markup, tc.source, "https://chat.example.test", func(string) bool { return false })
		if !ok || frame == "" || height != tc.height || ratio != tc.ratio || strings.Contains(frame, "9999") {
			t.Fatalf("Google frame escaped fixed app policy: %q %d %f %t", frame, height, ratio, ok)
		}
		for _, unsafe := range []string{
			strings.Replace(tc.frame, "public_policy_fixture", "different_content", 1),
			tc.frame + "&resourcekey=private",
		} {
			if unsafe == tc.frame {
				continue
			}
			if _, _, _, _, safe := prepareGenericWidget(`<iframe src="`+unsafe+`"></iframe>`, tc.source, "https://chat.example.test", func(string) bool { return false }); safe {
				t.Fatal("untrusted Google frame gained API authority")
			}
		}
	}
}
