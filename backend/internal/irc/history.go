package irc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lrstanley/girc"
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
}

const ircTimeLayout = "2006-01-02T15:04:05.000Z"

type historyRequest struct {
	sub          string
	conversation string
	orderBase    int64
	label        string
	target       string
	done         chan historyResult
	result       historyResult
}

func (c *connection) syncHistory(ctx context.Context, target, sub, reference string, limit int) (historyResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	select {
	case c.historyGate <- struct{}{}:
		defer func() { <-c.historyGate }()
	case <-ctx.Done():
		return historyResult{}, ctx.Err()
	case <-c.ctx.Done():
		return historyResult{}, ErrDisconnected
	}
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
		case <-tick.C:
		}
	}
	if !c.has("draft/chathistory") {
		return historyResult{}, ErrUnsupported
	}
	req := &historyRequest{sub: sub, conversation: c.conversationID(target), label: id(), target: target, done: make(chan historyResult, 1)}
	c.mu.Lock()
	c.historyRequests[req.label] = req
	c.unlabeledHistory = req
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.historyRequests, req.label)
		if c.unlabeledHistory == req {
			c.unlabeledHistory = nil
		}
		c.mu.Unlock()
	}()
	event := &girc.Event{Command: "CHATHISTORY", Params: []string{sub, target, reference, fmt.Sprint(limit)}}
	if c.has("labeled-response") {
		event.Tags = girc.Tags{"label": req.label}
	}
	if err := c.enqueue(event); err != nil {
		return historyResult{}, err
	}
	select {
	case result := <-req.done:
		return result, result.Err
	case <-ctx.Done():
		return historyResult{}, ctx.Err()
	case <-c.ctx.Done():
		return historyResult{}, ErrDisconnected
	case <-c.done:
		return historyResult{}, ErrDisconnected
	}
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
	if e.Command != "PRIVMSG" && e.Command != "NOTICE" && e.Command != "TAGMSG" {
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
	if req.result.Oldest.At.IsZero() || at.Before(req.result.Oldest.At) {
		req.result.Oldest = point
	}
	if req.result.Newest.At.IsZero() || !at.Before(req.result.Newest.At) {
		req.result.Newest = point
	}
}
func (c *connection) historyFailure(e girc.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	label := e.Tags["label"]
	if label == "" {
		label = c.batchLabels[e.Tags["batch"]]
	}
	req := c.historyRequests[label]
	if req == nil && !c.caps["labeled-response"] && len(e.Params) > 0 && e.Params[0] == "CHATHISTORY" {
		req = c.unlabeledHistory
	}
	if req != nil {
		select {
		case req.done <- historyResult{Err: errors.New("IRC history request was rejected")}:
		default:
		}
	}
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
	c.batchLabels = map[string]string{}
	c.historyResponses = map[string]*historyRequest{}
}
func (c *connection) catchup(target string) {
	if c.owner.store == nil {
		_ = c.enqueue(&girc.Event{Command: "CHATHISTORY", Params: []string{"LATEST", target, "*", "100"}})
		return
	}
	c.mu.Lock()
	if c.catching[target] {
		c.mu.Unlock()
		return
	}
	c.catching[target] = true
	c.mu.Unlock()
	c.workers.Add(1)
	c.manager.wg.Add(1)
	go func() {
		defer c.manager.wg.Done()
		defer c.workers.Done()
		defer func() { c.mu.Lock(); delete(c.catching, target); c.mu.Unlock() }()
		w := c.owner
		w.mu.Lock()
		conv := c.conversation(target)
		w.mu.Unlock()
		backoff := time.Second
		for c.ctx.Err() == nil {
			watermark, err := w.store.Watermark(w.ownerID, conv.ID)
			if err != nil {
				return
			}
			sub, ref := "LATEST", "*"
			if !watermark.At.IsZero() && watermark.At.UnixNano() > 0 {
				sub = "AFTER"
				ref = "msgid=" + watermark.ID
				if watermark.ID == "" {
					ref = "timestamp=" + watermark.At.UTC().Format(ircTimeLayout)
				}
			}
			ctx, cancel := context.WithTimeout(c.ctx, 15*time.Second)
			result, err := c.syncHistory(ctx, target, sub, ref, 100)
			cancel()
			if err != nil && sub == "AFTER" && strings.HasPrefix(ref, "msgid=") {
				ctx, cancel = context.WithTimeout(c.ctx, 15*time.Second)
				result, err = c.syncHistory(ctx, target, sub, "timestamp="+watermark.At.UTC().Format(ircTimeLayout), 100)
				cancel()
			}
			if errors.Is(err, ErrUnsupported) {
				w.mu.Lock()
				w.emit("history.gap", map[string]string{"conversationId": conv.ID, "reason": "This network cannot recover messages missed while disconnected."})
				w.mu.Unlock()
				_ = w.store.CompleteGap(w.ownerID, conv.ID)
				return
			}
			if err != nil {
				if errors.Is(err, ErrDisconnected) || c.ctx.Err() != nil {
					return
				}
				w.mu.Lock()
				w.emit("history.sync.failed", map[string]string{"conversationId": conv.ID, "message": "IRC history synchronization will retry."})
				w.mu.Unlock()
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff = min(backoff*2, 30*time.Second)
				continue
			}
			w.mu.Lock()
			storageErr := w.storageErr
			w.mu.Unlock()
			if storageErr != nil {
				return
			}
			if sub == "LATEST" || result.End {
				if sub == "LATEST" && result.End {
					_ = w.store.SetHistoryEnd(w.ownerID, conv.ID, true)
				}
				if err = w.store.CompleteGap(w.ownerID, conv.ID); err != nil {
					return
				}
				w.mu.Lock()
				conv = w.conversations[conv.ID]
				conv.UnreadCount, _ = w.store.Unread(w.ownerID, conv.ID)
				conv.MentionCount, _ = w.store.Mentions(w.ownerID, conv.ID)
				w.conversations[conv.ID] = conv
				w.emit("conversation.updated", conv)
				w.emit("history.synced", map[string]string{"conversationId": conv.ID})
				w.mu.Unlock()
				return
			}
			if result.Newest.At.IsZero() || (!watermark.At.IsZero() && !result.Newest.At.After(watermark.At) && result.Newest.ID == watermark.ID) {
				return
			}
			if err = w.store.AdvanceGap(w.ownerID, conv.ID, result.Newest.ID, result.Newest.At); err != nil {
				return
			}
		}
	}()
}
func (m *Manager) HistoryContext(ctx context.Context, owner, conversation, before string, limit int) (MessagePage, error) {
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
	// The HTTP caller sees committed history. Older imports never become live
	// messages or change the stream cursor merely because somebody scrolls up.
	page, err := w.store.History(owner, conversation, before, limit)
	if err != nil {
		return page, err
	}
	if c != nil && c.has("draft/chathistory") && !w.store.HistoryEnded(owner, conversation) && len(page.Items) < limit {
		for attempt := 0; attempt < 10; attempt++ {
			oldest, oldErr := w.store.Oldest(owner, conversation)
			sub, ref := "LATEST", "*"
			if oldErr == nil {
				sub = "BEFORE"
				ref = "timestamp=" + oldest.CreatedAt.UTC().Format(ircTimeLayout)
				if oldest.UpstreamID != "" && !strings.HasPrefix(oldest.UpstreamID, "_") {
					ref = "msgid=" + oldest.UpstreamID
				}
			} else if !errors.Is(oldErr, storage.ErrNotFound) {
				return page, oldErr
			}
			result, e := c.syncHistory(ctx, c.target(conv), sub, ref, 100)
			if e != nil {
				if len(page.Items) > 0 {
					next := storage.Cursor(page.Items[0])
					page.NextCursor = &next
					page.SyncError = "Older upstream history is temporarily unavailable; retry this page."
					return page, nil
				}
				return page, e
			}
			if result.End {
				if e = w.store.SetHistoryEnd(owner, conversation, true); e != nil {
					return page, e
				}
			}
			page, e = w.store.History(owner, conversation, before, limit)
			if e != nil {
				return page, e
			}
			if len(page.Items) >= limit || result.End {
				break
			}
		}
	}
	if page.NextCursor == nil && len(page.Items) > 0 && c != nil && c.has("draft/chathistory") && !w.store.HistoryEnded(owner, conversation) {
		next := storage.Cursor(page.Items[0])
		page.NextCursor = &next
	}
	w.mu.Lock()
	for _, msg := range page.Items {
		m.schedulePreview(w, conv, msg)
	}
	w.mu.Unlock()
	return page, nil
}

// Recover known DMs and discover conversations which arrived while this account
// was offline. Upstream retention/always-on policy determines what is recoverable.
func (c *connection) recoverDirects() {
	if c.owner.store == nil {
		return
	}
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
		c.catchup(target)
	}
	if c.config.SASLUser == "" || !c.has("draft/chathistory") {
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
