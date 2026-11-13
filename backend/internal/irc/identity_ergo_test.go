package irc

import (
	"context"
	"encoding/json"
	"ircord/backend/internal/storage"
	"testing"
	"time"
)

func TestErgoRealnamesProfilesAndNicknameRejection(t *testing.T) {
	host, port := ergo(t)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner+"-identity-test"); err != nil {
			t.Fatal(err)
		}
	}
	m := New(Options{Store: store, AllowPrivate: true})
	defer m.Close()
	suffix := id()[:8]
	alice := "namea" + suffix
	bob := "nameb" + suffix
	room := "#names" + suffix
	for _, cfg := range []struct{ owner, nick, name string }{{"alice", alice, "Alice Example"}, {"bob", bob, ""}} {
		if _, err = m.Connect(cfg.owner, NetworkConfig{ID: "net", Name: "Ergo", Host: host, Port: port, Nick: cfg.nick, Realname: cfg.name}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = m.WaitConnected(ctx, cfg.owner, "net")
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = m.Join(cfg.owner, "net", room); err != nil {
			t.Fatal(err)
		}
	}
	// A fresh account has no profile name, so its profile on the network shows its nickname.
	space := m.space("bob")
	space.mu.Lock()
	user := space.networkSelf("net")
	space.mu.Unlock()
	if user.DisplayName != bob || user.NameSource != "nick" {
		t.Fatal("fresh account did not fall back to its nickname", user)
	}
	peerName := func(nick, name, source string) bool {
		for _, v := range m.Snapshot("bob").Members {
			if v.Nick == nick {
				return v.DisplayName == name && v.NameSource == source
			}
		}
		return false
	}
	eventually(t, func() bool { return peerName(alice, "Alice Example", "realname") })
	if err = m.Realname("alice", "net", "Alice Updated"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return peerName(alice, "Alice Updated", "realname") })
	// girc does not track SETNAME; a later roster refresh must not restore its stale WHO name.
	connection, _ := m.connection("bob", "net")
	connection.mu.Lock()
	client := connection.client
	connection.mu.Unlock()
	connection.syncMembers(client)
	if !peerName(alice, "Alice Updated", "realname") {
		t.Fatal("roster refresh overwrote live realname")
	}
	if err = m.Disconnect("alice", "net"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		connection.owner.mu.Lock()
		defer connection.owner.mu.Unlock()
		return len(connection.realnames) == 0
	})
	if _, err = m.Connect("alice", NetworkConfig{ID: "net", Name: "Ergo", Host: host, Port: port, Nick: alice, Realname: "Fresh session"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = m.WaitConnected(ctx, "alice", "net")
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Join("alice", "net", room); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return peerName(alice, "Fresh session", "realname") })
	if _, err = m.SetNetworkProfile("alice", "net", Profile{DisplayName: profileName("Explicit profile"), Bio: "First line\nSecond line"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return peerName(alice, "Explicit profile", "profile") })
	eventually(t, func() bool {
		for _, v := range m.Snapshot("bob").Members {
			if v.Nick == alice {
				return v.Bio == "First line Second line"
			}
		}
		return false
	})
	if saved, err := m.NetworkProfile("alice", "net"); err != nil || saved.Bio != "First line\nSecond line" {
		t.Fatal("IRC metadata flattened the saved account bio")
	}
	if err = m.Realname("alice", "net", "Alice Again"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range m.Snapshot("bob").Members {
			if v.Nick == alice {
				return v.DisplayName == "Explicit profile" && v.Realname == "Alice Again"
			}
		}
		return false
	})
	head := m.Snapshot("alice").Cursor
	_, events, _, stop := m.Subscribe("alice", head)
	defer stop()
	if err = m.Nick("alice", "net", bob); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(10 * time.Second)
	rejected := false
	for !rejected {
		select {
		case e := <-events:
			rejected = e.Type == "command.error"
		case <-deadline:
			t.Fatal("taken nickname had no actionable error")
		}
	}
	c, _ := m.connection("alice", "net")
	c.mu.Lock()
	permanent := c.permanent
	c.mu.Unlock()
	if permanent {
		t.Fatal("taken nickname disabled reconnect")
	}
	renamed := alice + "x"
	if err = m.Nick("alice", "net", renamed); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		return m.Snapshot("alice").Networks[0].Nick == renamed && peerName(renamed, "Explicit profile", "profile")
	})
	entities, err := store.Entities("bob")
	if err != nil {
		t.Fatal(err)
	}
	persisted := false
	for _, raw := range entities["member"] {
		var member Member
		_ = json.Unmarshal(raw, &member)
		if member.Nick == renamed {
			persisted = member.Realname == "Alice Again" && member.NameSource == "profile" && member.DisplayName == "Explicit profile"
		}
	}
	if !persisted {
		t.Fatal("identity source and realname were not persisted")
	}
	// Clearing the profile name removes the shared display-name so peers fall back to the
	// IRC real name, and the server profile falls back to that real name too.
	if _, err = m.SetNetworkProfile("alice", "net", Profile{DisplayName: profileName(""), Bio: "First line\nSecond line"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return peerName(renamed, "Alice Again", "realname") })
	if user, err := m.NetworkProfile("alice", "net"); err != nil || user.NameSource != "realname" || user.DisplayName != "Alice Again" {
		t.Fatal("cleared profile name did not fall back", user)
	}
}
