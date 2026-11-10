package irc

import (
	"context"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/activity"
	"ircord/backend/internal/notify"
)

func routingConnection(m *Manager, w *workspace, caps ...string) *connection {
	set := map[string]bool{}
	for _, c := range caps {
		set[c] = true
	}
	c := &connection{owner: w, manager: m, ctx: context.Background(), ready: true, queue: make(chan []*girc.Event, 16), caps: set, config: NetworkConfig{ID: "net", Nick: "alice"}, nicks: map[string]string{}}
	w.mu.Lock()
	w.connections["net"] = c
	w.mu.Unlock()
	return c
}

// setAway records a network's own away state as RPL_NOWAWAY / RPL_UNAWAY do.
func setAway(w *workspace, network string, away bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.awayOn == nil {
		w.awayOn = map[string]bool{}
	}
	w.awayOn[network] = away
}

func drain(c *connection) []*girc.Event {
	out := []*girc.Event{}
	for len(c.queue) > 0 {
		out = append(out, <-c.queue...)
	}
	return out
}

// Automatic away (#136) uses IRC's own AWAY, so every client sees it, and never overrides an
// away the user chose.
func TestAutoAwayUsesIRCAwayAndRespectsManualAway(t *testing.T) {
	m := New(Options{AllowPrivate: true})
	defer m.Close()
	w := m.space("alice")
	c := routingConnection(m, w)

	m.SetAutoAway("alice", true)
	sent := drain(c)
	if len(sent) != 1 || sent[0].Command != "AWAY" || len(sent[0].Params) != 1 || sent[0].Params[0] != autoAwayReason {
		t.Fatalf("expected AWAY with the automatic reason, got %v", sent)
	}
	m.SetAutoAway("alice", true)
	if sent := drain(c); len(sent) != 0 {
		t.Fatalf("away is sent once, got %v", sent)
	}
	m.SetAutoAway("alice", false)
	sent = drain(c)
	if len(sent) != 1 || sent[0].Command != "AWAY" || len(sent[0].Params) != 0 {
		t.Fatalf("coming back clears the automatic away, got %v", sent)
	}

	// Already away by choice on this network: nothing is sent either way.
	setAway(w, "net", true)
	m.SetAutoAway("alice", true)
	m.SetAutoAway("alice", false)
	if sent := drain(c); len(sent) != 0 {
		t.Fatalf("a manual away is left alone, got %v", sent)
	}
	// Away on one network says nothing about another: the other one still goes away.
	other := &connection{owner: w, manager: m, ctx: context.Background(), ready: true, queue: make(chan []*girc.Event, 16), caps: map[string]bool{}, config: NetworkConfig{ID: "other", Nick: "alice"}, nicks: map[string]string{}}
	w.mu.Lock()
	w.connections["other"] = other
	w.mu.Unlock()
	m.SetAutoAway("alice", true)
	if sent := drain(other); len(sent) != 1 || sent[0].Command != "AWAY" {
		t.Fatalf("a present network goes away even when another is away, got %v", sent)
	}
	if sent := drain(c); len(sent) != 0 {
		t.Fatalf("the network away by choice is still left alone, got %v", sent)
	}
	m.SetAutoAway("alice", false)
	drain(other)
	w.mu.Lock()
	delete(w.connections, "other")
	w.mu.Unlock()

	// The user sets away themselves while automatically away: coming back keeps their choice.
	setAway(w, "net", false)
	m.SetAutoAway("alice", true)
	drain(c)
	if err := m.SetAway("alice", "net", "lunch"); err != nil {
		t.Fatal(err)
	}
	drain(c)
	m.SetAutoAway("alice", false)
	if sent := drain(c); len(sent) != 0 {
		t.Fatalf("the user's own away must survive coming back, got %v", sent)
	}

	// A reconnect while automatically away goes away again on the new connection.
	m.SetAutoAway("alice", true)
	drain(c)
	c.resumePresence()
	if sent := drain(c); len(sent) != 1 || sent[0].Command != "AWAY" || sent[0].Params[0] != autoAwayReason {
		t.Fatalf("a new connection is away again, got %v", sent)
	}
}

// The device form factor is shared with the ircord/device metadata key only when the network
// negotiated draft/metadata-3, and other IRCord users' values are shown.
func TestDeviceMetadataIsCapabilityGated(t *testing.T) {
	m := New(Options{AllowPrivate: true})
	defer m.Close()
	w := m.space("alice")
	plain := routingConnection(m, w)
	m.SetDevice("alice", activity.Phone)
	if sent := drain(plain); len(sent) != 0 {
		t.Fatalf("no metadata without draft/metadata-3, got %v", sent)
	}
	if m.Snapshot("alice").User.Device != "phone" {
		t.Fatal("our own device is shown on our profile")
	}
	c := routingConnection(m, w, "draft/metadata-3")
	m.SetDevice("alice", activity.Tablet)
	sent := drain(c)
	if len(sent) != 1 || sent[0].Command != "METADATA" || len(sent[0].Params) != 4 || sent[0].Params[2] != deviceKey || sent[0].Params[3] != "tablet" {
		t.Fatalf("expected METADATA * SET ircord/device tablet, got %v", sent)
	}
	m.SetDevice("alice", "desktop")
	sent = drain(c)
	if len(sent) != 1 || len(sent[0].Params) != 3 {
		t.Fatalf("a desktop clears the key, got %v", sent)
	}
	m.SetDevice("alice", "")
	if sent := drain(c); len(sent) != 0 {
		t.Fatalf("an unchanged device is not sent again, got %v", sent)
	}

	event := func(line string) girc.Event { return *girc.ParseEvent(line) }
	device := func() string {
		for _, v := range m.Snapshot("alice").Members {
			if v.Nick == "bob" {
				return v.Device
			}
		}
		return "missing"
	}
	c.metadata(event(":irc.test METADATA bob ircord/device * :phone"))
	if got := device(); got != "phone" {
		t.Fatalf("peer device not shown: %q", got)
	}
	c.metadata(event(":irc.test METADATA bob ircord/device * :watch"))
	if got := device(); got != "phone" {
		t.Fatalf("unknown values are ignored: %q", got)
	}
	c.metadata(event(":irc.test METADATA bob ircord/device *"))
	if got := device(); got != "" {
		t.Fatalf("a removed key clears the device: %q", got)
	}
}

// Escalations recheck a candidate right before alerting with it (#84).
func TestEligibleRechecksReadDeletedMutedAndQuiet(t *testing.T) {
	m := New(Options{AllowPrivate: true})
	defer m.Close()
	w := m.space("alice")
	at := time.Now().Add(-time.Minute)
	w.mu.Lock()
	w.conversations["dm"] = Conversation{ID: "dm", Kind: "direct", NetworkID: "net"}
	w.conversations["room"] = Conversation{ID: "room", Kind: "channel", NetworkID: "net"}
	w.messages["dm"] = []Message{{ID: "m1", ConversationID: "dm", CreatedAt: at, Kind: "message"}, {ID: "m2", ConversationID: "dm", CreatedAt: at.Add(time.Second), Kind: "message", Deleted: true}}
	w.messages["room"] = []Message{{ID: "r1", ConversationID: "room", CreatedAt: at, Kind: "message"}}
	w.mu.Unlock()
	item := activity.Item{Conversation: "dm", Message: "m1", At: at, Kind: "mention"}
	if !m.Eligible("alice", item) {
		t.Fatal("an unread message is eligible")
	}
	if m.Eligible("alice", activity.Item{Conversation: "dm", Message: "m2", At: at.Add(time.Second), Kind: "mention"}) {
		t.Fatal("a deleted message is not eligible")
	}
	if m.Eligible("alice", activity.Item{Conversation: "dm", Message: "gone", At: at, Kind: "mention"}) {
		t.Fatal("an unknown message is not eligible")
	}
	if m.Eligible("alice", activity.Item{Conversation: "room", Message: "r1", At: at, Kind: "message"}) {
		t.Fatal("a plain channel message is not eligible under the default mentions level")
	}
	w.mu.Lock()
	w.readTimes = map[string]time.Time{"dm": at}
	w.mu.Unlock()
	if m.Eligible("alice", item) {
		t.Fatal("a read message is not eligible")
	}
	w.mu.Lock()
	w.readTimes = nil
	w.mu.Unlock()
	item.SenderNick, item.SenderUser, item.SenderHost = "troll", "t", "example.net"
	if !m.Eligible("alice", item) {
		t.Fatal("not ignored yet")
	}
	ignoring := notify.Defaults()
	ignoring.Ignore = []string{"troll"}
	w.mu.Lock()
	w.policy = notify.New(ignoring)
	w.mu.Unlock()
	if m.Eligible("alice", item) {
		t.Fatal("a sender ignored during the interval is not eligible")
	}
	w.mu.Lock()
	settings := notify.Defaults()
	settings.Levels = map[string]string{"dm": "none"}
	w.policy = notify.New(settings)
	w.mu.Unlock()
	if m.Eligible("alice", item) {
		t.Fatal("a muted conversation is not eligible")
	}
	settings = notify.Defaults()
	now := time.Now().UTC()
	settings.QuietHours = notify.QuietHours{Enabled: true, Start: now.Add(-time.Hour).Format("15:04"), End: now.Add(time.Hour).Format("15:04"), TimeZone: "UTC"}
	w.mu.Lock()
	w.policy = notify.New(settings)
	w.mu.Unlock()
	if m.Eligible("alice", item) {
		t.Fatal("quiet hours hold back escalations")
	}
}

type recordingPusher struct{ alerts []activity.Alert }

func (p *recordingPusher) Route(_ string, alert activity.Alert, _ activity.Target, _ func()) {
	p.alerts = append(p.alerts, alert)
}
func (p *recordingPusher) HasSubscription(string, string) bool { return true }

// Live messages go through the router when one is configured, which holds them back while a
// client is active.
func TestNotifyMessageGoesThroughTheRouter(t *testing.T) {
	pusher := &recordingPusher{}
	router := activity.New(activity.Options{Push: pusher})
	_, stopDevice := router.Subscribe("alice", "device", "tab")
	defer stopDevice()
	m := New(Options{AllowPrivate: true, Router: router})
	defer m.Close()
	router.SetSource(m)
	w := m.space("alice")
	conv := Conversation{ID: "dm", Kind: "direct", NetworkID: "net"}
	sender := Member{ID: "bob", Nick: "bob", DisplayName: "Bob"}
	w.mu.Lock()
	m.notifyMessage(w, conv, Message{ID: "m1", ConversationID: "dm", Kind: "message", Text: "hi", CreatedAt: time.Now()}, sender, "alice")
	w.mu.Unlock()
	if len(pusher.alerts) != 1 || pusher.alerts[0].Title != "Bob" || pusher.alerts[0].Kind != "mention" || pusher.alerts[0].MessageID != "m1" {
		t.Fatalf("expected one routed alert, got %+v", pusher.alerts)
	}
	router.Report("alice", "laptop", "tab", true, activity.Desktop, false)
	w.mu.Lock()
	m.notifyMessage(w, conv, Message{ID: "m2", ConversationID: "dm", Kind: "message", Text: "again", CreatedAt: time.Now()}, sender, "alice")
	w.mu.Unlock()
	if len(pusher.alerts) != 1 {
		t.Fatalf("an active client holds alerts back, got %+v", pusher.alerts)
	}
}

// A permitted group mention alerts like a personal mention; a gated one does not.
func TestNotifyMessageAlertsForGroupMentions(t *testing.T) {
	pusher := &recordingPusher{}
	router := activity.New(activity.Options{Push: pusher})
	_, stopDevice := router.Subscribe("alice", "device", "tab")
	defer stopDevice()
	m := New(Options{AllowPrivate: true, Router: router})
	defer m.Close()
	router.SetSource(m)
	w := m.space("alice")
	conv := Conversation{ID: "net:#dev", Kind: "channel", Name: "dev", NetworkID: "net", MemberModes: map[string]string{"op": "@"}, ChannelSettings: &ChannelSettings{Notifications: "default", GroupMentions: "ops"}}
	w.mu.Lock()
	m.notifyMessage(w, conv, Message{ID: "m1", ConversationID: conv.ID, SenderID: "bob", Kind: "message", Text: "@here hi", CreatedAt: time.Now()}, Member{ID: "bob", Nick: "bob"}, "alice")
	m.notifyMessage(w, conv, Message{ID: "m2", ConversationID: conv.ID, SenderID: "op", Kind: "message", Text: "@here hi", CreatedAt: time.Now()}, Member{ID: "op", Nick: "op"}, "alice")
	w.mu.Unlock()
	if len(pusher.alerts) != 1 || pusher.alerts[0].MessageID != "m2" || pusher.alerts[0].Kind != "mention" {
		t.Fatalf("expected one mention alert for the operator's @here, got %+v", pusher.alerts)
	}
}

func TestNotifyMessageRepliesRemainEligibleByDefault(t *testing.T) {
	pusher := &recordingPusher{}
	router := activity.New(activity.Options{Push: pusher})
	_, stop := router.Subscribe("alice", "device", "tab")
	defer stop()
	m := New(Options{AllowPrivate: true, Router: router})
	defer m.Close()
	router.SetSource(m)
	w := m.space("alice")
	conv := Conversation{ID: "room", Kind: "channel", Name: "room", NetworkID: "net"}
	sender := Member{ID: "bob", Nick: "bob"}
	msg := Message{ID: "reply", ConversationID: "room", SenderID: "bob", Kind: "message", Text: "a reply without a mention", CreatedAt: time.Now()}
	w.mu.Lock()
	m.notifyMessage(w, conv, msg, sender, "alice", true)
	w.mu.Unlock()
	if len(pusher.alerts) != 1 || pusher.alerts[0].Kind != "mention" {
		t.Fatal("reply did not notify under default mentions", pusher.alerts)
	}
}

// /afk and /brb (#240) are plain IRC AWAY: the text shows on your own presence once the server
// confirms it, and the next message you send ends it. Any other away lasts until /back.
func TestBriefAwayShowsOnPresenceAndEndsWithTheNextMessage(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{})
	c.pending = map[string]pending{}
	w := c.owner
	send := func(id string) {
		t.Helper()
		if err := m.Send("alice", SendInput{ClientID: id, ConversationID: conv.ID, Text: "hi"}); err != nil {
			t.Fatal(err)
		}
	}
	awayAfterSend := func(id string) bool {
		t.Helper()
		send(id)
		for _, e := range drain(c) {
			if e.Command == "AWAY" {
				if len(e.Params) != 0 {
					t.Fatalf("sending a message set away again: %v", e)
				}
				return true
			}
		}
		return false
	}

	if err := m.RawCommand("alice", "network", "afk1", "AWAY :AFK: lunch"); err != nil {
		t.Fatal(err)
	}
	drain(c)
	c.receive(nil, *girc.ParseEvent(":server.example 306 Alice :You have been marked as being away"))
	w.mu.Lock()
	self := w.self
	w.mu.Unlock()
	if self.Status != "away" || self.AwayMessage != "AFK: lunch" {
		t.Fatalf("your presence shows the AFK text, got %q %q", self.Status, self.AwayMessage)
	}
	if !awayAfterSend("m1") {
		t.Fatal("sending a message ends an AFK away")
	}
	if awayAfterSend("m2") {
		t.Fatal("the away ends once")
	}
	c.receive(nil, *girc.ParseEvent(":server.example 305 Alice :You are no longer marked as being away"))
	w.mu.Lock()
	self = w.self
	w.mu.Unlock()
	if self.Status != "online" || self.AwayMessage != "" {
		t.Fatalf("back clears the away text, got %q %q", self.Status, self.AwayMessage)
	}

	if err := m.SetAway("alice", "network", "BRB"); err != nil {
		t.Fatal(err)
	}
	drain(c)
	if !awayAfterSend("m3") {
		t.Fatal("sending a message ends a BRB away")
	}

	for _, line := range []string{"AWAY :Lunch", "AWAY :afkish"} {
		if err := m.RawCommand("alice", "network", "away", line); err != nil {
			t.Fatal(err)
		}
		drain(c)
		if awayAfterSend("m-" + line[6:]) {
			t.Fatalf("%q lasts until /back", line)
		}
	}
	// /back after /afk: nothing left to clear.
	_ = m.RawCommand("alice", "network", "afk2", "AWAY :AFK")
	_ = m.RawCommand("alice", "network", "back", "AWAY")
	drain(c)
	if awayAfterSend("m4") {
		t.Fatal("after /back a message sends no AWAY")
	}
}

// Other people's AWAY text (away-notify) is kept on their member.
func TestAwayNotifyKeepsTheAwayText(t *testing.T) {
	_, c, _, _ := redactFixture(t, map[string]bool{})
	c.receive(nil, *girc.ParseEvent(":Peer!p@host AWAY :BRB: coffee"))
	v := c.owner.members["peer"]
	if v.Status != "away" || v.AwayMessage != "BRB: coffee" {
		t.Fatalf("got %q %q", v.Status, v.AwayMessage)
	}
	c.receive(nil, *girc.ParseEvent(":Peer!p@host AWAY"))
	v = c.owner.members["peer"]
	if v.Status != "online" || v.AwayMessage != "" {
		t.Fatalf("got %q %q", v.Status, v.AwayMessage)
	}
}

// A reconnect during automatic away re-sends the automatic reason, and presence follows it
// rather than an earlier /afk text.
func TestReconnectDuringAutoAwayShowsTheAutomaticReason(t *testing.T) {
	m, c, _, _ := redactFixture(t, map[string]bool{})
	w := c.owner
	if err := m.RawCommand("alice", "network", "afk", "AWAY :AFK: lunch"); err != nil {
		t.Fatal(err)
	}
	c.receive(nil, *girc.ParseEvent(":server.example 306 Alice :away"))
	w.mu.Lock()
	w.autoAway = true
	w.mu.Unlock()
	drain(c)
	c.resumePresence()
	drain(c)
	c.receive(nil, *girc.ParseEvent(":server.example 306 Alice :away"))
	w.mu.Lock()
	got := w.self.AwayMessage
	w.mu.Unlock()
	if got != autoAwayReason {
		t.Fatalf("want %q, got %q", autoAwayReason, got)
	}
}
