package irc

import (
	"slices"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

func TestWhoisLinesFillTheProfile(t *testing.T) {
	d := model.WhoisDetails{Nickname: "jay", Channels: []string{}}
	for _, e := range []girc.Event{
		{Command: "311", Params: []string{"me", "Jay", "jay", "cloak.example", "*", "Jay Doe"}},
		{Command: "312", Params: []string{"me", "Jay", "irc.example.org", "Example server"}},
		{Command: "313", Params: []string{"me", "Jay", "is an IRC operator"}},
		{Command: "317", Params: []string{"me", "Jay", "42", "1700000000", "seconds idle, signon time"}},
		{Command: "319", Params: []string{"me", "Jay", "@#dev #general"}},
		{Command: "319", Params: []string{"me", "Jay", "+#ops"}},
		{Command: "330", Params: []string{"me", "Jay", "jayacct", "is logged in as"}},
		{Command: "671", Params: []string{"me", "Jay", "is using a secure connection"}},
		{Command: "301", Params: []string{"me", "Jay", "lunch"}},
	} {
		applyWhoisLine(&d, e)
	}
	if !d.Found || d.Nickname != "Jay" || d.Username != "jay" || d.Host != "cloak.example" ||
		d.RealName != "Jay Doe" || d.Server != "irc.example.org" || d.ServerInfo != "Example server" ||
		!d.Operator || !d.Secure || d.Account != "jayacct" || d.Away != "lunch" {
		t.Fatalf("details: %+v", d)
	}
	if !slices.Equal(d.Channels, []string{"@#dev", "#general", "+#ops"}) {
		t.Fatalf("channels: %v", d.Channels)
	}
	if d.IdleSeconds == nil || *d.IdleSeconds != 42 || d.SignedOnAt == nil || !d.SignedOnAt.Equal(time.Unix(1700000000, 0)) {
		t.Fatalf("idle %v signon %v", d.IdleSeconds, d.SignedOnAt)
	}
}

func TestWhoisWithoutAnAccountLeavesItEmpty(t *testing.T) {
	d := model.WhoisDetails{Channels: []string{}}
	applyWhoisLine(&d, girc.Event{Command: "330", Params: []string{"me", "jay", "*", "is logged in as"}})
	if d.Account != "" {
		t.Fatalf("account %q", d.Account)
	}
}

func TestCTCPTrafficNeverBecomesAMessage(t *testing.T) {
	f := newInboxFixture(t)
	count := func() int {
		w := f.c.owner
		w.mu.Lock()
		defer w.mu.Unlock()
		total := 0
		for _, messages := range w.messages {
			total += len(messages)
		}
		return total
	}
	before := count()
	src := &girc.Source{Name: "bob", Ident: "bob", Host: "h"}
	f.c.message(girc.Event{Command: "NOTICE", Source: src, Params: []string{"alice", "\x01VERSION SomeClient 1.0\x01"}, Tags: girc.Tags{"msgid": "c1"}})
	f.c.message(girc.Event{Command: "PRIVMSG", Source: src, Params: []string{"alice", "\x01VERSION\x01"}, Tags: girc.Tags{"msgid": "c2"}})
	if got := count(); got != before {
		t.Fatalf("CTCP stored as messages: %d -> %d", before, got)
	}
	f.c.message(girc.Event{Command: "PRIVMSG", Source: src, Params: []string{"alice", "\x01ACTION waves\x01"}, Tags: girc.Tags{"msgid": "c3"}})
	if got := count(); got != before+1 {
		t.Fatalf("ACTION not stored: %d -> %d", before, got)
	}
}
