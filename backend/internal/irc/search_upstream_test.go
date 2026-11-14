package irc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
)

type searchRead struct {
	page SearchPage
	err  error
}

func startSearchFixture(t *testing.T) (*inboxFixture, string) {
	t.Helper()
	f := historyFixture(t)
	f.say(t, "alice", "#test", "retained", "hello retained")
	f.c.mu.Lock()
	for _, cap := range []string{"soju.im/search", "batch", "server-time", "message-tags", "labeled-response"} {
		f.c.caps[cap] = true
	}
	f.c.mu.Unlock()
	return f, f.c.conversationID("#test")
}

func searchDispatch(t *testing.T, c *connection) *girc.Event {
	t.Helper()
	select {
	case events := <-c.queue:
		if len(events) != 1 || events[0].Command != "SEARCH" {
			t.Fatalf("unexpected request: %+v", events)
		}
		c.mu.Lock()
		req := c.searchWrites[events[0]]
		delete(c.searchWrites, events[0])
		started := req != nil && c.startSearchWrite(req, c.client)
		c.mu.Unlock()
		if !started {
			t.Fatal("search did not dispatch")
		}
		return events[0]
	case <-time.After(2 * time.Second):
		t.Fatal("search not queued")
	}
	return nil
}

func searchEvent(c *connection, event girc.Event) { c.receive(c.client, event) }

func TestUpstreamSearchBatchIsolationAndContext(t *testing.T) {
	f, conversation := startSearchFixture(t)
	before, err := f.store.History("alice", conversation, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	watermark, err := f.store.Watermark("alice", conversation)
	if err != nil {
		t.Fatal(err)
	}
	f.c.owner.mu.Lock()
	sequence, unread := f.c.owner.seq, f.c.owner.conversations[conversation].UnreadCount
	f.c.owner.mu.Unlock()
	inboxBefore := len(f.inbox(t, "alice", "all").Items)
	results := make(chan searchRead, 1)
	go func() {
		p, e := f.m.SearchMessagesContext(context.Background(), "alice", SearchQuery{Query: "hello", Conversations: []string{conversation}, Source: "upstream"})
		results <- searchRead{p, e}
	}()
	request := searchDispatch(t, f.c)
	if !strings.Contains(request.Params[0], "in=#test;text=hello;limit=50") {
		t.Fatal(request.Params)
	}
	searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+outer", "soju.im/search"}, Tags: girc.Tags{"label": request.Tags["label"]}})
	searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+nested", "example/nested"}, Tags: girc.Tags{"batch": "outer"}})
	for _, item := range []girc.Event{
		{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", "hello stale"}, Tags: girc.Tags{"batch": "nested", "time": f.at.Format(time.RFC3339Nano), "msgid": "retained"}},
		{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", "hello remote"}, Tags: girc.Tags{"batch": "nested", "time": f.at.Format(time.RFC3339Nano)}},
		{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", "hello second"}, Tags: girc.Tags{"batch": "nested", "time": f.at.Add(time.Second).Format(time.RFC3339Nano)}},
	} {
		searchEvent(f.c, item)
	}
	searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-nested"}})
	searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-outer"}})
	result := <-results
	if result.err != nil || result.page.Source.Actual != "upstream" || result.page.NextCursor != nil || len(result.page.Items) != 3 {
		t.Fatalf("result %+v %v", result.page, result.err)
	}
	retained := 0
	ephemeral := map[string]bool{}
	for i, hit := range result.page.Items {
		if i > 0 && hit.Message.CreatedAt.After(result.page.Items[i-1].Message.CreatedAt) {
			t.Fatal("hits are not sorted newest first")
		}
		if i > 0 && hit.Message.CreatedAt.Equal(result.page.Items[i-1].Message.CreatedAt) && hit.Message.ID > result.page.Items[i-1].Message.ID {
			t.Fatal("equal-time hits are not sorted by descending ID")
		}
		if hit.Context.Available {
			if hit.Context.Identity != "retained" || hit.Message.Text != "hello retained" || hit.SenderNick != "" {
				t.Fatal(hit)
			}
			retained++
		} else {
			if hit.Context.Identity != "ephemeral" || hit.Context.Reason != "missing-msgid" || hit.SenderNick != "bob" {
				t.Fatal(hit)
			}
			ephemeral[hit.Message.ID] = true
		}
	}
	if retained != 1 || len(ephemeral) != 2 {
		t.Fatal("retained or missing-ID hits lost", result.page.Items)
	}
	after, err := f.store.History("alice", conversation, "", 100)
	if err != nil || len(after.Items) != len(before.Items) {
		t.Fatalf("search changed history: %+v %v", after, err)
	}
	// A foreign or late batch is also discarded, even after the request ends.
	searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+late", "soju.im/search"}, Tags: girc.Tags{"label": "foreign"}})
	searchEvent(f.c, girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", "hello late"}, Tags: girc.Tags{"batch": "late", "time": f.at.Format(time.RFC3339Nano)}})
	searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-late"}})
	after, err = f.store.History("alice", conversation, "", 100)
	if err != nil || len(after.Items) != len(before.Items) {
		t.Fatal("late search mutated history", err)
	}
	now, err := f.store.Watermark("alice", conversation)
	if err != nil || now != watermark {
		t.Fatal("search changed watermark", now, err)
	}
	f.c.owner.mu.Lock()
	if f.c.owner.seq != sequence || f.c.owner.conversations[conversation].UnreadCount != unread {
		t.Error("search changed replay or unread")
	}
	f.c.owner.mu.Unlock()
	if len(f.inbox(t, "alice", "all").Items) != inboxBefore {
		t.Error("search created inbox activity")
	}
}

func TestSearchRetainedEditsAndTombstonesWin(t *testing.T) {
	f, conversation := startSearchFixture(t)
	w := f.c.owner
	w.mu.Lock()
	conv := w.conversations[conversation]
	msg := w.messages[conversation][0]
	msg.Text = "hello edited"
	w.messages[conversation][0] = msg
	w.mu.Unlock()
	event := girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", "hello stale"}, Tags: girc.Tags{"time": f.at.Format(time.RFC3339Nano), "msgid": "retained"}}
	query := SearchQuery{Query: "hello", Conversations: []string{conversation}, Source: "upstream"}
	attempt := searchAttempt{client: f.c.client, transport: f.c.transport}
	page, err := f.m.searchUpstreamResults(context.Background(), "alice", query, f.c, conv, attempt, []girc.Event{event})
	if err != nil || len(page.Items) != 1 || page.Items[0].Message.Text != "hello edited" {
		t.Fatal(page, err)
	}
	w.mu.Lock()
	msg.Deleted = true
	w.messages[conversation][0] = msg
	w.mu.Unlock()
	page, err = f.m.searchUpstreamResults(context.Background(), "alice", query, f.c, conv, attempt, []girc.Event{event})
	if err != nil || len(page.Items) != 0 {
		t.Fatal("tombstone lost", page, err)
	}
}

func TestSearchProviderCandidateSemanticsAndDateFallback(t *testing.T) {
	f, conversation := startSearchFixture(t)
	f.c.owner.mu.Lock()
	conv := f.c.owner.conversations[conversation]
	f.c.owner.mu.Unlock()
	attempt := searchAttempt{client: f.c.client, transport: f.c.transport}
	event := func(text string) girc.Event {
		return girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", text}, Tags: girc.Tags{"time": f.at.Format(time.RFC3339Nano)}}
	}
	// Simulated whole-token and substring stores can offer different candidates.
	for name, candidates := range map[string][]girc.Event{
		"whole-token": {event("world")},
		"substring":   {event("world"), event("sword")},
	} {
		page, err := f.m.searchUpstreamResults(context.Background(), "alice", SearchQuery{Query: "wor"}, f.c, conv, attempt, candidates)
		if err != nil || page.Source.Completeness != "unknown" || len(page.Items) != 1 || page.Items[0].Message.Text != "world" {
			t.Fatalf("%s: %+v %v", name, page, err)
		}
	}
	for _, query := range []SearchQuery{
		{Query: "hello", Conversations: []string{conversation}, Source: "upstream", After: f.at},
		{Query: "hello", Conversations: []string{conversation}, Source: "upstream", Before: f.at},
	} {
		page, err := f.m.SearchMessages("alice", query)
		if err != nil || page.Source.Actual != "retained" || page.Source.Reason != "unsupported-filters" {
			t.Fatal(page, err)
		}
		select {
		case event := <-f.c.queue:
			t.Fatal("date query reached upstream", event)
		default:
		}
	}
}

func TestSearchBatchCapAndEmpty(t *testing.T) {
	for _, excess := range []bool{false, true} {
		t.Run(fmt.Sprint(excess), func(t *testing.T) {
			f, conversation := startSearchFixture(t)
			finished := make(chan searchRead, 1)
			go func() {
				p, e := f.m.SearchMessagesContext(context.Background(), "alice", SearchQuery{Query: "hello", Conversations: []string{conversation}, Source: "upstream"})
				finished <- searchRead{p, e}
			}()
			request := searchDispatch(t, f.c)
			searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+b", "soju.im/search"}, Tags: girc.Tags{"label": request.Tags["label"]}})
			if excess {
				for i := 0; i < 101; i++ {
					searchEvent(f.c, girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", "hello"}, Tags: girc.Tags{"batch": "b", "time": f.at.Format(time.RFC3339Nano)}})
				}
			}
			searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-b"}})
			result := <-finished
			if result.err != nil {
				t.Fatal(result.err)
			}
			if excess && (result.page.Source.Actual != "retained" || result.page.Source.Reason != "upstream-unavailable") {
				t.Fatal(result.page)
			}
			if !excess && (result.page.Source.Actual != "upstream" || len(result.page.Items) != 0) {
				t.Fatal(result.page)
			}
		})
	}
}

func TestSearchCancellationAndManagedRawCommand(t *testing.T) {
	f, conversation := startSearchFixture(t)
	if _, err := rawEvent("SEARCH in=#test;text=hello"); err == nil {
		t.Fatal("raw SEARCH accepted")
	}
	if _, err := rawEvent("WHO #test"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan error, 1)
	go func() {
		_, err := f.m.SearchMessagesContext(ctx, "alice", SearchQuery{Query: "hello", Conversations: []string{conversation}, Source: "upstream"})
		results <- err
	}()
	request := searchDispatch(t, f.c)
	cancel()
	if err := <-results; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+late", "soju.im/search"}, Tags: girc.Tags{"label": request.Tags["label"]}})
	searchEvent(f.c, girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", "hello late"}, Tags: girc.Tags{"batch": "late", "time": f.at.Format(time.RFC3339Nano)}})
	searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-late"}})
	page, err := f.store.History("alice", conversation, "", 100)
	if err != nil || len(page.Items) != 1 {
		t.Fatal(page, err)
	}
}

func TestSearchCancellationBeforeWriterDispatch(t *testing.T) {
	f, conversation := startSearchFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		_, err := f.m.SearchMessagesContext(ctx, "alice", SearchQuery{Query: "hello", Conversations: []string{conversation}, Source: "upstream"})
		finished <- err
	}()
	var event *girc.Event
	select {
	case events := <-f.c.queue:
		event = events[0]
	case <-time.After(2 * time.Second):
		t.Fatal("search not queued")
	}
	cancel()
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	f.c.mu.Lock()
	req := f.c.searchWrites[event]
	delete(f.c.searchWrites, event)
	started := f.c.startSearchWrite(req, f.c.client)
	f.c.mu.Unlock()
	if req == nil || started {
		t.Fatal("canceled queued search was dispatched")
	}
}

func TestSearchCapabilityOfferAckNakAndDel(t *testing.T) {
	f, _ := startSearchFixture(t)
	f.c.mu.Lock()
	delete(f.c.caps, "soju.im/search")
	f.c.mu.Unlock()
	capEvent := func(kind string) {
		f.c.capabilityEvent(f.c.client, girc.Event{Command: "CAP", Params: []string{"*", kind, "soju.im/search"}})
	}
	usable := func() bool { f.c.mu.Lock(); defer f.c.mu.Unlock(); return f.c.searchUsableLocked() }
	capEvent("LS")
	if usable() {
		t.Fatal("offer enabled search")
	}
	capEvent("NAK")
	if usable() {
		t.Fatal("NAK enabled search")
	}
	capEvent("ACK")
	if !usable() {
		t.Fatal("ACK did not enable search")
	}
	capEvent("DEL")
	if usable() {
		t.Fatal("DEL left search enabled")
	}
}

func TestSearchDispatchAndResponseTimeouts(t *testing.T) {
	for _, mode := range []string{"dispatch", "response"} {
		t.Run(mode, func(t *testing.T) {
			f, conversation := startSearchFixture(t)
			finished := make(chan error, 1)
			go func() {
				_, _, err := f.c.upstreamSearchWithBudgets(context.Background(), "#test", "hello", 5, 50*time.Millisecond, 50*time.Millisecond)
				finished <- err
			}()
			var event *girc.Event
			if mode == "response" {
				event = searchDispatch(t, f.c)
			} else {
				select {
				case events := <-f.c.queue:
					event = events[0]
				case <-time.After(time.Second):
					t.Fatal("not queued")
				}
			}
			if err := <-finished; !errors.Is(err, errSearchUnavailable) {
				t.Fatal(err)
			}
			if mode == "dispatch" {
				f.c.mu.Lock()
				req := f.c.searchWrites[event]
				delete(f.c.searchWrites, event)
				started := f.c.startSearchWrite(req, f.c.client)
				f.c.mu.Unlock()
				if req == nil || started {
					t.Fatal("timed out search dispatched")
				}
			} else {
				searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+late", "soju.im/search"}, Tags: girc.Tags{"label": event.Tags["label"]}})
				searchEvent(f.c, girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", "hello late"}, Tags: girc.Tags{"batch": "late", "time": f.at.Format(time.RFC3339Nano)}})
				searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-late"}})
				page, err := f.store.History("alice", conversation, "", 100)
				if err != nil || len(page.Items) != 1 {
					t.Fatal("late timeout result entered history", page, err)
				}
			}
		})
	}
}

func TestSearchReconnectFencesOldAttempt(t *testing.T) {
	f, conversation := startSearchFixture(t)
	attempt := make(chan struct{})
	f.c.mu.Lock()
	oldClient := f.c.client
	f.c.attemptDone = attempt
	f.c.mu.Unlock()
	finished := make(chan searchRead, 1)
	go func() {
		page, err := f.m.SearchMessagesContext(context.Background(), "alice", SearchQuery{Query: "hello", Conversations: []string{conversation}, Source: "upstream"})
		finished <- searchRead{page, err}
	}()
	_ = searchDispatch(t, f.c)
	f.c.mu.Lock()
	f.c.client = girc.New(girc.Config{Server: "replacement", Nick: "alice", User: "alice", Name: "alice"})
	f.c.caps = map[string]bool{}
	close(attempt)
	f.c.mu.Unlock()
	result := <-finished
	if result.err != nil || result.page.Source.Actual != "retained" || result.page.Source.Reason != "upstream-unavailable" {
		t.Fatal(result.page, result.err)
	}
	f.c.receiveFromAttempt(oldClient, girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", "hello old attempt"}, Tags: girc.Tags{"time": f.at.Format(time.RFC3339Nano)}})
	page, err := f.store.History("alice", conversation, "", 100)
	if err != nil || len(page.Items) != 1 {
		t.Fatal("old connection changed history", page, err)
	}
}

func TestSearchReconnectAfterReceiptBeforeMapping(t *testing.T) {
	f, conversation := startSearchFixture(t)
	type received struct {
		events  []girc.Event
		attempt searchAttempt
		err     error
	}
	finished := make(chan received, 1)
	go func() {
		events, attempt, err := f.c.upstreamSearch(context.Background(), "#test", "hello", 5)
		finished <- received{events, attempt, err}
	}()
	request := searchDispatch(t, f.c)
	searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+b", "soju.im/search"}, Tags: girc.Tags{"label": request.Tags["label"]}})
	searchEvent(f.c, girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", "hello remote"}, Tags: girc.Tags{"batch": "b", "time": f.at.Format(time.RFC3339Nano)}})
	searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-b"}})
	got := <-finished
	if got.err != nil || len(got.events) != 1 {
		t.Fatal(got.err)
	}
	w := f.c.owner
	w.mu.Lock()
	conv := w.conversations[conversation]
	w.mu.Unlock()
	f.c.mu.Lock()
	f.c.client = girc.New(girc.Config{Server: "replacement", Nick: "alice", User: "alice", Name: "alice"})
	f.c.mu.Unlock()
	_, err := f.m.searchUpstreamResults(context.Background(), "alice", SearchQuery{Query: "hello"}, f.c, conv, got.attempt, got.events)
	if !errors.Is(err, errSearchUnavailable) {
		t.Fatal("replaced attempt accepted", err)
	}
}

func TestSearchTagEscaping(t *testing.T) {
	if got := searchTagValue("a;b c\\d\r\n"); got != `a\:b\sc\\d\r\n` {
		t.Fatalf("encoded %q", got)
	}
}

func TestSearchCapabilityLossAndRevocation(t *testing.T) {
	for _, mode := range []string{"cap-del", "fail", "batch-fail", "revoked", "account-revoked", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			f, conversation := startSearchFixture(t)
			finished := make(chan searchRead, 1)
			go func() {
				page, err := f.m.SearchMessagesContext(context.Background(), "alice", SearchQuery{Query: "hello", Conversations: []string{conversation}, Source: "upstream"})
				finished <- searchRead{page, err}
			}()
			request := searchDispatch(t, f.c)
			switch mode {
			case "cap-del":
				f.c.capabilityEvent(f.c.client, girc.Event{Command: "CAP", Params: []string{"*", "DEL", "soju.im/search"}})
			case "fail":
				searchEvent(f.c, girc.Event{Command: "FAIL", Params: []string{"SEARCH", "INTERNAL_ERROR", "private server detail"}, Tags: girc.Tags{"label": request.Tags["label"]}})
			case "batch-fail":
				searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+wrapper", "labeled-response"}, Tags: girc.Tags{"label": request.Tags["label"]}})
				searchEvent(f.c, girc.Event{Command: "FAIL", Params: []string{"SEARCH", "INTERNAL_ERROR", "private server detail"}, Tags: girc.Tags{"batch": "wrapper"}})
				searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-wrapper"}})
			case "revoked", "account-revoked", "malformed":
				if mode == "revoked" {
					f.c.owner.mu.Lock()
					delete(f.c.owner.conversations, conversation)
					f.c.owner.mu.Unlock()
				}
				if mode == "account-revoked" {
					f.c.owner.mu.Lock()
					f.c.owner.deleted = true
					f.c.owner.mu.Unlock()
				}
				searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+b", "soju.im/search"}, Tags: girc.Tags{"label": request.Tags["label"]}})
				if mode == "malformed" {
					searchEvent(f.c, girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#foreign", "hello"}, Tags: girc.Tags{"batch": "b", "time": f.at.Format(time.RFC3339Nano)}})
				}
				searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-b"}})
			}
			select {
			case result := <-finished:
				if mode == "revoked" || mode == "account-revoked" {
					if !errors.Is(result.err, ErrNotFound) {
						t.Fatalf("revocation: %+v %v", result.page, result.err)
					}
				} else if result.err != nil || result.page.Source.Actual != "retained" || result.page.Source.Reason != "upstream-unavailable" {
					t.Fatalf("fallback: %+v %v", result.page, result.err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("search did not settle")
			}
		})
	}
}

func TestSearchAdmissionAndBatchBudgets(t *testing.T) {
	for _, mode := range []string{"admission", "events", "bytes", "nesting"} {
		t.Run(mode, func(t *testing.T) {
			f, conversation := startSearchFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan searchRead, 1)
			query := SearchQuery{Query: "hello", Conversations: []string{conversation}, Source: "upstream", Limit: 100}
			go func() { p, e := f.m.SearchMessagesContext(ctx, "alice", query); finished <- searchRead{p, e} }()
			request := searchDispatch(t, f.c)
			if mode == "admission" {
				second, err := f.m.SearchMessagesContext(context.Background(), "alice", query)
				if err != nil || second.Source.Actual != "retained" || second.Source.Reason != "upstream-unavailable" {
					t.Fatal(second, err)
				}
				select {
				case event := <-f.c.queue:
					t.Fatal("second search queued", event)
				default:
				}
			}
			searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+root", "soju.im/search"}, Tags: girc.Tags{"label": request.Tags["label"]}})
			switch mode {
			case "events":
				for i := 0; i < 260; i++ {
					ref := fmt.Sprintf("n%d", i)
					searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+" + ref, "test"}, Tags: girc.Tags{"batch": "root"}})
					searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-" + ref}})
				}
			case "bytes":
				for i := 0; i < 100; i++ {
					searchEvent(f.c, girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "bob"}, Params: []string{"#test", strings.Repeat("x", 3000)}, Tags: girc.Tags{"batch": "root", "time": f.at.Format(time.RFC3339Nano)}})
				}
			case "nesting":
				parent := "root"
				for i := 0; i < 8; i++ {
					ref := fmt.Sprintf("n%d", i)
					searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"+" + ref, "test"}, Tags: girc.Tags{"batch": parent}})
					parent = ref
				}
				f.c.mu.Lock()
				quarantined := f.c.searchQuarantined
				f.c.mu.Unlock()
				if !quarantined {
					t.Fatal("excess nesting did not quarantine")
				}
				cancel()
				if result := <-finished; !errors.Is(result.err, context.Canceled) {
					t.Fatal(result.err)
				}
				return
			}
			searchEvent(f.c, girc.Event{Command: "BATCH", Params: []string{"-root"}})
			result := <-finished
			if result.err != nil {
				t.Fatal(result.err)
			}
			if mode != "admission" && (result.page.Source.Actual != "retained" || result.page.Source.Reason != "upstream-unavailable") {
				t.Fatal(result.page)
			}
		})
	}
}

// Exercise actual capability negotiation, IRC serialization, and response delivery.
func TestSearchLoopbackWire(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	wire := make(chan string, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		scan := bufio.NewScanner(conn)
		for scan.Scan() {
			e := girc.ParseEvent(scan.Text())
			if e == nil {
				continue
			}
			switch e.Command {
			case "CAP":
				switch e.Params[0] {
				case "LS":
					fmt.Fprint(conn, ":test CAP * LS :soju.im/search batch server-time message-tags labeled-response\r\n")
				case "REQ":
					fmt.Fprintf(conn, ":test CAP * ACK :%s\r\n", e.Last())
				case "END":
					fmt.Fprint(conn, ":test 001 alice :Welcome\r\n:test 422 alice :No MOTD\r\n")
				}
			case "PING":
				fmt.Fprintf(conn, "PONG :%s\r\n", e.Last())
			case "SEARCH":
				wire <- scan.Text()
				fmt.Fprintf(conn, "@label=%s :test BATCH +wrapper labeled-response\r\n", e.Tags["label"])
				fmt.Fprint(conn, "@batch=wrapper :test BATCH +s soju.im/search\r\n@batch=s;time=2026-01-01T00:00:00.000Z :bob!u@fixture PRIVMSG #test :hello hi\r\n:test BATCH -s\r\n:test BATCH -wrapper\r\n")
			}
		}
	}()
	m := New(Options{AllowPrivate: true, ReconnectMin: time.Second})
	t.Cleanup(func() { m.Close(); <-serverDone })
	_, err = m.Connect("alice", NetworkConfig{ID: "net", Name: "net", Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Nick: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	w := m.space("alice")
	w.mu.Lock()
	c := w.connections["net"]
	conv := c.conversation("#test")
	w.mu.Unlock()
	page, err := m.SearchMessagesContext(ctx, "alice", SearchQuery{Query: "hello; hi", Conversations: []string{conv.ID}, Source: "upstream"})
	if err != nil || page.Source.Actual != "upstream" || len(page.Items) != 1 || page.Items[0].Context.Identity != "ephemeral" {
		t.Fatalf("loopback: %+v %v", page, err)
	}
	select {
	case line := <-wire:
		if !strings.Contains(line, `in=#test;text=hello\:\shi;limit=50`) {
			t.Fatalf("unexpected wire command %q", line)
		}
	case <-ctx.Done():
		t.Fatal("no wire search")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.messages[conv.ID]) != 0 {
		t.Fatal("upstream result entered timeline")
	}
	for _, line := range w.trace["net"] {
		if line.Command == "SEARCH" || strings.Contains(line.Text, "hello hi") {
			t.Fatal("search leaked to diagnostic trace")
		}
	}
}
