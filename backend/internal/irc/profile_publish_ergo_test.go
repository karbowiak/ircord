package irc

import (
	"testing"
	"time"

	"github.com/lrstanley/girc"
)

// profileChecked waits until a backend's connection has checked its own profile and queued
// whatever it publishes, then until the server has processed everything queued so far.
func profileChecked(t *testing.T, m *Manager, owner string) {
	t.Helper()
	eventually(t, func() bool {
		c, err := m.connection(owner, "net")
		if err != nil {
			return false
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.ready && c.metadataSubscribed && c.profileCheckDone
	})
	flushed(t, m, owner)
}

// flushed sends a PING through the connection's own FIFO queue and waits for its PONG, so every
// line queued before it, such as profile SETs, has been processed by the server.
func flushed(t *testing.T, m *Manager, owner string) {
	t.Helper()
	c, err := m.connection(owner, "net")
	if err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	token := "flush" + id()[:8]
	pong := make(chan struct{}, 1)
	handler := client.Handlers.Add("PONG", func(_ *girc.Client, e girc.Event) {
		if e.Last() == token {
			select {
			case pong <- struct{}{}:
			default:
			}
		}
	})
	defer client.Handlers.Remove(handler)
	if err := c.enqueue(&girc.Event{Command: "PING", Params: []string{token}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pong:
	case <-time.After(10 * time.Second):
		t.Fatal("queued lines did not reach the server")
	}
}

// restartAlice stops alice's backend and restores it from storage, as a deploy would.
func (f *metadataFixture) restartAlice() {
	f.t.Helper()
	f.aliceSide.Close()
	f.aliceSide = New(Options{Store: f.store, AllowPrivate: true})
	if err := f.aliceSide.Restore(); err != nil {
		f.t.Fatal(err)
	}
}

// Two IRCord backends signed in to one upstream account (production and a local instance)
// share one set of metadata on the server. A backend that never published a field must not
// clear or replace what the other one shares, on connect or when its user edits another field.
func TestErgoSecondBackendOnSameAccountKeepsProfile(t *testing.T) {
	f := newMetadataFixture(t, "two")
	password := alwaysOnAccount(t, f.host, f.port, f.alice)
	if _, err := f.store.CreateAccount("local", "local-two-test"); err != nil {
		t.Fatal(err)
	}
	f.connect(f.bobSide, "bob", f.config(f.bob, ""), true)
	f.connect(f.aliceSide, "alice", f.config(f.alice, password), true)
	f.setAlice("Alice Prod", testAvatar)
	eventually(t, func() bool { return f.peer().AvatarURL == testAvatar })

	// A fresh local backend with an empty profile signs in as the same account.
	local := New(Options{Store: f.store, AllowPrivate: true})
	t.Cleanup(local.Close)
	f.connect(local, "local", f.config(f.alice, password), true)
	profileChecked(t, local, "local")
	if own, err := local.NetworkProfile("local", "net"); err != nil || own.DisplayName != "Alice Prod" || own.AvatarURL != testAvatar {
		t.Fatal("a fresh backend did not learn its own upstream profile", own, err)
	}
	if !f.server(f.alice, "display-name", "Alice Prod") || !f.server(f.alice, "avatar", testAvatar) {
		t.Fatal("a fresh backend cleared the profile another backend shares")
	}

	// Its user sets only a name there: the avatar shared by production stays.
	if _, err := local.SetNetworkProfile("local", "net", Profile{DisplayName: profileName("Alice Local")}); err != nil {
		t.Fatal(err)
	}
	flushed(t, local, "local")
	if !f.server(f.alice, "display-name", "Alice Local") {
		t.Fatal("the edited name was not published")
	}
	if !f.server(f.alice, "avatar", testAvatar) {
		t.Fatal("editing the name cleared the avatar another backend shares")
	}
	eventually(t, func() bool { v := f.peer(); return v.DisplayName == "Alice Local" && v.AvatarURL == testAvatar })
	eventually(t, func() bool {
		own, _ := f.aliceSide.NetworkProfile("alice", "net")
		return own.DisplayName == "Alice Local" && own.AvatarURL == testAvatar
	})

	// Production restarts (a deploy) and does not put back its older name.
	f.restartAlice()
	profileChecked(t, f.aliceSide, "alice")
	if !f.server(f.alice, "display-name", "Alice Local") || !f.server(f.alice, "avatar", testAvatar) {
		t.Fatal("a restarted backend replaced a newer value set by another backend")
	}
	if own, err := f.aliceSide.NetworkProfile("alice", "net"); err != nil || own.DisplayName != "Alice Local" || own.AvatarURL != testAvatar {
		t.Fatal("a restarted backend did not restore the effective upstream profile", own, err)
	}
	if _, err := local.SetNetworkProfile("local", "net", Profile{AvatarURL: profileName("")}); err != nil {
		t.Fatal(err)
	}
	flushed(t, local, "local")
	eventually(t, func() bool {
		own, _ := f.aliceSide.NetworkProfile("alice", "net")
		return own.DisplayName == "Alice Local" && own.AvatarURL == ""
	})
}

// A single backend's own edit made while IRC is unreachable reaches the server once it is back,
// also after the backend restarted before the edit.
func TestErgoOfflineEditPublishedAfterReconnect(t *testing.T) {
	for _, restart := range []bool{false, true} {
		f := newMetadataFixture(t, "off")
		password := alwaysOnAccount(t, f.host, f.port, f.alice)
		f.connect(f.aliceSide, "alice", f.config(f.alice, password), true)
		f.setAlice("Alice One", testAvatar)
		if restart {
			// Also as if upgraded from a version that kept no record of what it published: the
			// value the server already holds must count as ours.
			flushed(t, f.aliceSide, "alice")
			f.aliceSide.Close()
			if err := f.store.SaveProfilePublished("alice", "net", map[string]string{}); err != nil {
				t.Fatal(err)
			}
			f.restartAlice()
		}
		profileChecked(t, f.aliceSide, "alice")
		c, err := f.aliceSide.connection("alice", "net")
		if err != nil {
			t.Fatal(err)
		}
		c.mu.Lock()
		transport := c.transport
		c.mu.Unlock()
		_ = transport.Close()
		eventually(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return !c.ready })
		if _, err := f.aliceSide.SetNetworkProfile("alice", "net", Profile{DisplayName: profileName("Alice Two"), AvatarURL: profileName(testAvatar)}); err != nil {
			t.Fatal(err)
		}
		profileChecked(t, f.aliceSide, "alice")
		if !f.server(f.alice, "display-name", "Alice Two") {
			profile, _ := f.aliceSide.NetworkProfile("alice", "net")
			t.Fatalf("restart=%v: an edit made while IRC was down never reached the server; local=%q published=%q", restart, profile.DisplayName, c.publishedProfile()["display-name"])
		}
	}
}

// With no other session holding the account's metadata, connecting publishes the stored
// profile, including values set while no IRC connection existed.
func TestErgoConnectPublishesStoredProfile(t *testing.T) {
	f := newMetadataFixture(t, "pub")
	cfg := f.config(f.alice, "")
	if err := f.store.SaveNetwork("alice", cfg, false); err != nil {
		t.Fatal(err)
	}
	// Materialize the saved, offline network before editing its own profile.
	if _, err := f.aliceSide.SetNetworkProfile("alice", "net", Profile{DisplayName: profileName("Alice Stored"), AvatarURL: profileName(testAvatar)}); err != nil {
		t.Fatal(err)
	}
	f.connect(f.aliceSide, "alice", f.config(f.alice, ""), false)
	profileChecked(t, f.aliceSide, "alice")
	if !f.server(f.alice, "display-name", "Alice Stored") || !f.server(f.alice, "avatar", testAvatar) {
		t.Fatal("the stored profile was not published on connect")
	}
}
