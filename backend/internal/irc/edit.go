package irc

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

// editTag marks a PRIVMSG as a correction of an earlier message. Its value is
// the original message's msgid. IRC has no interoperable edit command yet
// (draft/message-edit is unfinished and no server implements it), so an edit is
// sent as a normal message: IRCord clients fold it into the original, everyone
// else sees the corrected text as a new line.
const editTag = "+ircord/edit"

// maxEdits bounds the edit history kept on one message.
const maxEdits = 20

// EditTimeout bounds how long Edit waits for the network to echo an edit.
var EditTimeout = 10 * time.Second

// Edit replaces the text of one of the caller's own messages for everyone. It
// sends the new text as a PRIVMSG tagged +ircord/edit=<original msgid> and
// applies it when the network echoes it back, so the stored copy changes only
// once the network accepted the edit.
func (m *Manager) Edit(owner, conversation, messageID, text string) (Message, error) {
	if strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 4000 || len(text) > 4096 || strings.ContainsAny(text, "\r\x00") {
		return Message{}, fmt.Errorf("invalid message (maximum 4000 characters / 4096 UTF-8 bytes)")
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	if !ok || c == nil {
		w.mu.Unlock()
		return Message{}, ErrNotFound
	}
	target, found := c.lookupMessage(conversation, messageID)
	self := w.self.ID
	network := w.networks[conv.NetworkID]
	isupport := clone(network.ISupport)
	speak := canSpeak(conv, network.ISupport, self)
	w.mu.Unlock()
	if !found || target.ConversationID != conversation {
		return Message{}, ErrNotFound
	}
	if target.SenderID != self || (target.Kind != "message" && target.Kind != "action") {
		return Message{}, ErrEditForbidden
	}
	if target.Deleted {
		return Message{}, ErrAlreadyDeleted
	}
	if target.GIF != nil || target.Attachment != nil {
		return Message{}, fmt.Errorf("%w: GIFs and uploads cannot be edited", ErrUnsupported)
	}
	if target.Text == text {
		return Message{}, ErrEditUnchanged
	}
	if conv.Kind == "channel" && !conv.Joined {
		return Message{}, ErrDisconnected
	}
	if !speak {
		return Message{}, fmt.Errorf("this channel is moderated; voice or operator privileges are required")
	}
	c.mu.Lock()
	ready := c.ready
	features := c.features(isupport)
	labeled := c.caps["labeled-response"]
	c.mu.Unlock()
	if !ready {
		return Message{}, ErrDisconnected
	}
	if !features.Edit {
		return Message{}, ErrEditUnsupported
	}
	if !addressable(target) {
		return Message{}, fmt.Errorf("%w: the network did not confirm this message, so it has no network ID", ErrEditUnsupported)
	}
	// Receivers accept an edit only from the account that sent the original.
	if authorAccount(target) == "" || !strings.EqualFold(authorAccount(target), network.Account) {
		return Message{}, fmt.Errorf("%w: this message was not sent from the account you are logged into", ErrEditForbidden)
	}
	wireText := text
	if target.Kind == "action" {
		wireText = "\x01ACTION " + text + "\x01"
	}
	tags := girc.Tags{editTag: target.UpstreamID}
	var wait chan string
	label := ""
	if labeled {
		label = "edit:" + id()
		tags["label"] = label
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
	events, err := messageEvents(features, c.target(conv), wireText, tags)
	if err != nil {
		forget()
		return Message{}, err
	}
	if err := c.enqueue(events...); err != nil {
		forget()
		return Message{}, err
	}
	if wait == nil {
		// Without labeled-response the echo applies the edit and publishes
		// message.updated; report the expected result now.
		at := time.Now().UTC()
		target.Text, target.EditedAt = text, &at
		return target, nil
	}
	timer := time.NewTimer(EditTimeout)
	defer timer.Stop()
	select {
	case reason := <-wait:
		if reason == editTargetDeleted {
			return Message{}, fmt.Errorf("%w: %s", ErrAlreadyDeleted, reason)
		}
		if reason != "" {
			return Message{}, fmt.Errorf("%w: %s", ErrEditRefused, reason)
		}
	case <-timer.C:
		forget()
		return Message{}, ErrEditUnconfirmed
	case <-c.ctx.Done():
		forget()
		return Message{}, ErrDisconnected
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.storageErr != nil {
		return Message{}, ErrStorage
	}
	current, ok := c.lookupMessage(conversation, target.ID)
	if !ok {
		return Message{}, ErrNotFound
	}
	return current, nil
}

type editOutcome int

const (
	// editRejected: store the edit as an ordinary line; it does not change
	// the message it names.
	editRejected editOutcome = iota
	// editPending: the original has not arrived; store the edit as a line
	// that remembers its original, to fold in when the original arrives.
	editPending
	// editConsumed: the edit was applied, was a duplicate, or targeted a
	// deleted message; nothing more is stored.
	editConsumed
)

// editTargetDeleted is the outcome reported to a waiting Edit when the message
// was deleted before its edit arrived.
const editTargetDeleted = "the message was deleted before the edit arrived"

// senderAccount is the account an incoming event carries, empty without one.
func senderAccount(e girc.Event) string {
	if account := e.Tags["account"]; account != "*" {
		return account
	}
	return ""
}

// authorAccount is the account the network attributed a stored message to.
func authorAccount(msg Message) string {
	if msg.SenderAccount != "" {
		return msg.SenderAccount
	}
	return msg.Account
}

// mayEdit reports whether an edit from sender/account may change target. Only
// the original sender may edit, and only from the account the original was
// sent from: member ids follow nicks, so a nick alone could be taken over by
// someone else later. A message sent without an account cannot be edited.
func mayEdit(target Message, sender, account string) bool {
	original := authorAccount(target)
	return sender == target.SenderID && original != "" && strings.EqualFold(account, original)
}

func editable(msg Message) bool {
	return (msg.Kind == "message" || msg.Kind == "action") && msg.GIF == nil && msg.Attachment == nil
}

// addEdit records one edit on target; it reports false for a duplicate.
func addEdit(target *Message, edit Message) bool {
	for _, prior := range target.Edits {
		if prior.UpstreamID == edit.UpstreamID {
			return false
		}
	}
	if len(target.Edits) == 0 {
		target.OriginalText = target.Text
	}
	target.Edits = append(target.Edits, model.MessageEdit{UpstreamID: edit.UpstreamID, Text: edit.Text, At: edit.CreatedAt})
	sort.SliceStable(target.Edits, func(i, j int) bool { return target.Edits[i].At.Before(target.Edits[j].At) })
	if len(target.Edits) > maxEdits {
		target.Edits = append([]model.MessageEdit(nil), target.Edits[len(target.Edits)-maxEdits:]...)
	}
	target.Kind = edit.Kind
	return true
}

// incomingEdit folds a PRIVMSG tagged +ircord/edit into the message it names.
// A rejected edit is stored as an ordinary message, which is what every other
// IRC client shows, so nothing is hidden. Callers hold w.mu.
func (c *connection) incomingEdit(conv Conversation, sender Member, e girc.Event, edit Message, historical bool) editOutcome {
	original := e.Tags[editTag]
	label := c.eventLabel(e)
	if original == "" || edit.UpstreamID == "" || (edit.Kind != "message" && edit.Kind != "action") {
		return editRejected
	}
	target, found := c.lookupMessage(conv.ID, original)
	if !found || target.ConversationID != conv.ID {
		c.editResult(label, "the edited message is unavailable")
		return editPending
	}
	if !editable(target) || !mayEdit(target, sender.ID, senderAccount(e)) {
		c.editResult(label, "the edit was not attributed to the message's author")
		return editRejected
	}
	if target.Deleted {
		// The author deletes an edited message's edits along with it; an edit
		// that crossed the deletion must not bring its text back as a new line.
		c.editResult(label, editTargetDeleted)
		return editConsumed
	}
	if addEdit(&target, edit) {
		c.saveEdited(conv, target, historical)
	}
	c.editResult(label, "")
	return editConsumed
}

// applyPendingEdits folds edits that arrived before msg, their original, and
// removes the lines they were stored as. Callers hold w.mu.
func (c *connection) applyPendingEdits(conv Conversation, msg Message, historical bool) {
	w := c.owner
	if msg.UpstreamID == "" || msg.EditOf != "" || !editable(msg) {
		return
	}
	pending := map[string]Message{}
	for _, v := range w.messages[conv.ID] {
		if v.EditOf == msg.UpstreamID {
			pending[v.ID] = clone(v)
		}
	}
	if w.store != nil {
		stored, err := w.store.PendingEdits(w.ownerID, conv.ID, msg.UpstreamID)
		if err != nil {
			w.storageErr = err
			return
		}
		for _, v := range stored {
			if _, ok := pending[v.ID]; !ok {
				pending[v.ID] = v
			}
		}
	}
	if len(pending) == 0 {
		return
	}
	target, ok := c.lookupMessage(conv.ID, msg.ID)
	if !ok || target.Deleted {
		return
	}
	changed := false
	for _, edit := range pending {
		if edit.Deleted || edit.UpstreamID == "" || !mayEdit(target, edit.SenderID, edit.SenderAccount) {
			continue
		}
		changed = addEdit(&target, edit) || changed
		kept := w.messages[conv.ID][:0]
		for _, v := range w.messages[conv.ID] {
			if v.ID != edit.ID {
				kept = append(kept, v)
			}
		}
		w.messages[conv.ID] = kept
		if w.store != nil {
			if err := w.store.RemoveMessage(w.ownerID, conv.ID, edit); err != nil {
				w.storageErr = err
				return
			}
		}
	}
	if changed {
		c.saveEdited(conv, target, historical)
	}
}

// revertEdit handles a REDACT naming an edit rather than a message: the edit is
// dropped and the message shows its previous text again. Callers hold w.mu.
func (c *connection) revertEdit(conv Conversation, upstream string, e girc.Event, historical bool) {
	w := c.owner
	var target Message
	found := false
	for _, msg := range w.messages[conv.ID] {
		if hasEdit(msg, upstream) {
			target, found = clone(msg), true
			break
		}
	}
	if !found && w.store != nil {
		msg, err := w.store.EditTarget(w.ownerID, conv.ID, upstream)
		if err == nil {
			target, found = msg, true
		}
	}
	if !found || target.Deleted || !c.mayRedact(conv, target, e) {
		return
	}
	edits := target.Edits[:0:0]
	for _, v := range target.Edits {
		if v.UpstreamID != upstream {
			edits = append(edits, v)
		}
	}
	target.Edits = edits
	c.saveEdited(conv, target, historical)
}

func hasEdit(msg Message, upstream string) bool {
	for _, v := range msg.Edits {
		if v.UpstreamID == upstream {
			return true
		}
	}
	return false
}

// saveEdited derives the current text from the edit list and stores the
// message, refreshing the quoted excerpt in replies to it. Callers hold w.mu.
func (c *connection) saveEdited(conv Conversation, msg Message, historical bool) {
	w := c.owner
	if len(msg.Edits) == 0 {
		if msg.OriginalText != "" {
			msg.Text = msg.OriginalText
		}
		msg.OriginalText, msg.EditedAt, msg.Edits = "", nil, nil
	} else {
		latest := msg.Edits[len(msg.Edits)-1]
		at := latest.At
		msg.Text, msg.EditedAt = latest.Text, &at
	}
	msg.Mention = msg.SenderID != w.self.ID && (conv.Kind == "direct" || w.policy.Highlight(msg.Text, c.currentNick()))
	children := map[string]Message{}
	for _, child := range w.messages[conv.ID] {
		if child.ReplyTo != nil && child.ReplyTo.ID == msg.ID {
			children[child.ID] = clone(child)
		}
	}
	if w.store != nil {
		stored, err := w.store.Replies(w.ownerID, conv.ID, msg.ID)
		if err != nil {
			w.storageErr = err
			return
		}
		for _, child := range stored {
			if _, ok := children[child.ID]; !ok {
				children[child.ID] = child
			}
		}
	}
	for i, old := range w.messages[conv.ID] {
		if old.ID == msg.ID {
			w.messages[conv.ID][i] = msg
		}
	}
	if !c.saveRedaction(msg, historical) {
		return
	}
	for _, child := range children {
		if child.ReplyTo.Text == msg.Text {
			continue
		}
		child.ReplyTo = &Reply{ID: msg.ID, Nick: child.ReplyTo.Nick, Text: msg.Text}
		for i, old := range w.messages[conv.ID] {
			if old.ID == child.ID {
				w.messages[conv.ID][i] = child
			}
		}
		if !c.saveRedaction(child, historical) {
			return
		}
	}
	// An edit that adds or removes your nick changes the mention count. It
	// does not notify again, as in Discord.
	if w.store != nil {
		if !msg.Mention {
			if err := w.store.ClearMention(w.ownerID, conv.ID, msg.ID); err != nil {
				w.storageErr = err
				return
			}
		}
		if current, ok := w.conversations[conv.ID]; ok {
			count, err := w.store.Mentions(w.ownerID, conv.ID)
			if err != nil {
				w.storageErr = err
				return
			}
			if count != current.MentionCount {
				current.MentionCount = count
				w.conversations[conv.ID] = current
				w.emitCounts(current)
			}
		}
	}
}

// editResult completes a waiting outgoing edit. reason is empty on success.
func (c *connection) editResult(label, reason string) {
	if strings.HasPrefix(label, "edit:") {
		c.redactionResult(label, reason)
	}
}
