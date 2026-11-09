package irc

import (
	"strings"
	"time"

	"github.com/lrstanley/girc"

	"ircord/backend/internal/model"
)

// Workspace helpers run under w.mu. Persisted history remains authoritative even
// when its latest message has been evicted from the bounded in-memory cache.
func (w *workspace) latestReadMessage(conversation string, through time.Time, serverOnly bool) (time.Time, error) {
	if w.store != nil {
		return w.store.LatestMessageTime(w.ownerID, conversation, through, serverOnly)
	}
	var latest time.Time
	for _, msg := range w.messages[conversation] {
		if msg.Kind != "message" && msg.Kind != "action" && msg.Kind != "notice" {
			continue
		}
		if !through.IsZero() && msg.CreatedAt.After(through) || serverOnly && (msg.UpstreamID == "" || strings.HasPrefix(msg.UpstreamID, "_")) {
			continue
		}
		if msg.CreatedAt.After(latest) {
			latest = msg.CreatedAt
		}
	}
	return latest, nil
}

func (w *workspace) readTime(conversation string) (time.Time, error) {
	if w.store != nil {
		return w.store.ReadTime(w.ownerID, conversation)
	}
	return w.readTimes[conversation], nil
}

// applyReadTime advances the monotonic read position. Reading further than before
// also ends a manual unread boundary: the conversation was read past it.
func (w *workspace) applyReadTime(conv Conversation, at time.Time) (Conversation, error) {
	previous, err := w.readTime(conv.ID)
	if err != nil {
		return conv, err
	}
	if at.After(previous) {
		if err = w.clearUnread(conv.ID); err != nil {
			return conv, err
		}
	} else {
		at = previous
	}
	if !at.IsZero() {
		if w.store != nil {
			if err = w.store.MarkRead(w.ownerID, conv.ID, at); err != nil {
				return conv, err
			}
		} else {
			if w.readTimes == nil {
				w.readTimes = map[string]time.Time{}
			}
			w.readTimes[conv.ID] = at
		}
	}
	return w.countUnread(conv)
}

func (w *workspace) clearUnread(conversation string) error {
	if w.store != nil {
		return w.store.ClearUnread(w.ownerID, conversation)
	}
	delete(w.unreadFrom, conversation)
	return nil
}

// refreshUnread recounts from storage, keeping the cached counts when that fails.
func (w *workspace) refreshUnread(conv Conversation) Conversation {
	if v, err := w.countUnread(conv); err == nil {
		return v
	}
	return conv
}

// emitCounts publishes a conversation's unread state without the rest of it.
func (w *workspace) emitCounts(conv Conversation) {
	w.emit("conversation.counts", model.ConversationCounts{ID: conv.ID, UnreadCount: conv.UnreadCount, MentionCount: conv.MentionCount, UnreadFrom: conv.UnreadFrom})
}

// recount refreshes a conversation's counts from storage and publishes them.
func (w *workspace) recount(id string) {
	conv, ok := w.conversations[id]
	if !ok {
		return
	}
	conv = w.refreshUnread(conv)
	w.conversations[id] = conv
	w.emitCounts(conv)
}

// countUnread sets the unread and mention counts and any manual unread boundary.
func (w *workspace) countUnread(conv Conversation) (Conversation, error) {
	var err error
	conv.UnreadCount, conv.MentionCount, conv.UnreadFrom = 0, 0, ""
	if w.store != nil {
		if conv.UnreadCount, err = w.store.Unread(w.ownerID, conv.ID); err != nil {
			return conv, err
		}
		if conv.MentionCount, err = w.store.Mentions(w.ownerID, conv.ID); err != nil {
			return conv, err
		}
		conv.UnreadFrom, err = w.store.UnreadFrom(w.ownerID, conv.ID)
		return conv, err
	}
	at := w.readTimes[conv.ID]
	var from time.Time
	if id := w.unreadFrom[conv.ID]; id != "" {
		for _, msg := range w.messages[conv.ID] {
			if msg.ID == id {
				from, conv.UnreadFrom = msg.CreatedAt, id
			}
		}
	}
	for _, msg := range w.messages[conv.ID] {
		if msg.Kind != "message" && msg.Kind != "action" && msg.Kind != "notice" {
			continue
		}
		if conv.UnreadFrom != "" {
			if msg.CreatedAt.Before(from) || msg.SenderID == w.self.ID && msg.ID != conv.UnreadFrom {
				continue
			}
		} else if msg.SenderID == w.self.ID || !msg.CreatedAt.After(at) {
			continue
		}
		member := w.members[msg.SenderID]
		if w.policy.Ignored(member.Nick, member.Username, member.Host) {
			continue
		}
		conv.UnreadCount++
		if msg.Mention && msg.SenderID != w.self.ID {
			conv.MentionCount++
		}
	}
	return conv, nil
}

// MarkUnread starts the conversation's unread messages at one of its messages. Only
// IRCord keeps this boundary: read markers are monotonic upstream, so nothing is sent.
func (m *Manager) MarkUnread(owner, conversation, message string) (Conversation, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	v, ok := w.conversations[conversation]
	if !ok || w.deleted {
		return Conversation{}, ErrNotFound
	}
	found := false
	if w.store != nil {
		var err error
		if found, err = w.store.MarkUnread(owner, conversation, message); err != nil {
			return Conversation{}, ErrStorage
		}
	} else {
		for _, msg := range w.messages[conversation] {
			found = found || msg.ID == message && (msg.Kind == "message" || msg.Kind == "action" || msg.Kind == "notice")
		}
		if found {
			if w.unreadFrom == nil {
				w.unreadFrom = map[string]string{}
			}
			w.unreadFrom[conversation] = message
		}
	}
	if !found {
		return Conversation{}, ErrNotFound
	}
	v, err := w.countUnread(v)
	if err != nil {
		return Conversation{}, ErrStorage
	}
	w.conversations[conversation] = v
	w.emitCounts(v)
	return clone(v), nil
}

func (c *connection) requestReadMarker(target string) {
	if c.hasAll("draft/read-marker", "server-time") {
		_ = c.enqueue(&girc.Event{Command: "MARKREAD", Params: []string{target}})
	}
}

func (c *connection) sendReadMarker(target string, at time.Time) {
	if !at.IsZero() && c.hasAll("draft/read-marker", "server-time") {
		_ = c.enqueue(&girc.Event{Command: "MARKREAD", Params: []string{target, "timestamp=" + at.UTC().Format(ircTimeLayout)}})
	}
}

func (c *connection) receiveReadMarker(e girc.Event) {
	if !c.hasAll("draft/read-marker", "server-time") || len(e.Params) != 2 {
		return
	}
	var at time.Time
	if e.Params[1] != "*" {
		var err error
		at, err = time.Parse(time.RFC3339Nano, strings.TrimPrefix(e.Params[1], "timestamp="))
		if err != nil || !strings.HasPrefix(e.Params[1], "timestamp=") {
			return
		}
	}
	w := c.owner
	w.mu.Lock()
	key := c.conversationID(e.Params[0])
	if c.readMarkersSeen == nil {
		c.readMarkersSeen = map[string]bool{}
	}
	first := !c.readMarkersSeen[key]
	c.mu.Lock()
	ready := c.ready
	c.mu.Unlock()
	// Ergo may replay a marker before CONNECTED. Reconciliation cannot be sent
	// yet; leave the first ready reply eligible instead of consuming it here.
	if ready {
		c.readMarkersSeen[key] = true
	}
	local, err := w.readTime(key)
	if err != nil {
		w.storageErr = err
		w.mu.Unlock()
		return
	}
	var reconcile time.Time
	// A read performed while disconnected may be ahead of the initial server
	// marker. Publish only a known message time, once per target/connection, so
	// clamped or rejected server timestamps cannot cause an echo loop.
	if first && ready && local.After(at) {
		reconcile, err = w.latestReadMessage(key, local, true)
		if err != nil {
			w.storageErr = err
			w.mu.Unlock()
			return
		}
	}
	if conv, ok := w.conversations[key]; ok {
		conv, err = w.applyReadTime(conv, at)
		if err == nil {
			w.conversations[key] = conv
			w.emitCounts(conv)
		}
	} else if w.store != nil && !at.IsZero() {
		err = w.store.MarkRead(w.ownerID, key, at)
	}
	if err != nil {
		w.storageErr = err
	}
	w.mu.Unlock()
	if reconcile.After(at) {
		c.sendReadMarker(e.Params[0], reconcile)
	}
}
