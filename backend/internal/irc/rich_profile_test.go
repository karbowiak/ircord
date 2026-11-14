package irc

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

func TestRichProfileRoundTripAndValidation(t *testing.T) {
	m, _ := storedProfileNetworks(t, false)
	languages := []string{"en-us", "EN-US", "da"}
	interests := []string{"IRC", "irc", "Music"}
	p := ProfilePatch{Pronouns: profileName("they/them"), HomepageURL: profileName("https://example.com/"), StatusText: profileName("Busy"), StatusEmoji: profileName("☕"), Headline: profileName("Hello"), BannerURL: profileName("https://example.com/banner"), ProfileBackgroundURL: profileName("https://example.com/background"), ProfileAccentColor: profileName("#123aBc"), AvatarFrame: profileName("round-1"), Location: profileName("Copenhagen"), Timezone: profileName("Europe/Copenhagen"), Languages: &languages, Interests: &interests}
	got, err := m.ApplyProfilePatch("alice", "one", p, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"pronouns": "they/them", "homepage": "https://example.com/", "status": "Busy", "ircord/status-emoji": "☕", "ircord/headline": "Hello", "ircord/banner": "https://example.com/banner", "ircord/profile-background": "https://example.com/background", "ircord/profile-accent": "#123aBc", "ircord/avatar-frame": "round-1", "ircord/location": "Copenhagen", "ircord/timezone": "Europe/Copenhagen", "ircord/languages": "[\"en-US\",\"da\"]", "ircord/interests": "[\"IRC\",\"Music\"]"}
	values := profileValues(got)
	for key, value := range want {
		if values[key] != value {
			t.Errorf("%s = %q, want %q", key, values[key], value)
		}
	}
	languages[0] = "fr"
	got.Languages[0] = "de"
	again, _ := m.NetworkProfile("alice", "one")
	if again.Languages[0] != "en-US" {
		t.Fatal("list ownership leaked", again.Languages)
	}
	other, _ := m.NetworkProfile("alice", "two")
	if other.Pronouns != "" {
		t.Fatal("other network changed", other)
	}
	if _, err = m.NetworkProfile("bob", "one"); err == nil {
		t.Fatal("foreign account read succeeded")
	}
	if _, err = m.ApplyProfilePatch("alice", "one", ProfilePatch{Location: profileName("Elsewhere")}, nil); err != nil {
		t.Fatal(err)
	}
	again, _ = m.NetworkProfile("alice", "one")
	if again.Pronouns != "they/them" {
		t.Fatal("omitted field cleared")
	}
	invalid := []ProfilePatch{{Pronouns: profileName(strings.Repeat("é", 41))}, {StatusText: profileName("bad\x01")}, {Headline: profileName(string([]byte{0xff}))}, {HomepageURL: profileName("https://user:pass@example.com/")}, {BannerURL: profileName("javascript:alert(1)")}, {ProfileAccentColor: profileName("#zzzzzz")}, {AvatarFrame: profileName("bad_frame")}, {Timezone: profileName("Mars/Olympus")}, {Timezone: profileName("Local")}, {Languages: &[]string{"not_a_tag"}}, {Interests: &[]string{strings.Repeat("x", 33)}}}
	for _, bad := range invalid {
		bad.Pronouns = firstString(bad.Pronouns, profileName("changed"))
		if _, err := m.ApplyProfilePatch("alice", "one", bad, nil); err == nil {
			t.Errorf("accepted invalid patch %#v", bad)
		}
	}
	after, _ := m.NetworkProfile("alice", "one")
	if after.Pronouns != "they/them" {
		t.Fatal("invalid patch applied partially", after)
	}
	for key := range want {
		var v Member
		if !applyProfileMetadata(&v, key, want[key]) {
			t.Errorf("rejected metadata %s", key)
			continue
		}
		if !clearMetadata(&v, key) {
			t.Errorf("did not clear %s", key)
		}
		if profileValues(v)[key] != "" {
			t.Errorf("clear retained %s", key)
		}
	}
	if applyProfileMetadata(&after, "ircord/profile-accent", "#zzzzzz") || applyProfileMetadata(&after, "ircord/languages", "[\"invalid_tag\"]") {
		t.Fatal("accepted invalid incoming metadata")
	}
}

func firstString(a, b *string) *string {
	if a != nil {
		return a
	}
	return b
}

func TestRichProfilePublicationAndExpiry(t *testing.T) {
	m, store := storedProfileNetworks(t, false)
	c := profileConnection(m, "alice", "one")
	expires := time.Now().Add(80 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusText: profileName("Old"), StatusEmoji: profileName("☕"), StatusExpiresAt: &expires}, nil); err != nil {
		t.Fatal(err)
	}
	got := queuedProfile(c)
	if !maps.Equal(got, map[string]string{"status": "Old", "ircord/status-emoji": "☕"}) {
		t.Fatal("unexpected changed keys", got)
	}
	for key, value := range got {
		c.recordPublished(map[string]string{key: value})
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		p, _ := m.NetworkProfile("alice", "one")
		if p.StatusExpiresAt == nil {
			if p.StatusText != "" || p.StatusEmoji != "" {
				t.Fatal("status did not expire", p)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	p, _ := m.NetworkProfile("alice", "one")
	if p.StatusExpiresAt != nil || p.StatusText != "" {
		t.Fatal("expiry did not run", p)
	}
	if got := queuedProfile(c); !maps.Equal(got, map[string]string{"status": "", "ircord/status-emoji": ""}) {
		t.Fatal("expiry did not publish clears", got)
	}
	fresh := New(Options{Store: store, AllowPrivate: true})
	defer fresh.Close()
	restored, err := fresh.NetworkProfile("alice", "one")
	if err != nil || restored.StatusExpiresAt != nil || restored.StatusText != "" {
		t.Fatal("expiry not durable", restored, err)
	}
}

func TestRichProfileExpiryReplacementAndExternalStatus(t *testing.T) {
	m, _ := storedProfileNetworks(t, false)
	c := profileConnection(m, "alice", "one")
	first := time.Now().Add(100 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	later := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusText: profileName("old"), StatusExpiresAt: &first}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusText: profileName("new"), StatusExpiresAt: &later}, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(180 * time.Millisecond)
	p, _ := m.NetworkProfile("alice", "one")
	if p.StatusText != "new" || p.StatusExpiresAt == nil {
		t.Fatal("old deadline cleared replacement", p)
	}
	c.metadata(*girc.ParseEvent(":irc.test METADATA onenick status * :external"))
	p, _ = m.NetworkProfile("alice", "one")
	if p.StatusText != "external" || p.StatusExpiresAt != nil {
		t.Fatal("external status did not cancel deadline", p)
	}
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusExpiresAt: &later}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusEmoji: profileName("!")}, nil); err != nil {
		t.Fatal(err)
	}
	p, _ = m.NetworkProfile("alice", "one")
	if p.StatusText != "external" || p.StatusExpiresAt != nil {
		t.Fatal("status edit did not cancel old deadline", p)
	}
}

func TestRichProfileExpiryRetriesStorageFailure(t *testing.T) {
	m, store := storedProfileNetworks(t, false)
	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusText: profileName("saved"), StatusExpiresAt: &expires}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TRIGGER reject_profile_expiry BEFORE UPDATE ON entities WHEN NEW.kind = 'network' BEGIN SELECT RAISE(FAIL, 'profile expiry test'); END`); err != nil {
		t.Fatal(err)
	}
	w := m.space("alice")
	w.mu.Lock()
	next := w.expireProfiles(time.Now().Add(2 * time.Hour))
	p := w.networkSelf("one")
	failed := w.profileExpiryStorageErr
	w.mu.Unlock()
	if !failed || next.IsZero() || p.StatusText != "saved" || p.StatusExpiresAt == nil {
		t.Fatal("failed expiry lost state or retry", p, next)
	}
	if _, err := store.DB.Exec(`DROP TRIGGER reject_profile_expiry`); err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.expireProfiles(time.Now().Add(2 * time.Hour))
	p = w.networkSelf("one")
	err := w.storageErr
	w.mu.Unlock()
	if err != nil || p.StatusText != "" || p.StatusExpiresAt != nil {
		t.Fatal("expiry retry failed", p, err)
	}
	fresh := New(Options{Store: store, AllowPrivate: true})
	defer fresh.Close()
	p, err = fresh.NetworkProfile("alice", "one")
	if err != nil || p.StatusText != "" || p.StatusExpiresAt != nil {
		t.Fatal("retry was not durable", p, err)
	}
}

func TestRichProfileExpiryShutdownAndOwnerRemoval(t *testing.T) {
	m, store := storedProfileNetworks(t, false)
	expires := time.Now().Add(150 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusText: profileName("keep"), StatusExpiresAt: &expires}, nil); err != nil {
		t.Fatal(err)
	}
	if err := m.DropOwner("alice"); err != nil {
		t.Fatal(err)
	}
	// Leave the manager running past the old deadline to test deletion fencing.
	time.Sleep(200 * time.Millisecond)
	done := make(chan struct{})
	go func() { m.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not join expiry worker")
	}
	fresh := New(Options{Store: store, AllowPrivate: true})
	defer fresh.Close()
	// A manager that has not restored or loaded this owner cannot mutate it.
	entities, err := store.Entities("alice")
	if err != nil || len(entities["network"]) == 0 {
		t.Fatal("removed workspace lost its durable record", err)
	}
	var network Network
	for _, raw := range entities["network"] {
		if json.Unmarshal(raw, &network) == nil && network.ID == "one" {
			break
		}
	}
	if network.Profile == nil || network.Profile.StatusText != "keep" {
		t.Fatal("removed workspace expired after removal", network)
	}
}

func TestRichProfileExpiryShutdownWithoutDeadlines(t *testing.T) {
	m := New(Options{})
	m.space("owner")
	done := make(chan struct{})
	go func() { m.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("idle expiry worker did not finish")
	}
}

func TestRichProfileExpiryDoesNotCrossIdentity(t *testing.T) {
	m, _ := storedProfileNetworks(t, false)
	expires := time.Now().Add(100 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusText: profileName("old"), StatusExpiresAt: &expires}, nil); err != nil {
		t.Fatal(err)
	}
	w := m.space("alice")
	w.mu.Lock()
	cfg := w.configs["one"]
	cfg.Host = "different.invalid"
	n := w.networks["one"]
	if err := w.prepareNetworkProfile(&n, cfg); err != nil {
		w.mu.Unlock()
		t.Fatal(err)
	}
	w.configs["one"], w.networks["one"] = cfg, n
	w.mu.Unlock()
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusText: profileName("replacement")}, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(180 * time.Millisecond)
	p, err := m.NetworkProfile("alice", "one")
	if err != nil || p.StatusText != "replacement" || p.StatusExpiresAt != nil {
		t.Fatal("old deadline crossed identity", p, err)
	}
}

func TestRichProfileExpiryAfterRestore(t *testing.T) {
	m, store := storedProfileNetworks(t, false)
	expires := time.Now().Add(100 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusText: profileName("temporary"), StatusExpiresAt: &expires}, nil); err != nil {
		t.Fatal(err)
	}
	m.Close()
	time.Sleep(150 * time.Millisecond)
	fresh := New(Options{Store: store, AllowPrivate: true})
	defer fresh.Close()
	if err := fresh.Restore(); err != nil {
		t.Fatal(err)
	}
	limit := time.Now().Add(2 * time.Second)
	for time.Now().Before(limit) {
		p, err := fresh.NetworkProfile("alice", "one")
		if err != nil {
			t.Fatal(err)
		}
		if p.StatusExpiresAt == nil && p.StatusText == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("restored expired status was not cleared")
}

func TestLegacyProfileExpiryRestoresWithoutNetworks(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.CreateAccount("alice", "profile-expiry-alice"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store})
	expires := time.Now().Add(100 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	if _, err := m.ApplyProfilePatch("alice", "", ProfilePatch{StatusText: profileName("temporary"), StatusExpiresAt: &expires}, nil); err != nil {
		t.Fatal(err)
	}
	m.Close()
	time.Sleep(150 * time.Millisecond)
	fresh := New(Options{Store: store})
	defer fresh.Close()
	if err := fresh.Restore(); err != nil {
		t.Fatal(err)
	}
	limit := time.Now().Add(2 * time.Second)
	for time.Now().Before(limit) {
		entities, err := store.Entities("alice")
		if err != nil {
			t.Fatal(err)
		}
		var p Member
		for _, raw := range entities["member"] {
			if err := json.Unmarshal(raw, &p); err != nil {
				t.Fatal(err)
			}
		}
		if p.StatusExpiresAt == nil && p.StatusText == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("legacy account deadline was not restored")
}

func TestRichProfileWireBudgets(t *testing.T) {
	m, _ := storedProfileNetworks(t, false)
	c := profileConnection(m, "alice", "one")
	url := "https://example.com/" + strings.Repeat("x", 300-len("https://example.com/"))
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{ProfileBackgroundURL: &url}, nil); err != nil {
		t.Fatal(err)
	}
	if got := queuedProfile(c); got["ircord/profile-background"] != url {
		t.Fatal("maximum URL did not fit default line", got)
	}
	c.mu.Lock()
	c.lineBytes = safeMessageLineBytes(girc.DefaultMaxLineLength - girc.DefaultMaxPrefixLength)
	c.mu.Unlock()
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{BannerURL: &url}, nil); err != nil {
		t.Fatal(err)
	}
	if got := queuedProfile(c); got["ircord/banner"] != url {
		t.Fatal("maximum URL did not fit ordinary IRC budget", got)
	}
	interests := []string{}
	for i := range 10 {
		interests = append(interests, fmt.Sprintf("%d%s", i, strings.Repeat("x", 25)))
	}
	text := strings.Repeat("s", 200)
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{StatusText: &text, Interests: &interests}, nil); err != nil {
		t.Fatal(err)
	}
	if got := queuedProfile(c); got["status"] != text || len(got["ircord/interests"]) > 300 || got["ircord/interests"] == "" {
		t.Fatal("bounded values did not fit default line", got)
	}
	for _, command := range []string{"GET", "SUB"} {
		for _, event := range metadataKeyEvents(command, subscribedKeys()) {
			if len(event.Params) > 15 || len(event.String()) > 510 {
				t.Fatal("metadata key batch exceeds IRC bounds", event)
			}
		}
	}
	c.mu.Lock()
	c.lineBytes = 100
	c.mu.Unlock()
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{HomepageURL: &url}, nil); err == nil {
		t.Fatal("small line accepted an unpublishable edit")
	}
	p, _ := m.NetworkProfile("alice", "one")
	if p.HomepageURL != "" {
		t.Fatal("rejected wire value was persisted", p)
	}
}

func TestRichProfileReconcileAndChangedFieldPublication(t *testing.T) {
	m, _ := storedProfileNetworks(t, false)
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{Headline: profileName("offline")}, nil); err != nil {
		t.Fatal(err)
	}
	c := profileConnection(m, "alice", "one")
	c.recordPublished(map[string]string{"ircord/headline": "previous"})
	answerProfileCheck(c, map[string]string{"ircord/headline": "another client", "ircord/languages": `["en-US"]`})
	if len(c.queue) != 0 {
		t.Fatal("reconciliation overwrote a newer upstream value")
	}
	p, err := m.NetworkProfile("alice", "one")
	if err != nil || p.Headline != "another client" || len(p.Languages) != 1 || p.Languages[0] != "en-US" {
		t.Fatal("upstream profile was not adopted", p, err)
	}
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{Pronouns: profileName("they/them")}, nil); err != nil {
		t.Fatal(err)
	}
	if got := queuedProfile(c); !maps.Equal(got, map[string]string{"pronouns": "they/them"}) {
		t.Fatal("edit published unrelated fields", got)
	}
	before := p
	if applyProfileMetadata(&p, "ircord/languages", `["en_US"]`) || !equal(before, p) {
		t.Fatal("malformed metadata corrupted member", p)
	}
}
