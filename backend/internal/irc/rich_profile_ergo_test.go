package irc

import (
	"slices"
	"testing"
	"time"
)

// Exercise the complete metadata key registry against Ergo, including keys in
// later GET and SUB batches, and observe expiry on a second backend.
func TestErgoRichProfileMetadataAndStatusExpiry(t *testing.T) {
	f := newMetadataFixture(t, "rich")
	f.connect(f.bobSide, "bob", f.config(f.bob, ""), true)
	f.connect(f.aliceSide, "alice", f.config(f.alice, ""), true)
	profileChecked(t, f.aliceSide, "alice")
	profileChecked(t, f.bobSide, "bob")
	for _, side := range []struct {
		manager *Manager
		owner   string
	}{{f.aliceSide, "alice"}, {f.bobSide, "bob"}} {
		c, err := side.manager.connection(side.owner, "net")
		if err != nil {
			t.Fatal(err)
		}
		published := c.publishedProfile()
		for _, key := range []string{"ircord/timezone", "ircord/languages", "ircord/interests"} {
			if _, ok := published[key]; !ok {
				t.Fatalf("%s did not complete the profile GET for %s", side.owner, key)
			}
		}
	}
	eventually(t, func() bool { return f.peer().Nick == f.alice })

	languages := []string{"en-us", "da"}
	interests := []string{"IRC", "Music theory"}
	patch := ProfilePatch{
		Pronouns: profileName("they/them"), HomepageURL: profileName("https://example.com/home"),
		StatusText: profileName("Working"), StatusEmoji: profileName("☕"),
		Headline: profileName("Hello from IRC"), BannerURL: profileName("https://example.com/banner.png"),
		ProfileBackgroundURL: profileName("https://example.com/background.png"), ProfileAccentColor: profileName("#123aBc"),
		AvatarFrame: profileName("round-1"), Location: profileName("Copenhagen"), Timezone: profileName("Europe/Copenhagen"),
		Languages: &languages, Interests: &interests,
	}
	if _, err := f.aliceSide.ApplyProfilePatch("alice", "net", patch, nil); err != nil {
		t.Fatal(err)
	}
	wantWire := map[string]string{
		"pronouns": "they/them", "homepage": "https://example.com/home", "status": "Working", "ircord/status-emoji": "☕",
		"ircord/headline": "Hello from IRC", "ircord/banner": "https://example.com/banner.png",
		"ircord/profile-background": "https://example.com/background.png", "ircord/profile-accent": "#123aBc",
		"ircord/avatar-frame": "round-1", "ircord/location": "Copenhagen", "ircord/timezone": "Europe/Copenhagen",
		"ircord/languages": `["en-US","da"]`, "ircord/interests": `["IRC","Music theory"]`,
	}
	for key, want := range wantWire {
		key, want := key, want
		eventually(t, func() bool { return f.server(f.alice, key, want) })
	}
	eventually(t, func() bool {
		v := f.peer()
		return v.Pronouns == "they/them" && v.HomepageURL == "https://example.com/home" &&
			v.StatusText == "Working" && v.StatusEmoji == "☕" && v.Headline == "Hello from IRC" &&
			v.BannerURL == "https://example.com/banner.png" && v.ProfileBackgroundURL == "https://example.com/background.png" &&
			v.ProfileAccentColor == "#123aBc" && v.AvatarFrame == "round-1" && v.Location == "Copenhagen" &&
			v.Timezone == "Europe/Copenhagen" && slices.Equal(v.Languages, []string{"en-US", "da"}) &&
			slices.Equal(v.Interests, []string{"IRC", "Music theory"})
	})

	if err := f.aliceSide.SetAway("alice", "net", "Away for this test"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { v := f.peer(); return v.Status == "away" && v.AwayMessage == "Away for this test" })
	expires := time.Now().Add(5 * time.Second).UTC().Format(time.RFC3339Nano)
	if _, err := f.aliceSide.ApplyProfilePatch("alice", "net", ProfilePatch{StatusText: profileName("Temporary"), StatusEmoji: profileName("🌿"), StatusExpiresAt: &expires}, nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		v := f.peer()
		return v.StatusText == "Temporary" && v.StatusEmoji == "🌿" && v.Status == "away"
	})
	eventually(t, func() bool {
		own, err := f.aliceSide.NetworkProfile("alice", "net")
		v := f.peer()
		return err == nil && own.StatusText == "" && own.StatusEmoji == "" && own.StatusExpiresAt == nil &&
			own.Status == "away" && own.AwayMessage == "Away for this test" && v.StatusText == "" && v.StatusEmoji == "" &&
			v.Status == "away" && v.AwayMessage == "Away for this test"
	})
	if !f.server(f.alice, "status", "") || !f.server(f.alice, "ircord/status-emoji", "") {
		t.Fatal("expired status remained on Ergo")
	}
}
