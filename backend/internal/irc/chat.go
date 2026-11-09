package irc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

type pending struct {
	input     SendInput
	messageID string
	at        time.Time
}

func validAttachment(a model.Attachment) bool {
	u, err := url.Parse(a.URL)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && clean(a.Name, 200) && len(a.URL) < 2048 && a.Width >= 0 && a.Height >= 0 && a.Width <= 16384 && a.Height <= 16384 && a.Size >= 0 && a.Size <= 100<<20
}

func (c *connection) message(e girc.Event) { c.messageWithHistory(e, false) }

func (c *connection) messageWithHistory(e girc.Event, forceHistory bool, refs ...string) {
	if e.Source == nil || len(e.Params) < 2 {
		return
	}
	c.mu.Lock()
	historical := forceHistory || c.historyBatches[e.Tags["batch"]]
	ref := e.Tags["batch"]
	if len(refs) > 0 {
		ref = refs[0]
	}
	position := int64(0)
	req := c.historyResponses[ref]
	if req != nil && req.sub == "BEFORE" && req.orderBase != 0 {
		position = req.orderBase + int64(req.result.Count)
	}
	// Catch-up after a disconnect or restart (CHATHISTORY AFTER the stored
	// watermark) recovers messages that were missed, unlike scrolling back
	// (BEFORE) or a new conversation's first fill (LATEST).
	recovered := historical && req != nil && req.sub == "AFTER"
	c.mu.Unlock()
	target := e.Params[0]
	if !girc.IsValidChannel(target) && c.fold(target) == c.fold(c.currentNick()) {
		target = e.Source.Name
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	conv := c.conversation(target)
	upstream := e.Tags["msgid"]
	messageID := id()
	if upstream != "" {
		messageID = c.config.ID + ":" + upstream
	}
	lookupKey := messageID
	if upstream != "" {
		lookupKey = upstream
	}
	existing, known := c.lookupMessage(conv.ID, lookupKey)
	if known && existing.ClientID != "" {
		return
	}
	sender := c.member(e.Source.Name, e.Tags["account"])
	if e.Source.Ident != "" && (sender.Username != e.Source.Ident || sender.Host != e.Source.Host) {
		sender.Username = e.Source.Ident
		sender.Host = e.Source.Host
		w.members[sender.ID] = sender
		w.emit("member.updated", sender)
	}
	at := e.Timestamp
	if parsed, err := time.Parse(time.RFC3339Nano, e.Tags["time"]); err == nil {
		at = parsed
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	msg := Message{Position: position, ID: messageID, ConversationID: conv.ID, SenderID: sender.ID, Text: e.Last(), CreatedAt: at, Kind: "message", Reactions: []Reaction{}, UpstreamID: upstream}
	if encoded := e.Tags["+ircord/gif"]; len(encoded) > 0 && len(encoded) <= 2500 {
		if raw, err := base64.RawURLEncoding.DecodeString(encoded); err == nil {
			var gif GIF
			if json.Unmarshal(raw, &gif) == nil && validGIF(&gif) && gif.URL == msg.Text {
				msg.GIF = &gif
			}
		}
	}
	if strings.HasPrefix(msg.Text, "\x01ACTION ") && strings.HasSuffix(msg.Text, "\x01") {
		msg.Kind = "action"
		msg.Text = strings.TrimSuffix(strings.TrimPrefix(msg.Text, "\x01ACTION "), "\x01")
	}
	if encoded := e.Tags["+ircord/attachment"]; len(encoded) > 0 && len(encoded) <= 1800 {
		if raw, err := base64.RawURLEncoding.DecodeString(encoded); err == nil {
			var a model.Attachment
			if json.Unmarshal(raw, &a) == nil && a.URL == msg.Text && validAttachment(a) {
				msg.Attachment = &a
			}
		}
	}
	if e.Command == "NOTICE" {
		msg.Kind = "notice"
	}
	if historical && strings.EqualFold(e.Source.Name, "HistServ") && strings.HasPrefix(upstream, "_") {
		msg.Kind = "system"
	}
	msg.SenderAccount = senderAccount(e)
	if original := e.Tags[editTag]; original != "" {
		switch c.incomingEdit(conv, sender, e, msg, historical) {
		case editConsumed:
			return
		case editPending:
			msg.EditOf = original
		}
	}
	needsAcknowledgment := false
	if sender.ID == w.self.ID {
		if account := e.Tags["account"]; account != "" && account != "*" {
			msg.Account = account
		}
		clientID := e.Tags["+ircord/client-id"]
		c.mu.Lock()
		p, exists := c.pending[clientID]
		if !exists && !historical {
			for key, candidate := range c.pending {
				if candidate.input.ConversationID == conv.ID && candidate.input.Text == msg.Text {
					clientID = key
					p = candidate
					exists = true
					break
				}
			}
		}
		if exists {
			delete(c.pending, clientID)
		}
		c.mu.Unlock()
		if !exists && clientID != "" && w.store != nil {
			saved, err := w.store.Command(w.ownerID, clientID)
			if err == nil && saved.ConversationID == conv.ID && saved.Input.Text == msg.Text {
				p = pending{input: saved.Input, messageID: saved.MessageID}
				exists = true
				needsAcknowledgment = saved.State != "sent"
			}
		}
		if exists {
			msg.ClientID = clientID
			msg.GIF = p.input.GIF
			if len(p.input.Attachments) == 1 && c.manager.options.Files != nil {
				msg.Attachment, _ = c.manager.options.Files.Attachment(w.ownerID, c.config.ID, p.input.Attachments[0])
			}
			if upstream == "" || p.messageID != "" && w.store != nil {
				if p.messageID != "" {
					msg.ID = p.messageID
				}
			}
			if p.input.ReplyToID != "" {
				if prior, ok := c.lookupMessage(conv.ID, p.input.ReplyToID); ok {
					msg.ReplyTo = &Reply{ID: prior.ID, Nick: w.members[prior.SenderID].Nick, Text: prior.Text}
				}
			}
		}
	}
	if known && !needsAcknowledgment {
		return
	}
	msg.Historical = historical
	msg.Mention = sender.ID != w.self.ID && msg.Kind != "system" && msg.Kind != "event" && (conv.Kind == "direct" || w.policy.Mentioned(msg.Text, w.recipient(conv, c.currentNick(), msg)))
	reply := e.Tags["+reply"]
	if reply == "" {
		reply = e.Tags["+draft/reply"]
	}
	// Only a reply tag naming a message IRCord has stored counts as a reply to you.
	replyToSelf := false
	if reply != "" && msg.ReplyTo == nil {
		if prior, ok := c.lookupMessage(conv.ID, reply); ok {
			msg.ReplyTo = &Reply{ID: prior.ID, Nick: w.members[prior.SenderID].Nick, Text: prior.Text}
			replyToSelf = prior.SenderID == w.self.ID && !prior.Deleted
		}
	}
	if w.store != nil && upstream != "" {
		if reactions, err := w.store.Reactions(w.ownerID, conv.ID, upstream); err == nil {
			msg.Reactions = reactions
		} else {
			w.storageErr = err
			return
		}
	}
	replaced := false
	for i, old := range w.messages[conv.ID] {
		if old.ID == msg.ID {
			w.messages[conv.ID][i] = msg
			replaced = true
			break
		}
	}
	if !replaced {
		w.messages[conv.ID] = append(w.messages[conv.ID], msg)
	}
	sort.SliceStable(w.messages[conv.ID], func(i, j int) bool { return w.messages[conv.ID][i].CreatedAt.Before(w.messages[conv.ID][j].CreatedAt) })
	if len(w.messages[conv.ID]) > 500 {
		w.messages[conv.ID] = append([]Message(nil), w.messages[conv.ID][len(w.messages[conv.ID])-500:]...)
	}
	w.boundCache(conv.ID)
	if !historical && sender.ID != w.self.ID && !w.policy.Ignored(sender.Nick, sender.Username, sender.Host) {
		conv.UnreadCount++
		if msg.Mention {
			conv.MentionCount++
		}
		w.conversations[conv.ID] = conv
		w.emitCounts(conv)
	}
	if historical && w.store != nil && !needsAcknowledgment {
		inserted, err := w.store.HistoryInsert(w.ownerID, msg)
		if err != nil {
			w.storageErr = err
		} else if inserted && recovered {
			c.manager.recordInbox(w, conv, msg, sender, c.currentNick(), replyToSelf)
		}
	} else {
		w.emit("message.created", msg)
		if !historical {
			c.manager.notifyMessage(w, conv, msg, sender, c.currentNick())
			c.manager.recordInbox(w, conv, msg, sender, c.currentNick(), replyToSelf)
		}
	}
	if w.storageErr == nil {
		c.applyPendingEdits(conv, msg, historical)
	}
}

func validGIF(g *GIF) bool {
	if g == nil {
		return true
	}
	if g.Provider != "klipy" || len(g.ID) > 100 || !clean(g.Slug, 180) || len(g.Title) > 200 || g.Width < 1 || g.Height < 1 || g.Width > 8192 || g.Height > 8192 {
		return false
	}
	// Klipy media hosts are kept in step with klipyMediaHosts in frontend/shared/utils/gif-hosts.ts.
	for _, raw := range []string{g.URL, g.PreviewURL, g.StillURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || len(raw) > 4096 || !slices.Contains([]string{"static.klipy.com", "media.klipy.com", "cdn.klipy.com"}, u.Hostname()) {
			return false
		}
	}
	return true
}

func (m *Manager) Send(owner string, input SendInput) error {
	if input.Kind == "" {
		input.Kind = "message"
	}
	if input.Kind != "message" && input.Kind != "action" {
		return fmt.Errorf("invalid message kind")
	}
	if !token(input.ClientID, 100) || strings.TrimSpace(input.Text) == "" || utf8.RuneCountInString(input.Text) > 4000 || len(input.Text) > 4096 || strings.ContainsAny(input.Text, "\r\x00") || !validGIF(input.GIF) || (input.GIF != nil && input.Text != input.GIF.URL) {
		return fmt.Errorf("invalid message (maximum 4000 characters / 4096 UTF-8 bytes)")
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[input.ConversationID]
	c := w.connections[conv.NetworkID]
	if !ok || c == nil {
		w.mu.Unlock()
		return ErrNotFound
	}
	if len(input.Attachments) > 1 {
		w.mu.Unlock()
		return fmt.Errorf("attach one upload per message")
	}
	var attachment *model.Attachment
	if len(input.Attachments) > 0 {
		if m.options.Files == nil {
			w.mu.Unlock()
			return ErrUnsupported
		}
		var e error
		attachment, e = m.options.Files.Attachment(owner, conv.NetworkID, input.Attachments[0])
		if e != nil || input.Text != attachment.URL {
			w.mu.Unlock()
			return fmt.Errorf("upload is unavailable in this conversation")
		}
	}
	for _, msg := range w.messages[conv.ID] {
		if w.store == nil && msg.ClientID == input.ClientID {
			if msg.Text != input.Text || msg.Kind != input.Kind || (w.store == nil && (input.GIF != nil || input.ReplyToID != "")) {
				w.mu.Unlock()
				return fmt.Errorf("clientId already used")
			}
			w.emit("message.created", msg)
			w.mu.Unlock()
			return nil
		}
	}
	if !canSpeak(conv, w.networks[conv.NetworkID].ISupport, w.self.ID) {
		w.mu.Unlock()
		return fmt.Errorf("this channel is moderated; voice or operator privileges are required")
	}
	tags := girc.Tags{"+ircord/client-id": input.ClientID, "label": input.ClientID}
	if input.ReplyToID != "" {
		found := false
		replyMessages := w.messages[conv.ID]
		if w.store != nil {
			if saved, e := w.store.Message(owner, conv.ID, input.ReplyToID); e == nil {
				replyMessages = append([]Message{saved}, replyMessages...)
			}
		}
		for _, msg := range replyMessages {
			if msg.ID == input.ReplyToID && addressable(msg) {
				tags["+reply"] = msg.UpstreamID
				found = true
				break
			}
		}
		if !found {
			w.mu.Unlock()
			return fmt.Errorf("reply target is unavailable in this conversation")
		}
	}
	isupport := clone(w.networks[conv.NetworkID].ISupport)
	w.mu.Unlock()
	c.mu.Lock()
	if !c.ready || conv.Kind == "channel" && !conv.Joined {
		c.mu.Unlock()
		return ErrDisconnected
	}
	features := c.features(isupport)
	if input.ReplyToID != "" && !features.Replies {
		c.mu.Unlock()
		return fmt.Errorf("%w: replies are disabled by this network", ErrUnsupported)
	}
	if previous, ok := c.pending[input.ClientID]; ok {
		if !equal(previous.input, input) {
			c.mu.Unlock()
			return fmt.Errorf("clientId already used")
		}
		c.mu.Unlock()
		return nil
	}
	if len(c.pending) >= 4 || len(c.queue) > 8 {
		c.mu.Unlock()
		return ErrBusy
	}
	if !c.caps["message-tags"] {
		tags = nil
		if input.ReplyToID != "" {
			c.mu.Unlock()
			return ErrUnsupported
		}
	}
	if !c.caps["labeled-response"] {
		delete(tags, "label")
	}
	wireText := input.Text
	if input.Kind == "action" {
		wireText = "\x01ACTION " + wireText + "\x01"
	}
	if input.GIF != nil && tags != nil && tagAllowed(isupport["CLIENTTAGDENY"], "ircord/gif") {
		raw, _ := json.Marshal(input.GIF)
		encoded := base64.RawURLEncoding.EncodeToString(raw)
		if len(encoded) <= 2500 {
			tags["+ircord/gif"] = encoded
		}
	}
	if attachment != nil && tags != nil && tagAllowed(isupport["CLIENTTAGDENY"], "ircord/attachment") {
		raw, _ := json.Marshal(attachment)
		encoded := base64.RawURLEncoding.EncodeToString(raw)
		if len(encoded) <= 1800 {
			tags["+ircord/attachment"] = encoded
		}
	}
	if !tagAllowed(isupport["CLIENTTAGDENY"], "ircord/client-id") {
		delete(tags, "+ircord/client-id")
	}
	events, eventErr := messageEvents(features, c.target(conv), wireText, tags)
	if eventErr != nil {
		c.mu.Unlock()
		return eventErr
	}
	echo := c.caps["echo-message"]
	if w.store != nil {
		command, fresh, err := w.store.BeginCommand(owner, input)
		if err != nil {
			c.mu.Unlock()
			return err
		}
		if !fresh {
			switch command.State {
			case "sent", "unconfirmed":
				c.mu.Unlock()
				saved, e := w.store.Message(owner, conv.ID, command.MessageID)
				if e != nil {
					return ErrStorage
				}
				w.mu.Lock()
				w.emit("message.created", saved)
				w.mu.Unlock()
				return nil
			case "pending":
				c.mu.Unlock()
				return nil
			case "uncertain":
				c.mu.Unlock()
				return ErrUncertain
			case "failed":
				if e := w.store.RetryFailed(owner, input.ClientID); e != nil {
					c.mu.Unlock()
					return ErrStorage
				}
			}
		}
	}
	c.pending[input.ClientID] = pending{input: clone(input), messageID: id(), at: time.Now()}
	c.mu.Unlock()
	if err := c.enqueue(events...); err != nil {
		c.mu.Lock()
		delete(c.pending, input.ClientID)
		c.mu.Unlock()
		if w.store != nil {
			_ = w.store.FailCommand(owner, input.ClientID, err.Error())
		}
		return err
	}
	if !echo {
		// Without echo-message IRC cannot confirm acceptance. Mark this as a local
		// sent copy only; it has no upstream ID and cannot be a reaction target.
		c.message(girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: c.currentNick()}, Params: []string{c.target(conv), wireText}, Tags: girc.Tags{"+ircord/client-id": input.ClientID}, Timestamp: time.Now()})
	}
	m.endBriefAway(owner, conv.NetworkID, c)
	return nil
}
