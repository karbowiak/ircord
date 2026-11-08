package irc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/storage"
)

func registrationEvents(m *Manager, owner, conversation string) []string {
	page, _ := m.History(owner, conversation, "", 100)
	out := []string{}
	for _, msg := range page.Items {
		if msg.Event != nil && (msg.Event.Type == "registered" || msg.Event.Type == "registration-failed") {
			out = append(out, msg.Event.Type+":"+msg.Event.Text)
		}
	}
	return out
}

func joinedChannel(m *Manager, owner, name string) bool {
	for _, v := range m.Snapshot(owner).Conversations {
		if v.Name == strings.TrimPrefix(name, "#") {
			return v.Joined
		}
	}
	return false
}

func TestErgoCreatedChannelsAreRegistered(t *testing.T) {
	host, port := ergo(t)
	suffix := id()[:8]
	alice, bob := "ara"+suffix, "arb"+suffix
	password := id()
	registerErgoAccount(t, host, port, alice, password)
	registerErgoAccount(t, host, port, bob, password)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"alice", "bob", "carol", "anon"} {
		if _, err := store.CreateAccount(owner, owner); err != nil {
			t.Fatal(err)
		}
	}
	m := New(Options{Store: store, AllowPrivate: true, ReconnectMin: time.Hour, Home: NetworkConfig{ID: "net", Host: host, Port: port}})
	defer func() { m.Close(); store.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// Carol shares alice's account in another IRCord account, and has turned the setting off.
	for _, v := range []struct{ owner, nick, account string }{{"alice", alice, alice}, {"bob", bob, bob}, {"carol", "arc" + suffix, alice}, {"anon", "aranon" + suffix, ""}} {
		cfg := NetworkConfig{ID: "net", Name: "Ergo", Host: host, Port: port, Nick: v.nick}
		if v.account != "" {
			cfg.SASLUser, cfg.SASLPassword = v.account, password
		}
		if _, err := m.Connect(v.owner, cfg); err != nil {
			t.Fatal(err)
		}
		if err := m.WaitConnected(ctx, v.owner, "net"); err != nil {
			t.Fatal(err)
		}
	}
	off, _ := json.Marshal(map[string]any{"preferences": map[string]any{"registerChannels": false}})
	if _, err := m.SaveAppData("carol", "client-preferences.updated", off); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return m.Snapshot("alice").Networks[0].Account == alice })

	// Creating a channel registers it to you.
	created := "#arnew" + suffix
	conv, err := m.Join("alice", "net", created)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(registrationEvents(m, "alice", conv.ID)) > 0 })
	if events := registrationEvents(m, "alice", conv.ID); events[0] != "registered:" {
		t.Fatal("unexpected registration event", events)
	}
	info, err := m.ChannelRegistration(ctx, "alice", conv.ID)
	if err != nil || !info.Registered || !strings.EqualFold(info.Founder, alice) || !info.Automatic {
		t.Fatal("created channel not registered automatically", info, err)
	}

	// Joining a channel someone else created doesn't, even when it isn't registered.
	existing := "#arold" + suffix
	if err := m.Raw("bob", "net", "JOIN "+existing); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return joinedChannel(m, "bob", existing) })
	joined, err := m.Join("alice", "net", existing)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return joinedChannel(m, "alice", existing) })
	// Bob's join wasn't asked for through IRCord, so it isn't a creation either.
	time.Sleep(time.Second)
	if info, err := m.ChannelRegistration(ctx, "alice", joined.ID); err != nil || info.Registered || len(registrationEvents(m, "alice", joined.ID)) != 0 {
		t.Fatal("joining an existing channel registered it", info, err)
	}

	// With the setting off, nothing is registered.
	quiet := "#aroff" + suffix
	conv, err = m.Join("carol", "net", quiet)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return joinedChannel(m, "carol", quiet) })
	time.Sleep(time.Second)
	if info, err := m.ChannelRegistration(ctx, "carol", conv.ID); err != nil || info.Registered {
		t.Fatal("registered with the setting off", info, err)
	}

	// Without an IRC account, the channel explains why it wasn't registered.
	conv, err = m.Join("anon", "net", "#aranon"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(registrationEvents(m, "anon", conv.ID)) > 0 })
	if events := registrationEvents(m, "anon", conv.ID); !strings.HasPrefix(events[0], "registration-failed:Log in") {
		t.Fatal("missing login explanation", events)
	}
}

func TestChannelLimitIsNotReportedAsAlreadyRegistered(t *testing.T) {
	err := serviceFailure([]string{"You have already registered the maximum number of channels; try dropping some with /CS UNREGISTER"})
	if failure, ok := err.(*ServiceError); !ok || !strings.Contains(failure.Detail, "limit") {
		t.Fatal("limit reply mapped to", err)
	}
}
