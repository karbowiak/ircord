package irc

import (
	"encoding/json"
	"errors"
	"ircord/backend/internal/testsupport"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

// settingsChannel creates only local state; it never opens an IRC connection.
func settingsChannel(t *testing.T, m *Manager, owner, id, host, network, modes string, caps map[string]bool) Conversation {
	t.Helper()
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	c := &connection{owner: w, manager: m, config: NetworkConfig{ID: id, Host: host, Port: 6697, Nick: owner}, caps: caps}
	w.connections[id] = c
	w.networks[id] = Network{ID: id, Host: host, Port: 6697, Nick: owner, ISupport: map[string]string{"NETWORK": network}}
	conv := c.conversation("#dev")
	conv.Joined = true
	conv.MemberModes = map[string]string{owner: modes}
	c.applyChannelSettings(&conv)
	w.conversations[conv.ID] = conv
	w.emit("conversation.updated", conv)
	if w.storageErr != nil {
		t.Fatal(w.storageErr)
	}
	return conv
}

func TestNSFWChannelSettingsRightsSharedEventsAndRestart(t *testing.T) {
	f := newInboxFixture(t)
	op := settingsChannel(t, f.m, "alice", "op", "irc.example.test", "Example", "@", nil)
	member := settingsChannel(t, f.m, "bob", "member", "irc.example.test", "Example", "", nil)
	if op.ChannelSettings.NSFW || member.ChannelSettings.NSFW {
		t.Fatal("new channels are not default-off")
	}
	yes, no := true, false
	if _, err := f.m.SetChannelSettings("bob", member.ID, ChannelSettingsInput{NSFW: &yes}); !errors.Is(err, ErrNotChannelOperator) {
		t.Fatalf("ordinary member changed classification: %v", err)
	}
	if _, err := f.m.SetChannelSettings("bob", op.ID, ChannelSettingsInput{NSFW: &yes}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another account changed a foreign conversation: %v", err)
	}
	cursor := f.m.Snapshot("bob").Cursor
	changed, err := f.m.SetChannelSettings("alice", op.ID, ChannelSettingsInput{NSFW: &yes})
	if err != nil || changed.ChannelSettings == nil || !changed.ChannelSettings.NSFW || changed.ChannelSettings.Notifications != "default" || changed.ChannelSettings.GroupMentions != "ops" {
		t.Fatalf("operator update: %+v, %v", changed, err)
	}
	if got := f.m.Snapshot("bob").Conversations[0].ChannelSettings; got == nil || !got.NSFW || got.UpdatedAt == nil {
		t.Fatalf("member did not get the shared flag: %+v", got)
	}
	events, _, valid, stop := f.m.Subscribe("bob", cursor)
	stop()
	if !valid || len(events) != 1 || events[0].Type != "conversation.updated" || !strings.Contains(string(events[0].Data), `"nsfw":true`) {
		t.Fatalf("private classification replay: %+v, valid=%v", events, valid)
	}
	if strings.Contains(string(events[0].Data), "_channelIdentity") || strings.Contains(string(events[0].Data), "irc.example.test") {
		t.Fatal("internal canonical route leaked into public conversation JSON")
	}
	stamp := changed.ChannelSettings.UpdatedAt
	if same, err := f.m.SetChannelSettings("alice", op.ID, ChannelSettingsInput{NSFW: &yes}); err != nil || !same.ChannelSettings.UpdatedAt.Equal(*stamp) {
		t.Fatalf("same flag was written again: %+v, %v", same, err)
	}
	level := "mentions"
	if changed, err = f.m.SetChannelSettings("alice", op.ID, ChannelSettingsInput{Notifications: &level}); err != nil || !changed.ChannelSettings.NSFW {
		t.Fatalf("partial update lost classification: %+v, %v", changed, err)
	}
	if changed, err = f.m.SetChannelSettings("alice", op.ID, ChannelSettingsInput{NSFW: &no}); err != nil || changed.ChannelSettings.NSFW || changed.ChannelSettings.Notifications != level {
		t.Fatalf("explicit false did not preserve other settings: %+v, %v", changed, err)
	}
	if _, err = f.m.SetChannelSettings("alice", op.ID, ChannelSettingsInput{NSFW: &yes}); err != nil {
		t.Fatal(err)
	}
	// Reopen the real durable store, without restoring any IRC connections.
	f.m.Close()
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := New(Options{Store: reopened})
	defer restarted.Close()
	for owner, conv := range map[string]Conversation{"alice": op, "bob": member} {
		if nsfw, known := restarted.linkSourceNSFW(restarted.space(owner), conv.ID, conv.NetworkID); !known || !nsfw {
			t.Fatalf("%s lost canonical shared classification after restart: nsfw=%v known=%v", owner, nsfw, known)
		}
	}
}

func TestNSFWChannelSettingsNetworkAndBouncerIsolation(t *testing.T) {
	f := newInboxFixture(t)
	op := settingsChannel(t, f.m, "alice", "base", "irc.example.test", "Example", "@", nil)
	others := []Conversation{
		settingsChannel(t, f.m, "bob", "host", "irc.other.test", "Example", "@", nil),
		settingsChannel(t, f.m, "bob", "network", "irc.example.test", "Another", "@", nil),
		settingsChannel(t, f.m, "bob", "bouncer", "irc.example.test", "Example", "@", map[string]bool{"soju.im/bouncer-networks": true}),
	}
	yes := true
	if _, err := f.m.SetChannelSettings("alice", op.ID, ChannelSettingsInput{NSFW: &yes}); err != nil {
		t.Fatal(err)
	}
	for _, conv := range others {
		if nsfw, known := f.m.linkSourceNSFW(f.m.space("bob"), conv.ID, conv.NetworkID); !known || nsfw {
			t.Fatalf("same-named source on %s borrowed another route: nsfw=%v known=%v", conv.NetworkID, nsfw, known)
		}
	}
	// Two accounts on a capability-identified bouncer stay account-isolated.
	a := settingsChannel(t, f.m, "alice", "bouncer", "bouncer.test", "Example", "@", map[string]bool{"znc.in/self-message": true})
	b := settingsChannel(t, f.m, "bob", "znc", "bouncer.test", "Example", "@", map[string]bool{"znc.in/self-message": true})
	if _, err := f.m.SetChannelSettings("alice", a.ID, ChannelSettingsInput{NSFW: &yes}); err != nil {
		t.Fatal(err)
	}
	if nsfw, known := f.m.linkSourceNSFW(f.m.space("bob"), b.ID, b.NetworkID); !known || nsfw {
		t.Fatalf("bouncer accounts shared classification: nsfw=%v known=%v", nsfw, known)
	}
	f.m.Close()
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := New(Options{Store: reopened})
	defer restarted.Close()
	for owner, conv := range map[string]Conversation{"alice": a, "bob": b} {
		if nsfw, known := restarted.linkSourceNSFW(restarted.space(owner), conv.ID, conv.NetworkID); !known || nsfw != (owner == "alice") {
			t.Fatalf("bouncer scope changed after restart for %s: nsfw=%v known=%v", owner, nsfw, known)
		}
	}
}

func TestNSFWLinksReclassifyRetainedHistoryWithoutChangingAccess(t *testing.T) {
	f := newLinkFixture(t)
	one := settingsChannel(t, f.m, "alice", "one", "irc.example.test", "First", "@", nil)
	two := settingsChannel(t, f.m, "alice", "two", "irc.example.test", "Second", "@", nil)
	op := settingsChannel(t, f.m, "bob", "bob", "irc.example.test", "First", "@", nil)
	f.postAs(t, one, "one:old", "peer", "https://example.test/old", time.Unix(2000, 0).UTC(), nil)
	f.postAs(t, two, "two:old", "peer", "https://example.test/old", time.Unix(2001, 0).UTC(), nil)
	before := summarize(f.links(t, "alice", LinkQuery{}))
	// Keep authorized retained history with no live channel settings.
	w := f.m.space("alice")
	w.mu.Lock()
	retained := w.conversations[one.ID]
	retained.Joined = false
	clearChannelSettings(&retained)
	w.conversations[one.ID] = retained
	delete(w.connections, one.NetworkID)
	w.emit("conversation.updated", retained)
	w.mu.Unlock()
	cursor := f.m.Snapshot("alice").Cursor
	yes := true
	if _, err := f.m.SetChannelSettings("bob", op.ID, ChannelSettingsInput{NSFW: &yes}); err != nil {
		t.Fatal(err)
	}
	page := f.links(t, "alice", LinkQuery{})
	if !equalCards(before, summarize(page)) || len(page.Items) != 1 || page.Items[0].Count != 2 {
		t.Fatalf("classification changed history/counts/deduplication: %+v", page)
	}
	for _, source := range page.Items[0].Sources {
		if !source.NSFWKnown || source.NSFW != (source.NetworkID == "one") {
			t.Fatalf("old source classification: %+v", source)
		}
	}
	events, _, valid, stop := f.m.Subscribe("alice", cursor)
	stop()
	if !valid || len(events) != 1 || events[0].Type != "conversation.updated" {
		t.Fatalf("retained source had no private Links invalidation: %+v", events)
	}
	var invalidated Conversation
	if err := json.Unmarshal(events[0].Data, &invalidated); err != nil || invalidated.ID != one.ID || invalidated.ChannelSettings != nil {
		t.Fatalf("former member received current shared settings: %+v, %v", invalidated, err)
	}
	if got := f.links(t, "bob", LinkQuery{}); len(got.Items) != 0 {
		t.Fatalf("shared classification pooled another account's history: %+v", got)
	}
	// A legacy source with no verified identity is unknown, never guessed safe.
	w.mu.Lock()
	legacy := w.conversations[two.ID]
	legacy.ChannelIdentity, legacy.ChannelSettings = nil, nil
	w.conversations[two.ID] = legacy
	w.mu.Unlock()
	if nsfw, known := f.m.linkSourceNSFW(w, two.ID, two.NetworkID); known || nsfw {
		t.Fatalf("unavailable metadata guessed a classification: nsfw=%v known=%v", nsfw, known)
	}
	w.mu.Lock()
	legacy.ChannelSettings = &ChannelSettings{NSFW: true}
	w.conversations[two.ID] = legacy
	w.mu.Unlock()
	if nsfw, known := f.m.linkSourceNSFW(w, two.ID, two.NetworkID); known || !nsfw {
		t.Fatalf("unavailable metadata discarded a retained sensitive flag: nsfw=%v known=%v", nsfw, known)
	}
	if nsfw, known := f.m.linkSourceNSFW(w, one.ID, two.NetworkID); known || nsfw {
		t.Fatalf("source network mismatch borrowed another identity: nsfw=%v known=%v", nsfw, known)
	}
	// Closing an account's conversation still excludes it entirely.
	w.mu.Lock()
	delete(w.conversations, one.ID)
	w.emit("conversation.removed", model.Removal{ID: one.ID})
	w.mu.Unlock()
	page = f.links(t, "alice", LinkQuery{})
	if len(page.Items) != 1 || page.Items[0].Count != 1 || len(page.Items[0].Sources) != 1 || page.Items[0].Sources[0].NetworkID != "two" {
		t.Fatalf("closed source access changed: %+v", page)
	}
}

func TestLegacyChannelSettingsReplayDefaultsNSFWOff(t *testing.T) {
	f := newInboxFixture(t)
	conv := settingsChannel(t, f.m, "alice", "net", "irc.example.test", "Example", "@", nil)
	// Simulate a durable conversation frame from before the NSFW schema.
	if _, err := f.store.DB.Exec("UPDATE events SET payload=json_remove(payload,'$.data.channelSettings.nsfw') WHERE owner=?", "alice"); err != nil {
		t.Fatal(err)
	}
	events, _, valid, stop := f.m.Subscribe("alice", 0)
	stop()
	if !valid {
		t.Fatal("legacy replay was rejected")
	}
	found := false
	for _, event := range events {
		if event.Type == "conversation.updated" && strings.Contains(string(event.Data), `"nsfw":false`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("legacy settings for %s did not acquire the default field: %+v", conv.ID, events)
	}
}

func TestConvertedChannelAllowsOrdinarySharedSettings(t *testing.T) {
	f := newInboxFixture(t)
	conv := settingsChannel(t, f.m, "alice", "net", "irc.example.test", "Example", "@", nil)
	legacy := storage.PartyRecord{Party: model.Party{ID: "converted", ConversationID: conv.ID, NetworkID: "net", OwnerID: "alice", State: "converted", Revision: 1}, Channel: "#dev", Creator: "alice", ClientID: "converted"}
	testsupport.SeedLegacyParty(t, f.store, legacy)
	yes := true
	updated, err := f.m.SetChannelSettings("alice", conv.ID, ChannelSettingsInput{NSFW: &yes})
	if err != nil || updated.ChannelSettings == nil || !updated.ChannelSettings.NSFW {
		t.Fatalf("converted channel could not update ordinary shared metadata: %+v, %v", updated, err)
	}
}
