package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type mentionFixtures struct {
	Nick []struct {
		Text        string `json:"text"`
		Nick        string `json:"nick"`
		CaseMapping string `json:"caseMapping"`
		Want        bool   `json:"want"`
	} `json:"nick"`
	Highlight []struct {
		Text  string   `json:"text"`
		Nick  string   `json:"nick"`
		Words []string `json:"words"`
		Want  bool     `json:"want"`
	} `json:"highlight"`
}

func loadMentionFixtures(t *testing.T) mentionFixtures {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "mentions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f mentionFixtures
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// testdata/mentions.json is the contract the browser rule (frontend/shared/mentions.ts) follows too.
func TestMentionFixtures(t *testing.T) {
	f := loadMentionFixtures(t)
	for _, tc := range f.Nick {
		if got := NickMentioned(tc.Text, tc.Nick, tc.CaseMapping); got != tc.Want {
			t.Errorf("NickMentioned(%q, %q, %q) = %v, want %v", tc.Text, tc.Nick, tc.CaseMapping, got, tc.Want)
		}
	}
	for _, tc := range f.Highlight {
		settings := Defaults()
		settings.HighlightWords = tc.Words
		if got := New(settings).Mentioned(tc.Text, Recipient{Nick: tc.Nick, Channel: true}); got != tc.Want {
			t.Errorf("Mentioned(%q) for %q with %q = %v, want %v", tc.Text, tc.Nick, tc.Words, got, tc.Want)
		}
	}
}

func TestNameMentionsNeedAnAt(t *testing.T) {
	at := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	p := New(Defaults())
	alice := Recipient{Nick: "alice", Channel: true}
	notify := func(kind, shared, text string, r Recipient, sender string, self bool) string {
		return p.Notification("room", kind, shared, "message", text, r, sender, "u", "h", self, at)
	}
	if got := notify("channel", "", "Alice is working on it", alice, "bob", false); got != "" {
		t.Fatalf("a bare name notified: %q", got)
	}
	if got := notify("channel", "", "hey @alice", alice, "bob", false); got != "mention" {
		t.Fatalf("@alice did not notify: %q", got)
	}
	if got := notify("channel", "all", "Alice is working on it", alice, "bob", false); got != "message" {
		t.Fatalf("a bare name in an All channel: %q, want message", got)
	}
	if got := notify("direct", "", "Alice, are you there?", Recipient{Nick: "alice"}, "bob", false); got != "mention" {
		t.Fatalf("a direct message without @ did not notify: %q", got)
	}
	if got := notify("channel", "", "@alice", alice, "alice", true); got != "" {
		t.Fatalf("own message notified: %q", got)
	}
	if got := p.Attention("room", "channel", "", "message", "Alice, see above", alice, "bob", "u", "h", false, true); len(got) != 1 || got[0] != "reply" {
		t.Fatalf("a reply that names alice without @: %v, want only reply", got)
	}
	if got := p.Attention("room", "channel", "", "message", "Alice, see above", alice, "bob", "u", "h", false, false); got != nil {
		t.Fatalf("a bare name reached the inbox: %v", got)
	}
	ignoring := Defaults()
	ignoring.Ignore = []string{"bob"}
	if got := New(ignoring).Notification("room", "channel", "", "message", "@alice", alice, "bob", "u", "h", false, at); got != "" {
		t.Fatalf("ignored sender's @mention notified: %q", got)
	}
	// The network's CASEMAPPING decides which spellings are the same nick.
	if !p.Mentioned("@{alice}", Recipient{Nick: "[alice]", Channel: true}) || p.Mentioned("@{alice}", Recipient{Nick: "[alice]", Channel: true, CaseMapping: "ascii"}) {
		t.Fatal("casemapping not applied to @mentions")
	}
}

// Each account's rules are its own: one account's highlight words do not
// highlight another account, and a message names each account by its own nick.
func TestMentionsArePerRecipient(t *testing.T) {
	withWord := Defaults()
	withWord.HighlightWords = []string{"alice"}
	alicePolicy, bobPolicy := New(withWord), New(Defaults())
	alice, bob := Recipient{Nick: "alice", Channel: true}, Recipient{Nick: "bob", Channel: true}
	text := "alice and bob, lunch?"
	if !alicePolicy.Mentioned(text, alice) || bobPolicy.Mentioned(text, bob) {
		t.Fatal("highlight words were not personal")
	}
	if bobPolicy.Mentioned("@alice", bob) || !bobPolicy.Mentioned("@alice @bob", bob) {
		t.Fatal("an @mention of one account highlighted another")
	}
}
