package irc

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

type typingSent struct {
	ActiveAt, UpdatedAt time.Time
	State               model.TypingState
}

// SubscribeTyping has a separate lossy queue: ephemeral activity must never
// consume durable replay space or disconnect a client waiting on real messages.
func (m *Manager) SubscribeTyping(owner string) (<-chan model.TypingEvent, func()) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	ch := make(chan model.TypingEvent, 16)
	if w.deleted || len(w.typingSubscribers) >= 16 {
		close(ch)
		return ch, func() {}
	}
	if w.typingSubscribers == nil {
		w.typingSubscribers = map[chan model.TypingEvent]struct{}{}
	}
	w.typingSubscribers[ch] = struct{}{}
	var once sync.Once
	return ch, func() {
		once.Do(func() { w.mu.Lock(); defer w.mu.Unlock(); delete(w.typingSubscribers, ch); close(ch) })
	}
}

func (w *workspace) publishTyping(data model.TypingData) {
	if w.deleted || w.storageErr != nil {
		return
	}
	event := model.TypingEvent{V: 1, Type: "typing", Data: data}
	for ch := range w.typingSubscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func tagAllowed(policy, name string) bool {
	allowed := true
	for _, rule := range strings.Split(policy, ",") {
		permit := strings.HasPrefix(rule, "-")
		pattern := strings.TrimPrefix(rule, "-")
		if pattern == "*" || pattern == name {
			allowed = permit
		}
	}
	return allowed
}

func (m *Manager) SetTyping(owner, conversation string, state model.TypingState) error {
	if !state.Valid() {
		return fmt.Errorf("invalid typing state")
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, exists := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	speaking := canSpeak(conv, w.networks[conv.NetworkID].ISupport, w.self.ID)
	supported := false
	if c != nil {
		c.mu.Lock()
		supported = c.features(w.networks[conv.NetworkID].ISupport).Typing
		c.mu.Unlock()
	}
	deleted, storageErr := w.deleted, w.storageErr
	w.mu.Unlock()
	if !exists || deleted {
		return ErrNotFound
	}
	if storageErr != nil {
		return ErrStorage
	}
	if !speaking {
		return nil
	}
	if c == nil || conv.Kind == "channel" && !conv.Joined {
		return ErrDisconnected
	}
	c.mu.Lock()
	if !c.ready {
		c.mu.Unlock()
		return ErrDisconnected
	}
	c.mu.Unlock()
	if !supported {
		return nil
	}
	now := time.Now()
	c.mu.Lock()
	if c.typingSent == nil {
		c.typingSent = map[string]typingSent{}
	}
	for id, previous := range c.typingSent {
		if now.Sub(previous.UpdatedAt) > 30*time.Second {
			delete(c.typingSent, id)
		}
	}
	previous := c.typingSent[conversation]
	if state == previous.State && state != model.TypingActive || state == model.TypingActive && now.Sub(previous.ActiveAt) < 3*time.Second {
		c.mu.Unlock()
		return nil
	}
	// Bound transitions as well as active refreshes; a malicious socket must not
	// turn alternating paused/done frames into an IRC command flood.
	if now.Sub(c.typingWindow) >= time.Second {
		c.typingWindow, c.typingBurst = now, 0
	}
	if c.typingBurst >= 8 {
		c.mu.Unlock()
		return nil
	}
	c.typingBurst++
	if state == model.TypingActive {
		previous.ActiveAt = now
	}
	previous.State, previous.UpdatedAt = state, now
	c.typingSent[conversation] = previous
	c.mu.Unlock()
	if err := c.enqueue(&girc.Event{Command: "TAGMSG", Params: []string{c.target(conv)}, Tags: girc.Tags{"+typing": string(state)}}); err != nil {
		return err
	}
	w.mu.Lock()
	w.publishTyping(model.TypingData{ConversationID: conv.ID, MemberID: w.self.ID, State: state})
	w.mu.Unlock()
	return nil
}

func (c *connection) typing(e girc.Event) {
	state := model.TypingState(e.Tags["+typing"])
	if state == "" {
		state = model.TypingState(e.Tags["+draft/typing"])
	}
	if state == "" {
		state = model.TypingState(e.Tags["draft/typing"])
	}
	if !state.Valid() || e.Source == nil || len(e.Params) != 1 {
		return
	}
	c.mu.Lock()
	historical := c.historyBatches[e.Tags["batch"]]
	c.mu.Unlock()
	if historical || c.fold(e.Source.Name) == c.fold(c.currentNick()) {
		return
	}
	target := e.Params[0]
	if !girc.IsValidChannel(target) {
		if c.fold(target) != c.fold(c.currentNick()) {
			return
		}
		target = e.Source.Name
	}
	conversation := c.conversationID(target)
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	conv, ok := w.conversations[conversation]
	member, known := w.members[c.nicks[c.fold(e.Source.Name)]]
	if !ok || !known || conv.Kind == "channel" && !conv.Joined {
		return
	}
	if conv.Kind == "channel" && !slices.Contains(conv.MemberIDs, member.ID) {
		return
	}
	user, host := e.Source.Ident, e.Source.Host
	if user == "" {
		user = member.Username
	}
	if host == "" {
		host = member.Host
	}
	if w.policy.Ignored(member.Nick, user, host) {
		return
	}
	// Coalesce repeated remote activity, including servers that do not enforce
	// the typing draft's three-second active interval. State is bounded and never
	// written to disk or replayed.
	now := time.Now()
	key := conversation + ":" + member.ID
	c.mu.Lock()
	if c.typingReceived == nil {
		c.typingReceived = map[string]typingSent{}
	}
	previous := c.typingReceived[key]
	if previous.State == state && now.Sub(previous.UpdatedAt) < 3*time.Second {
		c.mu.Unlock()
		return
	}
	if len(c.typingReceived) >= 4096 {
		for id, entry := range c.typingReceived {
			if now.Sub(entry.UpdatedAt) > 30*time.Second {
				delete(c.typingReceived, id)
			}
		}
		if _, exists := c.typingReceived[key]; !exists && len(c.typingReceived) >= 4096 {
			c.mu.Unlock()
			return
		}
	}
	c.typingReceived[key] = typingSent{State: state, UpdatedAt: now}
	c.mu.Unlock()
	w.publishTyping(model.TypingData{ConversationID: conv.ID, MemberID: member.ID, State: state})
}
