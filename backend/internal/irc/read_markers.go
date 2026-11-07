package irc

import (
	"strings"
	"time"

	"github.com/lrstanley/girc"
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

func (w *workspace) applyReadTime(conv Conversation, at time.Time) (Conversation, error) {
	previous, err := w.readTime(conv.ID)
	if err != nil {
		return conv, err
	}
	if at.Before(previous) {
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
	conv.UnreadCount, conv.MentionCount = 0, 0
	if w.store != nil {
		conv.UnreadCount, err = w.store.Unread(w.ownerID, conv.ID)
		if err == nil {
			conv.MentionCount, err = w.store.Mentions(w.ownerID, conv.ID)
		}
		return conv, err
	}
	for _, msg := range w.messages[conv.ID] {
		if msg.SenderID == w.self.ID || !msg.CreatedAt.After(at) || (msg.Kind != "message" && msg.Kind != "action" && msg.Kind != "notice") {
			continue
		}
		member := w.members[msg.SenderID]
		if w.policy.Ignored(member.Nick, member.Username, member.Host) {
			continue
		}
		conv.UnreadCount++
		if msg.Mention {
			conv.MentionCount++
		}
	}
	return conv, nil
}

func (c *connection) requestReadMarker(target string) {
	if c.has("draft/read-marker") {
		_ = c.enqueue(&girc.Event{Command: "MARKREAD", Params: []string{target}})
	}
}

func (c *connection) sendReadMarker(target string, at time.Time) {
	if !at.IsZero() && c.has("draft/read-marker") && c.has("server-time") {
		_ = c.enqueue(&girc.Event{Command: "MARKREAD", Params: []string{target, "timestamp=" + at.UTC().Format(ircTimeLayout)}})
	}
}

func (c *connection) receiveReadMarker(e girc.Event) {
	if !c.has("draft/read-marker") || len(e.Params) != 2 {
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
	c.readMarkersSeen[key] = true
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
	if first && local.After(at) {
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
			w.emit("conversation.updated", conv)
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
