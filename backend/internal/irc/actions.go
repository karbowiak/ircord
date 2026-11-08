package irc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

func (c *connection) has(cap string) bool { c.mu.Lock(); defer c.mu.Unlock(); return c.caps[cap] }
func (m *Manager) Reconnect(owner, network string) error {
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	w.mu.Lock()
	cfg, ok := w.configs[network]
	active := w.connections[network]
	w.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	if active != nil {
		if err := m.Disconnect(owner, network); err != nil {
			return err
		}
	}
	_, err := m.Connect(owner, cfg)
	return err
}
func (c *connection) fold(s string) string {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client != nil {
		if mapping, _ := client.GetServerOption("CASEMAPPING"); mapping == "ascii" {
			return strings.ToLower(s)
		}
	}
	return girc.ToRFC1459(s)
}
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
	if err = c.enqueue(&girc.Event{Command: "JOIN", Params: strings.Fields(entry)}); err != nil {
		return Conversation{}, err
	}
	c.owner.mu.Lock()
	defer c.owner.mu.Unlock()
	return clone(c.conversation(target)), nil
}

// foldUnlocked is used only with c.mu held, and does not call the IRC client.
func (c *connection) foldUnlocked(s string) string { return strings.ToLower(s) }
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
func validGIF(g *GIF) bool {
	if g == nil {
		return true
	}
	if g.Provider != "klipy" || len(g.ID) > 100 || !clean(g.Slug, 180) || len(g.Title) > 200 || g.Width < 1 || g.Height < 1 || g.Width > 8192 || g.Height > 8192 {
		return false
	}
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
	return nil
}
func (c *connection) currentNick() string {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client != nil {
		return client.GetNick()
	}
	return c.config.Nick
}
func (m *Manager) ToggleReaction(owner, conversation, messageID, emoji string) (Message, error) {
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
	c.reaction(girc.Event{Source: &girc.Source{Name: c.currentNick()}, Params: []string{c.target(conv)}, Tags: girc.Tags{"+reply": target.UpstreamID, tag: emoji}})
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

// RedactTimeout bounds how long Redact waits for the network to accept a REDACT.
var RedactTimeout = 10 * time.Second

// Redact deletes the caller's own message for everyone through
// draft/message-redaction. The stored copy changes only after the network
// accepts the request when labeled-response can report the outcome. It returns
// the deleted message and the attachment it carried, which stays uploaded.
func (m *Manager) Redact(owner, conversation, messageID string) (Message, *model.Attachment, error) {
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	if !ok || c == nil {
		w.mu.Unlock()
		return Message{}, nil, ErrNotFound
	}
	target, found := c.lookupMessage(conversation, messageID)
	self := w.self.ID
	w.mu.Unlock()
	if !found || target.ConversationID != conversation {
		return Message{}, nil, ErrNotFound
	}
	if target.SenderID != self || target.Kind == "system" || target.Kind == "event" {
		return Message{}, nil, ErrForbidden
	}
	if target.Deleted {
		return Message{}, nil, ErrAlreadyDeleted
	}
	if conv.Kind == "channel" && !conv.Joined {
		return Message{}, nil, ErrDisconnected
	}
	if !c.has("draft/message-redaction") {
		return Message{}, nil, ErrRedactUnsupported
	}
	if !addressable(target) {
		return Message{}, nil, fmt.Errorf("%w: the network did not confirm this message, so it has no network ID", ErrRedactUnsupported)
	}
	event := &girc.Event{Command: "REDACT", Params: []string{c.target(conv), target.UpstreamID}}
	var wait chan string
	label := ""
	if c.has("labeled-response") {
		label = "redact:" + id()
		event.Tags = girc.Tags{"label": label}
		wait = make(chan string, 1)
		c.mu.Lock()
		if c.redactions == nil {
			c.redactions = map[string]chan string{}
		}
		c.redactions[label] = wait
		c.mu.Unlock()
	}
	forget := func() {
		c.mu.Lock()
		delete(c.redactions, label)
		c.mu.Unlock()
	}
	if err := c.enqueue(event); err != nil {
		forget()
		return Message{}, nil, err
	}
	if wait != nil {
		timer := time.NewTimer(RedactTimeout)
		defer timer.Stop()
		select {
		case reason := <-wait:
			if reason != "" {
				return Message{}, nil, fmt.Errorf("%w: %s", ErrRedactRefused, reason)
			}
		case <-timer.C:
			forget()
			return Message{}, nil, ErrRedactUnconfirmed
		case <-c.ctx.Done():
			forget()
			return Message{}, nil, ErrDisconnected
		}
	}
	// Without labeled-response a refusal arrives later as an unlabeled FAIL, which
	// the client shows as a command error; apply the deletion locally now.
	w.mu.Lock()
	defer w.mu.Unlock()
	current, ok := c.lookupMessage(conversation, target.ID)
	if !ok {
		return Message{}, nil, ErrNotFound
	}
	deleted := c.applyRedaction(conv, current, false)
	if w.storageErr != nil {
		// The network already removed it; report that IRCord could not record it.
		return Message{}, nil, ErrStorage
	}
	return deleted, target.Attachment, nil
}
func (m *Manager) History(owner, conversation, before string, limit int) (MessagePage, error) {
	if limit < 1 || limit > 100 {
		return MessagePage{}, fmt.Errorf("limit must be between 1 and 100")
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.conversations[conversation]; !ok {
		return MessagePage{}, ErrNotFound
	}
	items := w.messages[conversation]
	end := len(items)
	if before != "" {
		end = -1
		for i := range items {
			if items[i].ID == before {
				end = i
				break
			}
		}
		if end < 0 {
			return MessagePage{}, fmt.Errorf("invalid history cursor")
		}
	}
	start := max(0, end-limit)
	page := MessagePage{Items: clone(items[start:end])}
	if page.Items == nil {
		page.Items = []Message{}
	}
	if start > 0 {
		cursor := items[start].ID
		page.NextCursor = &cursor
	}
	return page, nil
}

// RequestHistory imports server history asynchronously. Incoming msgids dedupe
// overlapping auto-replay, explicit requests, and reconnect history.
func (m *Manager) RequestHistory(owner, conversation, before string, limit int) error {
	if limit < 1 || limit > 100 {
		return fmt.Errorf("invalid history limit")
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	w.mu.Unlock()
	if !ok || c == nil {
		return ErrNotFound
	}
	if !c.hasAll("draft/chathistory", "batch", "server-time") {
		return ErrUnsupported
	}
	sub, ref := "LATEST", "*"
	if before != "" {
		if !clean(before, 100) {
			return fmt.Errorf("invalid upstream cursor")
		}
		sub = "BEFORE"
		ref = "msgid=" + before
	}
	return c.enqueue(&girc.Event{Command: "CHATHISTORY", Params: []string{sub, c.target(conv), ref, fmt.Sprint(limit)}})
}
func (m *Manager) UpdateConversation(owner, conversation string, read *bool, favorite *bool) (Conversation, error) {
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
	w.emit("conversation.updated", v)
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
func (m *Manager) SetProfile(owner string, p Profile) (Member, error) {
	p.Bio = strings.ReplaceAll(p.Bio, "\r\n", "\n")
	if p.DisplayName != nil && !clean(*p.DisplayName, 80) {
		return Member{}, fmt.Errorf("invalid profile")
	}
	if !utf8.ValidString(p.Bio) || utf8.RuneCountInString(p.Bio) > 300 || strings.ContainsAny(p.Bio, "\r\x00") {
		return Member{}, fmt.Errorf("About you must contain 300 characters or fewer and no control characters")
	}
	if p.NameColor != nil && *p.NameColor != "" {
		if len(*p.NameColor) != 7 || (*p.NameColor)[0] != '#' {
			return Member{}, fmt.Errorf("invalid name color")
		}
		for _, r := range (*p.NameColor)[1:] {
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return Member{}, fmt.Errorf("invalid name color")
			}
		}
	}
	if p.AvatarURL != nil && *p.AvatarURL != "" {
		u, err := url.Parse(*p.AvatarURL)
		if err != nil || (u.Scheme != "https" && (m.options.Files == nil || !m.options.Files.OwnsURL(owner, *p.AvatarURL))) || u.Host == "" || u.User != nil || !clean(*p.AvatarURL, 300) {
			return Member{}, fmt.Errorf("avatar must be an HTTPS URL")
		}
	}
	w := m.space(owner)
	w.mu.Lock()
	if w.deleted {
		w.mu.Unlock()
		return Member{}, ErrNotFound
	}
	self := w.self
	before := self
	if p.DisplayName != nil {
		if name := strings.TrimSpace(*p.DisplayName); name != "" {
			self.DisplayName, self.NameSource = name, "profile"
		} else {
			self.NameSource = ""
			resolveMemberName(&self)
		}
	}
	self.Bio = p.Bio
	self.NameColor = ""
	if p.NameColor != nil {
		self.NameColor = *p.NameColor
	}
	if p.AvatarURL != nil {
		self.AvatarURL = *p.AvatarURL
	}
	w.self = self
	w.members[self.ID] = self
	w.emit("profile.updated", self)
	if w.storageErr != nil {
		w.mu.Unlock()
		return Member{}, ErrStorage
	}
	// Only the fields this edit changed are published, so an edit made here does not clear
	// values another IRCord instance signed in to the same account has shared. Publishing
	// under the workspace mutex keeps the SETs in the same order as the edits, also against a
	// connect-time publish (profileCheckReply). enqueue never blocks.
	changed := changedProfileKeys(before, self)
	for _, c := range w.connections {
		c.pushProfileKeys(self, changed)
	}
	w.mu.Unlock()
	return clone(self), nil
}
func (m *Manager) SetAway(owner, network, reason string) error {
	if !clean(reason, 200) {
		return fmt.Errorf("invalid away reason")
	}
	c, err := m.connection(owner, network)
	if err != nil {
		return err
	}
	return c.enqueue(&girc.Event{Command: "AWAY", Params: []string{reason}})
}
func (m *Manager) Nick(owner, network, nick string) error {
	if err := validateNickname(nick, 0); err != nil {
		return err
	}
	c, err := m.connection(owner, network)
	if err != nil {
		return err
	}
	c.owner.mu.Lock()
	maximum := c.nicknameLimit()
	c.owner.mu.Unlock()
	if err := validateNickname(nick, maximum); err != nil {
		return err
	}
	return c.enqueue(&girc.Event{Command: "NICK", Params: []string{nick}})
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
func (m *Manager) Preferences(owner string, p Preferences) Preferences {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if p.Theme != "light" {
		p.Theme = "dark"
	}
	w.preferences = p
	w.emit("preferences.updated", p)
	return p
}

type SearchResult struct {
	Message      Message      `json:"message"`
	Conversation Conversation `json:"conversation"`
}

func (m *Manager) Search(owner, query, conversation string) ([]SearchResult, error) {
	if len(query) < 2 || len(query) > 200 {
		return nil, fmt.Errorf("query must have 2 to 200 bytes")
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if conversation != "" {
		if _, ok := w.conversations[conversation]; !ok {
			return nil, ErrNotFound
		}
	}
	result := []SearchResult{}
	if w.store != nil {
		messages, err := w.store.Search(owner, query, conversation, 50)
		if err != nil {
			return nil, ErrStorage
		}
		for _, msg := range messages {
			if conv, ok := w.conversations[msg.ConversationID]; ok {
				result = append(result, SearchResult{msg, conv})
			}
		}
		return result, nil
	}
	for cid, msgs := range w.messages {
		if conversation != "" && cid != conversation {
			continue
		}
		for _, msg := range msgs {
			if strings.Contains(strings.ToLower(msg.Text), strings.ToLower(query)) {
				result = append(result, SearchResult{msg, w.conversations[cid]})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Message.CreatedAt.After(result[j].Message.CreatedAt) })
	return clone(result[:min(len(result), 50)]), nil
}

// WaitConnected is a convenience for integration tests and service startup.
func (m *Manager) WaitConnected(ctx context.Context, owner, network string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot := m.Snapshot(owner)
		for _, n := range snapshot.Networks {
			if n.ID == network && n.Status == "connected" {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *connection) conversationID(target string) string {
	digest := sha256.Sum256([]byte(c.fold(target)))
	return c.config.ID + "_" + hex.EncodeToString(digest[:12])
}
