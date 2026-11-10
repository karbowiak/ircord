package irc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/lrstanley/girc"
)

// Keep the local row boundary in the ordinary opaque storage cursor, with an
// optional upstream position for pages of reactions or consumed edits. A retry
// can continue upstream without skipping retained timeline rows.
func historyContinuation(before string) *historyPoint {
	raw, err := base64.RawURLEncoding.DecodeString(before)
	var cursor struct {
		Upstream *historyPoint `json:"upstreamBefore"`
	}
	if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Upstream == nil || cursor.Upstream.At.IsZero() || !clean(cursor.Upstream.ID, 100) {
		return nil
	}
	return cursor.Upstream
}

func historyContinueBefore(before string, point *historyPoint) string {
	if point == nil {
		return before
	}
	raw, err := base64.RawURLEncoding.DecodeString(before)
	var cursor map[string]json.RawMessage
	if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor == nil {
		return before
	}
	cursor["upstreamBefore"], _ = json.Marshal(point)
	raw, _ = json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// historyPage captures the retained page and its event position under the same
// workspace lock used by message mutations. No upstream wait holds this lock.
func (w *workspace) historyPage(ctx context.Context, conversation, before string, limit int) (MessagePage, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	page, err := w.store.HistoryContext(ctx, w.ownerID, conversation, before, limit)
	page.Cursor = w.seq
	return page, err
}

func (m *Manager) History(owner, conversation, before string, limit int) (MessagePage, error) {
	if limit < 1 || limit > 100 {
		return MessagePage{}, fmt.Errorf("limit must be between 1 and 100")
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.conversations[conversation]; !ok {
		return MessagePage{}, ErrNotFound
	}
	items := w.messages[conversation]
	end := len(items)
	if before != "" {
		end = -1
		for i := range items {
			if items[i].ID == before {
				end = i
				break
			}
		}
		if end < 0 {
			return MessagePage{}, fmt.Errorf("invalid history cursor")
		}
	}
	start := max(0, end-limit)
	page := MessagePage{Items: clone(items[start:end]), Cursor: w.seq}
	if page.Items == nil {
		page.Items = []Message{}
	}
	if start > 0 {
		cursor := items[start].ID
		page.NextCursor = &cursor
	}
	return page, nil
}

// RequestHistory imports server history asynchronously. Incoming msgids dedupe
// overlapping auto-replay, explicit requests, and reconnect history.
func (m *Manager) RequestHistory(owner, conversation, before string, limit int) error {
	if limit < 1 || limit > 100 {
		return fmt.Errorf("invalid history limit")
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	w.mu.Unlock()
	if !ok || c == nil {
		return ErrNotFound
	}
	if !c.hasAll("draft/chathistory", "batch", "server-time") {
		return ErrUnsupported
	}
	sub, ref := "LATEST", "*"
	if before != "" {
		if !clean(before, 100) {
			return fmt.Errorf("invalid upstream cursor")
		}
		sub = "BEFORE"
		ref = "msgid=" + before
	}
	return c.enqueue(&girc.Event{Command: "CHATHISTORY", Params: []string{sub, c.target(conv), ref, fmt.Sprint(limit)}})
}
