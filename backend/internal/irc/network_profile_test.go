package irc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func profileConnection(m *Manager, owner, network string) *connection {
	w := m.space(owner)
	c := &connection{manager: m, owner: w, config: w.configs[network], ctx: context.Background(), ready: true, profileCheckDone: true, caps: map[string]bool{"draft/metadata-3": true}, queue: make(chan []*girc.Event, 32), nicks: map[string]string{}}
	w.connections[network] = c
	return c
}

func queuedProfile(c *connection) map[string]string {
	values := map[string]string{}
	for len(c.queue) > 0 {
		for _, e := range <-c.queue {
			if e.Command == "METADATA" && len(e.Params) >= 3 && e.Params[1] == "SET" {
				value := ""
				if len(e.Params) > 3 {
					value = e.Params[3]
				}
				values[e.Params[2]] = value
			}
		}
	}
	return values
}

func answerProfileCheck(c *connection, values map[string]string) {
	c.checkPublishedProfile()
	<-c.queue // GET
	for _, key := range metadataKeys {
		line := fmt.Sprintf(":irc.test 766 %s %s %s :Key is not set", c.config.Nick, c.config.Nick, key)
		if value := values[key]; value != "" {
			line = fmt.Sprintf(":irc.test 761 %s %s %s * :%s", c.config.Nick, c.config.Nick, key, value)
		}
		c.profileCheckReply(*girc.ParseEvent(line))
	}
}

func storedProfileNetworks(t *testing.T, legacy bool) (*Manager, *storage.Store) {
	t.Helper()
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := s.CreateAccount("alice", "network-profile-alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAccount("bob", "network-profile-bob"); err != nil {
		t.Fatal(err)
	}
	if legacy {
		p := Member{ID: "alice", Nick: "oldnick", DisplayName: "Legacy name", NameSource: "profile", Bio: "Legacy bio", AvatarURL: testAvatar, NameColor: "#123456", Status: "online", Role: "member", Color: "lavender"}
		if _, err := s.Commit("alice", "profile.updated", p); err != nil {
			t.Fatal(err)
		}
	}
	for _, network := range []string{"one", "two"} {
		cfg := NetworkConfig{ID: network, Name: network, Host: network + ".test", Port: 6697, TLS: true, Nick: network + "nick", Username: network + "user", Realname: network + " realname"}
		if err := s.SaveNetwork("alice", cfg, false); err != nil {
			t.Fatal(err)
		}
		n := Network{ID: network, Name: cfg.Name, Host: cfg.Host, Port: cfg.Port, TLS: cfg.TLS, Nick: cfg.Nick, Status: "disconnected", Capabilities: []string{}}
		if !legacy {
			n.Profile = &Member{ID: "alice", Nick: cfg.Nick, DisplayName: cfg.Realname, Realname: cfg.Realname, NameSource: "realname", Status: "offline", Role: "member", Color: "lavender"}
		}
		applyConfig(&n, cfg)
		if _, err := s.Commit("alice", "network.updated", n); err != nil {
			t.Fatal(err)
		}
	}
	m := New(Options{Store: s, AllowPrivate: true})
	t.Cleanup(m.Close)
	return m, s
}

func TestNetworkProfileEditsPublishOnlyTheirServer(t *testing.T) {
	m, s := storedProfileNetworks(t, false)
	one, two := profileConnection(m, "alice", "one"), profileConnection(m, "alice", "two")
	before, _ := m.NetworkProfile("alice", "two")
	self := m.Self("alice")
	p, err := m.SetNetworkProfile("alice", "one", Profile{DisplayName: profileName("One Name"), AvatarURL: profileName(testAvatar), Bio: "One bio", NameColor: profileName("#abcdef")})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"display-name": "One Name", "avatar": testAvatar, "bio": "One bio", "color": "#abcdef"}
	if !maps.Equal(queuedProfile(one), want) || len(two.queue) != 0 {
		t.Fatal("profile was not published only on its server")
	}
	if p.ID != self.ID || !equal(m.Self("alice"), self) {
		t.Fatal("profile changed the stable identity or account fallback")
	}
	after, _ := m.NetworkProfile("alice", "two")
	if !equal(before, after) {
		t.Fatal("another network changed", after)
	}
	if _, err := m.SetProfile("alice", Profile{DisplayName: profileName("Legacy local"), Bio: "legacy"}); err != nil {
		t.Fatal(err)
	}
	if len(one.queue) != 0 || len(two.queue) != 0 {
		t.Fatal("legacy account edit broadcast to IRC")
	}
	if got, _ := m.NetworkProfile("alice", "one"); !maps.Equal(profileValues(got), want) {
		t.Fatal("legacy edit changed a server profile", got)
	}
	if _, err := m.NetworkProfile("bob", "one"); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign profile was readable", err)
	}
	if _, err := m.SetNetworkProfile("bob", "one", Profile{Bio: "intruder"}); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign profile was editable", err)
	}
	if _, err := m.SetNetworkProfile("alice", "missing", Profile{}); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing network was editable", err)
	}
	events, valid, err := s.Replay("alice", 0, 100)
	if err != nil || !valid {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Type != "network.updated" {
			continue
		}
		var n Network
		if err := json.Unmarshal(event.Data, &n); err != nil {
			t.Fatal(err)
		}
		if n.ID == "one" && n.Profile != nil && n.Profile.DisplayName == "One Name" {
			found = true
		}
	}
	if !found {
		t.Fatal("scoped profile missing from durable network events")
	}
}

func TestProfilePatchMergesFieldsAndFencesIdentity(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			m, store := storedProfileNetworks(t, false)
			c := profileConnection(m, "alice", "one")
			first := ProfilePatch{Bio: profileName("bio")}
			second := ProfilePatch{NameColor: profileName("#123456")}
			if reverse {
				first, second = second, first
			}
			scope, err := m.PrepareProfilePatch("alice", "one", first)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = m.ApplyProfilePatch("alice", "one", second, nil); err != nil {
				t.Fatal(err)
			}
			if _, err = m.ApplyProfilePatch("alice", "one", first, scope); err != nil {
				t.Fatal(err)
			}
			p, err := m.NetworkProfile("alice", "one")
			if err != nil || p.Bio != "bio" || p.NameColor != "#123456" {
				t.Fatal(p, err)
			}
			if got := queuedProfile(c); len(got) != 2 || got["bio"] != "bio" || got["color"] != "#123456" {
				t.Fatal("unrelated publication", got)
			}
			if _, err = m.ApplyProfilePatch("alice", "one", ProfilePatch{Bio: profileName("last")}, nil); err != nil {
				t.Fatal(err)
			}
			if p, err = m.NetworkProfile("alice", "one"); err != nil || p.Bio != "last" || p.NameColor != "#123456" {
				t.Fatal(p, err)
			}
			if _, err = m.ApplyProfilePatch("alice", "one", ProfilePatch{Bio: profileName("must not apply"), NameColor: profileName("bad")}, nil); err == nil {
				t.Fatal("invalid patch accepted")
			}
			if after, err := m.NetworkProfile("alice", "one"); err != nil || after.Bio != "last" {
				t.Fatal("invalid patch mutated profile", after, err)
			}
			if _, err = m.ApplyProfilePatch("bob", "one", first, nil); !errors.Is(err, ErrNotFound) {
				t.Fatal(err)
			}
			if other, err := m.NetworkProfile("alice", "two"); err != nil || other.Bio != "" || other.NameColor != "" {
				t.Fatal(other, err)
			}
			fresh := New(Options{Store: store})
			defer fresh.Close()
			if restored, err := fresh.NetworkProfile("alice", "one"); err != nil || restored.Bio != "last" || restored.NameColor != "#123456" {
				t.Fatal("partial patch did not survive restart", restored, err)
			}
		})
	}
	m, _ := storedProfileNetworks(t, false)
	scope, err := m.PrepareProfilePatch("alice", "one", ProfilePatch{Bio: profileName("stale")})
	if err != nil {
		t.Fatal(err)
	}
	w := m.space("alice")
	w.mu.Lock()
	cfg, network := w.configs["one"], w.networks["one"]
	w.mu.Unlock()
	if err := m.DeleteNetwork("alice", "one"); err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.configs["one"], w.networks["one"] = cfg, network
	w.mu.Unlock()
	if _, err := m.ApplyProfilePatch("alice", "one", ProfilePatch{Bio: profileName("stale")}, scope); !errors.Is(err, ErrNotFound) {
		t.Fatal("recreated identity accepted stale edit", err)
	}
	scope, err = m.PrepareProfilePatch("alice", "one", ProfilePatch{Bio: profileName("stale")})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Host = "replacement.test"
	if _, err = m.UpdateNetwork("alice", "one", cfg); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ApplyProfilePatch("alice", "one", ProfilePatch{Bio: profileName("stale")}, scope); !errors.Is(err, ErrNotFound) {
		t.Fatal("replaced identity accepted stale edit", err)
	}
	scope, err = m.PrepareProfilePatch("alice", "one", ProfilePatch{Bio: profileName("stale")})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.DropOwner("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ApplyProfilePatch("alice", "one", ProfilePatch{Bio: profileName("stale")}, scope); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted account accepted stale edit", err)
	}
}

func TestProfilePatchConcurrentDistinctFields(t *testing.T) {
	m, _ := storedProfileNetworks(t, false)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, patch := range []ProfilePatch{{Bio: profileName("concurrent bio")}, {NameColor: profileName("#abcdef")}} {
		go func(p ProfilePatch) { <-start; _, err := m.ApplyProfilePatch("alice", "one", p, nil); results <- err }(patch)
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	p, err := m.NetworkProfile("alice", "one")
	if err != nil || p.Bio != "concurrent bio" || p.NameColor != "#abcdef" {
		t.Fatal(p, err)
	}
}

func TestNetworkProfileOwnMetadataAndRestartStayIsolated(t *testing.T) {
	m, s := storedProfileNetworks(t, false)
	c := profileConnection(m, "alice", "one")
	before, _ := m.NetworkProfile("alice", "two")
	values := map[string]string{"display-name": "External name", "avatar": testAvatar, "bio": "External bio", "color": "#456789"}
	for key, value := range values {
		c.metadata(*girc.ParseEvent(fmt.Sprintf(":irc.test METADATA onenick %s * :%s", key, value)))
	}
	p, _ := m.NetworkProfile("alice", "one")
	if !maps.Equal(profileValues(p), values) || len(c.queue) != 0 {
		t.Fatal("own server metadata was not learned without publication", p)
	}
	answerProfileCheck(c, values)
	if len(c.queue) != 0 {
		t.Fatal("reconnect published unchanged remote metadata")
	}
	m.Close()
	fresh := New(Options{Store: s, AllowPrivate: true})
	defer fresh.Close()
	restored, _ := fresh.NetworkProfile("alice", "one")
	other, _ := fresh.NetworkProfile("alice", "two")
	if !maps.Equal(profileValues(restored), values) || !equal(other, before) {
		t.Fatal("restart mixed server profiles", restored, other)
	}
	if restored.ID != "alice" || fresh.Snapshot("alice").Networks[0].Profile == nil {
		t.Fatal("bootstrap lost the profile or stable identity")
	}
	// Another backend's newer value wins on reconnect; our later offline edit
	// is still publishable when that backend has not changed it again.
	c = profileConnection(fresh, "alice", "one")
	values["display-name"] = "Newer elsewhere"
	answerProfileCheck(c, values)
	if len(c.queue) != 0 {
		t.Fatal("a reconnect replaced another backend's edit")
	}
	c.ready = false
	if _, err := fresh.SetNetworkProfile("alice", "one", Profile{DisplayName: profileName("Offline edit"), Bio: "External bio", NameColor: profileName("#456789")}); err != nil {
		t.Fatal(err)
	}
	c.metadata(*girc.ParseEvent(":irc.test METADATA onenick display-name * :Newer elsewhere"))
	if p, _ := fresh.NetworkProfile("alice", "one"); p.DisplayName != "Offline edit" {
		t.Fatal("late disconnected metadata cancelled an offline edit", p)
	}
	fresh.Close()
	last := New(Options{Store: s, AllowPrivate: true})
	defer last.Close()
	c = profileConnection(last, "alice", "one")
	answerProfileCheck(c, values)
	if got := queuedProfile(c); !maps.Equal(got, map[string]string{"display-name": "Offline edit"}) {
		t.Fatal("offline edit did not survive restart/reconnect", got)
	}
	c.metadata(*girc.ParseEvent(":irc.test METADATA onenick display-name * :Newer elsewhere"))
	if p, _ := last.NetworkProfile("alice", "one"); p.DisplayName != "Offline edit" {
		t.Fatal("late echo cancelled an unsent local edit", p)
	}
}

func TestNetworkProfileLegacyMigrationIsIndependentAndNeverPublishes(t *testing.T) {
	m, s := storedProfileNetworks(t, true)
	one, two := profileConnection(m, "alice", "one"), profileConnection(m, "alice", "two")
	first, _ := m.NetworkProfile("alice", "one")
	second, _ := m.NetworkProfile("alice", "two")
	if !maps.Equal(profileValues(first), profileValues(second)) || first.Nick != "onenick" || second.Nick != "twonick" {
		t.Fatal("legacy appearance or network identities were lost", first, second)
	}
	answerProfileCheck(one, nil)
	answerProfileCheck(two, nil)
	if len(one.queue) != 0 || len(two.queue) != 0 {
		t.Fatal("migration published copied metadata")
	}
	beforeRestart := m.Cursor("alice")
	m.Close()
	fresh := New(Options{Store: s, AllowPrivate: true})
	defer fresh.Close()
	c := profileConnection(fresh, "alice", "one")
	if fresh.Cursor("alice") != beforeRestart {
		t.Fatal("legacy profile was imported more than once")
	}
	answerProfileCheck(c, nil)
	if len(c.queue) != 0 {
		t.Fatal("restart forgot that migration is not publication intent")
	}
	if _, err := fresh.SetNetworkProfile("alice", "one", Profile{DisplayName: profileName("Changed one"), Bio: first.Bio, AvatarURL: &first.AvatarURL, NameColor: &first.NameColor}); err != nil {
		t.Fatal(err)
	}
	if got := queuedProfile(c); !maps.Equal(got, map[string]string{"display-name": "Changed one"}) {
		t.Fatal("editing a migrated profile published unrelated fields", got)
	}
	if other, _ := fresh.NetworkProfile("alice", "two"); !equal(other, second) {
		t.Fatal("legacy copies remain linked", other)
	}
}

func TestNetworkProfileNewAndReplacementNetworksStartClean(t *testing.T) {
	for _, change := range []string{"new", "endpoint", "account", "interrupted save"} {
		t.Run(change, func(t *testing.T) {
			m, s := storedProfileNetworks(t, false)
			if _, err := m.SetNetworkProfile("alice", "one", Profile{DisplayName: profileName("Old name"), Bio: "Old bio", AvatarURL: profileName(testAvatar), NameColor: profileName("#123456")}); err != nil {
				t.Fatal(err)
			}
			w := m.space("alice")
			cfg := clone(w.configs["one"])
			cfg.Nick = "replacement"
			cfg.Realname = "New realname"
			if err := s.SaveProfilePublished("alice", "one", map[string]string{"display-name": "Old name"}); err != nil {
				t.Fatal(err)
			}
			var n Network
			var err error
			switch change {
			case "new":
				cfg.ID, cfg.Host, cfg.Port, cfg.TLS = "three", "127.0.0.1", 1, false
				n, err = m.Connect("alice", cfg)
			case "endpoint":
				cfg.Host = "replacement.test"
				n, err = m.UpdateNetwork("alice", "one", cfg)
			case "account":
				cfg.Auth = &model.AuthInput{Method: "sasl-plain", Account: "replacement"}
				n, err = m.UpdateNetwork("alice", "one", cfg)
			case "interrupted save":
				cfg.Host = "replacement.test"
				if err = s.SaveNetwork("alice", cfg, false); err != nil {
					t.Fatal(err)
				}
				m.Close()
				fresh := New(Options{Store: s, AllowPrivate: true})
				defer fresh.Close()
				n, err = fresh.Network("alice", "one")
			}
			if err != nil {
				t.Fatal(err)
			}
			if n.Profile == nil || n.Profile.ID != "alice" || n.Profile.DisplayName != "New realname" || n.Profile.AvatarURL != "" || n.Profile.Bio != "" || n.Profile.NameColor != "" {
				t.Fatal("new identity inherited old server profile", n.Profile)
			}
			if change != "new" {
				if published, err := s.ProfilePublished("alice", "one"); err != nil || len(published) != 0 {
					t.Fatal("replacement retained publication authority", published, err)
				}
			}
		})
	}
}

func TestNetworkProfileIdentityHooksStayOnTheirServer(t *testing.T) {
	m, _ := storedProfileNetworks(t, false)
	c := profileConnection(m, "alice", "one")
	before, _ := m.NetworkProfile("alice", "two")
	c.receiveUser(nil, *girc.ParseEvent(":onenick!u@h AWAY :One away"))
	c.receiveUser(nil, *girc.ParseEvent(":onenick!u@h SETNAME :One changed realname"))
	c.nicks["onenick"] = "alice"
	client := girc.New(girc.Config{Nick: "newnick", User: "fixture", Name: "fixture", Server: "fixture.test", Port: 6697})
	c.receiveUser(client, *girc.ParseEvent(":onenick!u@h NICK :newnick"))
	p, _ := m.NetworkProfile("alice", "one")
	if p.Status != "away" || p.AwayMessage != "One away" || p.Realname != "One changed realname" || p.DisplayName != "One changed realname" || p.Nick != "newnick" {
		t.Fatal("own identity hooks did not update scoped profile", p)
	}
	other, _ := m.NetworkProfile("alice", "two")
	if !equal(other, before) || m.Self("alice").Realname != "" {
		t.Fatal("identity hooks crossed servers", other)
	}
}

func TestNetworkProfileSearchUsesTheConversationSelfNickname(t *testing.T) {
	for _, durable := range []bool{false, true} {
		t.Run(fmt.Sprint(durable), func(t *testing.T) {
			m, _ := storedProfileNetworks(t, false)
			w := m.space("alice")
			w.mu.Lock()
			w.members["peer"] = Member{ID: "peer", Nick: "peer"}
			at := time.Now().UTC()
			for _, network := range []string{"one", "two"} {
				conv := Conversation{ID: network + ":room", NetworkID: network, Kind: "channel", Name: "#private"}
				w.conversations[conv.ID] = conv
				w.emit("conversation.updated", conv)
				for _, nick := range []string{"onenick", "twonick"} {
					msg := Message{ID: network + ":" + nick, ConversationID: conv.ID, SenderID: "peer", Text: "hello @" + nick, Kind: "text", CreatedAt: at}
					w.messages[conv.ID] = append(w.messages[conv.ID], msg)
					w.emit("message.created", msg)
				}
			}
			if !durable {
				w.store = nil
			}
			w.mu.Unlock()
			q := SearchQuery{Mentions: []string{"alice"}, Limit: 1}
			ids := []string{}
			for {
				page, err := m.SearchMessages("alice", q)
				if err != nil {
					t.Fatal(err)
				}
				for _, result := range page.Items {
					ids = append(ids, result.Message.ID)
				}
				if page.NextCursor == nil {
					break
				}
				q.Cursor = *page.NextCursor
				if len(ids) > 2 {
					t.Fatal("mention pagination repeated results", ids)
				}
			}
			slices.Sort(ids)
			if !slices.Equal(ids, []string{"one:onenick", "two:twonick"}) {
				t.Fatal("self mentions used another server nickname", ids)
			}
		})
	}
}

func TestNetworkProfileCanLearnLiveMetadataAfterCheckTimeout(t *testing.T) {
	previous := profileCheckTimeout
	profileCheckTimeout = time.Millisecond
	defer func() { profileCheckTimeout = previous }()
	m, _ := storedProfileNetworks(t, false)
	c := profileConnection(m, "alice", "one")
	c.checkPublishedProfile()
	<-c.queue
	eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.profileCheckDone
	})
	if len(c.queue) != 0 {
		t.Fatal("an incomplete check published metadata")
	}
	c.metadata(*girc.ParseEvent(":irc.test METADATA onenick display-name * :Live upstream name"))
	p, _ := m.NetworkProfile("alice", "one")
	if p.DisplayName != "Live upstream name" {
		t.Fatal("a failed check disabled later own profile updates", p)
	}
}

func TestNetworkProfileOwnMetadataClearsStayOnTheirServer(t *testing.T) {
	for _, command := range []string{"METADATA", "761", "766"} {
		t.Run(command, func(t *testing.T) {
			m, _ := storedProfileNetworks(t, false)
			c := profileConnection(m, "alice", "one")
			before, _ := m.NetworkProfile("alice", "two")
			if _, err := m.SetNetworkProfile("alice", "one", Profile{DisplayName: profileName("Name"), Bio: "Bio", AvatarURL: profileName(testAvatar), NameColor: profileName("#123456")}); err != nil {
				t.Fatal(err)
			}
			c.recordPublished(queuedProfile(c))
			for _, key := range metadataKeys {
				switch command {
				case "METADATA":
					c.metadata(*girc.ParseEvent(":irc.test METADATA onenick " + key + " *"))
				case "761":
					c.metadata(*girc.ParseEvent(":irc.test 761 * onenick " + key + " *"))
				case "766":
					c.receiveMetadataCleared(*girc.ParseEvent(":irc.test 766 * onenick " + key + " :Key is not set"))
				}
			}
			p, _ := m.NetworkProfile("alice", "one")
			if p.NameSource == "profile" || p.DisplayName != p.Realname || p.Bio != "" || p.NameColor != "" || p.AvatarURL != "" {
				t.Fatal("own server metadata clear was ignored", p)
			}
			if other, _ := m.NetworkProfile("alice", "two"); !equal(other, before) {
				t.Fatal("metadata clears crossed servers", other)
			}
		})
	}
}

func TestNetworkProfileRetainedOwnNickUsesItsServerProfile(t *testing.T) {
	m, _ := storedProfileNetworks(t, false)
	c := profileConnection(m, "alice", "one")
	c.nicks["oldownnick"] = "alice"
	w := m.space("alice")
	w.mu.Lock()
	member := c.member("oldownnick", "")
	replyAuthor := w.networkMember("one", "alice")
	w.mu.Unlock()
	if member.Nick != "onenick" || replyAuthor.Nick != "onenick" {
		t.Fatal("retained self ID resolved to the legacy account profile", member, replyAuthor)
	}
}
