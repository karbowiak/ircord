package irc

import (
	"context"
	"errors"
	"fmt"
	"ircord/backend/internal/storage"
	"time"
)

type OfflineSnapshot struct {
	State      *Snapshot `json:"state,omitempty"`
	Cursor     uint64    `json:"cursor"`
	Since      time.Time `json:"since"`
	Items      []Message `json:"items" nullable:"false"`
	NextCursor *string   `json:"nextCursor"`
}

// OfflineSnapshot captures each page and its account position together. The
// client merges concurrent replay by revision; a page never advances replay.
func (m *Manager) OfflineSnapshot(ctx context.Context, owner, before string, limit int) (OfflineSnapshot, error) {
	if limit < 1 || limit > 500 {
		return OfflineSnapshot{}, fmt.Errorf("limit must be 1 to 500")
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return OfflineSnapshot{}, ErrNotFound
	}
	if w.storageErr != nil {
		return OfflineSnapshot{}, ErrStorage
	}
	if w.store == nil {
		return OfflineSnapshot{}, ErrUnsupported
	}
	items, next, since, err := w.store.OfflineHistory(ctx, owner, before, limit)
	page := OfflineSnapshot{Cursor: w.seq, Since: since, Items: items, NextCursor: next}
	if before == "" && err == nil {
		snapshot := w.snapshot()
		page.State = &snapshot
	}
	return page, err
}

type OfflineMessageRef struct {
	ConversationID string `json:"conversationId"`
	MessageID      string `json:"messageId"`
}
type OfflinePinned struct {
	Items  []Message `json:"items" nullable:"false"`
	Cursor uint64    `json:"cursor"`
}

func (m *Manager) OfflinePinned(ctx context.Context, owner string, refs []OfflineMessageRef) (OfflinePinned, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	result := OfflinePinned{Items: []Message{}, Cursor: w.seq}
	if w.deleted {
		return result, ErrNotFound
	}
	if w.store == nil {
		return result, ErrUnsupported
	}
	for _, ref := range refs {
		if _, ok := w.conversations[ref.ConversationID]; !ok {
			continue
		}
		message, err := w.store.MessageContext(ctx, owner, ref.ConversationID, ref.MessageID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, message)
	}
	return result, nil
}
