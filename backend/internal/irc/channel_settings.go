package irc

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"ircord/backend/internal/model"
)

// ErrNotChannelOperator rejects a channel-wide settings change from a member
// without operator (or higher) status in that channel.
var ErrNotChannelOperator = errors.New("changing channel settings needs channel operator status")

type ChannelSettings = model.ChannelSettings

// ChannelSettingsInput is a partial update; nil fields keep their value.
type ChannelSettingsInput struct {
	Notifications *string
	GroupMentions *string
	NSFW          *bool
}

func defaultChannelSettings() ChannelSettings {
	return ChannelSettings{Notifications: "default", GroupMentions: "ops"}
}

// sharedNetworkKey identifies the upstream network independently of the
// account-local network ID: the lower-cased host:port (as per-network
// passwords are keyed) plus the server's ISUPPORT NETWORK name, so two
// upstream networks behind one host:port do not share settings. A non-empty
// scope (the account ID) limits the key to one account; see bouncerRoute.
func sharedNetworkKey(cfg NetworkConfig, networkName, scope string) string {
	key := net.JoinHostPort(strings.ToLower(cfg.Host), strconv.Itoa(cfg.Port)) + "/" + strings.ToLower(networkName)
	if scope != "" {
		key += "/account:" + scope
	}
	return key
}

// bouncerRoute reports a connection through a bouncer (soju, ZNC), which
// serves several upstream networks on one host:port and picks one from the
// login (user/network). Its NETWORK name comes from whichever upstream the
// login reaches, so it cannot identify a network shared between accounts;
// such channels keep their settings to the one account.
func bouncerRoute(cfg NetworkConfig, caps map[string]bool, support map[string]string) bool {
	logins := []string{cfg.Username, cfg.SASLUser, cfg.ServerPassword}
	if cfg.Auth != nil {
		logins = append(logins, cfg.Auth.Account)
	}
	for _, v := range logins {
		if strings.Contains(v, "/") {
			return true
		}
	}
	for name := range caps {
		if name == "soju.im/bouncer-networks" || strings.HasPrefix(name, "znc.in/") {
			return true
		}
	}
	_, netID := support["BOUNCER_NETID"]
	return netID
}

// sharedChannelKey is the store key for a channel on c's upstream network.
// Callers hold w.mu; it takes c.mu briefly.
func (c *connection) sharedChannelKey(conv Conversation) (network, channel string) {
	w := c.owner
	support := w.networks[c.config.ID].ISupport
	c.mu.Lock()
	bouncer := bouncerRoute(c.config, c.caps, support)
	c.mu.Unlock()
	scope := ""
	if bouncer {
		scope = w.ownerID
	}
	return sharedNetworkKey(c.config, support["NETWORK"], scope), c.fold(c.target(conv))
}

// loadSharedChannels reads the stored settings once. A failed read is not
// remembered, so the next call retries. Callers hold sharedMu.
func (m *Manager) loadSharedChannels() error {
	if m.sharedLoaded {
		return nil
	}
	shared := map[string]ChannelSettings{}
	if m.options.Store != nil {
		rows, err := m.options.Store.SharedChannels()
		if err != nil {
			slog.Error("Loading channel settings failed; will retry", "error", err)
			return err
		}
		for _, row := range rows {
			v := defaultChannelSettings()
			if json.Unmarshal(row.Payload, &v) == nil {
				shared[row.Network+"\x00"+row.Channel] = normalChannelSettings(v)
			}
		}
	}
	m.shared, m.sharedLoaded = shared, true
	return nil
}

func normalChannelSettings(v ChannelSettings) ChannelSettings {
	switch v.Notifications {
	case "default", "all", "mentions", "none":
	default:
		v.Notifications = "default"
	}
	switch v.GroupMentions {
	case "all", "ops", "off":
	default:
		v.GroupMentions = "ops"
	}
	return v
}

// sharedChannel returns the stored settings or the defaults; ok is false when
// the settings cannot be read right now. Lock order: workspace mutex before
// sharedMu.
func (m *Manager) sharedChannel(network, channel string) (ChannelSettings, bool) {
	m.sharedMu.Lock()
	defer m.sharedMu.Unlock()
	if m.loadSharedChannels() != nil {
		return ChannelSettings{}, false
	}
	if v, ok := m.shared[network+"\x00"+channel]; ok {
		return v, true
	}
	return defaultChannelSettings(), true
}

// applyChannelSettings makes a joined channel carry the shared settings.
// When the settings cannot be
// read, the conversation keeps what it has rather than falling back to
// defaults. Callers hold w.mu.
func (c *connection) applyChannelSettings(conv *Conversation) {
	if conv.Party != nil {
		return
	}
	if conv.Kind != "channel" {
		return
	}
	network, channel := c.sharedChannelKey(*conv)
	conv.ChannelIdentity = &model.ChannelIdentity{Network: network, Channel: channel}
	value, ok := c.manager.sharedChannel(network, channel)
	if !ok {
		return
	}
	conv.ChannelSettings = &value
}

// clearChannelSettings removes channel policy from a channel this account is
// no longer in, so a non-member never sees values that look current.
func clearChannelSettings(conv *Conversation) {
	if conv.Kind == "channel" {
		conv.ChannelSettings = nil
	}
}

// SetChannelSettings changes a channel's shared settings. The caller must be
// joined and hold operator status or higher there right now; every joined
// IRCord member of the same upstream channel receives the new values.
func (m *Manager) SetChannelSettings(owner, conversation string, in ChannelSettingsInput) (Conversation, error) {
	if m.options.Store != nil {
		if party, e := m.options.Store.PartyByConversation(conversation); e == nil && party.State != "converted" {
			return Conversation{}, ErrForbidden
		}
	}
	if in.Notifications != nil {
		switch *in.Notifications {
		case "default", "all", "mentions", "none":
		default:
			return Conversation{}, fmt.Errorf("notifications must be default, all, mentions or none")
		}
	}
	if in.GroupMentions != nil {
		switch *in.GroupMentions {
		case "all", "ops", "off":
		default:
			return Conversation{}, fmt.Errorf("group mentions must be all, ops or off")
		}
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	if !ok || w.deleted || conv.Kind != "channel" {
		w.mu.Unlock()
		return Conversation{}, ErrNotFound
	}
	c := w.connections[conv.NetworkID]
	if c == nil || !conv.Joined {
		w.mu.Unlock()
		return Conversation{}, ErrNotChannelOperator
	}
	if !channelAt(conv, w.networks[conv.NetworkID].ISupport, w.self.ID, 'o') {
		w.mu.Unlock()
		return Conversation{}, ErrNotChannelOperator
	}
	network, channel := c.sharedChannelKey(conv)
	nick := w.networks[conv.NetworkID].Nick
	w.mu.Unlock()

	m.sharedMu.Lock()
	if err := m.loadSharedChannels(); err != nil {
		m.sharedMu.Unlock()
		return Conversation{}, ErrStorage
	}
	key := network + "\x00" + channel
	value, ok := m.shared[key]
	if !ok {
		value = defaultChannelSettings()
	}
	before := value
	if in.Notifications != nil {
		value.Notifications = *in.Notifications
	}
	if in.GroupMentions != nil {
		value.GroupMentions = *in.GroupMentions
	}
	if in.NSFW != nil {
		value.NSFW = *in.NSFW
	}
	if value.Notifications == before.Notifications && value.GroupMentions == before.GroupMentions && value.NSFW == before.NSFW {
		// Nothing changes: no write, no broadcast, and "last changed by" stays true.
		m.sharedMu.Unlock()
		w.mu.Lock()
		defer w.mu.Unlock()
		if conv, ok = w.conversations[conversation]; !ok || w.deleted {
			return Conversation{}, ErrNotFound
		}
		return clone(conv), nil
	}
	now := time.Now().UTC()
	value.UpdatedAt, value.UpdatedBy = &now, nick
	if m.options.Store != nil {
		raw, _ := json.Marshal(value)
		if err := m.options.Store.SaveSharedChannel(network, channel, raw); err != nil {
			m.sharedMu.Unlock()
			return Conversation{}, ErrStorage
		}
	}
	m.shared[key] = value
	m.sharedMu.Unlock()

	m.publishChannelSettings(network, channel)

	w.mu.Lock()
	defer w.mu.Unlock()
	conv, ok = w.conversations[conversation]
	if !ok || w.deleted {
		return Conversation{}, ErrNotFound
	}
	if w.storageErr != nil {
		return Conversation{}, ErrStorage
	}
	return clone(conv), nil
}

// publishChannelSettings delivers the current shared value to every workspace
// that retains the channel, through that account's own private event stream.
// Former members receive an unchanged conversation to invalidate Links, never
// the current shared settings. Closed conversations receive nothing.
func (m *Manager) publishChannelSettings(network, channel string) {
	m.mu.Lock()
	spaces := make([]*workspace, 0, len(m.owners))
	for _, w := range m.owners {
		spaces = append(spaces, w)
	}
	m.mu.Unlock()
	for _, w := range spaces {
		<-w.loaded
		w.mu.Lock()
		if !w.deleted {
			for id, conv := range w.conversations {
				c := w.connections[conv.NetworkID]
				if conv.Kind != "channel" || conv.Party != nil {
					continue
				}
				identity := conv.ChannelIdentity
				ready := false
				if c != nil {
					c.mu.Lock()
					ready = c.ready
					c.mu.Unlock()
				}
				if conv.Joined && ready {
					n, ch := c.sharedChannelKey(conv)
					identity = &model.ChannelIdentity{Network: n, Channel: ch}
				}
				if identity == nil || identity.Network != network || identity.Channel != channel {
					continue
				}
				before := conv.ChannelSettings
				if conv.Joined && c != nil {
					value, ok := m.sharedChannel(network, channel)
					if !ok {
						continue
					}
					conv.ChannelIdentity, conv.ChannelSettings = identity, &value
					if before != nil && equal(*before, value) {
						continue
					}
				} else {
					clearChannelSettings(&conv)
				}
				w.conversations[id] = conv
				w.emit("conversation.updated", conv)
			}
		}
		w.mu.Unlock()
	}
}
