package irc

import (
	"fmt"
	"maps"
	"testing"

	"github.com/lrstanley/girc"
)

func TestImportedProfileReconcileMatchingAndAdoptedValues(t *testing.T) {
	m, s := storedProfileNetworks(t, true)
	if err := s.SaveProfilePublished("alice", "one", map[string]string{"bio": "Legacy bio"}); err != nil {
		t.Fatal(err)
	}
	one := profileConnection(m, "alice", "one")
	other := profileConnection(m, "alice", "two")
	answerProfileCheck(one, map[string]string{"avatar": testAvatar, "bio": "Newer upstream bio"})
	if got := queuedProfile(one); len(got) != 0 {
		t.Fatal("reconciliation overwrote upstream values", got)
	}
	p, _ := m.NetworkProfile("alice", "one")
	if p.AvatarURL != testAvatar || p.Bio != "Newer upstream bio" {
		t.Fatal("did not retain matching and adopt newer upstream values", p)
	}
	flags, err := s.ProfileImports("alice", "one")
	if err != nil || flags["avatar"] || flags["bio"] || !flags["display-name"] || !flags["color"] {
		t.Fatal("incorrect persisted import flags", flags, err)
	}
	answerProfileCheck(other, nil)
	if got := queuedProfile(other); len(got) != 0 {
		t.Fatal("copied values leaked to another network", got)
	}
	otherFlags, _ := s.ProfileImports("alice", "two")
	if !otherFlags["avatar"] || !otherFlags["bio"] {
		t.Fatal("reconciliation crossed networks", otherFlags)
	}
	m.Close()
	fresh := New(Options{Store: s, AllowPrivate: true})
	defer fresh.Close()
	c := profileConnection(fresh, "alice", "one")
	answerProfileCheck(c, nil)
	if got := queuedProfile(c); !maps.Equal(got, map[string]string{"avatar": testAvatar, "bio": "Newer upstream bio"}) {
		t.Fatal("established fields did not survive restart and republish", got)
	}
}

func TestImportedProfileRecoversOnlyMatchingPublicationHistory(t *testing.T) {
	m, s := storedProfileNetworks(t, true)
	if err := s.SaveProfilePublished("alice", "one", map[string]string{"avatar": "https://example.com/older.png", "bio": "Legacy bio", "display-name": ""}); err != nil {
		t.Fatal(err)
	}
	c := profileConnection(m, "alice", "one")
	answerProfileCheck(c, nil)
	if got := queuedProfile(c); !maps.Equal(got, map[string]string{"bio": "Legacy bio"}) {
		t.Fatal("recovery did not require matching nonempty network history", got)
	}
	flags, err := s.ProfileImports("alice", "one")
	if err != nil || flags["bio"] || !flags["avatar"] || !flags["display-name"] || !flags["color"] {
		t.Fatal("incorrect recovered import flags", flags, err)
	}
	other := profileConnection(m, "alice", "two")
	answerProfileCheck(other, nil)
	if len(other.queue) != 0 {
		t.Fatal("publication history leaked to another network")
	}
}

func TestImportedProfileHistoryIsAccountScoped(t *testing.T) {
	m, s := storedProfileNetworks(t, true)
	if err := s.SaveProfilePublished("alice", "one", map[string]string{"bio": "Legacy bio"}); err != nil {
		t.Fatal(err)
	}
	bob := Member{ID: "bob", Nick: "bobnick", DisplayName: "bobnick", Bio: "Legacy bio"}
	if _, err := s.Commit("bob", "profile.updated", bob); err != nil {
		t.Fatal(err)
	}
	cfg := NetworkConfig{ID: "one", Name: "one", Host: "one.test", Port: 6697, TLS: true, Nick: "bobnick"}
	if err := s.SaveNetwork("bob", cfg, false); err != nil {
		t.Fatal(err)
	}
	n := Network{ID: "one", Name: cfg.Name, Host: cfg.Host, Port: cfg.Port, TLS: cfg.TLS, Nick: cfg.Nick, Status: "disconnected", Capabilities: []string{}}
	applyConfig(&n, cfg)
	if _, err := s.Commit("bob", "network.updated", n); err != nil {
		t.Fatal(err)
	}
	c := profileConnection(m, "bob", "one")
	answerProfileCheck(c, nil)
	flags, err := s.ProfileImports("bob", "one")
	if err != nil || !flags["bio"] || len(c.queue) != 0 {
		t.Fatal("another account's publication history established an import", flags, err)
	}
}

func TestImportedProfileWaitsForCompleteValidCheck(t *testing.T) {
	m, s := storedProfileNetworks(t, true)
	c := profileConnection(m, "alice", "one")
	c.checkPublishedProfile()
	<-c.queue
	c.profileCheckReply(*girc.ParseEvent(":irc.test 761 onenick onenick bio * :Legacy bio"))
	flags, _ := s.ProfileImports("alice", "one")
	if !flags["bio"] || len(c.queue) != 0 {
		t.Fatal("an incomplete check changed imports or published", flags)
	}
	for _, key := range []string{"display-name", "color"} {
		c.profileCheckReply(*girc.ParseEvent(fmt.Sprintf(":irc.test 766 onenick onenick %s :Key is not set", key)))
	}
	c.profileCheckReply(*girc.ParseEvent(":irc.test 761 onenick onenick avatar * :http://example.com/invalid.png"))
	flags, _ = s.ProfileImports("alice", "one")
	p, _ := m.NetworkProfile("alice", "one")
	if !flags["avatar"] || flags["bio"] || p.AvatarURL != testAvatar || len(c.queue) != 0 {
		t.Fatal("invalid upstream value was adopted or established", flags, p)
	}
}

func TestImportedProfileDoesNotReleaseFlagWhenStorageFails(t *testing.T) {
	m, s := storedProfileNetworks(t, true)
	c := profileConnection(m, "alice", "one")
	if _, err := s.DB.Exec(`CREATE TRIGGER reject_import_update BEFORE UPDATE ON app_data WHEN NEW.kind = 'profile-imported:one' BEGIN SELECT RAISE(FAIL, 'test storage failure'); END`); err != nil {
		t.Fatal(err)
	}
	answerProfileCheck(c, map[string]string{"avatar": testAvatar, "bio": "Newer upstream bio"})
	if m.StorageError("alice") == nil || len(c.queue) != 0 {
		t.Fatal("storage failure was not reported or allowed publication")
	}
	flags, err := s.ProfileImports("alice", "one")
	if err != nil || !flags["avatar"] || !flags["bio"] {
		t.Fatal("failed import save released flags", flags, err)
	}
	w := m.space("alice")
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.profileImports["one"]["avatar"] || !w.profileImports["one"]["bio"] {
		t.Fatal("failed import save released in-memory flags", w.profileImports)
	}
}
