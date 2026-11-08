package irc

import (
	"context"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
	"maps"
	"slices"
	"testing"

	"github.com/lrstanley/girc"
)

func TestShouldPublishOnlyWhereNothingElseWasShared(t *testing.T) {
	for _, tc := range []struct {
		name, ours, upstream string
		published            map[string]string
		want                 bool
	}{
		{"already shared", "A", "A", nil, false},
		{"nothing upstream", "A", "", nil, true},
		{"set elsewhere, fresh backend", "", "B", nil, false},
		{"set elsewhere, stale backend", "A", "B", nil, false},
		{"set elsewhere after our last publish", "A", "B", map[string]string{"avatar": "C"}, false},
		{"our own offline change", "A", "B", map[string]string{"avatar": "B"}, true},
		{"our own offline clear", "", "B", map[string]string{"avatar": "B"}, true},
	} {
		if got := shouldPublish(tc.ours, tc.upstream, tc.published, "avatar"); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestProfileValuesShareOnlyHTTPSAvatars(t *testing.T) {
	if v := profileValues(Member{AvatarURL: "http://localhost:8080/files/a/b/avatar"}); v["avatar"] != "" {
		t.Fatal("an http avatar receivers reject was shared", v)
	}
	if v := profileValues(Member{AvatarURL: testAvatar}); v["avatar"] != testAvatar {
		t.Fatal("https avatar not shared", v)
	}
	if keys := changedProfileKeys(Member{AvatarURL: testAvatar, Bio: "x"}, Member{AvatarURL: testAvatar, Bio: "y"}); !slices.Equal(keys, []string{"bio"}) {
		t.Fatal("an edit publishes unchanged fields", keys)
	}
}

// The check completes only on direct answers about our own nick; the server's unsolicited
// notifications of our values ("*" as client) and answers about other nicks are not answers.
func TestProfileCheckPublishesAfterAllAnswers(t *testing.T) {
	m := New(Options{AllowPrivate: true})
	defer m.Close()
	w := m.space("alice")
	w.self = Member{ID: "alice", Nick: "alice", DisplayName: "Alice", NameSource: "profile", Bio: "hello"}
	c := &connection{owner: w, ctx: context.Background(), ready: true, queue: make(chan []*girc.Event, 16), caps: map[string]bool{"draft/metadata-3": true}, config: NetworkConfig{Nick: "alice"}}
	c.checkPublishedProfile()
	if e := (<-c.queue)[0]; e.Command != "METADATA" || e.Params[1] != "GET" {
		t.Fatal("profile not checked first", e)
	}
	for _, line := range []string{
		":irc.test 761 alice alice display-name * :Alice Prod",
		":irc.test 766 alice alice avatar :Key is not set",
		":irc.test 766 alice alice color :Key is not set",
		// Neither of these answers the fourth key.
		":irc.test 761 * alice bio * :Someone Else",
		":irc.test 766 alice bob bio :Key is not set",
	} {
		c.profileCheckReply(*girc.ParseEvent(line))
	}
	if len(c.queue) != 0 || c.profileCheckDone {
		t.Fatal("published before every key was answered")
	}
	c.profileCheckReply(*girc.ParseEvent(":irc.test 766 alice alice bio :Key is not set"))
	var set []string
	for len(c.queue) > 0 {
		e := (<-c.queue)[0]
		set = append(set, e.Params[2])
	}
	// The name was shared by another session and stays; the bio was not set anywhere.
	if !slices.Equal(set, []string{"bio"}) {
		t.Fatal("unexpected keys published", set)
	}
	// Fields the server already holds with our value count as ours; a queued SET does not
	// count until it is sent.
	if v, ok := c.profilePublished["avatar"]; !ok || v != "" {
		t.Fatal("matching value not recorded", c.profilePublished)
	}
	if _, ok := c.profilePublished["bio"]; ok {
		t.Fatal("a queued SET was recorded before it was sent", c.profilePublished)
	}
	c.recordSentProfile(&girc.Event{Command: "METADATA", Params: []string{"*", "SET", "bio", "hello"}})
	if c.profilePublished["bio"] != "hello" {
		t.Fatal("sent value not recorded", c.profilePublished)
	}
}

// What an account published survives a backend restart, so a later connect still recognises
// its own offline edits; removing the network forgets it.
func TestPublishedProfileIsKeptInAccountStorage(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateAccount("alice", "alice-published-test"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store, AllowPrivate: true})
	defer m.Close()
	w := m.space("alice")
	c := &connection{owner: w, config: NetworkConfig{ID: "net"}}
	c.recordSentProfile(&girc.Event{Command: "METADATA", Params: []string{"*", "SET", "display-name", "Alice"}})
	c.recordSentProfile(&girc.Event{Command: "METADATA", Params: []string{"*", "SET", "avatar"}})
	c.recordSentProfile(&girc.Event{Command: "PRIVMSG", Params: []string{"*", "SET", "bio", "x"}})
	fresh := &connection{owner: w, config: NetworkConfig{ID: "net"}}
	if got := fresh.publishedProfile(); !maps.Equal(got, map[string]string{"display-name": "Alice", "avatar": ""}) {
		t.Fatal("published profile not restored", got)
	}
	if _, err = store.Commit("alice", "network.removed", model.Removal{ID: "net"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.ProfilePublished("alice", "net"); len(got) != 0 {
		t.Fatal("published profile kept after the network was removed", got)
	}
}
