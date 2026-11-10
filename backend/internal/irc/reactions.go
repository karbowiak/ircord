package irc

import (
	"fmt"
	"slices"
	"time"

	"github.com/lrstanley/girc"
)

func (c *connection) reaction(e girc.Event) {
	if e.Source == nil || len(e.Params) == 0 {
		return
	}
	reply := e.Tags["+reply"]
	if reply == "" {
		reply = e.Tags["+draft/reply"]
	}
	emoji, add := e.Tags["+draft/react"]
	if !add {
		emoji = e.Tags["+draft/unreact"]
	}
	if reply == "" || emoji == "" || len(emoji) > 64 {
		return
	}
	target := e.Params[0]
	if !girc.IsValidChannel(target) && c.fold(target) == c.fold(c.currentNick()) {
		target = e.Source.Name
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	conv := c.conversation(target)
	member := c.member(e.Source.Name, e.Tags["account"])
	if conv.Party != nil && senderAccount(e) == "" {
		return
	}
	if w.ownerID == partyControlOwner {
		party, err := c.manager.options.Store.PartyByConversation(conv.ID)
		if err != nil {
			return
		}
		at, err := time.Parse(time.RFC3339Nano, e.Tags["time"])
		if err != nil {
			return
		}
		c.mu.Lock()
		historical := c.historyBatches[e.Tags["batch"]]
		c.mu.Unlock()
		if err = c.manager.options.Store.SetPartyReaction(party.ID, reply, member.ID, emoji, add, at, !historical); err != nil {
			w.storageErr = err
			return
		}
		msg, ok := c.lookupMessage(conv.ID, reply)
		if !ok || msg.Deleted {
			return
		}
		msg.Reactions, err = c.manager.options.Store.PartyReactions(party.ID, reply)
		if err != nil {
			w.storageErr = err
			return
		}
		for i, old := range w.messages[conv.ID] {
			if old.ID == msg.ID {
				w.messages[conv.ID][i] = msg
			}
		}
		w.emit("message.updated", msg)
		return
	}
	if w.store != nil {
		at, err := time.Parse(time.RFC3339Nano, e.Tags["time"])
		if err != nil {
			at = time.Now().UTC()
		}
		c.mu.Lock()
		historical := c.historyBatches[e.Tags["batch"]]
		c.mu.Unlock()
		if err = w.store.SetReaction(w.ownerID, conv.ID, reply, member.ID, emoji, add, at, !historical); err != nil {
			w.storageErr = err
			return
		}
		msg, ok := c.lookupMessage(conv.ID, reply)
		if !ok || msg.Deleted {
			return
		}
		before := clone(msg)
		msg.Reactions, err = w.store.Reactions(w.ownerID, conv.ID, reply)
		if err != nil {
			w.storageErr = err
			return
		}
		for i, old := range w.messages[conv.ID] {
			if old.ID == msg.ID {
				w.messages[conv.ID][i] = msg
			}
		}
		if !equal(before, msg) {
			if historical {
				if err = w.store.SaveHistory(w.ownerID, msg); err != nil {
					w.storageErr = err
				}
			} else {
				w.emit("message.updated", msg)
			}
		}
		return
	}
	for i, msg := range w.messages[conv.ID] {
		if msg.UpstreamID != reply {
			continue
		}
		before := clone(msg)
		found := false
		for j, r := range msg.Reactions {
			if r.Emoji != emoji {
				continue
			}
			found = true
			if add && !slices.Contains(r.MemberIDs, member.ID) {
				r.MemberIDs = append(r.MemberIDs, member.ID)
			}
			if !add {
				r.MemberIDs = slices.DeleteFunc(r.MemberIDs, func(id string) bool { return id == member.ID })
			}
			msg.Reactions[j] = r
		}
		if add && !found {
			msg.Reactions = append(msg.Reactions, Reaction{Emoji: emoji, MemberIDs: []string{member.ID}})
		}
		msg.Reactions = slices.DeleteFunc(msg.Reactions, func(r Reaction) bool { return len(r.MemberIDs) == 0 })
		w.messages[conv.ID][i] = msg
		if !equal(before, msg) {
			w.emit("message.updated", msg)
		}
		return
	}
}

func (m *Manager) ToggleReaction(owner, conversation, messageID, emoji string) (Message, error) {
	if e := m.partyAccess(owner, conversation); e != nil {
		return Message{}, e
	}
	if emoji == "" || !clean(emoji, 64) {
		return Message{}, fmt.Errorf("invalid reaction")
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	if !ok || c == nil {
		w.mu.Unlock()
		return Message{}, ErrNotFound
	}
	var target Message
	if w.store != nil {
		target, _ = w.store.Message(owner, conversation, messageID)
	}
	for _, msg := range w.messages[conversation] {
		if msg.ID == messageID {
			target = clone(msg)
		}
	}
	if !canSpeak(conv, w.networks[conv.NetworkID].ISupport, w.self.ID) {
		w.mu.Unlock()
		return Message{}, fmt.Errorf("this channel is moderated; voice or operator privileges are required")
	}
	selfID := w.self.ID
	c.mu.Lock()
	features := c.features(w.networks[conv.NetworkID].ISupport)
	ready := c.ready
	c.mu.Unlock()
	w.mu.Unlock()
	if !ready || conv.Kind == "channel" && !conv.Joined {
		return Message{}, ErrDisconnected
	}
	if target.ID == "" {
		return Message{}, ErrNotFound
	}
	if !addressable(target) || !features.Reactions {
		return Message{}, ErrUnsupported
	}
	tag := "+draft/react"
	for _, r := range target.Reactions {
		if r.Emoji == emoji && slices.Contains(r.MemberIDs, selfID) {
			tag = "+draft/unreact"
		}
	}
	if err := c.enqueue(&girc.Event{Command: "TAGMSG", Params: []string{c.target(conv)}, Tags: girc.Tags{"+reply": target.UpstreamID, tag: emoji}}); err != nil {
		return Message{}, err
	}
	// Tag-only echoes are not guaranteed; apply the idempotent set operation now.
	c.reaction(girc.Event{Source: &girc.Source{Name: c.currentNick()}, Params: []string{c.target(conv)}, Tags: girc.Tags{"+reply": target.UpstreamID, tag: emoji, "account": c.config.SASLUser}})
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, msg := range w.messages[conversation] {
		if msg.ID == messageID {
			return clone(msg), nil
		}
	}
	if w.store != nil {
		return w.store.Message(owner, conversation, messageID)
	}
	return Message{}, ErrNotFound
}
