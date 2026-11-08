package irc

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

// SavedLimit bounds how many messages one account can save.
var SavedLimit = 1000

var ErrSavedLimit = errors.New("you have reached the saved-message limit; remove some saved messages first")

type SavedMessage = model.SavedMessage

// SavedItem is a bookmark resolved against the account's current history and
// workspace. Status "deleted" keeps only who sent it and when; "unavailable"
// (the conversation was left or removed, or the stored copy is gone) carries
// no message or conversation, so a bookmark never reveals more than the
// conversation itself would.
type SavedItem struct {
	MessageID      string        `json:"messageId"`
	ConversationID string        `json:"conversationId"`
	NetworkID      string        `json:"networkId"`
	SavedAt        time.Time     `json:"savedAt"`
	Status         string        `json:"status" enum:"available,deleted,unavailable"`
	Message        *Message      `json:"message,omitempty"`
	Conversation   *Conversation `json:"conversation,omitempty"`
}

type SavedPage struct {
	Items      []SavedItem `json:"items" nullable:"false"`
	NextCursor *string     `json:"nextCursor"`
}

func savable(msg Message) bool {
	return !msg.Deleted && (msg.Kind == "message" || msg.Kind == "action" || msg.Kind == "notice")
}

// resolveSaved must run with w.mu held.
func (w *workspace) resolveSaved(entry SavedMessage, msg *Message) SavedItem {
	item := SavedItem{MessageID: entry.MessageID, ConversationID: entry.ConversationID, NetworkID: entry.NetworkID, SavedAt: entry.SavedAt, Status: "unavailable"}
	conv, ok := w.conversations[entry.ConversationID]
	if !ok || conv.NetworkID != entry.NetworkID || msg == nil || msg.ConversationID != entry.ConversationID {
		return item
	}
	c := clone(conv)
	item.Conversation = &c
	if msg.Deleted {
		item.Status = "deleted"
		item.Message = &Message{ID: msg.ID, ConversationID: msg.ConversationID, SenderID: msg.SenderID, CreatedAt: msg.CreatedAt, Kind: msg.Kind, Deleted: true, Reactions: []model.Reaction{}}
		return item
	}
	item.Status = "available"
	m := clone(*msg)
	item.Message = &m
	return item
}

// memoryMessage finds a message of a workspace without durable storage; w.mu held.
func (w *workspace) memoryMessage(conversation, id string) *Message {
	for _, msg := range w.messages[conversation] {
		if msg.ID == id || (msg.UpstreamID != "" && msg.UpstreamID == id) {
			m := msg
			return &m
		}
	}
	return nil
}

// SaveMessage bookmarks a stored message of one of the account's conversations.
// Saving an already saved message returns the existing bookmark without a new event.
func (m *Manager) SaveMessage(owner, conversation, messageID string) (SavedItem, error) {
	if messageID == "" || len(messageID) > 200 {
		return SavedItem{}, ErrNotFound
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return SavedItem{}, ErrNotFound
	}
	conv, ok := w.conversations[conversation]
	if !ok {
		return SavedItem{}, ErrNotFound
	}
	var msg *Message
	if w.store != nil {
		stored, err := w.store.Message(owner, conversation, messageID)
		if errors.Is(err, storage.ErrNotFound) {
			return SavedItem{}, ErrNotFound
		}
		if err != nil {
			return SavedItem{}, ErrStorage
		}
		msg = &stored
	} else {
		msg = w.memoryMessage(conversation, messageID)
	}
	// Bookmarks name the canonical message ID that every route uses; an upstream
	// msgid alone is not accepted, so PUT and DELETE always agree on the key.
	if msg == nil || msg.ID != messageID {
		return SavedItem{}, ErrNotFound
	}
	if !savable(*msg) {
		return SavedItem{}, fmt.Errorf("only delivered chat messages can be saved")
	}
	existing, found, err := w.savedEntry(msg.ID)
	if err != nil {
		return SavedItem{}, err
	}
	if found {
		return w.resolveSaved(existing, msg), nil
	}
	count := len(w.saved)
	if w.store != nil {
		if count, err = w.store.SavedCount(owner); err != nil {
			return SavedItem{}, ErrStorage
		}
	}
	if count >= SavedLimit {
		return SavedItem{}, ErrSavedLimit
	}
	entry := SavedMessage{MessageID: msg.ID, ConversationID: conversation, NetworkID: conv.NetworkID, SavedAt: time.Now().UTC()}
	w.emit("saved.added", entry)
	if w.storageErr != nil {
		return SavedItem{}, ErrStorage
	}
	if w.store == nil {
		if w.saved == nil {
			w.saved = map[string]SavedMessage{}
		}
		w.saved[entry.MessageID] = entry
	}
	return w.resolveSaved(entry, msg), nil
}

// savedEntry reads one bookmark; w.mu held.
func (w *workspace) savedEntry(messageID string) (SavedMessage, bool, error) {
	if w.store == nil {
		v, ok := w.saved[messageID]
		return v, ok, nil
	}
	v, err := w.store.Saved(w.ownerID, messageID)
	if errors.Is(err, storage.ErrNotFound) {
		return v, false, nil
	}
	if err != nil {
		return v, false, ErrStorage
	}
	return v, true, nil
}

// UnsaveMessage removes the account's bookmark. Removing one that is not saved succeeds.
func (m *Manager) UnsaveMessage(owner, messageID string) error {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return ErrNotFound
	}
	existing, found, err := w.savedEntry(messageID)
	if err != nil || !found {
		return err
	}
	w.emit("saved.removed", model.SavedRemoval{MessageID: existing.MessageID, ConversationID: existing.ConversationID})
	if w.storageErr != nil {
		return ErrStorage
	}
	delete(w.saved, messageID)
	return nil
}

// SavedMessages pages the account's bookmarks, newest first. A query keeps only
// available messages whose text contains it, ignoring ASCII case.
func (m *Manager) SavedMessages(owner, cursor, query string, limit int) (SavedPage, error) {
	if limit < 1 || limit > 100 {
		return SavedPage{}, fmt.Errorf("limit must be between 1 and 100")
	}
	query = strings.TrimSpace(query)
	if len(query) > 200 {
		return SavedPage{}, fmt.Errorf("query must have at most 200 bytes")
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return SavedPage{}, ErrNotFound
	}
	page := SavedPage{Items: []SavedItem{}}
	// A search drops matches the account can no longer see, so it keeps reading
	// (a bounded number of pages) to fill the page rather than return a short or
	// empty page that still has more behind it.
	for reads := 0; reads < 10; reads++ {
		var entries []storage.SavedEntry
		more := false
		var err error
		if w.store != nil {
			entries, more, err = w.store.SavedList(owner, cursor, query, limit)
			if err != nil {
				if errors.Is(err, storage.ErrInvalidCursor) {
					return page, err
				}
				return page, ErrStorage
			}
		} else if entries, more, err = w.memorySaved(cursor, query, limit); err != nil {
			return page, err
		}
		for i, e := range entries {
			cursor = storage.SavedCursor(e.Saved)
			item := w.resolveSaved(e.Saved, e.Message)
			// A search only finds what the conversation would show.
			if query != "" && item.Status != "available" {
				continue
			}
			page.Items = append(page.Items, item)
			if len(page.Items) == limit {
				if more || i < len(entries)-1 {
					page.NextCursor = &cursor
				}
				return page, nil
			}
		}
		if !more {
			return page, nil
		}
	}
	page.NextCursor = &cursor
	return page, nil
}

// memorySaved mirrors Store.SavedList for a workspace without storage; w.mu held.
func (w *workspace) memorySaved(cursor, query string, limit int) ([]storage.SavedEntry, bool, error) {
	all := make([]SavedMessage, 0, len(w.saved))
	for _, v := range w.saved {
		all = append(all, v)
	}
	slices.SortFunc(all, func(a, b SavedMessage) int {
		if c := b.SavedAt.Compare(a.SavedAt); c != 0 {
			return c
		}
		return strings.Compare(b.MessageID, a.MessageID)
	})
	start := 0
	if cursor != "" {
		start = -1
		for i, v := range all {
			if storage.SavedCursor(v) == cursor {
				start = i + 1
				break
			}
		}
		if start < 0 {
			return nil, false, storage.ErrInvalidCursor
		}
	}
	out := []storage.SavedEntry{}
	for _, v := range all[start:] {
		msg := w.memoryMessage(v.ConversationID, v.MessageID)
		if query != "" && (msg == nil || msg.Deleted || !strings.Contains(asciiLower(msg.Text), asciiLower(query))) {
			continue
		}
		if len(out) == limit {
			return out, true, nil
		}
		out = append(out, storage.SavedEntry{Saved: v, Message: msg})
	}
	return out, false, nil
}

// SavedRefs lists which messages the account saved, so clients can show the
// saved state on any message without loading the bookmarks' content.
func (m *Manager) SavedRefs(owner string) ([]SavedMessage, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return nil, ErrNotFound
	}
	if w.store != nil {
		refs, err := w.store.SavedRefs(owner)
		if err != nil {
			return nil, ErrStorage
		}
		return refs, nil
	}
	refs := make([]SavedMessage, 0, len(w.saved))
	for _, v := range w.saved {
		refs = append(refs, v)
	}
	slices.SortFunc(refs, func(a, b SavedMessage) int { return b.SavedAt.Compare(a.SavedAt) })
	return refs, nil
}

// asciiLower matches SQLite's lower(), which the stored search uses.
func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, s)
}
