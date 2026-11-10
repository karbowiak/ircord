package irc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

func historyFixture(t *testing.T) *inboxFixture {
	t.Helper()
	f := newInboxFixture(t)
	c := f.c
	c.ctx = f.m.ctx
	c.ready = true
	c.done = make(chan struct{})
	c.queue = make(chan []*girc.Event, 8)
	c.caps = map[string]bool{"draft/chathistory": true, "batch": true, "server-time": true, "labeled-response": true}
	c.historyGate = make(chan struct{}, 1)
	c.historyRequests = map[string]*historyRequest{}
	c.batchLabels = map[string]string{}
	c.catching = map[string]bool{}
	c.batches = map[string]*multiline{}
	c.owner.connections["net"] = c
	return f
}

type historyRead struct {
	page MessagePage
	err  error
}

func historyCommand(t *testing.T, c *connection, sub, reference string, finished <-chan historyRead) *girc.Event {
	t.Helper()
	select {
	case events := <-c.queue:
		if len(events) != 1 || events[0].Command != "CHATHISTORY" || len(events[0].Params) != 4 || events[0].Params[0] != sub || events[0].Params[2] != reference || events[0].Params[3] != "100" {
			t.Fatalf("unexpected history request: %+v", events)
		}
		return events[0]
	case result := <-finished:
		t.Fatalf("history finished before %s retry: items=%d, error=%v", sub, len(result.page.Items), result.err)
	case <-time.After(2 * time.Second):
		t.Fatalf("history did not request %s", sub)
	}
	return nil
}

// historyReply uses the actual batch correlation and persistence paths, without
// connecting to an IRC server or exposing any real account history.
func historyReply(c *connection, request *girc.Event, end bool, items ...girc.Event) {
	ref := "history-test"
	tags := girc.Tags{"label": request.Tags["label"]}
	if end {
		tags["draft/chathistory-end"] = ""
	}
	opening := girc.Event{Command: "BATCH", Params: []string{"+" + ref, "chathistory", request.Params[1]}, Tags: tags}
	c.batch(opening)
	c.historyBatch(opening)
	for _, item := range items {
		item.Tags["batch"] = ref
		c.historyItem(ref, item)
		if item.Command == "PRIVMSG" || item.Command == "NOTICE" {
			c.message(item)
		}
	}
	closing := girc.Event{Command: "BATCH", Params: []string{"-" + ref}}
	c.batch(closing)
	c.historyBatch(closing)
}

func historyMessage(at time.Time, msgid string) girc.Event {
	return girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob", Ident: "bob", Host: "test.invalid"}, Params: []string{"#test", "fixture"}, Tags: girc.Tags{"msgid": msgid, "time": at.Format(time.RFC3339Nano)}}
}

func TestHistoryBeforeMissingUpstreamIDRetriesTimestamp(t *testing.T) {
	f := historyFixture(t)
	f.say(t, "alice", "#test", "removed-upstream", "fixture")
	conv := f.c.conversationID("#test")
	finished := make(chan historyRead, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() {
		page, err := f.m.HistoryContext(ctx, "alice", conv, "", 40)
		finished <- historyRead{page, err}
	}()
	request := historyCommand(t, f.c, "BEFORE", "msgid=removed-upstream", finished)
	// Ergo returns an empty successful batch for an unknown msgid, including
	// the end tag. This does not prove that older messages are unavailable.
	historyReply(f.c, request, true)
	request = historyCommand(t, f.c, "BEFORE", "timestamp="+f.at.Format(ircTimeLayout), finished)
	historyReply(f.c, request, true, historyMessage(f.at.Add(-time.Hour), "older"))
	result := <-finished
	if result.err != nil || len(result.page.Items) != 2 || result.page.Items[0].UpstreamID != "older" || result.page.NextCursor != nil {
		t.Fatalf("older history was lost: items=%d, cursor=%v, error=%v", len(result.page.Items), result.page.NextCursor, result.err)
	}
	if _, err := f.m.HistoryContext(ctx, "bob", conv, "", 40); err != ErrNotFound {
		t.Fatalf("another account read the conversation: %v", err)
	}
}

func TestHistoryCatchupMissingUpstreamIDRetriesTimestamp(t *testing.T) {
	f := historyFixture(t)
	f.say(t, "alice", "#test", "removed-upstream", "fixture")
	conv := f.c.conversationID("#test")
	if err := f.store.BeginGap("alice", conv); err != nil {
		t.Fatal(err)
	}
	anchor := f.at
	// A new live event must not advance an open gap past the missing messages.
	f.at = f.at.Add(24 * time.Hour)
	f.say(t, "alice", "#test", "phone-live", "fixture")
	f.c.catchup("#test")
	request := historyCommand(t, f.c, "AFTER", "msgid=removed-upstream", nil)
	historyReply(f.c, request, true)
	request = historyCommand(t, f.c, "AFTER", "timestamp="+anchor.Format(ircTimeLayout), nil)
	historyReply(f.c, request, true, historyMessage(anchor.Add(time.Hour), "missed"))
	eventually(t, func() bool {
		mark, err := f.store.Watermark("alice", conv)
		return err == nil && !mark.Gap
	})
	page, err := f.store.History("alice", conv, "", 40)
	if err != nil || len(page.Items) != 3 || page.Items[1].UpstreamID != "missed" {
		t.Fatalf("catch-up lost the gap: items=%d, error=%v", len(page.Items), err)
	}
}

func TestHistoryBeforeNonMessagePageAdvancesUpstream(t *testing.T) {
	for _, command := range []string{"TAGMSG", "REDACT"} {
		t.Run(command, func(t *testing.T) {
			testHistoryBeforeNonMessagePage(t, command)
		})
	}
}

func testHistoryBeforeNonMessagePage(t *testing.T, command string) {
	f := historyFixture(t)
	f.say(t, "alice", "#test", "anchor", "fixture")
	conv := f.c.conversationID("#test")
	stored, err := f.store.Oldest("alice", conv)
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan historyRead, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() {
		page, err := f.m.HistoryContext(ctx, "alice", conv, storage.Cursor(stored), 40)
		finished <- historyRead{page, err}
	}()
	request := historyCommand(t, f.c, "BEFORE", "msgid=anchor", finished)
	item := historyMessage(f.at.Add(-time.Minute), "reaction-only")
	item.Command = command
	historyReply(f.c, request, false, item)
	request = historyCommand(t, f.c, "BEFORE", "msgid=reaction-only", finished)
	historyReply(f.c, request, true, historyMessage(f.at.Add(-time.Hour), "older"))
	result := <-finished
	if result.err != nil || len(result.page.Items) != 1 || result.page.Items[0].UpstreamID != "older" {
		t.Fatalf("non-message page blocked older history: items=%d, error=%v", len(result.page.Items), result.err)
	}
}

func TestHistoryPaginationBoundDoesNotReportBeginning(t *testing.T) {
	f := historyFixture(t)
	f.say(t, "alice", "#test", "anchor", "fixture")
	conv := f.c.conversationID("#test")
	stored, err := f.store.Oldest("alice", conv)
	if err != nil {
		t.Fatal(err)
	}
	before := storage.Cursor(stored)
	finished := make(chan historyRead, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() {
		page, err := f.m.HistoryContext(ctx, "alice", conv, before, 40)
		finished <- historyRead{page, err}
	}()
	upstream := "anchor"
	for i := 0; i < 10; i++ {
		request := historyCommand(t, f.c, "BEFORE", "msgid="+upstream, finished)
		upstream = fmt.Sprintf("reaction-%d", i)
		item := historyMessage(f.at.Add(-time.Duration(i+1)*time.Minute), upstream)
		item.Command = "TAGMSG"
		historyReply(f.c, request, false, item)
	}
	result := <-finished
	if result.err != nil || len(result.page.Items) != 0 || result.page.NextCursor == nil || result.page.SyncError == "" {
		t.Fatalf("request bound became a false beginning: items=%d, retry=%v, warning=%v, error=%v", len(result.page.Items), result.page.NextCursor != nil, result.page.SyncError != "", result.err)
	}
	at, boundary, err := storage.ParseCursor(*result.page.NextCursor)
	if err != nil || at != stored.CreatedAt.UnixNano() || boundary != stored.ID {
		t.Fatalf("upstream continuation skipped retained rows: same_boundary=%v, error=%v", boundary == stored.ID, err)
	}
	if f.store.HistoryEnded("alice", conv) {
		t.Fatal("request bound persisted false exhaustion")
	}
	go func() {
		page, err := f.m.HistoryContext(ctx, "alice", conv, *result.page.NextCursor, 40)
		finished <- historyRead{page, err}
	}()
	request := historyCommand(t, f.c, "BEFORE", "msgid="+upstream, finished)
	historyReply(f.c, request, true, historyMessage(f.at.Add(-time.Hour), "older"))
	resumed := <-finished
	if resumed.err != nil || len(resumed.page.Items) != 1 || resumed.page.Items[0].UpstreamID != "older" {
		t.Fatalf("retry lost its upstream continuation: items=%d, error=%v", len(resumed.page.Items), resumed.err)
	}
}

func TestHistoryAutomaticReconnectProtectsWatermark(t *testing.T) {
	port, peers := capabilityServer(t, "message-tags batch server-time", "")
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err = store.CreateAccount("alice", "fixture-key"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store, AllowPrivate: true, ReconnectMin: 10 * time.Millisecond})
	t.Cleanup(m.Close)
	if _, err = m.Connect("alice", NetworkConfig{ID: "net", Name: "test", Host: "127.0.0.1", Port: port, Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	first := <-peers
	c, _ := m.connection("alice", "net")
	anchor := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	c.message(historyMessage(anchor, "anchor"))
	conv := c.conversationID("#test")
	if err = store.SetHistoryEnd("alice", conv, true); err != nil {
		t.Fatal(err)
	}
	if err = first.conn.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-peers:
	case <-ctx.Done():
		t.Fatal("automatic reconnect did not start")
	}
	// The gap must be open before the new transport can deliver live messages.
	c.message(historyMessage(anchor.Add(time.Hour), "phone-live"))
	mark, err := store.Watermark("alice", conv)
	if err != nil || !mark.Gap || mark.ID != "anchor" {
		t.Fatalf("reconnect skipped the missing interval: gap=%v, at_anchor=%v, error=%v", mark.Gap, mark.ID == "anchor", err)
	}
	if store.HistoryEnded("alice", conv) {
		t.Fatal("reconnect trusted a stale upstream exhaustion marker")
	}
}
