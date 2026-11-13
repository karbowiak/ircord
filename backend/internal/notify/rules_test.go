package notify

import (
	"testing"
	"time"
)

func TestAutoAwayDefaultAndSavedValues(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
	}{
		{`{}`, 30},
		{`{"autoAwayDelay":0}`, 0},
		{`{"autoAwayDelay":600}`, 600},
		{`{"autoAwayDelay":47}`, 47},
	} {
		if got := Compile([]byte(tc.raw)).Settings.AutoAwayDelay; got != tc.want {
			t.Errorf("settings %s: got %d, want %d", tc.raw, got, tc.want)
		}
	}
}

func TestRulesQuietHoursHighlightsAndIgnore(t *testing.T) {
	settings := Defaults()
	settings.HighlightWords = []string{"deploy"}
	settings.Ignore = []string{"bot*", "*!bad@*.example"}
	settings.QuietHours = QuietHours{Enabled: true, Start: "22:00", End: "07:00", TimeZone: "Europe/Copenhagen"}
	p := New(settings)
	alice := Recipient{Nick: "alice", Channel: true}
	if !p.Mentioned("\x02@Alice\x0f, hi", alice) || p.Mentioned("malice", alice) || p.Mentioned("Alice, hi", alice) || !p.Mentioned("a deploy!", alice) {
		t.Fatal("whole-word formatting highlights differ from browser")
	}
	if !p.Ignored("robot", "bad", "host.example") || !p.Ignored("bot42", "user", "host") || p.Ignored("alice", "good", "host.example") {
		t.Fatal("IRC mask evaluation failed")
	}
	at := time.Date(2026, 11, 7, 22, 30, 0, 0, time.UTC)
	if !p.Quiet(at) || p.Quiet(at.Add(-8*time.Hour)) {
		t.Fatal("quiet hours did not use local time across midnight")
	}
	if got := p.Notification("room", "direct", "", "message", "hello", Recipient{Nick: "alice"}, "bob", "u", "h", false, at); got != "" {
		t.Fatal("quiet hours notified")
	}
	at = at.Add(-8 * time.Hour)
	if got := p.Notification("room", "channel", "", "message", "a deploy!", Recipient{Nick: "alice"}, "bob", "u", "h", false, at); got != "mention" {
		t.Fatal("highlight did not notify")
	}
	if got := p.Notification("room", "channel", "", "message", "normal chat", Recipient{Nick: "alice"}, "bob", "u", "h", false, at); got != "" {
		t.Fatal("ordinary channel message notified with mentions-only")
	}
	if got := p.Notification("room", "direct", "", "message", "normal chat", Recipient{Nick: "alice"}, "bob", "u", "h", false, at); got != "mention" {
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
	if got := p.Notification("room", "channel", "default", "message", "normal chat", Recipient{Nick: "alice"}, "bob", "u", "h", false, at); got != "message" {
		t.Fatalf("default did not inherit the personal channel default: %q", got)
	}
	if got := p.Notification("room", "channel", "mentions", "message", "normal chat", Recipient{Nick: "alice"}, "bob", "u", "h", false, at); got != "" {
		t.Fatalf("shared mentions notified a plain message: %q", got)
	}
	if got := p.Notification("room", "channel", "mentions", "message", "hi @alice", Recipient{Nick: "alice"}, "bob", "u", "h", false, at); got != "mention" {
		t.Fatalf("shared mentions missed a mention: %q", got)
	}
	if got := p.Notification("room", "channel", "none", "message", "hi @alice", Recipient{Nick: "alice"}, "bob", "u", "h", false, at); got != "" {
		t.Fatalf("shared mute notified: %q", got)
	}
	// Direct messages keep their personal level.
	if got := p.Rule("dm", "direct", "all"); got != "none" {
		t.Fatalf("direct level %q, want the personal none", got)
	}
}

func TestAttentionKeepsQuietHoursButNotMutesOrIgnores(t *testing.T) {
	settings := Defaults()
	settings.HighlightWords = []string{"deploy"}
	settings.Ignore = []string{"bot*"}
	settings.Levels = map[string]string{"muted-dm": "none"}
	settings.QuietHours = QuietHours{Enabled: true, Start: "00:00", End: "23:59", TimeZone: "UTC"}
	p := New(settings)
	eq := func(got []string, want ...string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	if got := p.Attention("room", "channel", "", "message", "alice: the deploy", Recipient{Nick: "alice"}, "bob", "u", "h", false, false); !eq(got, "mention") {
		t.Fatalf("mention during quiet hours: %v", got)
	}
	if got := p.Attention("dm", "direct", "", "message", "hi @alice", Recipient{Nick: "alice"}, "bob", "u", "h", false, true); !eq(got, "mention", "reply", "direct") {
		t.Fatalf("all reasons: %v", got)
	}
	if got := p.Attention("room", "channel", "", "action", "waves", Recipient{Nick: "alice"}, "bob", "u", "h", false, false); got != nil {
		t.Fatalf("plain channel message: %v", got)
	}
	for name, got := range map[string][]string{
		"self":         p.Attention("dm", "direct", "", "message", "alice", Recipient{Nick: "alice"}, "alice", "u", "h", true, false),
		"ignored":      p.Attention("dm", "direct", "", "message", "alice", Recipient{Nick: "alice"}, "bot1", "u", "h", false, false),
		"event":        p.Attention("room", "channel", "", "event", "alice joined", Recipient{Nick: "alice"}, "bob", "u", "h", false, false),
		"muted dm":     p.Attention("muted-dm", "direct", "", "message", "alice", Recipient{Nick: "alice"}, "bob", "u", "h", false, false),
		"muted shared": p.Attention("room", "channel", "none", "message", "alice", Recipient{Nick: "alice"}, "bob", "u", "h", false, true),
	} {
		if got != nil {
			t.Fatalf("%s recorded: %v", name, got)
		}
	}
}
