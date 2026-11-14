package irc

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

// RedactTimeout bounds how long Redact waits for the network to accept a REDACT.
var RedactTimeout = 10 * time.Second

// Redact deletes the caller's own message for everyone through
// draft/message-redaction. The stored copy changes only after the network
// accepts the request when labeled-response can report the outcome. It returns
// the deleted message and its first attachment, which stays uploaded.
func (m *Manager) Redact(owner, conversation, messageID string) (Message, *model.Attachment, error) {
	if e := m.partyAccess(owner, conversation); e != nil {
		return Message{}, nil, e
	}
	msg, attachments, err := m.RedactAttachments(owner, conversation, messageID)
	if err != nil || len(attachments) == 0 {
		return msg, nil, err
	}
	return msg, &attachments[0], nil
}

// RedactAttachments deletes a message and returns a copy of every retained
// attachment. Removing a message never removes its uploaded files.
func (m *Manager) RedactAttachments(owner, conversation, messageID string) (Message, []model.Attachment, error) {
	r, err := m.RedactAs(context.Background(), owner, conversation, messageID, "")
	return r.Message, r.Attachments, err
}

// Who a deletion was carried out as. The author and channel operators use their
// own connection; a server owner's deletion goes through the moderation login.
const (
	AuthorityAuthor   = "author"
	AuthorityOperator = "operator"
	AuthorityOwner    = "owner"
)

// RedactResult describes one deletion. Original keeps the sender, network ID and
// time of the deleted message for audit records; its text is never kept.
type RedactResult struct {
	Message     Message
	Attachments []model.Attachment
	Authority   string
	// OwnerActing is set when a server owner deletes someone else's message on
	// the associated server, also when they are a channel operator there and the
	// deletion uses their own connection. It decides audit records and whether
	// uploaded files may be deleted with the message.
	OwnerActing bool
	Channel     string
	Sender      string
	Original    Message
}

// ReasonMaxBytes bounds the reason sent with a REDACT, so the line stays within
// IRC's 512-byte limit together with the channel and message ID.
const ReasonMaxBytes = 300

// clipReason trims reason to ReasonMaxBytes on a character boundary.
func clipReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) <= ReasonMaxBytes {
		return reason
	}
	cut := ReasonMaxBytes
	for cut > 0 && !utf8.RuneStart(reason[cut]) {
		cut--
	}
	return strings.TrimSpace(reason[:cut])
}

// RedactAs deletes a message for everyone. The author may delete their own
// message anywhere the network supports it; a channel operator (@ and above) any
// message in that channel; and a server owner any channel message on the
// associated server, through the moderation login. The first that applies is
// used. reason is sent with the REDACT only when it isn't empty. Authority and
// Original are set as soon as the caller's authority is known, also on failure.
func (m *Manager) RedactAs(ctx context.Context, owner, conversation, messageID, reason string) (RedactResult, error) {
	if e := m.partyAccess(owner, conversation); e != nil {
		return RedactResult{}, e
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	if !ok || c == nil {
		w.mu.Unlock()
		return RedactResult{}, ErrNotFound
	}
	target, found := c.lookupMessage(conversation, messageID)
	self := w.self.ID
	network := w.networks[conv.NetworkID]
	sender := w.members[target.SenderID].Nick
	w.mu.Unlock()
	if !found || target.ConversationID != conversation {
		return RedactResult{}, ErrNotFound
	}
	if target.Kind == "system" || target.Kind == "event" {
		return RedactResult{}, ErrDeleteForbidden
	}
	out := RedactResult{Original: target, Channel: c.target(conv), Sender: sender}
	out.Original.Text, out.Original.Edits, out.Original.OriginalText, out.Original.ReplyTo = "", nil, "", nil
	if target.SenderID == self {
		out.Authority = AuthorityAuthor
	} else if out.Authority = m.moderationAuthority(owner, c, conv, network, self); out.Authority == "" {
		return RedactResult{}, ErrDeleteForbidden
	} else {
		out.OwnerActing = out.Authority == AuthorityOwner || m.ownerActing(owner, c, conv)
	}
	reason = clipReason(reason)
	if target.Deleted {
		return out, ErrAlreadyDeleted
	}
	if out.Authority != AuthorityOwner && conv.Kind == "channel" && !conv.Joined {
		return out, ErrDisconnected
	}
	if !c.has("draft/message-redaction") {
		return out, ErrRedactUnsupported
	}
	if !addressable(target) {
		return out, fmt.Errorf("%w: the network did not confirm this message, so it has no network ID", ErrRedactUnsupported)
	}
	if out.Authority == AuthorityOwner {
		if err := m.moderator.redact(ctx, out.Channel, target.UpstreamID, reason); err != nil {
			return out, err
		}
		for _, edit := range target.Edits {
			// Best effort, as for the author's own edits below.
			_ = m.moderator.redact(ctx, out.Channel, edit.UpstreamID, "")
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		current, ok := c.lookupMessage(conversation, target.ID)
		if !ok {
			return out, ErrNotFound
		}
		out.Message = c.applyRedaction(conv, current, false)
		out.Attachments = target.AllAttachments()
		if w.storageErr != nil {
			return out, ErrStorage
		}
		return out, nil
	}
	params := []string{c.target(conv), target.UpstreamID}
	if reason != "" {
		params = append(params, reason)
	}
	event := &girc.Event{Command: "REDACT", Params: params}
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
		return out, err
	}
	if wait != nil {
		timer := time.NewTimer(RedactTimeout)
		defer timer.Stop()
		select {
		case reason := <-wait:
			if reason != "" {
				return out, refusedError(reason)
			}
		case <-timer.C:
			forget()
			return out, ErrRedactUnconfirmed
		case <-c.ctx.Done():
			forget()
			return out, ErrDisconnected
		}
	}
	// Without labeled-response a refusal arrives later as an unlabeled FAIL, which
	// the client shows as a command error; apply the deletion locally now.
	w.mu.Lock()
	defer w.mu.Unlock()
	current, ok := c.lookupMessage(conversation, target.ID)
	if !ok {
		return out, ErrNotFound
	}
	// Each edit is a message of its own on the network. Delete them too, or
	// their text would come back from history as ordinary lines. The network's
	// REDACT may already have cleared the stored edits, so use both copies.
	edits := map[string]bool{}
	for _, v := range append(target.Edits, current.Edits...) {
		edits[v.UpstreamID] = true
	}
	deleted := c.applyRedaction(conv, current, false)
	for edit := range edits {
		// Best effort: a refused or stale edit must not fail the deletion.
		_ = c.enqueue(&girc.Event{Command: "REDACT", Params: []string{c.target(conv), edit}})
	}
	if w.storageErr != nil {
		// The network already removed it; report that IRCord could not record it.
		return out, ErrStorage
	}
	out.Message, out.Attachments = deleted, target.AllAttachments()
	return out, nil
}

func (c *connection) redact(e girc.Event) {
	if e.Source == nil || len(e.Params) < 2 {
		return
	}
	label := c.eventLabel(e)
	c.mu.Lock()
	historical := c.historyBatches[e.Tags["batch"]]
	c.mu.Unlock()
	target := e.Params[0]
	if !girc.IsValidChannel(target) && c.fold(target) == c.fold(c.currentNick()) {
		target = e.Source.Name
	}
	w := c.owner
	w.mu.Lock()
	// Only an existing conversation and a stored message can be redacted; an
	// unsolicited REDACT must not create a conversation or member as a side effect.
	if conv, ok := w.conversations[c.conversationID(target)]; ok {
		if msg, found := c.lookupMessage(conv.ID, e.Params[1]); found {
			if c.mayRedact(conv, msg, e) {
				c.applyRedaction(conv, msg, historical)
			}
		} else {
			c.revertEdit(conv, e.Params[1], e, historical)
		}
	}
	w.mu.Unlock()
	c.redactionResult(label, "")
}

// mayRedact accepts a REDACT only from the message author, a channel operator
// (@ and above, as Ergo requires), or a server. This keeps a lax or hostile peer
// from erasing other people's messages in our stored history. On the associated
// server every relayed REDACT is accepted: that server is known to enforce its
// own rule, and a server owner's deletion arrives from the moderation login,
// which is an IRC operator but not a member of the channel.
func (c *connection) mayRedact(conv Conversation, msg Message, e girc.Event) bool {
	if e.Source.Ident == "" && e.Source.Host == "" && strings.Contains(e.Source.Name, ".") {
		return true
	}
	if c.home() {
		return true
	}
	source := c.knownMember(e.Source.Name, e.Tags["account"])
	if source == "" {
		return false
	}
	if source == msg.SenderID {
		return true
	}
	// Rank by the network's PREFIX, as outgoing deletions do: operator (o) or above.
	return channelAt(conv, c.owner.networks[c.config.ID].ISupport, source, 'o')
}

// isServerOwner reads the owner list on every call, so a revocation applies at once.
func (m *Manager) isServerOwner(owner string) bool {
	if m.options.Store == nil {
		return false
	}
	ok, err := m.options.Store.IsOwner(owner)
	return err == nil && ok
}

// applyRedaction replaces a message with a deletion marker and drops everything
// derived from its content: reactions, the mention marker and badge, reply
// excerpts that quote it, and copies of its text in the replay log and server
// window. It is idempotent. Callers hold w.mu; on a storage failure it sets
// w.storageErr and the caller reports it.
func (c *connection) applyRedaction(conv Conversation, msg Message, historical bool) Message {
	w := c.owner
	if msg.Deleted {
		return msg
	}
	clientID := msg.ClientID
	msg.Text = "Message deleted"
	msg.Deleted = true
	msg.Mention = false
	msg.GIF = nil
	msg.SetAttachments(nil)
	msg.ReplyTo = nil
	msg.Reactions = []Reaction{}
	editIDs := make([]string, 0, len(msg.Edits))
	for _, v := range msg.Edits {
		editIDs = append(editIDs, v.UpstreamID)
	}
	msg.Edits, msg.EditedAt, msg.OriginalText = nil, nil, ""
	children := map[string]Message{}
	for _, child := range w.messages[conv.ID] {
		if child.ReplyTo != nil && child.ReplyTo.ID == msg.ID {
			children[child.ID] = clone(child)
		}
	}
	if w.store != nil {
		if err := w.store.ClearRedacted(w.ownerID, conv.ID, msg.ID, msg.UpstreamID); err != nil {
			w.storageErr = err
			return msg
		}
		stored, err := w.store.Replies(w.ownerID, conv.ID, msg.ID)
		if err != nil {
			w.storageErr = err
			return msg
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
		return msg
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
			return msg
		}
	}
	c.scrubLines(msg, clientID)
	for _, edit := range editIDs {
		c.scrubLines(Message{UpstreamID: edit, Text: msg.Text}, "")
	}
	if w.store != nil && w.storageErr == nil {
		if err := w.store.ScrubRedacted(w.ownerID, c.config.ID, msg, clientID, editIDs...); err != nil {
			w.storageErr = err
			return msg
		}
	}
	// Refresh derived counts at the mutation, including historical updates,
	// so snapshots can use the shared account view without querying history.
	if w.store != nil && w.storageErr == nil {
		w.recount(conv.ID)
	}
	return msg
}

// scrubLines removes the deleted text from the in-memory server window and raw
// trace. Callers hold w.mu.
func (c *connection) scrubLines(msg Message, clientID string) {
	w := c.owner
	storage.ScrubServerLines(w.serverLogs[c.config.ID], msg.UpstreamID, clientID, msg.Text)
	storage.ScrubServerLines(w.trace[c.config.ID], msg.UpstreamID, clientID, msg.Text)
}

func (c *connection) saveRedaction(msg Message, historical bool) bool {
	w := c.owner
	if !historical {
		w.emit("message.updated", msg)
		return w.storageErr == nil
	}
	if w.store != nil {
		if err := w.store.SaveHistory(w.ownerID, msg); err != nil {
			w.storageErr = err
			return false
		}
	}
	return true
}

// redactionResult completes a waiting outgoing REDACT or edit. reason is empty
// on success.
func (c *connection) redactionResult(label, reason string) bool {
	if !strings.HasPrefix(label, "redact:") && !strings.HasPrefix(label, "edit:") {
		return false
	}
	c.mu.Lock()
	wait := c.redactions[label]
	delete(c.redactions, label)
	c.mu.Unlock()
	if wait != nil {
		wait <- reason
	}
	return true
}
