package irc

import (
	"strings"
	"time"

	"github.com/lrstanley/girc"

	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func messageBoundary(msg Message) storage.ReadBoundary {
	return storage.ReadBoundary{At: msg.CreatedAt, Position: msg.Position, ID: msg.ID}
}

func compareBoundary(a, b storage.ReadBoundary) int {
	if a.At.Before(b.At) {
		return -1
	}
	if a.At.After(b.At) {
		return 1
	}
	if a.Position < b.Position {
		return -1
	}
	if a.Position > b.Position {
		return 1
	}
	return strings.Compare(a.ID, b.ID)
}

func eligibleReadMessage(msg Message) bool {
	return msg.Kind == "message" || msg.Kind == "action" || msg.Kind == "notice"
}

func (w *workspace) advanceMemoryRead(conversation string, boundary storage.ReadBoundary) bool {
	previous, exact := w.readBoundaries[conversation]
	if exact && compareBoundary(boundary, previous) <= 0 {
		return false
	}
	if !exact && !boundary.At.After(w.readTimes[conversation]) {
		return false
	}
	if w.readBoundaries == nil {
		w.readBoundaries = map[string]storage.ReadBoundary{}
	}
	if w.readTimes == nil {
		w.readTimes = map[string]time.Time{}
	}
	w.readBoundaries[conversation] = boundary
	w.readTimes[conversation] = boundary.At
	return true
}

func (w *workspace) readTime(conversation string) (time.Time, error) {
	if w.store != nil {
		return w.store.ReadTime(w.ownerID, conversation)
	}
	return w.readTimes[conversation], nil
}

// applyReadTime applies an upstream timestamp without clearing local manual unread.
func (w *workspace) applyReadTime(conv Conversation, at time.Time) (Conversation, error) {
	previous, err := w.readTime(conv.ID)
	if err != nil {
		return conv, err
	}
	if !at.After(previous) {
		at = previous
	} else if w.store == nil {
		delete(w.readBoundaries, conv.ID)
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
	delete(w.unreadBoundaries, conversation)
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
	conv.UnreadFrom = w.unreadFrom[conv.ID]
	for _, msg := range w.messages[conv.ID] {
		if !eligibleReadMessage(msg) {
			continue
		}
		if !w.memoryMessageUnread(conv.ID, msg) {
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

func (w *workspace) memoryMessageUnread(conversation string, msg Message) bool {
	if id := w.unreadFrom[conversation]; id != "" {
		boundary, ok := w.unreadBoundaries[conversation]
		return ok && compareBoundary(messageBoundary(msg), boundary) >= 0 && (msg.SenderID != w.self.ID || msg.ID == id)
	}
	if msg.SenderID == w.self.ID {
		return false
	}
	if boundary, ok := w.readBoundaries[conversation]; ok {
		return compareBoundary(messageBoundary(msg), boundary) > 0
	}
	return msg.CreatedAt.After(w.readTimes[conversation])
}

func (w *workspace) safeReadMarker(conversation string) (time.Time, error) {
	if w.store != nil {
		return w.store.SafeReadMarker(w.ownerID, conversation)
	}
	var latest, earliestUnread time.Time
	hasUnread := false
	for _, msg := range w.messages[conversation] {
		member := w.members[msg.SenderID]
		if eligibleReadMessage(msg) && !w.policy.Ignored(member.Nick, member.Username, member.Host) && w.memoryMessageUnread(conversation, msg) {
			wire := msg.CreatedAt.Truncate(time.Millisecond)
			if !hasUnread || wire.Before(earliestUnread) {
				earliestUnread = wire
				hasUnread = true
			}
		}
	}
	for _, candidate := range w.messages[conversation] {
		if !eligibleReadMessage(candidate) || candidate.UpstreamID == "" || strings.HasPrefix(candidate.UpstreamID, "_") {
			continue
		}
		if boundary, ok := w.readBoundaries[conversation]; ok {
			if compareBoundary(messageBoundary(candidate), boundary) > 0 {
				continue
			}
		} else if candidate.CreatedAt.After(w.readTimes[conversation]) {
			continue
		}
		if (!hasUnread || candidate.CreatedAt.Truncate(time.Millisecond).Before(earliestUnread)) && candidate.CreatedAt.After(latest) {
			latest = candidate.CreatedAt
		}
	}
	return latest, nil
}

// MarkUnread starts the conversation's unread messages at one of its messages. Only
// IRCord keeps this boundary: read markers are monotonic upstream, so nothing is sent.
func (m *Manager) MarkUnread(owner, conversation, message string) (Conversation, error) {
	if e := m.partyAccess(owner, conversation); e != nil {
		return Conversation{}, e
	}
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
			if msg.ID == message && eligibleReadMessage(msg) {
				found = true
				if w.unreadBoundaries == nil {
					w.unreadBoundaries = map[string]storage.ReadBoundary{}
				}
				w.unreadBoundaries[conversation] = messageBoundary(msg)
			}
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
		reconcile, err = w.safeReadMarker(key)
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
