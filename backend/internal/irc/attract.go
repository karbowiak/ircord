package irc

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

// attractTag is a client-only tag on a text-less TAGMSG. Servers deliver TAGMSG
// only to clients that negotiated message-tags, so plain IRC clients never see
// it, and Ergo does not keep TAGMSGs in history unless they are whitelisted.
const attractTag = "+ircord/attract"

const (
	// attractCooldown applies per channel, attractBurst/attractBurstWindow per
	// connection across channels.
	attractCooldown    = 30 * time.Second
	attractBurst       = 5
	attractBurstWindow = 5 * time.Minute
	// A receiver plays at most one attract per channel while the effect runs,
	// and one per sender and channel inside attractRepeat. attractEffect is the
	// length of the browser effect: keep it equal to ATTRACT_MS in
	// frontend/shared/utils/attract.ts, which also drives the CSS animations.
	attractEffect = 3 * time.Second
	attractRepeat = 10 * time.Second
)

var (
	ErrRateLimited          = errors.New("too many requests")
	ErrAttractNeedsOperator = errors.New("/attract needs channel operator rights (op, admin or owner)")
	ErrAttractUnsupported   = errors.New("this network does not support /attract")
	ErrAttractNoMessage     error = notFoundError("send a message in this channel first, then /attract it")
)

// notFoundError maps to 404 with its own text instead of the generic one.
type notFoundError string

func (e notFoundError) Error() string         { return string(e) }
func (e notFoundError) Is(target error) bool { return target == ErrNotFound }

// RateLimitError says how long until the action is allowed again.
type RateLimitError struct {
	Wait  time.Duration
	Scope string
}

func (e RateLimitError) Error() string {
	seconds := int(math.Ceil(e.Wait.Seconds()))
	return fmt.Sprintf("You can use /attract %s again in %d s.", e.Scope, max(seconds, 1))
}
func (e RateLimitError) Is(target error) bool { return target == ErrRateLimited }

// Attract makes the caller's newest message in a channel sparkle for every
// IRCord user viewing it. Only channel operators and above may use it.
func (m *Manager) Attract(owner, conversation string) (Message, error) {
	w := m.space(owner)
	w.mu.Lock()
	conv, exists := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	isupport := w.networks[conv.NetworkID].ISupport
	operator := channelAt(conv, isupport, w.self.ID, 'o')
	deleted, storageErr, selfID := w.deleted, w.storageErr, w.self.ID
	var target Message
	for i := len(w.messages[conversation]) - 1; i >= 0; i-- {
		if msg := w.messages[conversation][i]; msg.SenderID == selfID && addressable(msg) {
			target = clone(msg)
			break
		}
	}
	store := w.store
	w.mu.Unlock()
	if !exists || deleted {
		return Message{}, ErrNotFound
	}
	if storageErr != nil {
		return Message{}, ErrStorage
	}
	if conv.Kind != "channel" {
		return Message{}, fmt.Errorf("/attract works inside a channel")
	}
	if c == nil || !conv.Joined {
		return Message{}, ErrDisconnected
	}
	c.mu.Lock()
	ready := c.ready
	supported := c.features(isupport).Attract
	c.mu.Unlock()
	if !ready {
		return Message{}, ErrDisconnected
	}
	if !supported {
		return Message{}, ErrAttractUnsupported
	}
	if !operator {
		return Message{}, ErrAttractNeedsOperator
	}
	if target.ID == "" && store != nil {
		// The in-memory ring can be empty after a restart; the store is authoritative.
		if page, err := store.History(owner, conversation, "", 100); err == nil {
			for _, msg := range page.Items {
				if msg.SenderID == selfID && addressable(msg) && (target.ID == "" || msg.CreatedAt.After(target.CreatedAt)) {
					target = msg
				}
			}
		}
	}
	if target.ID == "" {
		return Message{}, ErrAttractNoMessage
	}
	now := time.Now()
	c.mu.Lock()
	if c.attractSent == nil {
		c.attractSent = map[string]time.Time{}
	}
	if last, ok := c.attractSent[conversation]; ok && now.Sub(last) < attractCooldown {
		c.mu.Unlock()
		return Message{}, RateLimitError{Wait: attractCooldown - now.Sub(last), Scope: "in this channel"}
	}
	c.attractRecent = slices.DeleteFunc(c.attractRecent, func(at time.Time) bool { return now.Sub(at) >= attractBurstWindow })
	if len(c.attractRecent) >= attractBurst {
		c.mu.Unlock()
		return Message{}, RateLimitError{Wait: attractBurstWindow - now.Sub(c.attractRecent[0]), Scope: "on this network"}
	}
	for id, at := range c.attractSent {
		if now.Sub(at) >= attractCooldown {
			delete(c.attractSent, id)
		}
	}
	c.attractSent[conversation] = now
	c.attractRecent = append(c.attractRecent, now)
	c.mu.Unlock()
	if err := c.enqueue(&girc.Event{Command: "TAGMSG", Params: []string{c.target(conv)}, Tags: girc.Tags{attractTag: target.UpstreamID}}); err != nil {
		c.mu.Lock()
		delete(c.attractSent, conversation)
		c.attractRecent = c.attractRecent[:max(0, len(c.attractRecent)-1)]
		c.mu.Unlock()
		return Message{}, err
	}
	// The echo is dropped on receipt; the sender's own tabs play it from here.
	w.mu.Lock()
	w.publishEphemeral(model.AttractEvent{V: 1, Type: "attract", Data: model.AttractData{ConversationID: conv.ID, MemberID: selfID, MessageID: target.ID}})
	w.mu.Unlock()
	return target, nil
}

// attract turns a verified incoming TAGMSG into an ephemeral event. Anything
// that is not a live, channel-wide request from a channel operator about their
// own message is dropped.
func (c *connection) attract(e girc.Event) {
	reference, ok := e.Tags[attractTag]
	if !ok || reference == "" || e.Source == nil || len(e.Params) != 1 || !girc.IsValidChannel(e.Params[0]) {
		return
	}
	c.mu.Lock()
	historical := c.historyBatches[e.Tags["batch"]]
	c.mu.Unlock()
	if historical || c.fold(e.Source.Name) == c.fold(c.currentNick()) {
		return
	}
	conversation := c.conversationID(e.Params[0])
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	conv, ok := w.conversations[conversation]
	member, known := w.members[c.nicks[c.fold(e.Source.Name)]]
	if !ok || !known || conv.Kind != "channel" || !conv.Joined || !slices.Contains(conv.MemberIDs, member.ID) {
		return
	}
	if !channelAt(conv, w.networks[c.config.ID].ISupport, member.ID, 'o') {
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
	msg, found := c.lookupMessage(conv.ID, reference)
	if !found || msg.SenderID != member.ID || !addressable(msg) {
		return
	}
	now := time.Now()
	senderKey, channelKey := "m:"+conversation+":"+member.ID, "c:"+conversation
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attractReceived == nil {
		c.attractReceived = map[string]time.Time{}
	}
	if at, ok := c.attractReceived[senderKey]; ok && now.Sub(at) < attractRepeat {
		return
	}
	if at, ok := c.attractReceived[channelKey]; ok && now.Sub(at) < attractEffect {
		return
	}
	if len(c.attractReceived) >= 4096 {
		for key, at := range c.attractReceived {
			if now.Sub(at) >= attractRepeat {
				delete(c.attractReceived, key)
			}
		}
		if len(c.attractReceived) >= 4094 {
			return
		}
	}
	c.attractReceived[senderKey], c.attractReceived[channelKey] = now, now
	w.publishEphemeral(model.AttractEvent{V: 1, Type: "attract", Data: model.AttractData{ConversationID: conv.ID, MemberID: member.ID, MessageID: msg.ID}})
}
