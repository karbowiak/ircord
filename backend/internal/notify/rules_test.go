package notify

import (
	"testing"
	"time"
)

func TestRulesQuietHoursHighlightsAndIgnore(t *testing.T) {
	settings := Defaults()
	settings.HighlightWords = []string{"deploy"}
	settings.Ignore = []string{"bot*", "*!bad@*.example"}
	settings.QuietHours = QuietHours{Enabled: true, Start: "22:00", End: "07:00", TimeZone: "Europe/Copenhagen"}
	p := New(settings)
	if !p.Highlight("\x02@Alice\x0f, hi", "alice") || p.Highlight("malice", "alice") || !p.Highlight("a deploy!", "alice") {
		t.Fatal("whole-word formatting highlights differ from browser")
	}
	if !p.Ignored("robot", "bad", "host.example") || !p.Ignored("bot42", "user", "host") || p.Ignored("alice", "good", "host.example") {
		t.Fatal("IRC mask evaluation failed")
	}
	at := time.Date(2026, 11, 7, 22, 30, 0, 0, time.UTC)
	if !p.Quiet(at) || p.Quiet(at.Add(-8*time.Hour)) {
		t.Fatal("quiet hours did not use local time across midnight")
	}
	if got := p.Notification("room", "direct", "", "message", "hello", "alice", "bob", "u", "h", false, at); got != "" {
		t.Fatal("quiet hours notified")
	}
	at = at.Add(-8 * time.Hour)
	if got := p.Notification("room", "channel", "", "message", "a deploy!", "alice", "bob", "u", "h", false, at); got != "mention" {
		t.Fatal("highlight did not notify")
	}
	if got := p.Notification("room", "channel", "", "message", "normal chat", "alice", "bob", "u", "h", false, at); got != "" {
		t.Fatal("ordinary channel message notified with mentions-only")
	}
	if got := p.Notification("room", "direct", "", "message", "normal chat", "alice", "bob", "u", "h", false, at); got != "mention" {
		t.Fatal("direct message did not notify")
	}
	settings.QuietHours.Start = settings.QuietHours.End
	p = New(settings)
	if p.Quiet(at) {
		t.Fatal("equal endpoints should disable quiet hours")
	}
}

func TestChannelLevelIsSharedAndDefaultInheritsPersonalDefault(t *testing.T) {
	at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	settings := Defaults()
	settings.ChannelDefault = "all"
	// An old personal per-channel value must not override the channel's level.
	settings.Levels = map[string]string{"room": "none", "dm": "none"}
	p := New(settings)
	for shared, want := range map[string]string{"": "all", "default": "all", "all": "all", "mentions": "mentions", "none": "none", "bogus": "all"} {
		if got := p.Rule("room", "channel", shared); got != want {
			t.Fatalf("shared %q: rule %q, want %q", shared, got, want)
		}
	}
	if got := p.Notification("room", "channel", "default", "message", "normal chat", "alice", "bob", "u", "h", false, at); got != "message" {
		t.Fatalf("default did not inherit the personal channel default: %q", got)
	}
	if got := p.Notification("room", "channel", "mentions", "message", "normal chat", "alice", "bob", "u", "h", false, at); got != "" {
		t.Fatalf("shared mentions notified a plain message: %q", got)
	}
	if got := p.Notification("room", "channel", "mentions", "message", "hi alice", "alice", "bob", "u", "h", false, at); got != "mention" {
		t.Fatalf("shared mentions missed a mention: %q", got)
	}
	if got := p.Notification("room", "channel", "none", "message", "hi alice", "alice", "bob", "u", "h", false, at); got != "" {
		t.Fatalf("shared mute notified: %q", got)
	}
	// Direct messages keep their personal level.
	if got := p.Rule("dm", "direct", "all"); got != "none" {
		t.Fatalf("direct level %q, want the personal none", got)
	}
}
