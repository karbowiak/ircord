package irc

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/notify"
	"ircord/backend/internal/storage"
)

// InboxExcerptRunes bounds the text an inbox item carries.
const InboxExcerptRunes = 280

type InboxEntry = model.InboxEntry

// InboxItem is an inbox entry resolved against the account's current history
// and workspace. "deleted" keeps who sent it and when, without text;
// "unavailable" (the conversation was left or removed, or the stored copy is
// gone) keeps only the time, so an old reference never shows more than the
// conversation itself would.
type InboxItem struct {
	MessageID      string    `json:"messageId"`
	ConversationID string    `json:"conversationId"`
	NetworkID      string    `json:"networkId"`
	SenderID       string    `json:"senderId,omitempty"`
	Reasons        []string  `json:"reasons" nullable:"false" enum:"mention,reply,direct" doc:"Why the message is in the inbox"`
	CreatedAt      time.Time `json:"createdAt"`
	Dismissed      bool      `json:"dismissed"`
	Unread         bool      `json:"unread" doc:"The message is unread in its conversation; reading the conversation reads it, dismissing does not"`
	Status         string    `json:"status" enum:"available,deleted,unavailable"`
	Kind           string    `json:"kind,omitempty" enum:"message,action,notice"`
	Excerpt        string    `json:"excerpt,omitempty" doc:"The message as plain text, at most 280 characters"`
	Truncated      bool      `json:"truncated,omitempty"`
}

type InboxPage struct {
	Items      []InboxItem `json:"items" nullable:"false"`
	NextCursor *string     `json:"nextCursor"`
	// Unread is the inbox's unread indicator at the time of this page.
	Unread int `json:"unread"`
}

type InboxSummary struct {
	Unread int `json:"unread" doc:"Undismissed inbox items whose message is still unread in its conversation"`
}

// recordInbox adds a live message that needs the account's attention to its
// inbox, whether or not any client is open or subscribed to push. It runs with
// w.mu held after the message itself was stored. The store keeps one item per
// message, so replays and repeated deliveries never add a second one.
func (m *Manager) recordInbox(w *workspace, conv Conversation, msg Message, sender Member, nick string, replyToSelf bool) {
	if w.store == nil || msg.Historical || msg.Deleted || msg.Kind != "message" && msg.Kind != "action" && msg.Kind != "notice" {
		return
	}
	shared := ""
	if conv.ChannelSettings != nil {
		shared = conv.ChannelSettings.Notifications
	}
	reasons := w.policy.Attention(conv.ID, conv.Kind, shared, msg.Kind, msg.Text, nick, sender.Nick, sender.Username, sender.Host, sender.ID == w.self.ID, replyToSelf)
	if len(reasons) == 0 {
		return
	}
	w.emit("inbox.added", InboxEntry{MessageID: msg.ID, ConversationID: conv.ID, NetworkID: conv.NetworkID, SenderID: sender.ID, Reasons: reasons, CreatedAt: msg.CreatedAt})
}

// excerpt is a bounded plain-text copy of a message for the inbox.
func excerpt(text string) (string, bool) {
	runes := []rune(strings.Join(strings.Fields(notify.Plain(text)), " "))
	if len(runes) <= InboxExcerptRunes {
		return string(runes), false
	}
	return strings.TrimSpace(string(runes[:InboxExcerptRunes])) + "…", true
}

// resolveInbox must run with w.mu held.
func (w *workspace) resolveInbox(row storage.InboxRow) InboxItem {
	e := row.Entry
	item := InboxItem{MessageID: e.MessageID, ConversationID: e.ConversationID, NetworkID: e.NetworkID, Reasons: e.Reasons, CreatedAt: e.CreatedAt, Dismissed: row.Dismissed, Status: "unavailable"}
	conv, ok := w.conversations[e.ConversationID]
	if !ok || !row.Open || conv.NetworkID != e.NetworkID || row.Message == nil {
		return item
	}
	item.SenderID = e.SenderID
	item.Kind = row.Message.Kind
	if row.Message.Deleted {
		item.Status = "deleted"
		return item
	}
	item.Status = "available"
	item.Unread = row.Unread
	item.Excerpt, item.Truncated = excerpt(row.Message.Text)
	return item
}

// Inbox pages the account's attention inbox, newest first, under one of
// storage.InboxFilters. Without durable storage the inbox stays empty.
func (m *Manager) Inbox(owner, filter, cursor string, limit int) (InboxPage, error) {
	if limit < 1 || limit > 100 {
		return InboxPage{}, fmt.Errorf("limit must be between 1 and 100")
	}
	if filter == "" {
		filter = "all"
	}
	if !slices.Contains(storage.InboxFilters, filter) {
		return InboxPage{}, fmt.Errorf("unknown inbox filter %q", filter)
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return InboxPage{}, ErrNotFound
	}
	page := InboxPage{Items: []InboxItem{}}
	if w.store == nil {
		return page, nil
	}
	rows, more, err := w.store.InboxList(owner, filter, cursor, limit)
	if errors.Is(err, storage.ErrInvalidCursor) {
		return page, err
	}
	if err != nil {
		return page, ErrStorage
	}
	for _, row := range rows {
		page.Items = append(page.Items, w.resolveInbox(row))
	}
	if more && len(rows) > 0 {
		next := storage.InboxCursor(rows[len(rows)-1])
		page.NextCursor = &next
	}
	if page.Unread, err = w.store.InboxUnread(owner); err != nil {
		return page, ErrStorage
	}
	return page, nil
}

// InboxSummary is the inbox's lightweight unread indicator.
func (m *Manager) InboxSummary(owner string) (InboxSummary, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return InboxSummary{}, ErrNotFound
	}
	if w.store == nil {
		return InboxSummary{}, nil
	}
	n, err := w.store.InboxUnread(owner)
	if err != nil {
		return InboxSummary{}, ErrStorage
	}
	return InboxSummary{Unread: n}, nil
}

// DismissInbox dismisses or restores one inbox item on all of the account's
// clients. It changes only the inbox: the conversation's read position, push
// delivery and notification state are untouched. Repeating it changes nothing.
func (m *Manager) DismissInbox(owner, messageID string, dismissed bool) error {
	if messageID == "" || len(messageID) > 200 {
		return ErrNotFound
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted || w.store == nil {
		return ErrNotFound
	}
	row, err := w.store.InboxItem(owner, messageID)
	if errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return ErrStorage
	}
	if row.Dismissed == dismissed {
		return nil
	}
	w.emit("inbox.updated", model.InboxChange{MessageID: messageID, Dismissed: dismissed})
	if w.storageErr != nil {
		return ErrStorage
	}
	return nil
}
