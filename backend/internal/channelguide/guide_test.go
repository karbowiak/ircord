package channelguide

import (
	"context"
	"strings"
	"testing"

	"ircord/backend/internal/storage"
)

func TestNormalize(t *testing.T) {
	in := Guide{
		Groups: []Group{{Name: "  Games  "}, {Name: "Off\ttopic"}},
		Entries: []Entry{
			{Channel: " general ", State: Everyone, Description: "Say   hi"},
			{Channel: "#eve", Group: "GAMES", State: Suggested},
			{Channel: "#food", State: Listed},
		},
	}
	out, err := Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Groups[0].Name != "Games" || out.Groups[1].Name != "Off topic" {
		t.Fatalf("groups = %+v", out.Groups)
	}
	if e := out.Entries[0]; e.Channel != "#general" || e.Description != "Say hi" {
		t.Fatalf("entry = %+v", e)
	}
	if out.Entries[1].Group != "Games" || out.Entries[2].Group != "" {
		t.Fatalf("entry groups = %+v", out.Entries)
	}
	if got := strings.Join(out.EveryoneChannels(), ","); got != "#general" {
		t.Fatalf("everyone = %s", got)
	}

	refuse := map[string]Guide{
		"duplicate channel": {Entries: []Entry{{Channel: "#a", State: Listed}, {Channel: "#A", State: Listed}}},
		"irc case":          {Entries: []Entry{{Channel: "#a[", State: Listed}, {Channel: "#A{", State: Listed}}},
		"duplicate group":   {Groups: []Group{{Name: "x"}, {Name: "X"}}},
		"empty group":       {Groups: []Group{{Name: " "}}},
		"unknown group":     {Entries: []Entry{{Channel: "#a", Group: "x", State: Listed}}},
		"bad state":         {Entries: []Entry{{Channel: "#a", State: "pinned"}}},
		"bad name":          {Entries: []Entry{{Channel: "#a b", State: Listed}}},
		"control character": {Entries: []Entry{{Channel: "#a", State: Listed, Description: "a\x01b"}}},
		"long description":  {Entries: []Entry{{Channel: "#a", State: Listed, Description: strings.Repeat("x", MaxDescription+1)}}},
	}
	for name, g := range refuse {
		if _, err := Normalize(g); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	many := Guide{}
	for i := range MaxEveryone + 1 {
		many.Entries = append(many.Entries, Entry{Channel: "#c" + strings.Repeat("x", i), State: Everyone})
	}
	if _, err := Normalize(many); err == nil {
		t.Error("too many channels for everyone accepted")
	}
}

func TestLoadFallsBackToDefaultsUntilSaved(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	guide, _, saved, err := Load(ctx, store, []string{"#lobby", "help", "", "#LOBBY"})
	if err != nil || saved || strings.Join(guide.EveryoneChannels(), ",") != "#lobby,#help" {
		t.Fatalf("fallback = %+v %v %v", guide, saved, err)
	}
	if _, err = store.SaveChannelGuide(ctx, []byte(`{"groups":[],"entries":[{"channel":"#news","state":"everyone"},{"channel":"#eve","state":"suggested"}]}`), 0, "owner", nil); err != nil {
		t.Fatal(err)
	}
	guide, stored, saved, err := Load(ctx, store, []string{"#lobby"})
	if err != nil || !saved || stored.Revision != 1 || strings.Join(guide.EveryoneChannels(), ",") != "#news" {
		t.Fatalf("saved = %+v %v %v", guide, saved, err)
	}
	// An owner who saves an empty guide means it: new accounts join nothing.
	if _, err = store.SaveChannelGuide(ctx, []byte(`{"groups":[],"entries":[]}`), 1, "owner", nil); err != nil {
		t.Fatal(err)
	}
	if guide, _, _, _ = Load(ctx, store, []string{"#lobby"}); len(guide.EveryoneChannels()) != 0 {
		t.Fatalf("empty guide = %+v", guide)
	}
	if _, err = store.SaveChannelGuide(ctx, []byte(`{}`), 1, "owner", nil); err != storage.ErrStaleRevision {
		t.Fatalf("stale save = %v", err)
	}
}
