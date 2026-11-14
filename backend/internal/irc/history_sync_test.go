package irc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
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
	c.client = girc.New(girc.Config{Server: "test", Nick: "alice", User: "alice", Name: "alice"})
	conn, peer := net.Pipe()
	c.transport = conn
	attemptDone := make(chan struct{})
	c.attemptDone = attemptDone
	t.Cleanup(func() { _ = conn.Close(); _ = peer.Close(); close(attemptDone) })
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
		c.mu.Lock()
		req := c.takeHistoryWrite(events[0])
		started := req != nil && c.startHistoryWrite(req, c.client)
		c.mu.Unlock()
		if !started {
			t.Fatal("history request could not be dispatched")
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

func historyFailureEvent(request *girc.Event, reason string) girc.Event {
	return girc.Event{Command: "FAIL", Source: &girc.Source{Name: "irc.example"}, Tags: girc.Tags{"label": request.Tags["label"]}, Params: []string{"CHATHISTORY", "INVALID_PARAMS", request.Params[0], request.Params[1], request.Params[2], reason}}
}

func TestHistoryFailureCatchupRetriesWithoutCommandError(t *testing.T) {
	for _, mode := range []string{"labeled", "batch-labeled", "unlabeled", "labeled-short", "unlabeled-short", "unlabeled-no-context"} {
		t.Run(mode, func(t *testing.T) {
			f := historyFixture(t)
			if strings.HasPrefix(mode, "unlabeled") {
				delete(f.c.caps, "labeled-response")
			}
			f.say(t, "alice", "#test", "removed-upstream", "fixture")
			conv := f.c.conversationID("#test")
			if err := f.store.BeginGap("alice", conv); err != nil {
				t.Fatal(err)
			}
			anchor := f.at
			f.at = f.at.Add(24 * time.Hour)
			f.say(t, "alice", "#test", "phone-live", "fixture")
			_, live, _, stop := f.m.Subscribe("alice", 0)
			defer stop()
			f.c.catchup("#test")
			request := historyCommand(t, f.c, "AFTER", "msgid=removed-upstream", nil)
			failure := historyFailureEvent(request, "Invalid AFTER reference")
			if strings.HasSuffix(mode, "-short") {
				failure.Params = []string{"CHATHISTORY", "INVALID_PARAMS", "AFTER", "Invalid AFTER reference"}
			}
			if mode == "unlabeled-no-context" {
				failure.Params = []string{"CHATHISTORY", "INVALID_PARAMS", "Invalid AFTER reference"}
			}
			if mode == "batch-labeled" {
				f.c.mu.Lock()
				f.c.batchLabels["reply-batch"] = request.Tags["label"]
				f.c.mu.Unlock()
				failure.Tags = girc.Tags{"batch": "reply-batch"}
			}
			f.c.receive(f.c.client, failure)
			request = historyCommand(t, f.c, "AFTER", "timestamp="+anchor.Format(ircTimeLayout), nil)
			historyReply(f.c, request, true, historyMessage(anchor.Add(time.Hour), "missed"))
			eventually(t, func() bool {
				mark, err := f.store.Watermark("alice", conv)
				return err == nil && !mark.Gap
			})
			page, err := f.store.History("alice", conv, "", 40)
			if err != nil || len(page.Items) != 3 || page.Items[1].UpstreamID != "missed" || page.Items[2].UpstreamID != "phone-live" {
				t.Fatalf("catch-up lost source order: items=%+v, error=%v", page.Items, err)
			}
			for len(live) > 0 {
				if event := <-live; event.Type == "command.error" || event.Type == "history.sync.failed" {
					t.Fatalf("recovered failure was reported: %+v", event)
				}
			}
			f.c.owner.mu.Lock()
			trace := append([]ServerLine(nil), f.c.owner.trace["net"]...)
			f.c.owner.mu.Unlock()
			found := false
			for _, line := range trace {
				found = found || line.Command == "FAIL" && line.Text == "Invalid AFTER reference"
			}
			if !found {
				t.Fatal("server failure missing from diagnostics")
			}
		})
	}
}

func TestHistoryFailurePreservesUnrelatedAndRawErrors(t *testing.T) {
	for _, labeled := range []bool{true} {
		t.Run(fmt.Sprintf("labeled=%t", labeled), func(t *testing.T) {
			f := historyFixture(t)
			if !labeled {
				delete(f.c.caps, "labeled-response")
			}
			_, live, _, stop := f.m.Subscribe("alice", 0)
			defer stop()
			_, foreign, _, stopForeign := f.m.Subscribe("bob", 0)
			defer stopForeign()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			go func() { _, err := f.c.syncHistory(ctx, "#test", "AFTER", "msgid=anchor", 100); finished <- err }()
			request := historyCommand(t, f.c, "AFTER", "msgid=anchor", nil)
			if err := f.m.RawCommand("alice", "net", "raw-client", "CHATHISTORY AFTER #test msgid=anchor 100"); err != nil {
				t.Fatal(err)
			}
			raw := (<-f.c.queue)[0]
			failure := historyFailureEvent(raw, "raw failure")
			f.c.receiveCommandFailure(failure)
			select {
			case got := <-live:
				if got.Type != "command.error" || got.Message != "raw failure" || labeled && got.ClientID != "raw-client" {
					t.Fatalf("raw failure: %+v", got)
				}
			case <-time.After(time.Second):
				t.Fatal("raw failure was swallowed")
			}
			f.c.receiveCommandFailure(girc.Event{Command: "FAIL", Params: []string{"JOIN", "INVALID_TARGET", "#test", "unrelated failure"}})
			if got := <-live; got.Type != "command.error" || got.Message != "unrelated failure" {
				t.Fatalf("unrelated failure: %+v", got)
			}
			f.c.receiveCommandFailure(girc.Event{Command: "FAIL", Tags: girc.Tags{"label": "unknown"}, Params: []string{"CHATHISTORY", "INVALID_PARAMS", "unmatched failure"}})
			if got := <-live; got.Type != "command.error" || got.Message != "unmatched failure" {
				t.Fatalf("unmatched failure: %+v", got)
			}
			select {
			case err := <-finished:
				t.Fatalf("unrelated failure completed internal request: %v", err)
			default:
			}
			select {
			case got := <-foreign:
				t.Fatalf("failure crossed accounts: %+v", got)
			default:
			}
			historyReply(f.c, request, true)
			if err := <-finished; err != nil {
				t.Fatalf("internal request failed: %v", err)
			}
		})
	}
}

func TestHistoryFailureTerminalReasonUsesHistoryWorkflow(t *testing.T) {
	f := historyFixture(t)
	f.c.config.SASLPassword = "fixture-secret"
	f.say(t, "alice", "#test", "anchor", "fixture")
	conv := f.c.conversationID("#test")
	if err := f.store.BeginGap("alice", conv); err != nil {
		t.Fatal(err)
	}
	_, live, _, stop := f.m.Subscribe("alice", 0)
	defer stop()
	f.c.catchup("#test")
	request := historyCommand(t, f.c, "AFTER", "msgid=anchor", nil)
	f.c.receiveCommandFailure(historyFailureEvent(request, "Invalid AFTER reference"))
	request = historyCommand(t, f.c, "AFTER", "timestamp="+f.at.Format(ircTimeLayout), nil)
	f.c.receiveCommandFailure(historyFailureEvent(request, "Messages unavailable: fixture-secret"))
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-live:
			if event.Type == "command.error" {
				t.Fatalf("internal failure emitted command.error: %+v", event)
			}
			if event.Type == "history.sync.failed" {
				var data map[string]string
				if err := json.Unmarshal(event.Data, &data); err != nil {
					t.Fatal(err)
				}
				if data["conversationId"] != conv || !strings.Contains(data["message"], "Messages unavailable: [redacted]") || strings.Contains(data["message"], "fixture-secret") {
					t.Fatalf("incorrect terminal failure: %v", data)
				}
				mark, err := f.store.Watermark("alice", conv)
				if err != nil || !mark.Gap {
					t.Fatalf("failed catch-up closed gap: %+v, %v", mark, err)
				}
				return
			}
		case <-timer.C:
			t.Fatal("history failure was not surfaced")
		}
	}
}

func TestHistoryFailureUnlabeledMatchingIsConnectionLocal(t *testing.T) {
	first, second := historyFixture(t), historyFixture(t)
	delete(first.c.caps, "labeled-response")
	delete(second.c.caps, "labeled-response")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make([]chan error, 2)
	for i, f := range []*inboxFixture{first, second} {
		completed[i] = make(chan error, 1)
		go func(f *inboxFixture, done chan error) {
			_, err := f.c.syncHistory(ctx, "#test", "AFTER", "msgid=anchor", 100)
			done <- err
		}(f, completed[i])
	}
	request := historyCommand(t, first.c, "AFTER", "msgid=anchor", nil)
	other := historyCommand(t, second.c, "AFTER", "msgid=anchor", nil)
	_, live, _, stop := first.m.Subscribe("alice", 0)
	defer stop()
	unmatched := historyFailureEvent(request, "different reference")
	unmatched.Params[4] = "msgid=other"
	first.c.receiveCommandFailure(unmatched)
	if got := <-live; got.Type != "command.error" || got.Message != "different reference" {
		t.Fatalf("unmatched failure: %+v", got)
	}
	short := girc.Event{Command: "FAIL", Params: []string{"CHATHISTORY", "INVALID_PARAMS", "request rejected"}}
	first.c.receiveCommandFailure(short)
	if err := <-completed[0]; err == nil || err.Error() != "request rejected" {
		t.Fatalf("first request: %v", err)
	}
	select {
	case err := <-completed[1]:
		t.Fatalf("failure completed another connection's request: %v", err)
	default:
	}
	historyReply(second.c, other, true)
	if err := <-completed[1]; err != nil {
		t.Fatalf("second request: %v", err)
	}
}

func TestHistoryFailureUnlabeledRawBeforeInternalRequest(t *testing.T) {
	f := historyFixture(t)
	delete(f.c.caps, "labeled-response")
	if err := f.m.RawCommand("alice", "net", "raw-client", "CHATHISTORY AFTER #test msgid=anchor 100"); err != nil {
		t.Fatal(err)
	}
	<-f.c.queue
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make(chan error, 1)
	go func() { _, err := f.c.syncHistory(ctx, "#test", "AFTER", "msgid=anchor", 100); completed <- err }()
	_, live, _, stop := f.m.Subscribe("alice", 0)
	defer stop()
	f.c.receiveCommandFailure(girc.Event{Command: "FAIL", Params: []string{"CHATHISTORY", "INVALID_PARAMS", "raw failure"}})
	if got := <-live; got.Type != "command.error" || got.ClientID != "raw-client" || got.Message != "raw failure" {
		t.Fatalf("raw failure: %+v", got)
	}
	request := historyCommand(t, f.c, "AFTER", "msgid=anchor", nil)
	select {
	case err := <-completed:
		t.Fatalf("raw failure completed internal request: %v", err)
	default:
	}
	historyReply(f.c, request, true)
	if err := <-completed; err != nil {
		t.Fatalf("internal request: %v", err)
	}
}

func TestHistoryFailureRawGateRelease(t *testing.T) {
	for _, completion := range []string{"enqueue-failed", "batch", "disconnect"} {
		t.Run(completion, func(t *testing.T) {
			f := historyFixture(t)
			delete(f.c.caps, "labeled-response")
			line := "CHATHISTORY AFTER #test msgid=anchor 100"
			if completion == "enqueue-failed" {
				for i := 0; i < cap(f.c.queue); i++ {
					f.c.queue <- []*girc.Event{{Command: "PING"}}
				}
				if err := f.m.RawCommand("alice", "net", "raw-client", line); err != ErrBusy {
					t.Fatalf("full queue: %v", err)
				}
				for len(f.c.queue) > 0 {
					<-f.c.queue
				}
			} else {
				if err := f.m.RawCommand("alice", "net", "raw-client", line); err != nil {
					t.Fatal(err)
				}
				raw := (<-f.c.queue)[0]
				f.c.mu.Lock()
				rawWrite := f.c.takeRawHistoryWrite(raw)
				f.c.mu.Unlock()
				if err := f.m.RawCommand("alice", "net", "another-raw", line); err != ErrBusy {
					t.Fatalf("overlapping raw request: %v", err)
				}
				if completion == "batch" {
					historyReply(f.c, raw, true)
				} else {
					// A transport boundary marks the connection not ready before
					// failing pending history, as in connection.run.
					f.c.mu.Lock()
					f.c.ready = false
					f.c.mu.Unlock()
					f.c.failHistory()
					f.c.mu.Lock()
					if f.c.startRawHistoryWrite(rawWrite, nil) {
						t.Fatal("queued raw command survived its transport")
					}
					f.c.ready = true
					f.c.mu.Unlock()
				}
			}
			_, live, _, stop := f.m.Subscribe("alice", 0)
			defer stop()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := f.c.syncHistory(ctx, "#test", "AFTER", "msgid=anchor", 100); done <- err }()
			historyCommand(t, f.c, "AFTER", "msgid=anchor", nil)
			if err := f.m.RawCommand("alice", "net", "raw-client", line); err != ErrBusy {
				t.Fatalf("raw request overlapped internal request: %v", err)
			}
			f.c.receiveCommandFailure(girc.Event{Command: "FAIL", Params: []string{"CHATHISTORY", "INVALID_PARAMS", "AFTER", "Invalid AFTER reference"}})
			if err := <-done; err == nil || err.Error() != "Invalid AFTER reference" {
				t.Fatalf("internal failure: %v", err)
			}
			select {
			case got := <-live:
				if got.Type == "command.error" {
					t.Fatalf("internal failure leaked: %+v", got)
				}
			default:
			}
		})
	}
}

func TestHistoryFailureRawTimeoutIsBounded(t *testing.T) {
	f := historyFixture(t)
	delete(f.c.caps, "labeled-response")
	_, live, _, stop := f.m.Subscribe("alice", 0)
	defer stop()
	line := "CHATHISTORY AFTER #test msgid=anchor 100"
	if err := f.m.RawCommand("alice", "net", "raw-client", line); err != nil {
		t.Fatal(err)
	}
	oldEvent := (<-f.c.queue)[0]
	f.c.mu.Lock()
	old := f.c.rawHistory
	oldWrite := f.c.takeRawHistoryWrite(oldEvent)
	f.c.mu.Unlock()
	f.c.expireRawHistory(old)
	if got := <-live; got.Type != "command.error" || got.ClientID != "raw-client" || got.Message != "IRC did not respond to the history command." {
		t.Fatalf("missing raw timeout: %+v", got)
	}
	if err := f.m.RawCommand("alice", "net", "new-client", line); err != nil {
		t.Fatalf("gate remained locked after timeout: %v", err)
	}
	newEvent := (<-f.c.queue)[0]
	f.c.expireRawHistory(old)
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	f.c.mu.Lock()
	newWrite := f.c.takeRawHistoryWrite(newEvent)
	f.c.client = &girc.Client{}
	f.c.transport = conn
	oldAllowed := f.c.startRawHistoryWrite(oldWrite, f.c.client)
	newAllowed := f.c.startRawHistoryWrite(newWrite, f.c.client)
	f.c.mu.Unlock()
	if oldAllowed || !newAllowed {
		t.Fatalf("queued request fencing: old=%v new=%v", oldAllowed, newAllowed)
	}
	f.c.receiveCommandFailure(girc.Event{Command: "FAIL", Params: []string{"CHATHISTORY", "INVALID_PARAMS", "new failure"}})
	if got := <-live; got.Type != "command.error" || got.ClientID != "new-client" || got.Message != "new failure" {
		t.Fatalf("old expiry affected new request: %+v", got)
	}
}

func TestHistoryFailureRawWrittenTimeoutHoldsGateUntilTeardown(t *testing.T) {
	f := historyFixture(t)
	delete(f.c.caps, "labeled-response")
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	_, live, _, stop := f.m.Subscribe("alice", 0)
	defer stop()
	if err := f.m.RawCommand("alice", "net", "raw-client", "CHATHISTORY AFTER #test msgid=anchor 100"); err != nil {
		t.Fatal(err)
	}
	event := (<-f.c.queue)[0]
	f.c.mu.Lock()
	req := f.c.takeRawHistoryWrite(event)
	f.c.client = &girc.Client{}
	f.c.transport = conn
	if !f.c.startRawHistoryWrite(req, f.c.client) {
		t.Fatal("raw write was not accepted")
	}
	f.c.mu.Unlock()
	f.c.expireRawHistory(req)
	if got := <-live; got.Type != "command.error" || got.ClientID != "raw-client" {
		t.Fatalf("missing timeout: %+v", got)
	}
	if len(f.c.historyGate) != 1 {
		t.Fatal("written request released the gate before teardown")
	}
	if _, err := peer.Write([]byte("late reply")); err == nil {
		t.Fatal("timed-out transport remained usable")
	}
	f.c.receiveCommandFailure(girc.Event{Command: "FAIL", Params: []string{"CHATHISTORY", "INVALID_PARAMS", "late failure"}})
	if len(f.c.historyGate) != 1 {
		t.Fatal("late failure released gate before teardown")
	}
	// Teardown must wait for a callback that entered before the disconnect.
	f.c.historyCallbackMu.RLock()
	teardownDone := make(chan struct{})
	go func() {
		f.c.historyCallbackMu.Lock()
		f.c.mu.Lock()
		f.c.ready = false
		f.c.mu.Unlock()
		f.c.failHistory()
		f.c.historyCallbackMu.Unlock()
		close(teardownDone)
	}()
	select {
	case <-teardownDone:
		t.Fatal("teardown passed an in-flight history callback")
	default:
	}
	f.c.historyCallbackMu.RUnlock()
	select {
	case <-teardownDone:
	case <-time.After(time.Second):
		t.Fatal("history teardown remained blocked")
	}
	if len(f.c.historyGate) != 0 {
		t.Fatal("transport teardown did not release gate")
	}
}

func TestHistoryFailureRawDisconnectDrainsWriteTracking(t *testing.T) {
	f := historyFixture(t)
	delete(f.c.caps, "labeled-response")
	for i := 0; i < 4; i++ {
		if err := f.m.RawCommand("alice", "net", "raw-client", "CHATHISTORY AFTER #test msgid=anchor 100"); err != nil {
			t.Fatal(err)
		}
		f.c.mu.Lock()
		f.c.ready = false
		f.c.mu.Unlock()
		f.c.failHistory()
		f.c.drainQueue()
		f.c.mu.Lock()
		tracked := len(f.c.rawHistoryWrites)
		f.c.ready = true
		f.c.mu.Unlock()
		if tracked != 0 {
			t.Fatalf("raw write tracking retained after disconnect: %d", tracked)
		}
	}
}

func TestHistoryFailureRawWriterCancellationReleasesTracking(t *testing.T) {
	f := historyFixture(t)
	delete(f.c.caps, "labeled-response")
	ctx, cancel := context.WithCancel(f.m.ctx)
	f.c.ctx = ctx
	if err := f.m.RawCommand("alice", "net", "raw-client", "CHATHISTORY AFTER #test msgid=anchor 100"); err != nil {
		cancel()
		t.Fatal(err)
	}
	done := make(chan struct{})
	f.m.wg.Add(1)
	go func() { f.c.writer(); close(done) }()
	eventually(t, func() bool {
		f.c.mu.Lock()
		defer f.c.mu.Unlock()
		return len(f.c.rawHistoryWrites) == 0
	})
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("writer did not stop")
	}
	f.c.failHistory()
	f.c.mu.Lock()
	tracked := len(f.c.rawHistoryWrites)
	f.c.mu.Unlock()
	if tracked != 0 {
		t.Fatalf("writer cancellation retained %d queued requests", tracked)
	}
}

func TestHistoryFailureRawWrittenTimeoutReconnect(t *testing.T) {
	port, peers := capabilityServer(t, "message-tags batch server-time draft/chathistory", "")
	m := New(Options{AllowPrivate: true, ReconnectMin: 10 * time.Millisecond})
	t.Cleanup(m.Close)
	if _, err := m.Connect("alice", NetworkConfig{ID: "net", Name: "test", Host: "127.0.0.1", Port: port, Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	first := <-peers
	c, err := m.connection("alice", "net")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RawCommand("alice", "net", "raw-client", "CHATHISTORY AFTER #test msgid=anchor 100"); err != nil {
		t.Fatal(err)
	}
	var old *rawHistoryRequest
	var oldClient *girc.Client
	eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		old = c.rawHistory
		if old == nil || old.client == nil {
			return false
		}
		oldClient = old.client
		return true
	})
	c.expireRawHistory(old)
	var second *capabilityPeer
	select {
	case second = <-peers:
	case <-ctx.Done():
		t.Fatal("timed-out written request did not reconnect")
	}
	if err := m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	newClient := c.client
	c.mu.Unlock()
	if newClient == oldClient {
		t.Fatal("replacement reused the old IRC client")
	}
	c.expireRawHistory(old)
	finished := make(chan error, 1)
	go func() { _, err := c.syncHistory(ctx, "#test", "AFTER", "msgid=anchor", 100); finished <- err }()
	eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.unlabeledHistory != nil
	})
	// A callback already queued by the old client cannot reach the new request.
	c.receiveFromAttempt(oldClient, girc.Event{Command: "FAIL", Params: []string{"CHATHISTORY", "INVALID_PARAMS", "late raw failure"}})
	select {
	case err := <-finished:
		t.Fatalf("late old-transport failure completed replacement: %v", err)
	default:
	}
	first.send(":test FAIL CHATHISTORY INVALID_PARAMS :late raw failure")
	second.send(":test BATCH +history chathistory #test")
	second.send(":test BATCH -history")
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("replacement history request: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("replacement did not survive stale timer or reply")
	}
}

func TestHistoryBatchBeforeReadyRemainsHistorical(t *testing.T) {
	f := historyFixture(t)
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	client := girc.New(girc.Config{Server: "test", Nick: "alice", User: "alice", Name: "alice"})
	f.c.mu.Lock()
	f.c.client = client
	f.c.transport = conn
	f.c.ready = false
	f.c.mu.Unlock()
	f.c.receiveFromAttempt(client, girc.Event{Command: "BATCH", Params: []string{"+early", "chathistory", "#test"}})
	f.c.mu.Lock()
	opened := f.c.historyBatches["early"]
	f.c.mu.Unlock()
	if !opened {
		t.Fatal("current transport's history batch was dropped before ready")
	}
	historical := historyMessage(f.at, "early-history")
	historical.Params[1] = "hello @alice"
	historical.Tags["batch"] = "early"
	f.c.receiveFromAttempt(client, historical)
	f.c.receiveFromAttempt(client, girc.Event{Command: "BATCH", Params: []string{"-early"}})
	f.c.mu.Lock()
	stillOpen := f.c.historyBatches["early"]
	f.c.mu.Unlock()
	if stillOpen {
		t.Fatal("history batch did not close before ready")
	}
	conv := f.c.conversationID("#test")
	page, err := f.store.History("alice", conv, "", 40)
	if err != nil || len(page.Items) != 1 || page.Items[0].UpstreamID != "early-history" {
		t.Fatalf("historical message not stored: %+v, %v", page.Items, err)
	}
	if got := f.inbox(t, "alice", "all"); len(got.Items) != 0 {
		t.Fatalf("historical mention was treated as live: %+v", got.Items)
	}
	live := historyMessage(f.at.Add(time.Second), "live")
	live.Params[1] = "hello @alice"
	f.c.receiveFromAttempt(client, live)
	if got := f.inbox(t, "alice", "all"); len(got.Items) != 1 || got.Items[0].MessageID != "net:live" {
		t.Fatalf("message after batch was not live: %+v", got.Items)
	}
}

func TestHistoryFailureDuplicateRemainsOwned(t *testing.T) {
	f := historyFixture(t)
	_, live, _, stop := f.m.Subscribe("alice", 0)
	defer stop()
	req := &historyRequest{label: "tracked", done: make(chan historyResult, 1)}
	req.done <- historyResult{Err: fmt.Errorf("first result")}
	f.c.mu.Lock()
	f.c.historyRequests[req.label] = req
	f.c.mu.Unlock()
	f.c.receiveCommandFailure(girc.Event{Command: "FAIL", Tags: girc.Tags{"label": req.label}, Params: []string{"CHATHISTORY", "INVALID_PARAMS", "duplicate failure"}})
	select {
	case got := <-live:
		t.Fatalf("duplicate tracked failure leaked: %+v", got)
	default:
	}
	if got := <-req.done; got.Err == nil || got.Err.Error() != "first result" {
		t.Fatalf("duplicate replaced first result: %+v", got)
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
	parsed, err := storage.ParseHistoryCursor(*result.page.NextCursor, "alice", conv)
	if err != nil || parsed.UpstreamBefore == nil || parsed.UpstreamBefore.ID != upstream {
		t.Fatalf("scoped retry lost upstream continuation: %+v %v", parsed, err)
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

func TestHistoryRejectsWrongScopeBeforeUpstream(t *testing.T) {
	f := historyFixture(t)
	f.say(t, "alice", "#test", "anchor", "fixture")
	conv := f.c.conversationID("#test")
	stored, err := f.store.Oldest("alice", conv)
	if err != nil {
		t.Fatal(err)
	}
	wrong, err := storage.ScopedHistoryCursor("alice", "other", stored, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, before := range []string{wrong, "123", strings.Repeat("x", storage.MaxHistoryCursorLength+1), storage.SearchCursor(stored)} {
		if _, err = f.m.HistoryContext(context.Background(), "alice", conv, before, 40); !errors.Is(err, storage.ErrInvalidCursor) {
			t.Fatalf("invalid cursor: %v", err)
		}
		if len(f.c.queue) != 0 {
			t.Fatal("invalid cursor triggered an upstream request")
		}
	}
}

func TestHistoryFullLocalPagePreservesUpstreamContinuation(t *testing.T) {
	f := historyFixture(t)
	f.say(t, "alice", "#test", "anchor-a", "one")
	f.say(t, "alice", "#test", "anchor-b", "two")
	f.say(t, "alice", "#test", "anchor-c", "three")
	conv := f.c.conversationID("#test")
	page, err := f.store.History("alice", conv, "", 3)
	if err != nil || len(page.Items) != 3 {
		t.Fatal(page, err)
	}
	point := &storage.HistoryUpstream{ID: "prior-reaction", At: f.at.Add(-time.Hour)}
	before, err := storage.ScopedHistoryCursor("alice", conv, page.Items[2], point)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.m.HistoryContext(context.Background(), "alice", conv, before, 1)
	if err != nil || got.NextCursor == nil || len(got.Items) != 1 {
		t.Fatalf("page=%+v err=%v", got, err)
	}
	parsed, err := storage.ParseHistoryCursor(*got.NextCursor, "alice", conv)
	if err != nil || parsed.UpstreamBefore == nil || parsed.UpstreamBefore.ID != point.ID || !parsed.UpstreamBefore.At.Equal(point.At) {
		t.Fatalf("lost upstream point: %+v %v", parsed, err)
	}
	if len(f.c.queue) != 0 {
		t.Fatal("full retained page requested upstream")
	}
}

func TestHistoryRetryAfterQueueFailurePreservesBothBoundaries(t *testing.T) {
	f := historyFixture(t)
	f.say(t, "alice", "#test", "anchor-a", "one")
	f.say(t, "alice", "#test", "anchor-b", "two")
	conv := f.c.conversationID("#test")
	page, err := f.store.History("alice", conv, "", 2)
	if err != nil || len(page.Items) != 2 {
		t.Fatal(page, err)
	}
	point := &storage.HistoryUpstream{ID: "prior-reaction", At: f.at.Add(-time.Hour)}
	before, err := storage.ScopedHistoryCursor("alice", conv, page.Items[1], point)
	if err != nil {
		t.Fatal(err)
	}
	for len(f.c.queue) < cap(f.c.queue) {
		f.c.queue <- []*girc.Event{}
	}
	got, err := f.m.HistoryContext(context.Background(), "alice", conv, before, 40)
	if err != nil || got.NextCursor == nil || got.SyncError == "" || len(got.Items) != 1 {
		t.Fatalf("page=%+v err=%v", got, err)
	}
	parsed, err := storage.ParseHistoryCursor(*got.NextCursor, "alice", conv)
	if err != nil || parsed.ID != got.Items[0].ID || parsed.UpstreamBefore == nil || parsed.UpstreamBefore.ID != point.ID || !parsed.UpstreamBefore.At.Equal(point.At) {
		t.Fatalf("retry lost a boundary: %+v %v", parsed, err)
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
