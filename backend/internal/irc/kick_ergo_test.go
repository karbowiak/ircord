package irc

import (
	"context"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

// After a kick IRCord stays out of the channel, even when a dropped connection reconnects;
// the conversation says who kicked you and why until you join again yourself (#237).
func TestErgoKickIsNotRejoined(t *testing.T) {
	host, port := ergo(t)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateAccount("alice", "alice-key-hash"); err != nil {
		t.Fatal(err)
	}
	suffix := id()[:8]
	room, nick := "#kick"+suffix, "app"+suffix
	peer := startPeer(t, host, port, "op"+suffix)
	var rejoins atomic.Int32
	kicked, opped := make(chan struct{}), make(chan struct{}, 1)
	peer.Handlers.Add(girc.JOIN, func(_ *girc.Client, e girc.Event) {
		if e.Source == nil {
			return
		}
		if strings.EqualFold(e.Source.Name, "op"+suffix) {
			opped <- struct{}{}
		}
		if strings.EqualFold(e.Source.Name, nick) {
			select {
			case <-kicked:
				rejoins.Add(1)
			default:
			}
		}
	})
	// The first to join a new channel is its operator.
	peer.Cmd.Join(room)
	select {
	case <-opped:
	case <-time.After(10 * time.Second):
		t.Fatal("operator did not join the channel")
	}
	manager := New(Options{Store: store, AllowPrivate: true, ReconnectMin: 50 * time.Millisecond})
	defer manager.Close()
	if _, err = manager.Connect("alice", NetworkConfig{ID: "net", Name: "Ergo", Host: host, Port: port, Nick: nick}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = manager.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Join("alice", "net", room); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return findConversation(t, manager, "alice", "net", room).Joined })

	close(kicked)
	peer.Cmd.Kick(room, nick, "take a break")
	eventually(t, func() bool { return !findConversation(t, manager, "alice", "net", room).Joined })
	conv := findConversation(t, manager, "alice", "net", room)
	if conv.Kicked == nil || conv.Kicked.Reason != "take a break" {
		t.Fatalf("kicked conversation = %+v, want who kicked you and why", conv.Kicked)
	}
	if by := manager.Snapshot("alice").Members; !slices.ContainsFunc(by, func(m Member) bool { return m.ID == conv.Kicked.ByID && strings.EqualFold(m.Nick, "op"+suffix) }) {
		t.Fatalf("kick is by %q, not the operator", conv.Kicked.ByID)
	}
	saved, err := store.Networks("alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range append(saved[0].Config.Channels, saved[0].Config.AutoJoin...) {
		if strings.EqualFold(strings.Fields(entry)[0], room) {
			t.Fatal("the channel you were kicked from is still saved to rejoin")
		}
	}

	// A dropped connection is not a sign-out: IRCord reconnects by itself, without the channel.
	c, _ := manager.connection("alice", "net")
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	client.Close()
	eventually(t, func() bool { return manager.Snapshot("alice").Networks[0].Status != "connected" })
	eventually(t, func() bool { return manager.Snapshot("alice").Networks[0].Status == "connected" })
	time.Sleep(time.Second)
	if n := rejoins.Load(); n != 0 {
		t.Fatalf("IRCord rejoined the channel %d times after the kick", n)
	}
	if conv = findConversation(t, manager, "alice", "net", room); conv.Joined || conv.Kicked == nil {
		t.Fatalf("after reconnecting: joined=%v kicked=%+v", conv.Joined, conv.Kicked)
	}

	// Rejoining yourself works as before and clears the kick.
	if _, err = manager.Join("alice", "net", room); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		conv := findConversation(t, manager, "alice", "net", room)
		return conv.Joined && conv.Kicked == nil
	})
}
