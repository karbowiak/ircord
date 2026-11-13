package irc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

type historyPoint struct {
	ID string
	At time.Time
}
type historyTarget struct {
	Name string
	At   time.Time
}
type historyResult struct {
	Targets        []historyTarget
	Oldest, Newest historyPoint
	Count          int
	End            bool
	Err            error
	IDs            map[string]bool // every msgid the response carried
}

const ircTimeLayout = "2006-01-02T15:04:05.000Z"

// Unlabelled raw requests share the history gate and cannot block it forever.
const rawHistoryTimeout = 15 * time.Second

type rawHistoryRequest struct {
	clientID  string
	target    string
	sub       string
	batch     string
	timer     *time.Timer
	client    *girc.Client
	transport net.Conn
	expired   bool
}

// The caller holds c.mu. The writer takes ownership as soon as it dequeues;
// a reconnect drain uses the same operation before discarding an event.
func (c *connection) takeRawHistoryWrite(event *girc.Event) *rawHistoryRequest {
	raw, tracked := c.rawHistoryWrites[event]
	if !tracked {
		return nil
	}
	delete(c.rawHistoryWrites, event)
	return raw
}

// The caller holds c.mu. Mark the attempt before releasing the lock for Send,
// so a concurrent timeout always closes the transport if a write may occur.
func (c *connection) startRawHistoryWrite(raw *rawHistoryRequest, client *girc.Client) bool {
	if raw == nil {
		return true
	}
	if c.rawHistory != raw || raw.expired || !c.ready || client == nil || c.transport == nil {
		return false
	}
	raw.client = client
	raw.transport = c.transport
	return true
}

func (c *connection) enqueueRawHistory(event *girc.Event, clientID string) error {
	select {
	case c.historyGate <- struct{}{}:
	default:
		return ErrBusy
	}
	req := &rawHistoryRequest{clientID: clientID}
	if len(event.Params) > 0 {
		req.sub = event.Params[0]
	}
	if len(event.Params) > 1 {
		req.target = event.Params[1]
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.ready {
		<-c.historyGate
		return ErrDisconnected
	}
	select {
	case <-c.ctx.Done():
		<-c.historyGate
		return ErrDisconnected
	case c.queue <- []*girc.Event{event}:
		c.rawHistory = req
		if c.rawHistoryWrites == nil {
			c.rawHistoryWrites = map[*girc.Event]*rawHistoryRequest{}
		}
		c.rawHistoryWrites[event] = req
		req.timer = time.AfterFunc(rawHistoryTimeout, func() {
			c.expireRawHistory(req)
		})
		return nil
	default:
		<-c.historyGate
		return ErrBusy
	}
}

// The caller holds c.mu. Only the active request owns a gate token.
func (c *connection) finishRawHistory(req *rawHistoryRequest) bool {
	if req == nil || c.rawHistory != req {
		return false
	}
	c.rawHistory = nil
	if req.timer != nil {
		req.timer.Stop()
	}
	<-c.historyGate
	return true
}

func (c *connection) expireRawHistory(req *rawHistoryRequest) {
	c.mu.Lock()
	if req == nil || c.rawHistory != req || req.expired {
		c.mu.Unlock()
		return
	}
	req.expired = true
	var transport net.Conn
	if req.client == nil {
		c.finishRawHistory(req)
	} else if c.client == req.client && c.transport == req.transport {
		transport = req.transport
	}
	c.mu.Unlock()
	if transport != nil {
		_ = transport.Close()
	}
	c.owner.mu.Lock()
	c.owner.sendCommandError(req.clientID, "IRC did not respond to the history command.")
	c.owner.mu.Unlock()
}

func (c *connection) rawHistoryFailure(e girc.Event) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	req := c.rawHistory
	if req == nil || e.Command != "FAIL" || len(e.Params) < 3 || !strings.EqualFold(e.Params[0], "CHATHISTORY") || e.Tags["label"] != "" || (e.Tags["batch"] != "" && e.Tags["batch"] != req.batch) {
		return "", false
	}
	if !req.expired {
		c.finishRawHistory(req)
	}
	return req.clientID, true
}

type historyRequest struct {
	sub          string
	conversation string
	orderBase    int64
	label        string
	target       string
	reference    string
	done         chan historyResult
	result       historyResult
	dispatch     chan struct{}
	abandoned    bool
	labeled      bool
	client       *girc.Client
	transport    net.Conn
	attemptDone  <-chan struct{}
}

// The writer calls both hooks under c.mu. The request remains associated with
// the dequeued event while the writer waits for its send tick.
func (c *connection) takeHistoryWrite(event *girc.Event) *historyRequest {
	req := c.automaticHistoryWrites[event]
	delete(c.automaticHistoryWrites, event)
	return req
}

func (c *connection) startHistoryWrite(req *historyRequest, client *girc.Client) bool {
	if req == nil {
		return true
	}
	if req.abandoned || c.historyRequests[req.label] != req || !c.ready || c.client != client || client == nil || c.transport == nil || c.attemptDone == nil {
		return false
	}
	req.client, req.transport, req.attemptDone = client, c.transport, c.attemptDone
	req.dispatch <- struct{}{}
	return true
}

const historyOperationTimeout = 15 * time.Second

func (c *connection) syncHistory(ctx context.Context, target, sub, reference string, limit int) (historyResult, error) {
	return c.syncHistoryWithBudgets(ctx, target, sub, reference, limit, historyOperationTimeout, historyOperationTimeout)
}

func (c *connection) syncHistoryWithBudgets(ctx context.Context, target, sub, reference string, limit int, writerBudget, responseBudget time.Duration) (historyResult, error) {
	if err := ctx.Err(); err != nil {
		return historyResult{}, err
	}
	if c.ctx.Err() != nil {
		return historyResult{}, ErrDisconnected
	}
	scheduler := c.historyScheduling()
	select {
	case scheduler.slots <- struct{}{}:
		defer func() { <-scheduler.slots }()
	default:
		if err := ctx.Err(); err != nil {
			return historyResult{}, err
		}
		if c.ctx.Err() != nil {
			return historyResult{}, ErrDisconnected
		}
		return historyResult{}, ErrBusy
	}
	// Bound gate wait by the number of admitted requests and the two
	// per-operation budgets. Caller deadlines can end this wait sooner.
	queueDeadline := time.NewTimer(time.Duration(historyWaiterLimit) * (writerBudget + responseBudget))
	defer queueDeadline.Stop()
	select {
	case c.historyGate <- struct{}{}:
		defer func() { <-c.historyGate }()
	case <-ctx.Done():
		return historyResult{}, ctx.Err()
	case <-c.ctx.Done():
		return historyResult{}, ErrDisconnected
	case <-c.done:
		return historyResult{}, ErrDisconnected
	case <-queueDeadline.C:
		return historyResult{}, context.DeadlineExceeded
	}
	// Readiness and the bounded writer queue have a separate budget from the
	// upstream response. Waiting for the gate consumes neither budget.
	deadline := time.NewTimer(writerBudget)
	defer deadline.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		c.mu.Lock()
		ready := c.ready
		c.mu.Unlock()
		if ready {
			break
		}
		select {
		case <-ctx.Done():
			return historyResult{}, ctx.Err()
		case <-c.ctx.Done():
			return historyResult{}, ErrDisconnected
		case <-c.done:
			return historyResult{}, ErrDisconnected
		case <-deadline.C:
			return historyResult{}, context.DeadlineExceeded
		case <-tick.C:
		}
	}
	if !c.hasAll("draft/chathistory", "batch", "server-time") {
		return historyResult{}, ErrUnsupported
	}
	req := &historyRequest{sub: sub, conversation: c.conversationID(target), label: id(), target: target, reference: reference, done: make(chan historyResult, 1), dispatch: make(chan struct{}, 1)}
	event := &girc.Event{Command: "CHATHISTORY", Params: []string{sub, target, reference, fmt.Sprint(limit)}}
	c.mu.Lock()
	if !c.ready || c.ctx.Err() != nil || (c.attemptDone != nil && c.transport == nil) {
		c.mu.Unlock()
		return historyResult{}, ErrDisconnected
	}
	if err := ctx.Err(); err != nil {
		c.mu.Unlock()
		return historyResult{}, err
	}
	req.labeled = c.caps["labeled-response"]
	if req.labeled {
		event.Tags = girc.Tags{"label": req.label}
	}
	c.historyRequests[req.label] = req
	c.unlabeledHistory = req
	if c.automaticHistoryWrites == nil {
		c.automaticHistoryWrites = map[*girc.Event]*historyRequest{}
	}
	select {
	case c.queue <- []*girc.Event{event}:
		c.automaticHistoryWrites[event] = req
	default:
		delete(c.historyRequests, req.label)
		c.unlabeledHistory = nil
		c.mu.Unlock()
		return historyResult{}, ErrBusy
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		req.abandoned = true
		delete(c.historyRequests, req.label)
		if c.unlabeledHistory == req {
			c.unlabeledHistory = nil
		}
		for ref, pending := range c.historyResponses {
			if pending == req {
				delete(c.historyResponses, ref)
			}
		}
		c.mu.Unlock()
	}()
	dispatched := false
	var result historyResult
	var err error
	select {
	case <-req.dispatch:
		dispatched = true
	case result = <-req.done:
		return result, result.Err
	case <-ctx.Done():
		err = ctx.Err()
	case <-c.ctx.Done():
		err = ErrDisconnected
	case <-c.done:
		err = ErrDisconnected
	case <-deadline.C:
		err = context.DeadlineExceeded
	}
	if dispatched {
		deadline.Stop()
		deadline.Reset(responseBudget)
		select {
		case result = <-req.done:
			return result, result.Err
		case <-ctx.Done():
			err = ctx.Err()
		case <-c.ctx.Done():
			err = ErrDisconnected
		case <-c.done:
			err = ErrDisconnected
		case <-deadline.C:
			err = context.DeadlineExceeded
		}
	}
	// The writer can win the race with cancellation before the dispatch
	// signal is selected. Mark abandonment under the same lock as dispatch.
	c.mu.Lock()
	req.abandoned = true
	client, transport, attemptDone := req.client, req.transport, req.attemptDone
	current := client != nil && c.client == client && c.transport == transport
	labeled := req.labeled
	c.mu.Unlock()
	if !labeled && client != nil && attemptDone != nil {
		if current {
			_ = transport.Close()
		}
		select {
		case <-attemptDone:
		case <-c.ctx.Done():
		case <-c.done:
		}
	}
	return historyResult{}, err
}

// A server can return an empty successful batch when a msgid has expired or
// was deleted. Retry using its retained time before treating that as the end
// of history, in either pagination direction.
func (c *connection) syncHistoryPoint(ctx context.Context, target, sub string, point historyPoint, limit int) (historyResult, error) {
	timestamp := "timestamp=" + point.At.UTC().Format(ircTimeLayout)
	if point.ID == "" || strings.HasPrefix(point.ID, "_") {
		return c.syncHistory(ctx, target, sub, timestamp, limit)
	}
	result, err := c.syncHistory(ctx, target, sub, "msgid="+point.ID, limit)
	if (err != nil || result.Count == 0) && ctx.Err() == nil && c.ctx.Err() == nil && !errors.Is(err, ErrDisconnected) && !errors.Is(err, ErrUnsupported) {
		return c.syncHistory(ctx, target, sub, timestamp, limit)
	}
	return result, err
}

func (c *connection) historyBatch(e girc.Event) {
	if len(e.Params) == 0 || len(e.Params[0]) < 2 {
		return
	}
	ref := e.Params[0][1:]
	c.mu.Lock()
	defer c.mu.Unlock()
	if e.Params[0][0] == '+' {
		label := e.Tags["label"]
		if label == "" {
			label = c.batchLabels[e.Tags["batch"]]
		}
		if label != "" && len(c.batchLabels) < 128 {
			c.batchLabels[ref] = label
		}
		if len(e.Params) >= 2 && (e.Params[1] == "chathistory" || e.Params[1] == "draft/chathistory-targets") {
			if raw := c.rawHistory; raw != nil && label == "" && ((strings.EqualFold(raw.sub, "TARGETS") && e.Params[1] == "draft/chathistory-targets") || (!strings.EqualFold(raw.sub, "TARGETS") && e.Params[1] == "chathistory" && len(e.Params) >= 3 && strings.EqualFold(raw.target, e.Params[2]))) {
				raw.batch = ref
			}
			req := c.historyRequests[label]
			if req == nil && !c.caps["labeled-response"] {
				req = c.unlabeledHistory
			}
			if req != nil && (e.Params[1] == "draft/chathistory-targets" || len(e.Params) >= 3 && strings.EqualFold(req.target, e.Params[2])) {
				if _, end := e.Tags["draft/chathistory-end"]; end {
					req.result.End = true
				}
				c.historyResponses[ref] = req
			}
		}
		return
	}
	if raw := c.rawHistory; raw != nil && !raw.expired && raw.batch == ref {
		c.finishRawHistory(raw)
	}
	if req := c.historyResponses[ref]; req != nil {
		if _, end := e.Tags["draft/chathistory-end"]; end {
			req.result.End = true
		}
		if req.result.Count == 0 {
			req.result.End = true
		}
		select {
		case req.done <- req.result:
		default:
		}
		delete(c.historyResponses, ref)
	}
	delete(c.batchLabels, ref)
}
func (c *connection) historyItem(ref string, e girc.Event) {
	if e.Command == "CHATHISTORY" && len(e.Params) >= 3 && e.Params[0] == "TARGETS" {
		c.mu.Lock()
		defer c.mu.Unlock()
		if req := c.historyResponses[ref]; req != nil {
			at, err := time.Parse(time.RFC3339Nano, e.Params[2])
			if err == nil {
				req.result.Targets = append(req.result.Targets, historyTarget{e.Params[1], at})
				req.result.Count++
			}
		}
		return
	}
	if e.Command != "PRIVMSG" && e.Command != "NOTICE" && e.Command != "TAGMSG" && e.Command != "REDACT" {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, e.Tags["time"])
	if err != nil {
		at = e.Timestamp
	}
	point := historyPoint{ID: e.Tags["msgid"], At: at}
	c.mu.Lock()
	defer c.mu.Unlock()
	req := c.historyResponses[ref]
	if req == nil {
		return
	}
	if req.sub == "BEFORE" && c.owner.store != nil && req.orderBase == 0 {
		base, err := c.owner.store.HistoryBase(c.owner.ownerID, req.conversation)
		if err != nil {
			req.result.Err = err
		} else {
			req.orderBase = base
		}
	}
	req.result.Count++
	if point.ID != "" {
		if req.result.IDs == nil {
			req.result.IDs = map[string]bool{}
		}
		req.result.IDs[point.ID] = true
	}
	if req.result.Oldest.At.IsZero() || at.Before(req.result.Oldest.At) {
		req.result.Oldest = point
	}
	if req.result.Newest.At.IsZero() || !at.Before(req.result.Newest.At) {
		req.result.Newest = point
	}
}

type historyRejected struct{ reason string }

func (e historyRejected) Error() string { return e.reason }

func (c *connection) historyFailure(e girc.Event) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	label := e.Tags["label"]
	if label == "" {
		label = c.batchLabels[e.Tags["batch"]]
	}
	req := c.historyRequests[label]
	if req == nil && label == "" && !c.caps["labeled-response"] {
		candidate := c.unlabeledHistory
		// Standard replies can omit request context. The gate serializes raw
		// and internal history, so a CHATHISTORY failure belongs to the sole
		// active operation unless supplied context contradicts it.
		if candidate != nil && e.Command == "FAIL" && len(e.Params) >= 3 && strings.EqualFold(e.Params[0], "CHATHISTORY") && historyFailureContextMatches(candidate, e.Params) {
			req = candidate
		}
	}
	if req != nil {
		reason := c.redactSecrets(e.Last())
		if reason == "" {
			reason = "IRC history request was rejected"
		}
		select {
		case req.done <- historyResult{Err: historyRejected{reason: reason}}:
		default:
		}
		return true
	}
	return false
}

func historyFailureContextMatches(req *historyRequest, params []string) bool {
	if len(params) < 4 {
		return true
	}
	sub := params[2]
	if !strings.EqualFold(sub, "AFTER") && !strings.EqualFold(sub, "BEFORE") && !strings.EqualFold(sub, "LATEST") && !strings.EqualFold(sub, "TARGETS") && !strings.EqualFold(sub, "AROUND") && !strings.EqualFold(sub, "BETWEEN") {
		return true
	}
	if !strings.EqualFold(sub, req.sub) {
		return false
	}
	if len(params) >= 5 && !strings.HasPrefix(params[3], "msgid=") && !strings.HasPrefix(params[3], "timestamp=") && !strings.EqualFold(params[3], req.target) {
		return false
	}
	if len(params) >= 6 && (strings.HasPrefix(params[4], "msgid=") || strings.HasPrefix(params[4], "timestamp=")) && params[4] != req.reference {
		return false
	}
	return true
}
func (c *connection) failHistory() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, req := range c.historyRequests {
		select {
		case req.done <- historyResult{Err: ErrDisconnected}:
		default:
		}
	}
	c.finishRawHistory(c.rawHistory)
	c.batchLabels = map[string]string{}
	c.historyResponses = map[string]*historyRequest{}
}

// catchupOnce processes one page, allowing other targets to run between pages
// and between retries of an unavailable conversation.
func (c *connection) catchupOnce(target string) (retry, failed bool) {
	w := c.owner
	w.mu.Lock()
	conv := c.conversation(target)
	w.mu.Unlock()
	watermark, err := w.store.Watermark(w.ownerID, conv.ID)
	if err != nil {
		return false, false
	}
	sub := "LATEST"
	if !watermark.At.IsZero() && watermark.At.UnixNano() > 0 {
		sub = "AFTER"
	}
	var result historyResult
	if sub == "AFTER" {
		result, err = c.syncHistoryPoint(c.ctx, target, sub, historyPoint{ID: watermark.ID, At: watermark.At}, 100)
	} else {
		result, err = c.syncHistory(c.ctx, target, sub, "*", 100)
	}
	if errors.Is(err, ErrUnsupported) {
		w.mu.Lock()
		w.emit("history.gap", map[string]string{"conversationId": conv.ID, "reason": "This network cannot recover messages missed while disconnected."})
		w.mu.Unlock()
		_ = w.store.CompleteGap(w.ownerID, conv.ID)
		return false, false
	}
	if err != nil {
		if c.ctx.Err() != nil {
			return false, false
		}
		if !errors.Is(err, ErrDisconnected) && !errors.Is(err, ErrBusy) {
			message := "IRC history synchronization will retry."
			var rejected historyRejected
			if errors.As(err, &rejected) {
				message += " " + rejected.reason
			}
			w.mu.Lock()
			w.emit("history.sync.failed", map[string]string{"conversationId": conv.ID, "message": message})
			w.mu.Unlock()
		}
		return true, true
	}
	w.mu.Lock()
	storageErr := w.storageErr
	w.mu.Unlock()
	if storageErr != nil {
		return false, false
	}
	if sub == "LATEST" || result.End {
		if sub == "LATEST" && result.End {
			_ = w.store.SetHistoryEnd(w.ownerID, conv.ID, true)
		}
		if err = w.store.CompleteGap(w.ownerID, conv.ID); err != nil {
			return false, false
		}
		if sub != "LATEST" {
			c.reconcileDeletions(target, conv.ID)
		}
		w.mu.Lock()
		w.recount(conv.ID)
		w.emit("history.synced", map[string]string{"conversationId": conv.ID})
		w.mu.Unlock()
		return false, false
	}
	if result.Newest.At.IsZero() || (!watermark.At.IsZero() && !result.Newest.At.After(watermark.At) && result.Newest.ID == watermark.ID) {
		return false, false
	}
	if err = w.store.AdvanceGap(w.ownerID, conv.ID, result.Newest.ID, result.Newest.At); err != nil {
		return false, false
	}
	return true, false
}
func (m *Manager) HistoryContext(ctx context.Context, owner, conversation, before string, limit int) (MessagePage, error) {
	if e := m.partyAccess(owner, conversation); e != nil {
		return MessagePage{}, e
	}
	if err := ctx.Err(); err != nil {
		return MessagePage{}, err
	}
	if m.options.Store == nil {
		return m.History(owner, conversation, before, limit)
	}
	if limit < 1 || limit > 100 {
		return MessagePage{}, fmt.Errorf("limit must be 1 to 100")
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	w.mu.Unlock()
	if !ok {
		return MessagePage{}, ErrNotFound
	}
	if conv.Party != nil {
		return w.historyPage(ctx, conversation, before, limit)
	}
	// The HTTP caller sees committed history. Older imports do not become live
	// messages; derived count changes are published separately.
	page, err := w.historyPage(ctx, conversation, before, limit)
	if err != nil {
		return page, err
	}
	if c != nil && c.hasAll("draft/chathistory", "batch", "server-time") && !w.store.HistoryEndedContext(ctx, owner, conversation) && len(page.Items) < limit {
		upstreamBefore := historyContinuation(before)
		for attempt := 0; attempt < 10; attempt++ {
			oldest, oldErr := w.store.OldestContext(ctx, owner, conversation)
			if oldErr != nil && !errors.Is(oldErr, storage.ErrNotFound) {
				return page, oldErr
			}
			var result historyResult
			var e error
			point := historyPoint{ID: oldest.UpstreamID, At: oldest.CreatedAt}
			if upstreamBefore != nil && (oldErr != nil || !point.At.Before(upstreamBefore.At)) {
				point = *upstreamBefore
			}
			if oldErr == nil || upstreamBefore != nil {
				result, e = c.syncHistoryPoint(ctx, c.target(conv), "BEFORE", point, 100)
			} else {
				result, e = c.syncHistory(ctx, c.target(conv), "LATEST", "*", 100)
			}
			if e != nil {
				if ctx.Err() != nil {
					return page, ctx.Err()
				}
				if len(page.Items) > 0 {
					next := historyContinueBefore(storage.Cursor(page.Items[0]), upstreamBefore)
					page.NextCursor = &next
					page.SyncError = "Older messages are temporarily unavailable. Retry this page."
					return page, nil
				}
				return page, e
			}
			if result.End {
				if e = w.store.SetHistoryEnd(owner, conversation, true); e != nil {
					return page, e
				}
			}
			w.mu.Lock()
			w.recount(conv.ID)
			w.mu.Unlock()
			page, e = w.historyPage(ctx, conversation, before, limit)
			if e != nil {
				return page, e
			}
			if len(page.Items) >= limit || result.End {
				break
			}
			// Reactions and edits can advance upstream pagination without adding
			// a timeline row. Continue before the response, not the same stored
			// message, while keeping the per-read request bound.
			if result.Oldest.At.IsZero() || (result.Oldest.ID == point.ID && result.Oldest.At.Equal(point.At)) {
				break
			}
			upstreamBefore = &result.Oldest
		}
		if len(page.Items) < limit && !w.store.HistoryEndedContext(ctx, owner, conversation) {
			page.SyncError = "Older messages are temporarily unavailable. Retry this page."
			retry := before
			if len(page.Items) == 0 {
				if before == "" {
					return page, fmt.Errorf("%w: %s", ErrBusy, page.SyncError)
				}
			} else {
				retry = storage.Cursor(page.Items[0])
			}
			// Preserve both boundaries when the request bound was reached;
			// an empty local window does not prove the start of upstream history.
			retry = historyContinueBefore(retry, upstreamBefore)
			page.NextCursor = &retry
		}
	}
	if page.NextCursor == nil && len(page.Items) > 0 && c != nil && c.hasAll("draft/chathistory", "batch", "server-time") && !w.store.HistoryEndedContext(ctx, owner, conversation) {
		next := storage.Cursor(page.Items[0])
		page.NextCursor = &next
	}
	return page, ctx.Err()
}

// HistoryAround returns a bounded window of the conversation's stored history
// centred on one message, so a client can open a conversation at a message far
// beyond its loaded timeline. NextCursor continues to older messages through
// the ordinary history read; HasNewer says the window is not the live end.
func (m *Manager) HistoryAround(owner, conversation, message string, limit int) (MessagePage, error) {
	return m.HistoryAroundContext(context.Background(), owner, conversation, message, limit)
}

func (m *Manager) HistoryAroundContext(ctx context.Context, owner, conversation, message string, limit int) (MessagePage, error) {
	if e := m.partyAccess(owner, conversation); e != nil {
		return MessagePage{}, e
	}
	if err := ctx.Err(); err != nil {
		return MessagePage{}, err
	}
	if limit < 1 || limit > 100 {
		return MessagePage{}, fmt.Errorf("limit must be 1 to 100")
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	if !ok {
		w.mu.Unlock()
		return MessagePage{}, ErrNotFound
	}
	if w.store == nil {
		defer w.mu.Unlock()
		items := w.messages[conversation]
		index := slices.IndexFunc(items, func(msg Message) bool { return msg.ID == message })
		if index < 0 {
			return MessagePage{}, ErrNotFound
		}
		start := max(0, index-(limit-1)/2)
		end := min(len(items), start+limit)
		page := MessagePage{Items: clone(items[start:end]), HasNewer: end < len(items), Cursor: w.seq}
		if start > 0 {
			cursor := items[start].ID
			page.NextCursor = &cursor
		}
		return page, nil
	}
	page, older, err := w.store.HistoryAroundContext(ctx, owner, conversation, message, limit)
	page.Cursor = w.seq
	w.mu.Unlock()
	if ctx.Err() != nil {
		return page, ctx.Err()
	}
	if errors.Is(err, storage.ErrNotFound) {
		return page, ErrNotFound
	}
	if err != nil {
		return page, ErrStorage
	}
	// Older messages may still be fetched from the server when the store runs out.
	if !older && len(page.Items) > 0 && c != nil && c.hasAll("draft/chathistory", "batch", "server-time") && !w.store.HistoryEndedContext(ctx, owner, conversation) {
		next := storage.Cursor(page.Items[0])
		page.NextCursor = &next
	}
	return page, ctx.Err()
}

// Recover known DMs and discover conversations which arrived while this account
// was offline. Upstream retention/always-on policy determines what is recoverable.
func (c *connection) recoverDirects() {
	w := c.owner
	w.mu.Lock()
	targets := []string{}
	for _, conv := range w.conversations {
		if conv.NetworkID == c.config.ID && conv.Kind == "direct" {
			targets = append(targets, c.target(conv))
		}
	}
	w.mu.Unlock()
	for _, target := range targets {
		c.requestReadMarker(target)
		if w.store != nil {
			c.catchup(target)
		}
	}
	if w.store == nil || c.config.SASLUser == "" || !c.hasAll("draft/chathistory", "batch", "server-time") {
		return
	}
	c.workers.Add(1)
	c.manager.wg.Add(1)
	go func() {
		defer c.manager.wg.Done()
		defer c.workers.Done()
		upper := time.Now().UTC()
		for c.ctx.Err() == nil {
			result, err := c.syncHistory(c.ctx, "timestamp="+upper.Format(ircTimeLayout), "TARGETS", "timestamp=1970-01-01T00:00:00.000Z", 100)
			if err != nil {
				return
			}
			oldest := upper
			for _, item := range result.Targets {
				if item.At.Before(oldest) {
					oldest = item.At
				}
				if girc.IsValidChannel(item.Name) || !girc.IsValidNick(item.Name) {
					continue
				}
				w.mu.Lock()
				conv := c.conversation(item.Name)
				w.mu.Unlock()
				mark, err := w.store.Watermark(w.ownerID, conv.ID)
				if err != nil {
					return
				}
				if mark.At.IsZero() || mark.At.UnixNano() <= 0 {
					if err = w.store.BeginGap(w.ownerID, conv.ID); err != nil {
						return
					}
					if err = w.store.AdvanceGap(w.ownerID, conv.ID, "", time.Unix(0, 1)); err != nil {
						return
					}
				}
				c.catchup(item.Name)
			}
			if result.End || len(result.Targets) == 0 || !oldest.Before(upper) {
				return
			}
			upper = oldest
		}
	}()
}

// reconcileDeletions applies deletions that happened while this connection was
// offline. Ergo drops a redacted message from CHATHISTORY instead of replaying the
// REDACT, so after catching up, a stored channel message strictly inside the
// latest history window that the window no longer contains was deleted upstream.
// It only runs where the network negotiated message redaction, and only for
// channels: direct-message history depends on per-user settings, so a missing
// private message does not prove a deletion. Older deletions stay undetected.
func (c *connection) reconcileDeletions(target, conversation string) {
	w := c.owner
	if w.store == nil || !girc.IsValidChannel(target) || !c.has("draft/message-redaction") {
		return
	}
	result, err := c.syncHistory(c.ctx, target, "LATEST", "*", 100)
	if err != nil || result.Count < 2 || result.Oldest.At.IsZero() || !result.Oldest.At.Before(result.Newest.At) {
		return
	}
	stored, err := w.store.MessagesBetween(w.ownerID, conversation, result.Oldest.At, result.Newest.At)
	if err != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	conv, ok := w.conversations[conversation]
	if !ok {
		return
	}
	for _, msg := range stored {
		if msg.Deleted || msg.UpstreamID == "" || strings.HasPrefix(msg.UpstreamID, "_") {
			continue
		}
		current, found := c.lookupMessage(conversation, msg.ID)
		if !found {
			continue
		}
		if !result.IDs[msg.UpstreamID] {
			c.applyRedaction(conv, current, false)
		} else if kept := keptEdits(current.Edits, result); len(kept) != len(current.Edits) {
			// An edit deleted while offline is missing from the window too.
			current.Edits = kept
			c.saveEdited(conv, current, false)
		}
		if w.storageErr != nil {
			return
		}
	}
}

// keptEdits drops edits strictly inside the history window that the window no
// longer contains.
func keptEdits(edits []model.MessageEdit, result historyResult) []model.MessageEdit {
	kept := edits[:0:0]
	for _, v := range edits {
		inside := v.At.After(result.Oldest.At) && v.At.Before(result.Newest.At)
		if !inside || result.IDs[v.UpstreamID] {
			kept = append(kept, v)
		}
	}
	return kept
}
