package irc

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

type inboxFixture struct {
	m     *Manager
	store *storage.Store
	dir   string
	c     *connection
	at    time.Time
}

// newInboxFixture signs alice (nick "alice") into network "net" with no client
// open and no push subscription: only the backend sees what arrives.
func newInboxFixture(t *testing.T) *inboxFixture {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner); err != nil {
			t.Fatal(err)
		}
	}
	m := New(Options{Store: store})
	t.Cleanup(func() { m.Close(); store.Close() })
	f := &inboxFixture{m: m, store: store, dir: dir, at: time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)}
	f.c = &connection{owner: m.space("alice"), manager: m, config: NetworkConfig{ID: "net", Nick: "alice"}, nicks: map[string]string{}, historyBatches: map[string]bool{}, historyResponses: map[string]*historyRequest{}}
	return f
}

// say delivers a live PRIVMSG from nick to target with an upstream msgid.
func (f *inboxFixture) say(t *testing.T, nick, target, msgid, text string, tags ...string) {
	t.Helper()
	f.at = f.at.Add(time.Second)
	e := girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: nick, Ident: nick, Host: nick + ".example"}, Params: []string{target, text}, Tags: girc.Tags{"msgid": msgid, "time": f.at.Format(time.RFC3339Nano)}}
	for i := 0; i+1 < len(tags); i += 2 {
		e.Tags[tags[i]] = tags[i+1]
	}
	f.c.message(e)
	if err := f.c.owner.storageErr; err != nil {
		t.Fatal(err)
	}
}

func (f *inboxFixture) settings(t *testing.T, raw string) {
	t.Helper()
	if _, err := f.m.SaveAppData("alice", "notification-settings.updated", []byte(raw)); err != nil {
		t.Fatal(err)
	}
}

func (f *inboxFixture) inbox(t *testing.T, owner, filter string) InboxPage {
	t.Helper()
	page, err := f.m.Inbox(owner, filter, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func ids(page InboxPage) []string {
	out := []string{}
	for _, item := range page.Items {
		out = append(out, item.MessageID)
	}
	return out
}

func inboxEvents(t *testing.T, m *Manager, owner string) []string {
	t.Helper()
	events, _, ok, stop := m.Subscribe(owner, 0)
	defer stop()
	if !ok {
		t.Fatal("replay unavailable")
	}
	kinds := []string{}
	for _, e := range events {
		if strings.HasPrefix(e.Type, "inbox.") {
			kinds = append(kinds, e.Type)
		}
	}
	return kinds
}

func TestInboxRecordsMentionsRepliesAndDirectMessagesOnce(t *testing.T) {
	f := newInboxFixture(t)
	f.say(t, "bob", "#dev", "m1", "hello alice, are you there?")
	f.say(t, "bob", "#dev", "m2", "nothing for you here")
	f.say(t, "bob", "alice", "d1", "psst alice")
	// The same delivery again (replay, another device's echo) adds nothing.
	f.say(t, "bob", "#dev", "m1", "hello alice, are you there?")

	// A reply to alice's own stored message, without her nick in it.
	w := f.c.owner
	w.mu.Lock()
	dev := f.c.conversation("#dev")
	w.emit("message.created", Message{ID: "net:mine", UpstreamID: "mine", ConversationID: dev.ID, SenderID: w.self.ID, Kind: "message", Text: "my question", CreatedAt: f.at.Add(time.Second), Reactions: []Reaction{}})
	w.mu.Unlock()
	f.at = f.at.Add(time.Second)
	f.say(t, "bob", "#dev", "r1", "good question", "+draft/reply", "mine")
	// A reply to someone else's message is not a reply to you.
	f.say(t, "carol", "#dev", "r2", "agreed", "+draft/reply", "m2")

	all := f.inbox(t, "alice", "all")
	if got := ids(all); !slices.Equal(got, []string{"net:r1", "net:d1", "net:m1"}) {
		t.Fatalf("inbox items: %v", got)
	}
	reasons := map[string][]string{}
	for _, item := range all.Items {
		reasons[item.MessageID] = item.Reasons
		if item.Status != "available" || !item.Unread || item.Dismissed || item.NetworkID != "net" || item.SenderID == "" {
			t.Fatalf("item state: %+v", item)
		}
	}
	// One message matching several categories is one item carrying them all.
	if !slices.Equal(reasons["net:d1"], []string{"mention", "direct"}) || !slices.Equal(reasons["net:r1"], []string{"reply"}) || !slices.Equal(reasons["net:m1"], []string{"mention"}) {
		t.Fatalf("reasons: %v", reasons)
	}
	if all.Items[2].Excerpt != "hello alice, are you there?" {
		t.Fatalf("excerpt: %q", all.Items[2].Excerpt)
	}
	for filter, want := range map[string][]string{"mentions": {"net:d1", "net:m1"}, "replies": {"net:r1"}, "direct": {"net:d1"}, "unread": {"net:r1", "net:d1", "net:m1"}, "dismissed": {}} {
		if got := ids(f.inbox(t, "alice", filter)); !slices.Equal(got, want) {
			t.Fatalf("%s: %v, want %v", filter, got, want)
		}
	}
	if all.Unread != 3 || len(inboxEvents(t, f.m, "alice")) != 3 {
		t.Fatalf("unread %d, events %v", all.Unread, inboxEvents(t, f.m, "alice"))
	}
	// The inbox is account-private.
	if got := f.inbox(t, "bob", "all"); len(got.Items) != 0 || got.Unread != 0 {
		t.Fatalf("bob sees alice's inbox: %+v", got)
	}
	if err := f.m.DismissInbox("bob", "net:m1", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob dismissed alice's item: %v", err)
	}
}

// batch opens a CHATHISTORY response batch of the given subcommand.
func (f *inboxFixture) batch(ref, sub string) {
	f.c.historyBatches[ref] = true
	f.c.historyResponses[ref] = &historyRequest{sub: sub}
}

func TestInboxIgnoresScrollbackFirstFillOwnMessagesAndEvents(t *testing.T) {
	f := newInboxFixture(t)
	// History fetched while scrolling back, or a new conversation's first
	// fill, is not new attention.
	f.batch("b", "BEFORE")
	f.say(t, "bob", "#dev", "h1", "old news, alice", "batch", "b")
	f.batch("l", "LATEST")
	f.say(t, "bob", "#new", "h2", "older news, alice", "batch", "l")
	f.say(t, "alice", "#dev", "s1", "talking about alice myself")
	if got := f.inbox(t, "alice", "all"); len(got.Items) != 0 {
		t.Fatalf("recorded: %v", ids(got))
	}
}

func TestInboxRecordsMessagesRecoveredAfterADisconnect(t *testing.T) {
	f := newInboxFixture(t)
	f.say(t, "bob", "alice", "d0", "are you there?")
	// IRCord was offline; reconnecting catches up AFTER the stored watermark.
	f.batch("c", "AFTER")
	f.say(t, "bob", "alice", "d1", "missed while you were away", "batch", "c")
	f.say(t, "carol", "#dev", "m1", "alice, also missed", "batch", "c")
	f.say(t, "carol", "#dev", "m2", "not for you", "batch", "c")
	// Catch-up older than the inbox's retention (a long-forgotten DM found
	// through TARGETS) is history, not attention.
	at := f.at
	f.at = time.Now().Add(-storage.InboxRetention - time.Hour)
	f.say(t, "dave", "alice", "old", "from long ago", "batch", "c")
	f.at = at
	// The same catch-up again (a second reconnect) adds nothing.
	f.say(t, "bob", "alice", "d1", "missed while you were away", "batch", "c")
	page := f.inbox(t, "alice", "all")
	if got := ids(page); !slices.Equal(got, []string{"net:m1", "net:d1", "net:d0"}) {
		t.Fatalf("inbox after catch-up: %v", got)
	}
	if page.Unread != 3 || len(inboxEvents(t, f.m, "alice")) != 3 {
		t.Fatalf("unread %d, events %v", page.Unread, inboxEvents(t, f.m, "alice"))
	}
}

func TestInboxMergesReasonsForTheSameMessage(t *testing.T) {
	f := newInboxFixture(t)
	f.say(t, "bob", "#dev", "m1", "alice: hi")
	w := f.c.owner
	w.mu.Lock()
	w.emit("inbox.added", InboxEntry{MessageID: "net:m1", ConversationID: f.c.conversationID("#dev"), NetworkID: "net", SenderID: "x", Reasons: []string{"reply"}, CreatedAt: f.at})
	err := w.storageErr
	w.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	page := f.inbox(t, "alice", "all")
	if len(page.Items) != 1 || !slices.Equal(page.Items[0].Reasons, []string{"mention", "reply"}) {
		t.Fatalf("merged: %+v", page.Items)
	}
}

func TestInboxReplyToADeletedMessageIsNotAReply(t *testing.T) {
	f := newInboxFixture(t)
	w := f.c.owner
	w.mu.Lock()
	dev := f.c.conversation("#dev")
	w.emit("message.created", Message{ID: "net:mine", UpstreamID: "mine", ConversationID: dev.ID, SenderID: w.self.ID, Kind: "message", Text: "Message deleted", Deleted: true, CreatedAt: f.at, Reactions: []Reaction{}})
	w.mu.Unlock()
	f.say(t, "bob", "#dev", "r1", "replying anyway", "+draft/reply", "mine")
	if got := f.inbox(t, "alice", "all"); len(got.Items) != 0 {
		t.Fatalf("reply to a deleted message recorded: %+v", got.Items)
	}
}

func TestInboxItemsGoWithTheirNetwork(t *testing.T) {
	f := newInboxFixture(t)
	f.say(t, "bob", "alice", "d1", "hi")
	other := &connection{owner: f.c.owner, manager: f.m, config: NetworkConfig{ID: "two", Nick: "alice"}, nicks: map[string]string{}, historyBatches: map[string]bool{}, historyResponses: map[string]*historyRequest{}}
	other.message(girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob", Ident: "bob", Host: "h"}, Params: []string{"alice", "elsewhere"}, Tags: girc.Tags{"msgid": "x1", "time": f.at.Add(time.Minute).Format(time.RFC3339Nano)}})
	if got := f.inbox(t, "alice", "all"); len(got.Items) != 2 {
		t.Fatalf("before removal: %v", ids(got))
	}
	w := f.c.owner
	w.mu.Lock()
	w.emit("network.removed", model.Removal{ID: "net"})
	w.mu.Unlock()
	page := f.inbox(t, "alice", "all")
	if got := ids(page); !slices.Equal(got, []string{"two:x1"}) || page.Unread != 1 {
		t.Fatalf("after removal: %v unread %d", got, page.Unread)
	}
}

func TestInboxEligibilityKeepsQuietHoursButHonoursIgnoresAndMutes(t *testing.T) {
	f := newInboxFixture(t)
	// Quiet hours covering the whole day only hold back interruptions.
	f.settings(t, `{"quietHours":{"enabled":true,"start":"00:00","end":"23:59","timeZone":"UTC"},"ignore":["spammer"]}`)
	f.say(t, "bob", "#dev", "q1", "alice: during quiet hours")
	f.say(t, "spammer", "#dev", "i1", "alice buy now")
	f.say(t, "spammer", "alice", "i2", "hello")
	if got := ids(f.inbox(t, "alice", "all")); !slices.Equal(got, []string{"net:q1"}) {
		t.Fatalf("with quiet hours and ignores: %v", got)
	}
	// A direct conversation muted with level "none" and a channel whose
	// shared level is "none" record nothing.
	w := f.c.owner
	w.mu.Lock()
	direct := f.c.conversation("carol")
	muted := f.c.conversation("#muted")
	muted.ChannelSettings = &model.ChannelSettings{Notifications: "none"}
	w.conversations[muted.ID] = muted
	w.mu.Unlock()
	f.settings(t, fmt.Sprintf(`{"levels":{%q:"none"}}`, direct.ID))
	f.say(t, "carol", "carol", "n1", "alice?")
	f.say(t, "bob", "#muted", "n2", "alice in a muted channel")
	if got := ids(f.inbox(t, "alice", "all")); !slices.Equal(got, []string{"net:q1"}) {
		t.Fatalf("muted conversations recorded: %v", got)
	}

	// Ignoring a sender later hides their items and their count at once.
	f.say(t, "dave", "#dev", "x1", "alice look")
	if n, _ := f.m.InboxSummary("alice"); n.Unread != 2 {
		t.Fatalf("unread before ignore: %+v", n)
	}
	f.settings(t, `{"ignore":["dave"]}`)
	if got := ids(f.inbox(t, "alice", "all")); !slices.Equal(got, []string{"net:q1"}) {
		t.Fatalf("ignored sender still listed: %v", got)
	}
	if n, _ := f.m.InboxSummary("alice"); n.Unread != 1 {
		t.Fatalf("unread after ignore: %+v", n)
	}
}

func TestInboxDismissAndReadStateAreSeparate(t *testing.T) {
	f := newInboxFixture(t)
	f.say(t, "bob", "#dev", "m1", "alice one")
	f.say(t, "bob", "#dev", "m2", "alice two")
	dev := f.c.owner.conversations[f.c.conversationID("#dev")]

	if err := f.m.DismissInbox("alice", "net:m1", true); err != nil {
		t.Fatal(err)
	}
	// Repeating it is a no-op without another event.
	if err := f.m.DismissInbox("alice", "net:m1", true); err != nil {
		t.Fatal(err)
	}
	if got := inboxEvents(t, f.m, "alice"); !slices.Equal(got, []string{"inbox.added", "inbox.added", "inbox.updated"}) {
		t.Fatalf("events: %v", got)
	}
	// Dismissing is not reading: the conversation keeps both unread messages.
	if at, err := f.store.ReadTime("alice", dev.ID); err != nil || !at.IsZero() {
		t.Fatalf("dismiss moved the read marker: %v %v", at, err)
	}
	if n, err := f.store.Unread("alice", dev.ID); err != nil || n != 2 {
		t.Fatalf("conversation unread after dismiss: %d %v", n, err)
	}
	if got := ids(f.inbox(t, "alice", "all")); !slices.Equal(got, []string{"net:m2"}) {
		t.Fatalf("all after dismiss: %v", got)
	}
	if got := ids(f.inbox(t, "alice", "dismissed")); !slices.Equal(got, []string{"net:m1"}) {
		t.Fatalf("dismissed: %v", got)
	}
	if n, _ := f.m.InboxSummary("alice"); n.Unread != 1 {
		t.Fatalf("unread after dismiss: %+v", n)
	}
	if err := f.m.DismissInbox("alice", "net:m1", false); err != nil {
		t.Fatal(err)
	}
	if n, _ := f.m.InboxSummary("alice"); n.Unread != 2 {
		t.Fatalf("unread after restore: %+v", n)
	}

	// Reading the conversation reads its items, which stay listed.
	read := true
	if _, err := f.m.UpdateConversation("alice", dev.ID, &read, nil); err != nil {
		t.Fatal(err)
	}
	page := f.inbox(t, "alice", "all")
	if len(page.Items) != 2 || page.Items[0].Unread || page.Unread != 0 {
		t.Fatalf("after reading: %+v", page)
	}
	if got := f.inbox(t, "alice", "unread"); len(got.Items) != 0 {
		t.Fatalf("unread view after reading: %v", ids(got))
	}
	if err := f.m.DismissInbox("alice", "net:missing", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown item: %v", err)
	}
}

func TestInboxFollowsDeletionLeavingAndRestart(t *testing.T) {
	f := newInboxFixture(t)
	f.say(t, "bob", "#dev", "m1", "alice secret plans")
	f.say(t, "bob", "#gone", "g1", "alice in a channel you will leave")
	f.say(t, "bob", "#dev", "m2", "alice still here")
	w := f.c.owner
	w.mu.Lock()
	msg, ok := f.c.lookupMessage(f.c.conversationID("#dev"), "net:m1")
	if !ok {
		w.mu.Unlock()
		t.Fatal("message not stored")
	}
	msg.Deleted, msg.Text = true, "Message deleted"
	w.emit("message.updated", msg)
	gone := f.c.conversationID("#gone")
	delete(w.conversations, gone)
	w.emit("conversation.removed", model.Removal{ID: gone})
	w.mu.Unlock()

	check := func(m *Manager) {
		t.Helper()
		page, err := m.Inbox("alice", "all", "", 100)
		if err != nil {
			t.Fatal(err)
		}
		status := map[string]InboxItem{}
		for _, item := range page.Items {
			status[item.MessageID] = item
		}
		if d := status["net:m1"]; d.Status != "deleted" || d.Excerpt != "" || d.Unread {
			t.Fatalf("deleted item keeps content: %+v", d)
		}
		if g := status["net:g1"]; g.Status != "unavailable" || g.Excerpt != "" || g.SenderID != "" || g.Unread {
			t.Fatalf("left conversation still shown: %+v", g)
		}
		if s := status["net:m2"]; s.Status != "available" || !s.Unread {
			t.Fatalf("available item: %+v", s)
		}
		if page.Unread != 1 {
			t.Fatalf("unread %d", page.Unread)
		}
	}
	check(f.m)
	// A backend restart keeps the items and their state.
	f.m.Close()
	f.store.Close()
	store, err := storage.Open(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m := New(Options{Store: store})
	defer m.Close()
	<-m.space("alice").loaded
	check(m)
}

func TestInboxRetentionAndPaging(t *testing.T) {
	limit := storage.InboxLimit
	storage.InboxLimit = 4
	t.Cleanup(func() { storage.InboxLimit = limit })
	f := newInboxFixture(t)
	for i := range 6 {
		f.say(t, "bob", "#dev", fmt.Sprintf("m%d", i), "alice "+strings.Repeat("x", 400*(i%2)))
	}
	page, err := f.m.Inbox("alice", "all", "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(page); !slices.Equal(got, []string{"net:m5", "net:m4", "net:m3"}) || page.NextCursor == nil {
		t.Fatalf("first page: %v", got)
	}
	if e := page.Items[0].Excerpt; !page.Items[0].Truncated || len([]rune(e)) > InboxExcerptRunes+1 || !strings.HasSuffix(e, "…") {
		t.Fatalf("excerpt not bounded: %d runes", len([]rune(e)))
	}
	next, err := f.m.Inbox("alice", "all", *page.NextCursor, 3)
	if err != nil {
		t.Fatal(err)
	}
	// The two oldest were pruned when the limit was exceeded.
	if got := ids(next); !slices.Equal(got, []string{"net:m2"}) || next.NextCursor != nil {
		t.Fatalf("second page: %v", got)
	}
	if _, err = f.m.Inbox("alice", "all", "bogus", 3); !errors.Is(err, storage.ErrInvalidCursor) {
		t.Fatalf("bad cursor: %v", err)
	}
	if _, err = f.m.Inbox("alice", "everything", "", 3); err == nil {
		t.Fatal("unknown filter accepted")
	}

	// Items past retention are hidden at once and pruned.
	if _, err = f.store.DB.Exec("UPDATE inbox_items SET created_ns=? WHERE message='net:m2'", time.Now().Add(-storage.InboxRetention-time.Hour).UnixNano()); err != nil {
		t.Fatal(err)
	}
	if got := ids(f.inbox(t, "alice", "all")); slices.Contains(got, "net:m2") {
		t.Fatalf("expired item listed: %v", got)
	}
	if err = f.store.PruneInbox(time.Now()); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = f.store.DB.QueryRow("SELECT count(*) FROM inbox_items WHERE owner='alice'").Scan(&n); err != nil || n != 3 {
		t.Fatalf("after prune: %d %v", n, err)
	}
}

func TestGroupMentionsCountForTheMembersTheyInclude(t *testing.T) {
	f := newInboxFixture(t)
	w := f.c.owner
	w.mu.Lock()
	dev := f.c.conversation("#dev")
	dev.MemberModes = map[string]string{w.self.ID: "+", f.c.member("opal", "").ID: "@"}
	dev.ChannelSettings = &model.ChannelSettings{Notifications: "default", GroupMentions: "ops"}
	w.conversations[dev.ID] = dev
	w.mu.Unlock()
	setting := func(v string) {
		w.mu.Lock()
		conv := w.conversations[dev.ID]
		conv.ChannelSettings = &model.ChannelSettings{Notifications: "default", GroupMentions: v}
		w.conversations[dev.ID] = conv
		w.mu.Unlock()
	}

	// "ops" (the default): only an operator reaches the channel.
	f.say(t, "opal", "#dev", "g1", "@here standup")
	f.say(t, "bob", "#dev", "g0", "@everyone buy my stuff")
	// Role mentions are open to everyone, and match by your own status.
	f.say(t, "bob", "#dev", "g2", "@operator please kick the spammer")
	f.say(t, "bob", "#dev", "g3", "@voiced you can still talk")
	// A group mention in a direct message is just text, though DMs always reach you.
	f.say(t, "bob", "alice", "d1", "@operator hi")
	// "all": anyone may.
	setting("all")
	f.say(t, "bob", "#dev", "a1", "@channel lunch?")
	// "off": nobody, operators included.
	setting("off")
	f.say(t, "opal", "#dev", "o1", "@everyone fire drill")
	f.say(t, "bob", "#dev", "o2", "@voiced still works")

	// Away: @here and @channel skip you, @everyone does not.
	setting("all")
	// Away is per network: being away elsewhere changes nothing here.
	w.mu.Lock()
	w.awayOn = map[string]bool{"other": true}
	w.mu.Unlock()
	f.say(t, "bob", "#dev", "w0", "@here still around?")
	w.mu.Lock()
	w.awayOn["net"] = true
	w.mu.Unlock()
	f.say(t, "bob", "#dev", "w1", "@here anyone?")
	f.say(t, "bob", "#dev", "w2", "@everyone release is out")
	// Your own group mention never notifies you.
	f.say(t, "alice", "#dev", "s1", "@everyone hi")
	// Recovered after a disconnect: you were not there for @everyone, but a
	// role mention or your nick still reaches you.
	f.batch("c", "AFTER")
	f.say(t, "bob", "#dev", "r1", "@everyone while you were gone", "batch", "c")
	f.say(t, "bob", "#dev", "r2", "@voiced while you were gone", "batch", "c")
	f.say(t, "bob", "#dev", "r3", "alice, while you were gone", "batch", "c")

	reasons := map[string][]string{}
	for _, item := range f.inbox(t, "alice", "all").Items {
		reasons[item.MessageID] = item.Reasons
	}
	want := map[string][]string{"net:g1": {"mention"}, "net:g3": {"mention"}, "net:d1": {"direct"}, "net:a1": {"mention"}, "net:o2": {"mention"}, "net:w0": {"mention"}, "net:w2": {"mention"}, "net:r2": {"mention"}, "net:r3": {"mention"}}
	if len(reasons) != len(want) {
		t.Fatalf("inbox reasons %v, want %v", reasons, want)
	}
	for id, r := range want {
		if !slices.Equal(reasons[id], r) {
			t.Fatalf("inbox reasons %v, want %v", reasons, want)
		}
	}
	w.mu.Lock()
	count := w.conversations[dev.ID].MentionCount
	w.mu.Unlock()
	// Recovered messages are history: they are not counted live.
	if count != 6 {
		t.Fatalf("mention count %d, want 6", count)
	}
}
