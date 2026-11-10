package irc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

func (c *connection) conversation(target string) Conversation {
	w := c.owner
	key := c.conversationID(target)
	if existing, ok := w.conversations[key]; ok {
		return existing
	}
	kind := "direct"
	name := target
	if girc.IsValidChannel(target) {
		kind = "channel"
		name = target[1:]
	}
	conv := Conversation{ID: key, NetworkID: c.config.ID, Kind: kind, Name: name, MemberIDs: []string{w.self.ID}}
	if kind == "channel" {
		conv.Target = target
	}
	w.conversations[key] = conv
	w.messages[key] = []Message{}
	w.emit("conversation.updated", conv)
	if kind == "direct" {
		c.requestReadMarker(target)
	}
	return conv
}

func (c *connection) target(conv Conversation) string {
	if conv.Kind == "channel" {
		if conv.Target != "" {
			return conv.Target
		}
		return "#" + conv.Name
	}
	return conv.Name
}

func (c *connection) conversationID(target string) string {
	digest := sha256.Sum256([]byte(c.fold(target)))
	return c.config.ID + "_" + hex.EncodeToString(digest[:12])
}

func (m *Manager) Join(owner, network, target string) (Conversation, error) {
	return m.JoinKey(owner, network, target, "")
}

func (m *Manager) JoinKey(owner, network, target, key string) (Conversation, error) {
	c, err := m.connection(owner, network)
	if err != nil {
		return Conversation{}, err
	}
	if target != "" && !strings.ContainsRune("#&+!*~", rune(target[0])) {
		target = "#" + target
	}
	if !girc.IsValidChannel(target) || !clean(target, 100) || strings.ContainsAny(target, " ,:") {
		return Conversation{}, fmt.Errorf("invalid channel")
	}
	if owner != partyControlOwner && c.managedPartyTarget(target) {
		return Conversation{}, ErrForbidden
	}
	c.mu.Lock()
	if !clean(key, 100) || strings.ContainsAny(key, " ,:") {
		c.mu.Unlock()
		return Conversation{}, fmt.Errorf("invalid channel key")
	}
	entry := target
	if key != "" {
		entry += " " + key
	}
	c.mu.Unlock()
	c.owner.mu.Lock()
	joined := c.owner.conversations[c.conversationID(target)].Joined
	c.owner.mu.Unlock()
	c.mu.Lock()
	if !joined {
		c.arriveUnlocked(c.foldUnlocked(target))
	}
	c.desired[c.foldUnlocked(target)] = entry
	delete(c.closedTargets, c.foldUnlocked(target))
	c.mu.Unlock()
	if err := c.saveTargets(); err != nil {
		return Conversation{}, ErrStorage
	}
	if c.owner.store != nil {
		c.owner.mu.Lock()
		conv := c.conversation(target)
		c.owner.mu.Unlock()
		if err = c.owner.store.BeginGap(owner, conv.ID); err != nil {
			return Conversation{}, ErrStorage
		}
	}
	c.markCreating(target)
	if err = c.enqueue(&girc.Event{Command: "JOIN", Params: strings.Fields(entry)}); err != nil {
		return Conversation{}, err
	}
	c.owner.mu.Lock()
	defer c.owner.mu.Unlock()
	return clone(c.conversation(target)), nil
}

func (m *Manager) Direct(owner, network, nick string) (Conversation, error) {
	c, err := m.connection(owner, network)
	if err != nil {
		return Conversation{}, err
	}
	if !girc.IsValidNick(nick) {
		return Conversation{}, fmt.Errorf("invalid nickname")
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	v := c.conversation(nick)
	member := c.member(nick, "")
	v.MemberIDs = []string{w.self.ID, member.ID}
	w.conversations[v.ID] = v
	w.emit("conversation.updated", v)
	return clone(v), nil
}

func (m *Manager) Part(owner, conversation string) error {
	return m.PartReason(owner, conversation, "")
}

func (m *Manager) PartReason(owner, conversation, reason string) error {
	if !clean(reason, 300) {
		return fmt.Errorf("invalid part reason")
	}
	w := m.space(owner)
	w.mu.Lock()
	v, ok := w.conversations[conversation]
	c := w.connections[v.NetworkID]
	w.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	if v.Party != nil {
		return ErrForbidden
	}
	if c != nil {
		target := c.target(v)
		// The server's PART echo is dropped as a closed target, so forget the roster here.
		folded := c.fold(target)
		c.mu.Lock()
		delete(c.roster, folded)
		delete(c.desired, c.foldUnlocked(target))
		delete(c.arrivals, c.foldUnlocked(target))
		c.closedTargets[c.foldUnlocked(target)] = true
		c.mu.Unlock()
		if err := c.saveTargets(); err != nil {
			return ErrStorage
		}
		if v.Kind == "channel" {
			if err := c.enqueue(&girc.Event{Command: "PART", Params: []string{target, reason}}); err != nil {
				return err
			}
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emit("conversation.removed", model.Removal{ID: conversation})
	if w.storageErr != nil {
		return ErrStorage
	}
	delete(w.conversations, conversation)
	delete(w.messages, conversation)
	return nil
}

func (m *Manager) UpdateConversation(owner, conversation string, read *bool, favorite *bool) (Conversation, error) {
	if e := m.partyAccess(owner, conversation); e != nil {
		return Conversation{}, e
	}
	w := m.space(owner)
	w.mu.Lock()
	v, ok := w.conversations[conversation]
	if !ok || w.deleted {
		w.mu.Unlock()
		return Conversation{}, ErrNotFound
	}
	var readAt time.Time
	if read != nil && *read {
		// Reading explicitly also ends a manual unread boundary at the current position.
		err := w.clearUnread(conversation)
		var at time.Time
		if err == nil {
			at, err = w.latestReadMessage(conversation, time.Time{}, false)
		}
		if err == nil {
			v, err = w.applyReadTime(v, at)
		}
		if err == nil {
			readAt, err = w.latestReadMessage(conversation, at, true)
		}
		if err != nil {
			w.mu.Unlock()
			return Conversation{}, ErrStorage
		}
	}
	if favorite != nil {
		v.Favorite = *favorite
	}
	w.conversations[v.ID] = v
	// Neither field set (a mark-unread or preview-mode PATCH) changes nothing here;
	// those paths emit their own event.
	if favorite != nil {
		w.emit("conversation.updated", v)
	} else if read != nil {
		w.emitCounts(v)
	}
	if w.storageErr != nil {
		w.mu.Unlock()
		return Conversation{}, ErrStorage
	}
	c := w.connections[v.NetworkID]
	w.mu.Unlock()
	if c != nil {
		c.sendReadMarker(c.target(v), readAt)
	}
	return clone(v), nil
}

func (m *Manager) SetTopic(owner, conversation, topic string) error {
	if !clean(topic, 300) {
		return fmt.Errorf("invalid topic")
	}
	w := m.space(owner)
	w.mu.Lock()
	v, ok := w.conversations[conversation]
	c := w.connections[v.NetworkID]
	w.mu.Unlock()
	if !ok || c == nil {
		return ErrNotFound
	}
	event := &girc.Event{Command: "TOPIC", Params: []string{c.target(v), topic}}
	if err := c.validateChannelCommand(event); err != nil {
		return err
	}
	return c.enqueue(event)
}

func (c *connection) receiveConversation(e girc.Event) {
	switch e.Command {
	case "333", "329":
		if len(e.Params) >= 3 {
			w := c.owner
			w.mu.Lock()
			v := c.conversation(e.Params[1])
			index := 2
			if e.Command == "333" && len(e.Params) > 3 {
				v.TopicSetBy = e.Params[2]
				index = 3
			}
			if seconds, err := strconv.ParseInt(e.Params[index], 10, 64); err == nil {
				at := time.Unix(seconds, 0).UTC()
				if e.Command == "333" {
					v.TopicSetAt = &at
				} else {
					v.CreatedAt = &at
				}
			}
			w.conversations[v.ID] = v
			w.emit("conversation.updated", v)
			w.mu.Unlock()
		}
	case "328":
		if len(e.Params) >= 3 {
			w := c.owner
			w.mu.Lock()
			v := c.conversation(e.Params[1])
			v.URL = e.Last()
			w.conversations[v.ID] = v
			w.emit("conversation.updated", v)
			w.mu.Unlock()
		}
	case "TOPIC", "332":
		if len(e.Params) >= 2 {
			target := e.Params[0]
			if e.Command == "332" && len(e.Params) >= 3 {
				target = e.Params[1]
			}
			w := c.owner
			w.mu.Lock()
			v := c.conversation(target)
			v.Topic = e.Last()
			if e.Command == "TOPIC" && e.Source != nil {
				v.TopicSetBy = e.Source.String()
				at := time.Now().UTC()
				v.TopicSetAt = &at
			}
			w.conversations[v.ID] = v
			w.emit("conversation.updated", v)
			w.mu.Unlock()
		}
	}
}

func (c *connection) closedEvent(e girc.Event) bool {
	index := 0
	switch e.Command {
	case "PRIVMSG", "NOTICE", "TAGMSG", "REDACT", "JOIN", "PART", "KICK", "TOPIC", "MODE":
	case "332", "333", "329", "328", "324":
		index = 1
	default:
		return false
	}
	if len(e.Params) <= index || !girc.IsValidChannel(e.Params[index]) {
		return false
	}
	if c.owner.ownerID != partyControlOwner && c.managedPartyTarget(e.Params[index]) && c.partyTargetAccess(e.Params[index]) != nil {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closedTargets[c.foldUnlocked(e.Params[index])]
}
