package irc

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

func sortedMemoryHistory(messages []Message) []Message {
	items := slices.Clone(messages)
	slices.SortFunc(items, func(a, b Message) int {
		if order := a.CreatedAt.Compare(b.CreatedAt); order != 0 {
			return order
		}
		if order := cmp.Compare(a.Position, b.Position); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return items
}

// historyPage captures the retained page and its event position under the same
// workspace lock used by message mutations. No upstream wait holds this lock.
func (w *workspace) historyPage(ctx context.Context, conversation string, boundary *storage.HistoryCursor, limit int) (MessagePage, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	page, err := w.store.HistoryCursorContext(ctx, w.ownerID, conversation, boundary, limit)
	page.Cursor = w.seq
	if err == nil && page.NextCursor != nil && len(page.Items) > 0 && boundary != nil && boundary.UpstreamBefore != nil {
		var next string
		next, err = storage.ScopedHistoryCursor(w.ownerID, conversation, page.Items[0], boundary.UpstreamBefore)
		if err == nil {
			page.NextCursor = &next
		}
	}
	return page, err
}

func (m *Manager) History(owner, conversation, before string, limit int) (MessagePage, error) {
	if e := m.partyAccess(owner, conversation); e != nil {
		return MessagePage{}, e
	}
	if limit < 1 || limit > 100 {
		return MessagePage{}, fmt.Errorf("limit must be between 1 and 100")
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.conversations[conversation]; !ok {
		return MessagePage{}, ErrNotFound
	}
	items := sortedMemoryHistory(w.messages[conversation])
	end := len(items)
	var upstream *storage.HistoryUpstream
	if before != "" {
		if len(before) > storage.MaxHistoryCursorLength {
			return MessagePage{}, storage.ErrInvalidCursor
		}
		// Legacy memory cursors are raw message IDs, including IDs that look like base64.
		end = slices.IndexFunc(items, func(msg Message) bool { return msg.ID == before })
		if end < 0 {
			boundary, err := storage.ParseHistoryCursor(before, owner, conversation)
			if err != nil {
				return MessagePage{}, err
			}
			upstream = boundary.UpstreamBefore
			end = 0
			for end < len(items) {
				msg := items[end]
				if msg.CreatedAt.UnixNano() > boundary.At || (msg.CreatedAt.UnixNano() == boundary.At && (msg.Position > boundary.Position || (msg.Position == boundary.Position && msg.ID >= boundary.ID))) {
					break
				}
				end++
			}
		}
	}
	start := max(0, end-limit)
	page := MessagePage{Items: clone(items[start:end]), Cursor: w.seq}
	if page.Items == nil {
		page.Items = []Message{}
	}
	if start > 0 {
		cursor, err := storage.ScopedHistoryCursor(owner, conversation, items[start], upstream)
		if err != nil {
			return MessagePage{}, err
		}
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
