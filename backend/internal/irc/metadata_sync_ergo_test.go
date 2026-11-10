package irc

import (
	"context"
	"ircord/backend/internal/storage"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
)

// alwaysOnAccount registers nick on Ergo and opts it into always-on, so later SASL sessions
// reattach to channels the server kept joined.
func alwaysOnAccount(t *testing.T, host string, port int, nick string) string {
	t.Helper()
	password := id()
	registered := make(chan struct{}, 1)
	persistent := make(chan struct{}, 1)
	peer := startPeer(t, host, port, nick)
	peer.Handlers.Add("NOTICE", func(_ *girc.Client, e girc.Event) {
		if strings.Contains(e.Last(), "Account created") {
			select {
			case registered <- struct{}{}:
			default:
			}
		}
	})
	peer.Handlers.Add("PERSISTENCE", func(_ *girc.Client, e girc.Event) {
		if e.Last() == "ON" {
			select {
			case persistent <- struct{}{}:
			default:
			}
		}
	})
	peer.Send(&girc.Event{Command: "PRIVMSG", Params: []string{"NickServ", "REGISTER " + password}, Sensitive: true})
	select {
	case <-registered:
	case <-time.After(10 * time.Second):
		t.Fatal("registration failed")
	}
	peer.Send(&girc.Event{Command: "PERSISTENCE", Params: []string{"SET", "ON"}})
	select {
	case <-persistent:
	case <-time.After(10 * time.Second):
		t.Fatal("always-on opt-in failed")
	}
	peer.Close()
	return password
}

// serverMetadata returns a check that asks Ergo directly for a user's metadata value, so a
// test can wait until a change has been applied server-side instead of sleeping.
func serverMetadata(t *testing.T, host string, port int) func(nick, key, want string) bool {
	t.Helper()
	observer := girc.New(girc.Config{Server: host, Port: port, Nick: "mdobs" + id()[:8], User: "test", Name: "observer", AllowFlood: true, SupportedCaps: map[string][]string{"draft/metadata-3": nil}})
	replies := make(chan [3]string, 16)
	reply := func(nick, key, value string) {
		select {
		case replies <- [3]string{nick, key, value}:
		default:
		}
	}
	observer.Handlers.Add("761", func(_ *girc.Client, e girc.Event) {
		if len(e.Params) >= 5 {
			reply(e.Params[1], e.Params[2], e.Last())
		}
	})
	observer.Handlers.Add("766", func(_ *girc.Client, e girc.Event) {
		if len(e.Params) >= 3 {
			reply(e.Params[1], e.Params[2], "")
		}
	})
	ready := make(chan struct{})
	observer.Handlers.Add(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) { close(ready) })
	go func() { _ = observer.Connect() }()
	t.Cleanup(observer.Close)
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("metadata observer did not connect")
	}
	return func(nick, key, want string) bool {
		observer.Send(&girc.Event{Command: "METADATA", Params: []string{nick, "GET", key}})
		deadline := time.After(time.Second)
		for {
			select {
			case r := <-replies:
				if strings.EqualFold(r[0], nick) && r[1] == key {
					return r[2] == want
				}
			case <-deadline:
				return false
			}
		}
	}
}

type metadataFixture struct {
	t                  *testing.T
	host               string
	port               int
	store              *storage.Store
	alice, bob, room   string
	aliceSide, bobSide *Manager
	server             func(nick, key, want string) bool
}

func newMetadataFixture(t *testing.T, prefix string) *metadataFixture {
	t.Helper()
	host, port := ergo(t)
	suffix := id()[:8]
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner+"-"+prefix+"-test"); err != nil {
			t.Fatal(err)
		}
	}
	f := &metadataFixture{t: t, host: host, port: port, store: store, alice: prefix + "a" + suffix, bob: prefix + "b" + suffix, room: "#" + prefix + suffix, server: serverMetadata(t, host, port)}
	f.aliceSide = New(Options{Store: store, AllowPrivate: true})
	f.bobSide = New(Options{Store: store, AllowPrivate: true})
	t.Cleanup(func() { f.aliceSide.Close(); f.bobSide.Close() })
	return f
}

func (f *metadataFixture) connect(m *Manager, owner string, cfg NetworkConfig, join bool) {
	f.t.Helper()
	if _, err := m.Connect(owner, cfg); err != nil {
		f.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.WaitConnected(ctx, owner, "net"); err != nil {
		f.t.Fatal(err)
	}
	if join {
		f.join(m, owner)
	}
}

func (f *metadataFixture) join(m *Manager, owner string) {
	f.t.Helper()
	if _, err := m.Join(owner, "net", f.room); err != nil {
		f.t.Fatal(err)
	}
	c, err := m.connection(owner, "net")
	if err != nil {
		f.t.Fatal(err)
	}
	if err := c.enqueue(&girc.Event{Command: "MODE", Params: []string{f.room, "+s"}}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *metadataFixture) config(nick, password string) NetworkConfig {
	cfg := NetworkConfig{ID: "net", Name: "Ergo", Host: f.host, Port: f.port, Nick: nick}
	if password != "" {
		cfg.SASLUser, cfg.SASLPassword = nick, password
	}
	return cfg
}

// peer is alice as bob's backend currently sees her.
func (f *metadataFixture) peer() Member {
	for _, v := range f.bobSide.Snapshot("bob").Members {
		if v.Nick == f.alice {
			return v
		}
	}
	return Member{}
}

// setAlice changes alice's profile and waits until Ergo has applied it.
func (f *metadataFixture) setAlice(name, avatar string) {
	f.t.Helper()
	if _, err := f.aliceSide.SetNetworkProfile("alice", "net", Profile{DisplayName: profileName(name), AvatarURL: &avatar}); err != nil {
		f.t.Fatal(err)
	}
	eventually(f.t, func() bool {
		return f.server(f.alice, "display-name", name) && f.server(f.alice, "avatar", avatar)
	})
}

// restartBob stops bob's backend, runs change while it is away, and restores it from
// storage as a deploy would.
func (f *metadataFixture) restartBob(change func()) {
	f.t.Helper()
	f.bobSide.Close()
	change()
	f.bobSide = New(Options{Store: f.store, AllowPrivate: true})
	if err := f.bobSide.Restore(); err != nil {
		f.t.Fatal(err)
	}
}

const testAvatar = "https://example.com/alice.png"

// A bouncer session replays its channel JOINs before IRCord subscribes to metadata, so the
// server never volunteers the profiles of members already there. Names and avatars changed or
// cleared while the backend was away must still be corrected when it reconnects.
func TestErgoProfileChangesWhileAwayReachReconnectedBackend(t *testing.T) {
	f := newMetadataFixture(t, "away")
	bobPassword := alwaysOnAccount(t, f.host, f.port, f.bob)
	f.connect(f.aliceSide, "alice", f.config(f.alice, ""), true)
	f.connect(f.bobSide, "bob", f.config(f.bob, bobPassword), true)
	eventually(t, func() bool { return f.peer().DisplayName == f.alice })

	f.restartBob(func() { f.setAlice("Alice (Bot)", testAvatar) })
	eventually(t, func() bool {
		v := f.peer()
		return v.DisplayName == "Alice (Bot)" && v.NameSource == "profile" && v.AvatarURL == testAvatar
	})

	f.restartBob(func() { f.setAlice("", "") })
	eventually(t, func() bool {
		v := f.peer()
		return v.DisplayName == f.alice && v.NameSource == "nick" && v.AvatarURL == ""
	})

	// Live changes still arrive once the reconnected backend has subscribed.
	f.setAlice("Alice Live", "")
	eventually(t, func() bool { return f.peer().DisplayName == "Alice Live" })
}

// Without always-on, the backend rejoins its channels after subscribing. Values a peer cleared
// meanwhile must be dropped once the rejoined channel's member list is complete.
func TestErgoProfileClearedWhileAwayWithoutAlwaysOn(t *testing.T) {
	f := newMetadataFixture(t, "clr")
	f.connect(f.aliceSide, "alice", f.config(f.alice, ""), true)
	f.connect(f.bobSide, "bob", f.config(f.bob, ""), true)
	f.setAlice("Alice (Bot)", testAvatar)
	eventually(t, func() bool { v := f.peer(); return v.DisplayName == "Alice (Bot)" && v.AvatarURL == testAvatar })

	f.restartBob(func() { f.setAlice("", "") })
	eventually(t, func() bool {
		v := f.peer()
		return v.DisplayName == f.alice && v.NameSource == "nick" && v.AvatarURL == ""
	})
}

// A user who sets a profile before joining a channel we are already in is never announced to
// us by the server; it is only shared with the joiner and with co-members at change time.
func TestErgoProfileSetBeforePeerJoins(t *testing.T) {
	f := newMetadataFixture(t, "pre")
	f.connect(f.bobSide, "bob", f.config(f.bob, ""), true)
	f.connect(f.aliceSide, "alice", f.config(f.alice, ""), false)
	f.setAlice("Alice (Bot)", testAvatar)
	f.join(f.aliceSide, "alice")
	eventually(t, func() bool {
		v := f.peer()
		return v.DisplayName == "Alice (Bot)" && v.NameSource == "profile" && v.AvatarURL == testAvatar
	})
}
