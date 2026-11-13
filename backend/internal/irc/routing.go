package irc

import (
	"regexp"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/activity"
)

// The workspace's side of activity routing (#84) and automatic presence (#136). These are the
// activity.Source methods; the router calls them without holding its own lock.

// deviceKey is the draft/metadata-3 key carrying the form factor of the device an IRCord user
// is on: "phone" or "tablet", unset on a desktop or when no device is connected. It is a
// client-specific key, so other clients simply ignore it, and it is only sent and read when
// the network negotiated draft/metadata-3.
const deviceKey = "ircord/device"

// autoAwayReason is the AWAY message other IRC users see while no client is active.
const autoAwayReason = "Away (no active IRCord client)"

// subscribedKeys are the metadata keys IRCord subscribes to: the profile keys and the device.
func subscribedKeys() []string { return append(append([]string{}, metadataKeys...), deviceKey) }

// Delays are the account's escalation thresholds.
func (m *Manager) Delays(owner string) activity.Delays {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	target, all := w.policy.Delays()
	return activity.Delays{Target: target, All: all}
}

// AutoAwayDelay is how long without an active client the account is marked away; 0 never.
func (m *Manager) AutoAwayDelay(owner string) time.Duration {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return 0
	}
	return time.Duration(w.policy.Settings.AutoAwayDelay) * time.Second
}

// Eligible rechecks an item right before an escalation alerts anyone with it: the message must
// still exist, be unread and not deleted, its sender must not have been ignored since, and the
// conversation's level and quiet hours must still allow it.
func (m *Manager) Eligible(owner string, item activity.Item) bool {
	if m.partyAccess(owner, item.Conversation) != nil {
		return false
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	conv, ok := w.conversations[item.Conversation]
	if !ok || w.deleted || w.policy.Quiet(time.Now()) || w.policy.Ignored(item.SenderNick, item.SenderUser, item.SenderHost) {
		return false
	}
	shared := ""
	if conv.ChannelSettings != nil {
		shared = conv.ChannelSettings.Notifications
	}
	switch w.policy.Rule(conv.ID, conversationPolicyKind(conv), shared) {
	case "none":
		return false
	case "mentions":
		if item.Kind != "mention" {
			return false
		}
	}
	if w.store != nil {
		unread, err := w.store.NotificationMessageUnread(owner, conv.ID, item.Message)
		return err == nil && unread
	}
	read, err := w.readTime(conv.ID)
	if err != nil || !item.At.After(read) {
		return false
	}
	for _, msg := range w.messages[conv.ID] {
		if msg.ID == item.Message {
			return !msg.Deleted
		}
	}
	if w.store != nil {
		msg, err := w.store.Message(w.ownerID, conv.ID, item.Message)
		return err == nil && !msg.Deleted
	}
	return false
}

// SetAutoAway marks the account away on every connected network when no client has been
// active for its delay, and back when one becomes active. It never touches an away the user set:
// a network that is already away is left alone (each network on its own), and only an away
// IRCord set automatically is cleared.
func (m *Manager) SetAutoAway(owner string, away bool) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return
	}
	w.autoAway = away
	for id, c := range w.connections {
		m.applyAutoAway(w, id, c)
	}
}

// applyAutoAway brings one connection in line with the account's automatic away. Requires w.mu.
func (m *Manager) applyAutoAway(w *workspace, network string, c *connection) {
	if w.autoAwaySent == nil {
		w.autoAwaySent = map[string]bool{}
	}
	switch {
	case w.autoAway && !w.autoAwaySent[network] && !w.awayOn[network]:
		if c.enqueue(&girc.Event{Command: "AWAY", Params: []string{autoAwayReason}}) == nil {
			w.autoAwaySent[network] = true
			w.setAwayText(network, autoAwayReason)
		}
	case !w.autoAway && w.autoAwaySent[network]:
		delete(w.autoAwaySent, network)
		_ = c.enqueue(&girc.Event{Command: "AWAY"})
	}
}

// manualAway records that the user set or cleared away on a network themselves, so coming back
// does not clear an away they chose.
// reason is the AWAY text sent ("" clears away).
func (m *Manager) manualAway(owner, network, reason string) {
	w := m.space(owner)
	w.mu.Lock()
	delete(w.autoAwaySent, network)
	w.setAwayText(network, reason)
	w.mu.Unlock()
}

// briefAwayText matches the AWAY text /afk and /brb send: "AFK" or "BRB", optionally followed
// by ": reason". It is plain AWAY, so every IRC client shows it; IRCord only reads the prefix.
var briefAwayText = regexp.MustCompile(`(?i)^(afk|brb)(:|$)`)

// setAwayText remembers the AWAY text last sent on a network, for the self member once the
// server confirms it, and whether it is a brief away that ends with the next message. Requires w.mu.
func (w *workspace) setAwayText(network, reason string) {
	if w.awayText == nil {
		w.awayText = map[string]string{}
	}
	if w.briefAway == nil {
		w.briefAway = map[string]bool{}
	}
	w.awayText[network] = reason
	w.briefAway[network] = briefAwayText.MatchString(reason)
}

// endBriefAway clears an /afk or /brb away on a network once the user sends a message there,
// the way coming back ends an automatic away. Any other away stays until /back.
func (m *Manager) endBriefAway(owner, network string, c *connection) {
	w := m.space(owner)
	w.mu.Lock()
	brief := w.briefAway[network]
	if brief {
		w.setAwayText(network, "")
	}
	w.mu.Unlock()
	if brief {
		_ = c.enqueue(&girc.Event{Command: "AWAY"})
	}
}

// SetDevice shares the form factor of the device the account is on ("phone", "tablet" or "").
func (m *Manager) SetDevice(owner, formFactor string) {
	if formFactor != activity.Phone && formFactor != activity.Tablet {
		formFactor = ""
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted || w.device == formFactor {
		return
	}
	w.device = formFactor
	w.self.Device = formFactor
	w.members[w.self.ID] = w.self
	w.emit("profile.updated", w.self)
	for network := range w.networks {
		p := w.networkSelf(network)
		p.Device = formFactor
		w.saveNetworkSelf(network, p)
	}
	for _, c := range w.connections {
		c.publishDevice(formFactor)
	}
}

// publishDevice shares the form factor on a connection that negotiated draft/metadata-3.
func (c *connection) publishDevice(formFactor string) {
	if !c.has("draft/metadata-3") {
		return
	}
	params := []string{"*", "SET", deviceKey}
	if formFactor != "" {
		params = append(params, formFactor)
	}
	_ = c.enqueue(&girc.Event{Command: "METADATA", Params: params})
}

// resumePresence re-applies automatic away and the device on a new connection: away and
// metadata do not survive reconnecting.
func (c *connection) resumePresence() {
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	// A new connection starts present, whatever the last one was.
	delete(w.awayOn, c.config.ID)
	if w.autoAway {
		if c.enqueue(&girc.Event{Command: "AWAY", Params: []string{autoAwayReason}}) == nil {
			if w.autoAwaySent == nil {
				w.autoAwaySent = map[string]bool{}
			}
			w.autoAwaySent[c.config.ID] = true
			w.setAwayText(c.config.ID, autoAwayReason)
		}
	} else if w.autoAwaySent != nil {
		delete(w.autoAwaySent, c.config.ID)
	}
	if w.device != "" {
		c.publishDevice(w.device)
	}
}

// Owners lists the workspaces loaded now, for the router to time auto-away after a restart.
func (m *Manager) Owners() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.owners))
	for owner := range m.owners {
		if !m.deletedOwners[owner] {
			out = append(out, owner)
		}
	}
	return out
}
