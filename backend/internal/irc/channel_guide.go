package irc

// Support for the channel guide (#258). The guide itself lives in the API layer; these helpers
// answer what it needs from the account's connection to the home network.

// HomeNetworkID is the ID of the IRCord home network, or "" when there is none.
func (m *Manager) HomeNetworkID() string { return m.options.Home.ID }

// HomeDefaultChannels are the configured channels every new account joined before a guide
// existed (IRCORD_HOME_NETWORK_CHANNELS).
func (m *Manager) HomeDefaultChannels() []string {
	return append([]string(nil), m.options.Home.AutoJoin...)
}

// IsHomeNetwork reports whether owner's network id is their connection to the home network.
func (m *Manager) IsHomeNetwork(owner, network string) bool {
	if m.options.Home.ID == "" || network != m.options.Home.ID {
		return false
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	cfg, ok := w.configs[network]
	return ok && !w.deleted && m.matchesHomeEndpoint(cfg)
}

// HomeChannel describes a channel conversation on the home network as the account sees it.
type HomeChannel struct {
	Name     string
	Nick     string
	Operator bool
}

// HomeChannelOf returns the home network channel behind a conversation, and whether the
// account holds operator status or higher there right now. Conversations on other networks,
// direct messages and parties are not found.
func (m *Manager) HomeChannelOf(owner, conversation string) (HomeChannel, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	conv, ok := w.conversations[conversation]
	if !ok || w.deleted || conv.Kind != "channel" || conv.Party != nil {
		return HomeChannel{}, ErrNotFound
	}
	cfg, ok := w.configs[conv.NetworkID]
	if !ok || conv.NetworkID != m.options.Home.ID || !m.matchesHomeEndpoint(cfg) {
		return HomeChannel{}, ErrNotFound
	}
	target := conv.Target
	if target == "" {
		target = "#" + conv.Name
	}
	c := w.connections[conv.NetworkID]
	op := c != nil && conv.Joined && channelAt(conv, w.networks[conv.NetworkID].ISupport, w.self.ID, 'o')
	return HomeChannel{Name: target, Nick: w.networks[conv.NetworkID].Nick, Operator: op}, nil
}

// HomeChannelsNSFW reports, keyed by the names given, which channels their operators marked as
// sensitive, using the account's own connection to the home network to find the shared
// settings. Without a connected home network nothing is reported.
func (m *Manager) HomeChannelsNSFW(owner string, channels []string) map[string]bool {
	out := map[string]bool{}
	home := m.options.Home.ID
	if home == "" {
		return out
	}
	w := m.space(owner)
	w.mu.Lock()
	c := w.connections[home]
	if c == nil || w.deleted || !m.matchesHomeEndpoint(c.config) {
		w.mu.Unlock()
		return out
	}
	support := w.networks[home].ISupport
	c.mu.Lock()
	bouncer := bouncerRoute(c.config, c.caps, support)
	c.mu.Unlock()
	scope := ""
	if bouncer {
		scope = w.ownerID
	}
	network := sharedNetworkKey(c.config, support["NETWORK"], scope)
	keys := make(map[string]string, len(channels))
	for _, channel := range channels {
		keys[channel] = c.fold(channel)
	}
	w.mu.Unlock()
	for channel, folded := range keys {
		if v, ok := m.sharedChannel(network, folded); ok && v.NSFW {
			out[channel] = true
		}
	}
	return out
}
