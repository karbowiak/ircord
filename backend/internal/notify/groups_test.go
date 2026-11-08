package notify

import (
	"slices"
	"testing"
	"time"
)

func TestGroupsAreWholeAtWords(t *testing.T) {
	got := Groups("\x02@HERE\x0f, @oper and @voiced: also @everyone! (@Channel) @owner")
	if want := []string{"here", "operator", "voiced", "everyone", "channel", "owner"}; !slices.Equal(got, want) {
		t.Fatalf("groups %v, want %v", got, want)
	}
	for _, text := range []string{"here", "everyone look", "@heresy", "mail@here", "@here-ish", "@opera", "@channel_1", "@voicedx"} {
		if got := Groups(text); len(got) != 0 {
			t.Fatalf("%q matched %v", text, got)
		}
	}
}

func TestRankForUsesNetworkPrefix(t *testing.T) {
	for _, tc := range []struct{ modes, prefix, want string }{
		{"", "", ""},
		{"+", "", "voiced"},
		{"@+", "", "operator"},
		{"~", "(qaohv)~&@%+", "owner"},
		{"&", "", "admin"},
		{"%", "", "halfop"},
		{"@", "(ov)@+", "operator"},
		// "~" means nothing on a network that does not define it.
		{"~", "(ov)@+", ""},
	} {
		if got := RankFor(tc.modes, tc.prefix); got != tc.want {
			t.Fatalf("RankFor(%q, %q) = %q, want %q", tc.modes, tc.prefix, got, tc.want)
		}
	}
}

func TestGroupMentionedFollowsTheRecipientsOwnView(t *testing.T) {
	here := Recipient{Nick: "alice", Channel: true, GroupMentions: "all"}
	away := Recipient{Nick: "alice", Channel: true, Away: true, GroupMentions: "all"}
	for _, tc := range []struct {
		text string
		r    Recipient
		want bool
	}{
		{"@here deploy", here, true},
		{"@channel deploy", here, true},
		{"@here deploy", away, false},
		{"@channel deploy", away, false},
		{"@everyone deploy", away, true},
		{"@everyone deploy", here, true},
		// Never in direct messages.
		{"@everyone", Recipient{Nick: "alice", GroupMentions: "all"}, false},
		// Roles include the ranks above them.
		{"@voiced", Recipient{Channel: true, Rank: "voiced"}, true},
		{"@voiced", Recipient{Channel: true, Rank: "halfop"}, true},
		{"@voiced", Recipient{Channel: true, Rank: "owner"}, true},
		{"@voiced", Recipient{Channel: true}, false},
		{"@operator", Recipient{Channel: true, Rank: "operator"}, true},
		{"@oper", Recipient{Channel: true, Rank: "admin"}, true},
		{"@oper", Recipient{Channel: true, Rank: "owner"}, true},
		{"@operator", Recipient{Channel: true, Rank: "halfop"}, false},
		{"@operator", Recipient{Channel: true, Rank: "voiced"}, false},
		{"@owner", Recipient{Channel: true, Rank: "owner"}, true},
		{"@owner", Recipient{Channel: true, Rank: "admin"}, false},
		// Role mentions ignore away: they name a rank, not who is around.
		{"@owner", Recipient{Channel: true, Away: true, Rank: "owner"}, true},
		{"just chatting", here, false},
	} {
		if got := GroupMentioned(tc.text, tc.r); got != tc.want {
			t.Fatalf("GroupMentioned(%q, %+v) = %v, want %v", tc.text, tc.r, got, tc.want)
		}
	}
}

func TestChannelSettingGatesChannelWideMentions(t *testing.T) {
	for _, tc := range []struct {
		setting  string
		operator bool
		want     bool
	}{
		{"all", false, true},
		{"all", true, true},
		{"ops", false, false},
		{"ops", true, true},
		// Channels whose settings are not known yet behave as "ops".
		{"", false, false},
		{"", true, true},
		{"off", false, false},
		{"off", true, false},
	} {
		for _, text := range []string{"@here", "@channel", "@everyone"} {
			r := Recipient{Channel: true, GroupMentions: tc.setting, SenderOperator: tc.operator}
			if got := GroupMentioned(text, r); got != tc.want {
				t.Fatalf("%s with %q, operator %v: %v, want %v", text, tc.setting, tc.operator, got, tc.want)
			}
		}
		// Role mentions stay open whatever the setting.
		if !GroupMentioned("@voiced", Recipient{Channel: true, Rank: "voiced", GroupMentions: tc.setting, SenderOperator: tc.operator}) {
			t.Fatalf("@voiced gated by %q", tc.setting)
		}
	}
}

func TestGroupMentionNotifiesLikeAPersonalMention(t *testing.T) {
	p := New(Settings{Ignore: []string{"bot1"}})
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	me := Recipient{Nick: "alice", Channel: true, SenderOperator: true}
	if got := p.Notification("room", "channel", "", "message", "@here standup", me, "bob", "u", "h", false, at); got != "mention" {
		t.Fatalf("@here: %q", got)
	}
	if got := p.Notification("room", "channel", "", "message", "@here standup", me, "alice", "u", "h", true, at); got != "" {
		t.Fatalf("own message notified: %q", got)
	}
	if got := p.Notification("room", "channel", "", "message", "@here standup", me, "bot1", "u", "h", false, at); got != "" {
		t.Fatalf("ignored sender notified: %q", got)
	}
	if got := p.Notification("room", "channel", "none", "message", "@everyone standup", me, "bob", "u", "h", false, at); got != "" {
		t.Fatalf("muted channel notified: %q", got)
	}
	if got := p.Attention("room", "channel", "", "message", "@everyone standup", me, "bob", "u", "h", false, false); !slices.Equal(got, []string{"mention"}) {
		t.Fatalf("attention: %v", got)
	}
	if got := p.Notification("room", "channel", "", "message", "@operator help", me, "bob", "u", "h", false, at); got != "" {
		t.Fatalf("@operator reached a regular member: %q", got)
	}
}
