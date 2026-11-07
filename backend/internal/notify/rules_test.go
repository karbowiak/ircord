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
	if got := p.Notification("room", "direct", "message", "hello", "alice", "bob", "u", "h", false, at); got != "" {
		t.Fatal("quiet hours notified")
	}
	at = at.Add(-8 * time.Hour)
	if got := p.Notification("room", "channel", "message", "a deploy!", "alice", "bob", "u", "h", false, at); got != "mention" {
		t.Fatal("highlight did not notify")
	}
	if got := p.Notification("room", "channel", "message", "normal chat", "alice", "bob", "u", "h", false, at); got != "" {
		t.Fatal("ordinary channel message notified with mentions-only")
	}
	if got := p.Notification("room", "direct", "message", "normal chat", "alice", "bob", "u", "h", false, at); got != "mention" {
		t.Fatal("direct message did not notify")
	}
	settings.QuietHours.Start = settings.QuietHours.End
	p = New(settings)
	if p.Quiet(at) {
		t.Fatal("equal endpoints should disable quiet hours")
	}
}
