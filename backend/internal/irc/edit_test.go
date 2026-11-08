package irc

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

var editCaps = map[string]bool{"message-tags": true, "echo-message": true, "account-tag": true, "draft/message-redaction": true}

// editFixture extends the redaction fixture with an editable message of your
// own (m4, sent from account alice) and a reply that quotes it.
func editFixture(t *testing.T, caps map[string]bool) (*Manager, *connection, Conversation) {
	t.Helper()
	m, c, conv, _ := redactFixture(t, caps)
	w := c.owner
	w.networks["network"] = Network{ID: "network", Account: "alice"}
	w.members["mallory"] = Member{ID: "mallory", Nick: "Mallory"}
	w.messages[conv.ID][1].SenderAccount = "peeracct" // m2, the peer's own message
	c.nicks["mallory"] = "mallory"
	w.messages[conv.ID] = append(w.messages[conv.ID],
		Message{ID: "network:m4", UpstreamID: "m4", ConversationID: conv.ID, SenderID: "alice", Account: "alice", Kind: "message", Text: "helo wrold", CreatedAt: time.Unix(100, 0).UTC(), Reactions: []Reaction{{Emoji: "👍", MemberIDs: []string{"peer"}}}},
		Message{ID: "network:m5", UpstreamID: "m5", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "what?", CreatedAt: time.Unix(101, 0).UTC(), Reactions: []Reaction{}, ReplyTo: &Reply{ID: "network:m4", Nick: "Alice", Text: "helo wrold"}},
	)
	return m, c, conv
}

func editEvent(nick, account, original, msgid, text string, at time.Time, extra girc.Tags) girc.Event {
	tags := girc.Tags{"msgid": msgid, editTag: original, "time": at.Format(time.RFC3339Nano)}
	if account != "" {
		tags["account"] = account
	}
	for k, v := range extra {
		tags[k] = v
	}
	return girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: nick, Ident: strings.ToLower(nick), Host: "h"}, Params: []string{"#test", text}, Tags: tags}
}

func TestEditFeatureNeedsTagsAndEcho(t *testing.T) {
	for _, tc := range []struct {
		caps     map[string]bool
		isupport map[string]string
		want     bool
	}{
		{map[string]bool{"message-tags": true, "echo-message": true, "account-tag": true}, nil, true},
		{map[string]bool{"message-tags": true, "account-tag": true}, nil, false},
		{map[string]bool{"echo-message": true, "account-tag": true}, nil, false},
		// Receivers verify an edit against the original's account.
		{map[string]bool{"message-tags": true, "echo-message": true}, nil, false},
		{map[string]bool{"message-tags": true, "echo-message": true, "account-tag": true}, map[string]string{"CLIENTTAGDENY": "*"}, false},
		{map[string]bool{"message-tags": true, "echo-message": true, "account-tag": true}, map[string]string{"CLIENTTAGDENY": "*,-ircord/edit"}, true},
		{map[string]bool{"message-tags": true, "echo-message": true, "account-tag": true}, map[string]string{"CLIENTTAGDENY": "ircord/edit"}, false},
	} {
		c := &connection{ready: true, caps: tc.caps}
		if got := c.features(tc.isupport).Edit; got != tc.want {
			t.Fatalf("%+v: edit=%v", tc, got)
		}
	}
}

func TestEditRejectsWhatCannotBeEdited(t *testing.T) {
	m, c, conv := editFixture(t, map[string]bool{"message-tags": true})
	if _, err := m.Edit("alice", conv.ID, "network:m4", "hello world"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("without echo-message: %v", err)
	}
	c.caps["echo-message"] = true
	c.caps["account-tag"] = true
	for _, tc := range []struct {
		message, text string
		want          error
	}{
		{"network:m2", "changed", ErrEditForbidden}, // someone else's message
		{"network:m1", "changed", ErrUnsupported},   // an upload
		{"local", "changed", ErrUnsupported},        // no network msgid
		{"network:m4", "helo wrold", ErrEditUnchanged},
		{"network:missing", "changed", ErrNotFound},
	} {
		if _, err := m.Edit("alice", conv.ID, tc.message, tc.text); !errors.Is(err, tc.want) {
			t.Fatalf("%+v: got %v", tc, err)
		}
	}
	c.owner.networks["network"] = Network{ID: "network", Account: "other"}
	if _, err := m.Edit("alice", conv.ID, "network:m4", "hello world"); !errors.Is(err, ErrEditForbidden) {
		t.Fatalf("another account: %v", err)
	}
	if len(c.queue) != 0 {
		t.Fatal("a rejected edit reached the network")
	}
	if got := stored(c, conv.ID, "network:m4"); got.Text != "helo wrold" || got.EditedAt != nil {
		t.Fatalf("rejected edit changed the message: %+v", got)
	}
}

func TestEditSendsTaggedResendAndFoldsTheEcho(t *testing.T) {
	m, c, conv := editFixture(t, editCaps)
	optimistic, err := m.Edit("alice", conv.ID, "network:m4", "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if optimistic.ID != "network:m4" || optimistic.Text != "hello world" || optimistic.EditedAt == nil {
		t.Fatalf("response: %+v", optimistic)
	}
	sent := <-c.queue
	if len(sent) != 1 || sent[0].Command != "PRIVMSG" || sent[0].Params[0] != "#test" || sent[0].Last() != "hello world" || sent[0].Tags[editTag] != "m4" {
		t.Fatalf("wire: %+v", sent)
	}
	if _, labeled := sent[0].Tags["label"]; labeled {
		t.Fatal("label sent without labeled-response")
	}
	if got := stored(c, conv.ID, "network:m4"); got.EditedAt != nil {
		t.Fatal("the edit was stored before the network echoed it")
	}
	at := time.Unix(200, 0).UTC()
	echo := editEvent("Alice", "alice", "m4", "e1", "hello world", at, nil)
	c.message(echo)
	got := stored(c, conv.ID, "network:m4")
	if got.Text != "hello world" || got.OriginalText != "helo wrold" || got.EditedAt == nil || !got.EditedAt.Equal(at) || len(got.Edits) != 1 || got.Edits[0].UpstreamID != "e1" {
		t.Fatalf("not folded: %+v", got)
	}
	if got.UpstreamID != "m4" || len(got.Reactions) != 1 || !got.CreatedAt.Equal(time.Unix(100, 0)) {
		t.Fatalf("identity, reactions or position changed: %+v", got)
	}
	if extra := stored(c, conv.ID, "e1"); extra.ID != "" {
		t.Fatal("the edit was also stored as a separate message")
	}
	if reply := stored(c, conv.ID, "network:m5"); reply.ReplyTo == nil || reply.ReplyTo.Text != "hello world" {
		t.Fatalf("reply excerpt not refreshed: %+v", reply.ReplyTo)
	}
	// History replay of the same edit is idempotent.
	c.messageWithHistory(echo, true)
	if got := stored(c, conv.ID, "network:m4"); len(got.Edits) != 1 {
		t.Fatalf("replayed edit applied twice: %+v", got.Edits)
	}
	updated := 0
	for _, e := range c.owner.events {
		var v Message
		if e.Type == "message.updated" && json.Unmarshal(e.Data, &v) == nil && v.ID == "network:m4" {
			updated++
		}
		if e.Type == "message.created" && strings.Contains(string(e.Data), "hello world") {
			t.Fatal("the edit was published as a new message")
		}
	}
	if updated != 1 {
		t.Fatalf("message.updated events: %d", updated)
	}
}

func TestEditWaitsForLabeledEcho(t *testing.T) {
	caps := map[string]bool{"labeled-response": true}
	for k, v := range editCaps {
		caps[k] = v
	}
	m, c, conv := editFixture(t, caps)
	run := func(text string) (chan Message, chan error, string) {
		done, fail := make(chan Message, 1), make(chan error, 1)
		go func() {
			msg, err := m.Edit("alice", conv.ID, "network:m4", text)
			done <- msg
			fail <- err
		}()
		sent := <-c.queue
		return done, fail, sent[0].Tags["label"]
	}
	done, fail, label := run("hello world")
	if !strings.HasPrefix(label, "edit:") {
		t.Fatalf("label %q", label)
	}
	c.message(editEvent("Alice", "alice", "m4", "e1", "hello world", time.Unix(200, 0).UTC(), girc.Tags{"label": label}))
	if msg, err := <-done, <-fail; err != nil || msg.Text != "hello world" || len(msg.Edits) != 1 {
		t.Fatalf("confirmed edit: %+v %v", msg, err)
	}
	done, fail, label = run("hello again")
	c.redactionResult(label, "Cannot send to channel")
	if _, err := <-done, <-fail; !errors.Is(err, ErrEditRefused) {
		t.Fatalf("refusal: %v", err)
	}
	old := EditTimeout
	EditTimeout = 20 * time.Millisecond
	defer func() { EditTimeout = old }()
	done, fail, _ = run("hello once more")
	if _, err := <-done, <-fail; !errors.Is(err, ErrEditUnconfirmed) {
		t.Fatalf("timeout: %v", err)
	}
	if got := stored(c, conv.ID, "network:m4"); got.Text != "hello world" {
		t.Fatalf("unconfirmed edits changed the message: %q", got.Text)
	}
}

func TestIncomingEditsRequireTheAuthorAndAccount(t *testing.T) {
	_, c, conv := editFixture(t, editCaps)
	at := time.Unix(300, 0).UTC()
	// The peer may edit the peer's own message from the same account.
	c.message(editEvent("Peer", "PeerAcct", "m2", "p1", "peer text, fixed", at, nil))
	if got := stored(c, conv.ID, "network:m2"); got.Text != "peer text, fixed" || got.EditedAt == nil {
		t.Fatalf("author edit not applied: %+v", got)
	}
	// Someone else's "edit" is an ordinary message and changes nothing.
	c.message(editEvent("Mallory", "mallory", "m2", "x1", "I said something else", at.Add(time.Second), nil))
	if got := stored(c, conv.ID, "network:m2"); got.Text != "peer text, fixed" {
		t.Fatalf("a non-author edited: %+v", got)
	}
	if got := stored(c, conv.ID, "x1"); got.Text != "I said something else" || got.SenderID != "mallory" || got.EditOf != "" {
		t.Fatalf("a rejected edit must show as an ordinary message: %+v", got)
	}
	// Whoever holds the peer's nick later cannot edit without the peer's
	// account: member ids follow nicks.
	c.message(editEvent("Peer", "", "m2", "x2", "nick only", at.Add(2*time.Second), nil))
	c.message(editEvent("Peer", "mallory", "m2", "x3", "other account", at.Add(3*time.Second), nil))
	if got := stored(c, conv.ID, "network:m2"); got.Text != "peer text, fixed" || len(got.Edits) != 1 {
		t.Fatalf("an edit from the same nick without the original account applied: %+v", got)
	}
	if stored(c, conv.ID, "x2").Text != "nick only" || stored(c, conv.ID, "x3").Text != "other account" {
		t.Fatal("refused edits must stay visible as ordinary lines")
	}
	// A message sent without an account cannot be edited at all.
	c.message(editEvent("Peer", "PeerAcct", "m3", "x4", "reply, fixed", at.Add(4*time.Second), nil))
	if got := stored(c, conv.ID, "network:m3"); got.Text != "reply" {
		t.Fatalf("a message without an account was edited: %+v", got)
	}
	// Your nick with a different account cannot edit your message.
	c.message(editEvent("Alice", "mallory", "m4", "x5", "forged", at.Add(5*time.Second), nil))
	if got := stored(c, conv.ID, "network:m4"); got.Text != "helo wrold" {
		t.Fatalf("an edit from another account applied: %+v", got)
	}
	// GIF and upload messages are not edited, even by their author.
	c.message(editEvent("Alice", "alice", "m1", "x6", "not a file any more", at.Add(6*time.Second), nil))
	if got := stored(c, conv.ID, "network:m1"); got.Attachment == nil || got.EditedAt != nil || got.Text != "https://files.example/a.png" {
		t.Fatalf("an upload was edited: %+v", got)
	}
	if stored(c, conv.ID, "x6").Text != "not a file any more" {
		t.Fatal("an edit of an upload must stay an ordinary line")
	}
}

func TestEditsOfDeletedMessagesAreDropped(t *testing.T) {
	_, c, conv := editFixture(t, editCaps)
	at := time.Unix(300, 0).UTC()
	c.message(editEvent("Peer", "peeracct", "m2", "p1", "secret correction", at, nil))
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "Peer", Ident: "p", Host: "h"}, Params: []string{"#test", "m2"}})
	got := stored(c, conv.ID, "network:m2")
	if !got.Deleted || got.Text != "Message deleted" || len(got.Edits) != 0 || got.EditedAt != nil || got.OriginalText != "" {
		t.Fatalf("deleted message after edit: %+v", got)
	}
	// An edit that crosses the deletion does not bring its text back.
	c.message(editEvent("Peer", "peeracct", "m2", "p2", "resurrected", at.Add(time.Second), nil))
	if extra := stored(c, conv.ID, "p2"); extra.ID != "" {
		t.Fatalf("an edit of a deleted message was stored as a line: %+v", extra)
	}
}

func TestDeletingAnEditedMessageDeletesItsEditsAndScrubsTheirLines(t *testing.T) {
	m, c, conv := editFixture(t, editCaps)
	at := time.Unix(300, 0).UTC()
	c.message(editEvent("Alice", "alice", "m4", "e1", "secret one", at, nil))
	c.message(editEvent("Alice", "alice", "m4", "e2", "secret two", at.Add(time.Second), nil))
	c.owner.serverLogs["network"] = []model.ServerLine{
		{Command: "PRIVMSG", Params: []string{"#test", "secret one"}, Text: "secret one", Tags: map[string]string{"msgid": "e1"}},
		{Command: "PRIVMSG", Params: []string{"#test", "secret two"}, Text: "secret two", Tags: map[string]string{"msgid": "e2"}},
	}
	if _, _, err := m.Redact("alice", conv.ID, "network:m4"); err != nil {
		t.Fatal(err)
	}
	redacted := map[string]bool{}
	for len(c.queue) > 0 {
		for _, e := range <-c.queue {
			if e.Command == "REDACT" {
				redacted[e.Params[1]] = true
			}
		}
	}
	if !redacted["m4"] || !redacted["e1"] || !redacted["e2"] {
		t.Fatalf("REDACTs sent: %v", redacted)
	}
	for _, line := range c.owner.serverLogs["network"] {
		if strings.Contains(line.Text, "secret") {
			t.Fatalf("an edit's text survived in the server window: %+v", line)
		}
	}
}

func TestEditArrivingBeforeItsOriginalIsFoldedLater(t *testing.T) {
	_, c, conv := editFixture(t, editCaps)
	at := time.Unix(500, 0).UTC()
	edit := editEvent("Peer", "peeracct", "late1", "le1", "corrected", at.Add(time.Minute), nil)
	c.messageWithHistory(edit, true)
	if got := stored(c, conv.ID, "le1"); got.Text != "corrected" || got.EditOf != "late1" {
		t.Fatalf("an early edit must wait as a line: %+v", got)
	}
	original := girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "Peer", Ident: "peer", Host: "h"}, Params: []string{"#test", "typo"}, Tags: girc.Tags{"msgid": "late1", "account": "peeracct", "time": at.Format(time.RFC3339Nano)}}
	c.messageWithHistory(original, true)
	got := stored(c, conv.ID, "late1")
	if got.Text != "corrected" || got.EditedAt == nil || got.OriginalText != "typo" {
		t.Fatalf("original after scrollback: %+v", got)
	}
	if extra := stored(c, conv.ID, "le1"); extra.ID != "" {
		t.Fatalf("the folded edit is still a separate line: %+v", extra)
	}
	// Replaying the edit afterwards is a no-op.
	c.messageWithHistory(edit, true)
	if got := stored(c, conv.ID, "late1"); len(got.Edits) != 1 {
		t.Fatalf("replay: %+v", got.Edits)
	}
	if extra := stored(c, conv.ID, "le1"); extra.ID != "" {
		t.Fatal("a replayed edit came back as a line")
	}
}

func TestEditOrderingRevertAndDeletion(t *testing.T) {
	_, c, conv := editFixture(t, editCaps)
	base := time.Unix(400, 0).UTC()
	// A later edit replayed before an earlier one still wins.
	c.messageWithHistory(editEvent("Alice", "alice", "m4", "e2", "second", base.Add(2*time.Second), nil), true)
	c.messageWithHistory(editEvent("Alice", "alice", "m4", "e1", "first", base.Add(time.Second), nil), true)
	got := stored(c, conv.ID, "network:m4")
	if got.Text != "second" || len(got.Edits) != 2 || got.OriginalText != "helo wrold" {
		t.Fatalf("ordering: %+v", got)
	}
	for _, e := range c.owner.events {
		if e.Type == "message.updated" {
			t.Fatal("a historical edit emitted a live message.updated")
		}
	}
	// Redacting the newest edit reverts to the previous version.
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "Alice", Ident: "a", Host: "h"}, Params: []string{"#test", "e2"}})
	if got := stored(c, conv.ID, "network:m4"); got.Text != "first" || len(got.Edits) != 1 {
		t.Fatalf("revert: %+v", got)
	}
	// Someone else cannot revert your edit.
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "Mallory", Ident: "m", Host: "h"}, Params: []string{"#test", "e1"}})
	if got := stored(c, conv.ID, "network:m4"); got.Text != "first" {
		t.Fatalf("a non-author reverted an edit: %+v", got)
	}
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "Alice", Ident: "a", Host: "h"}, Params: []string{"#test", "e1"}})
	if got := stored(c, conv.ID, "network:m4"); got.Text != "helo wrold" || got.EditedAt != nil || got.OriginalText != "" || len(got.Edits) != 0 {
		t.Fatalf("all edits reverted: %+v", got)
	}
}

func TestEditsChangeTheMentionCount(t *testing.T) {
	m, store, c, conv := storedRedactFixture(t, editCaps)
	c.owner.mu.Lock()
	c.owner.messages[conv.ID][0].SenderAccount = "peeracct" // m1 mentions Alice
	c.owner.mu.Unlock()
	count := func() int {
		n, err := store.Mentions("alice", conv.ID)
		if err != nil {
			t.Fatal(err)
		}
		c.owner.mu.Lock()
		defer c.owner.mu.Unlock()
		if got := c.owner.conversations[conv.ID].MentionCount; got != n {
			t.Fatalf("conversation shows %d mentions, storage %d", got, n)
		}
		return n
	}
	at := time.Now().UTC().Add(time.Minute)
	cursor := m.Snapshot("alice").Cursor
	c.message(editEvent("Peer", "peeracct", "m1", "p1", "a secret for everyone", at, nil))
	if n := count(); n != 0 {
		t.Fatalf("an edit that drops your nick still counts as a mention: %d", n)
	}
	if counts := countsAfter(t, m, "alice", cursor); counts.ID != conv.ID || counts.MentionCount != 0 {
		t.Fatalf("edit recount sent %+v", counts)
	}
	cursor = m.Snapshot("alice").Cursor
	c.message(editEvent("Peer", "peeracct", "m1", "p2", "a secret for Alice again", at.Add(time.Second), nil))
	if n := count(); n != 1 {
		t.Fatalf("an edit that names you does not count as a mention: %d", n)
	}
	if counts := countsAfter(t, m, "alice", cursor); counts.MentionCount != 1 {
		t.Fatalf("edit recount sent %+v", counts)
	}
}

func TestKeptEditsDropOnlyEditsMissingInsideTheWindow(t *testing.T) {
	base := time.Unix(1000, 0).UTC()
	result := historyResult{IDs: map[string]bool{"in-kept": true}}
	result.Oldest.At, result.Newest.At = base, base.Add(time.Hour)
	edits := []model.MessageEdit{
		{UpstreamID: "before", At: base.Add(-time.Minute)},
		{UpstreamID: "in-kept", At: base.Add(time.Minute)},
		{UpstreamID: "in-gone", At: base.Add(2 * time.Minute)},
		{UpstreamID: "after", At: base.Add(2 * time.Hour)},
	}
	var ids []string
	for _, v := range keptEdits(edits, result) {
		ids = append(ids, v.UpstreamID)
	}
	if strings.Join(ids, ",") != "before,in-kept,after" {
		t.Fatalf("kept %v", ids)
	}
}
