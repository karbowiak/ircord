package irc

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

// Network reads one account-owned network without rebuilding its workspace.
func (m *Manager) Network(owner, id string) (Network, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	network, ok := w.networks[id]
	if !ok || w.deleted {
		return Network{}, ErrNotFound
	}
	return clone(network), nil
}

func normalizeConfig(cfg NetworkConfig) (NetworkConfig, error) {
	cfg = clone(cfg)
	if err := validateNickname(cfg.Nick, defaultNickLength); err != nil {
		return cfg, err
	}
	if cfg.AltNick != "" {
		if err := validateNickname(cfg.AltNick, defaultNickLength); err != nil {
			return cfg, fmt.Errorf("alternate nickname: %w", err)
		}
	}
	if cfg.Username == "" {
		cfg.Username = "ircord"
	}
	if cfg.Realname == "" {
		cfg.Realname = "IRCord"
	}
	if !clean(cfg.Username, 32) || strings.ContainsAny(cfg.Username, " :@") || !clean(cfg.Realname, 128) || cfg.AltNick != "" && !girc.IsValidNick(cfg.AltNick) {
		return cfg, fmt.Errorf("invalid network identity")
	}
	if len(cfg.AutoJoin) > 100 || len(cfg.Channels) > 100 || len(cfg.Perform) > 20 {
		return cfg, fmt.Errorf("too many auto-join channels or perform lines")
	}
	if cfg.AutoJoin != nil {
		cfg.Channels = append([]string{}, cfg.AutoJoin...)
	}
	for _, entry := range cfg.Channels {
		fields := strings.Fields(entry)
		if len(fields) < 1 || len(fields) > 2 || !girc.IsValidChannel(fields[0]) || !clean(entry, 200) || strings.ContainsAny(fields[0], " ,:") {
			return cfg, fmt.Errorf("invalid auto-join channel")
		}
	}
	for _, line := range cfg.Perform {
		event, err := rawEvent(line)
		if event != nil && (event.Command == "OPER" || event.Command == "PRIVMSG" && len(event.Params) > 0 && strings.EqualFold(event.Params[0], "NickServ")) {
			return cfg, fmt.Errorf("use authentication settings for credentials, rather than perform lines")
		}
		if err != nil {
			return cfg, fmt.Errorf("invalid perform line: %w", err)
		}
	}
	if cfg.Auth != nil {
		cfg.SASLUser = ""
		cfg.SASLPassword = ""
		cfg.ServerPassword = ""
		password := ""
		if cfg.Auth.Password != nil {
			password = *cfg.Auth.Password
		}
		if !clean(password, 512) || !clean(cfg.Auth.Account, 100) {
			return cfg, fmt.Errorf("invalid authentication settings")
		}
		switch cfg.Auth.Method {
		case "none":
			cfg.Auth.Account = ""
			cfg.Auth.Password = nil
		case "sasl-plain":
			if cfg.Auth.Account == "" {
				return cfg, fmt.Errorf("SASL requires an account and password")
			}
			cfg.SASLUser = cfg.Auth.Account
			cfg.SASLPassword = password
		case "server-password":

			cfg.ServerPassword = password
		case "nickserv":

		default:
			return cfg, fmt.Errorf("unsupported authentication method")
		}
	}
	return cfg, nil
}
func applyConfig(n *Network, cfg NetworkConfig) {
	n.AltNick = cfg.AltNick
	n.Username = cfg.Username
	n.Realname = cfg.Realname
	n.AutoJoin = append([]string{}, cfg.Channels...)
	n.Perform = append([]string{}, cfg.Perform...)
	n.Auth = model.NetworkAuth{Method: "none"}
	if cfg.Auth != nil {
		n.Auth.Method = cfg.Auth.Method
		n.Auth.Account = cfg.Auth.Account
		n.Auth.PasswordSet = cfg.Auth.Password != nil && *cfg.Auth.Password != ""
	} else if cfg.SASLUser != "" {
		n.Auth = model.NetworkAuth{Method: "sasl-plain", Account: cfg.SASLUser, PasswordSet: cfg.SASLPassword != ""}
	} else if cfg.ServerPassword != "" {
		n.Auth = model.NetworkAuth{Method: "server-password", PasswordSet: true}
	}
	if n.ISupport == nil {
		n.ISupport = map[string]string{}
	}
	if n.Features.Metadata == nil {
		n.Features.Metadata = []string{}
	}
	if n.Profile != nil {
		p := *n.Profile
		p.Nick, p.Username = n.Nick, cfg.Username
		applyRealname(&p, cfg.Realname)
		n.Profile = &p
	}
}
func (m *Manager) UpdateNetwork(owner, network string, cfg NetworkConfig) (Network, error) {
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	return m.updateNetwork(owner, network, cfg)
}

func (m *Manager) updateNetwork(owner, network string, cfg NetworkConfig) (Network, error) {
	if err := m.validatePartyConfig(owner, cfg); err != nil {
		return Network{}, err
	}
	w := m.space(owner)
	w.mu.Lock()
	old, ok := w.configs[network]
	active := w.connections[network] != nil
	w.mu.Unlock()
	if !ok {
		return Network{}, ErrNotFound
	}
	cfg = clone(cfg)
	// A retained write-only credential belongs to its original endpoint. In
	// particular an operator-provided default must not be redirected elsewhere.
	if !strings.EqualFold(cfg.Host, old.Host) || cfg.Port != old.Port || cfg.TLS != old.TLS {
		if cfg.Auth != nil && old.Auth != nil && cfg.Auth.Password != nil && old.Auth.Password != nil && *cfg.Auth.Password == *old.Auth.Password {
			cfg.Auth.Password = nil
		}
		if cfg.SASLPassword == old.SASLPassword {
			cfg.SASLPassword = ""
			cfg.SASLUser = ""
		}
		if cfg.ServerPassword == old.ServerPassword {
			cfg.ServerPassword = ""
		}
		old.Auth = nil
		old.SASLPassword = ""
		old.ServerPassword = ""
	}
	cfg.ID = network
	if cfg.Auth != nil && cfg.Auth.Password == nil && old.Auth != nil && old.Auth.Method == cfg.Auth.Method {
		cfg = clone(cfg)
		cfg.Auth.Password = old.Auth.Password
	}
	// Support previously stored legacy configurations without exposing their secrets.
	if cfg.Auth != nil && cfg.Auth.Password == nil {
		switch cfg.Auth.Method {
		case "sasl-plain":
			if old.SASLPassword != "" {
				p := old.SASLPassword
				cfg.Auth.Password = &p
			}
		case "server-password":
			if old.ServerPassword != "" {
				p := old.ServerPassword
				cfg.Auth.Password = &p
			}
		}
	}
	if w.store != nil {
		if err := w.store.InvalidateChannels(owner, network); err != nil {
			return Network{}, ErrStorage
		}
	}
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return Network{}, err
	}
	// Validate all configuration before disrupting the running connection.
	if normalized.Host == "" || !clean(normalized.Host, 253) || strings.ContainsAny(normalized.Host, " /\\") || normalized.Port < 1 || normalized.Port > 65535 || !girc.IsValidNick(normalized.Nick) || !clean(normalized.Name, 100) || (!normalized.TLS && !m.options.AllowPrivate) {
		return Network{}, fmt.Errorf("invalid network configuration")
	}
	missingSecret := normalized.Auth != nil && normalized.Auth.Method != "none" && (normalized.Auth.Password == nil || *normalized.Auth.Password == "")
	if active && missingSecret {
		if err = m.Disconnect(owner, network); err != nil {
			return Network{}, err
		}
		active = false
	}
	if active {
		if err = m.Disconnect(owner, network); err != nil {
			return Network{}, err
		}
		return m.Connect(owner, normalized)
	}
	if w.store != nil {
		if err = w.store.SaveNetwork(owner, normalized, false); err != nil {
			return Network{}, ErrStorage
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	n := w.networks[network]
	n.Name = normalized.Name
	n.Host = normalized.Host
	n.Port = normalized.Port
	n.TLS = normalized.TLS
	n.Nick = normalized.Nick
	if err := w.prepareNetworkProfile(&n, normalized); err != nil {
		return Network{}, err
	}
	applyConfig(&n, normalized)
	w.configs[network] = normalized
	w.networks[network] = n
	w.emit("network.updated", n)
	if w.storageErr != nil {
		return Network{}, ErrStorage
	}
	return clone(n), nil
}
func (m *Manager) DeleteNetwork(owner, network string) error {
	w := m.space(owner)
	w.mu.Lock()
	_, ok := w.networks[network]
	active := w.connections[network] != nil
	w.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	if active {
		if err := m.Disconnect(owner, network); err != nil {
			return err
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emit("network.removed", model.Removal{ID: network})
	if w.storageErr != nil {
		return ErrStorage
	}
	for id, v := range w.invitations {
		if v.NetworkID == network {
			delete(w.invitations, id)
		}
	}
	for id, v := range w.saved {
		if v.NetworkID == network {
			delete(w.saved, id)
		}
	}
	delete(w.networks, network)
	delete(w.configs, network)
	delete(w.info, network)
	delete(w.latency, network)
	delete(w.serverLogs, network)
	delete(w.trace, network)
	for ch := range w.traceSubscribers[network] {
		close(ch)
	}
	delete(w.traceSubscribers, network)
	for key, v := range w.conversations {
		if v.NetworkID == network {
			delete(w.conversations, key)
			delete(w.messages, key)
		}
	}
	for key := range w.members {
		if strings.HasPrefix(key, network+":") {
			delete(w.members, key)
		}
	}
	return nil
}

func (m *Manager) PatchNetwork(owner, network string, fields map[string]json.RawMessage) (Network, error) {
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	w.mu.Lock()
	cfg, ok := w.configs[network]
	cfg = clone(cfg)
	w.mu.Unlock()
	if !ok {
		return Network{}, ErrNotFound
	}
	allowed := []string{"name", "host", "port", "tls", "nick", "altNick", "username", "realname", "auth", "autoJoin", "perform", "channels"}
	if len(fields) == 0 {
		return Network{}, fmt.Errorf("provide a network field to update")
	}
	for key := range fields {
		if !slices.Contains(allowed, key) {
			return Network{}, fmt.Errorf("unknown network field")
		}
	}
	if raw, ok := fields["auth"]; ok {
		var next model.AuthInput
		if string(raw) == "null" || json.Unmarshal(raw, &next) != nil {
			return Network{}, fmt.Errorf("invalid authentication settings")
		}
		if cfg.Auth != nil && cfg.Auth.Method == next.Method && next.Account == "" {
			next.Account = cfg.Auth.Account
		}
		cfg.Auth = &next
		delete(fields, "auth")
	}
	raw, _ := json.Marshal(fields)
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Network{}, err
	}
	return m.updateNetwork(owner, network, cfg)
}

// UpdateHomePassword never installs an account password on a repointed network.
// Hold the configuration lock across validation, persistence and reconnection.
func (m *Manager) UpdateHomePassword(owner string, home NetworkConfig, username, password string) error {
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	w.mu.Lock()
	cfg, ok := w.configs[home.ID]
	cfg = clone(cfg)
	w.mu.Unlock()
	if !ok {
		return nil
	}
	if !strings.EqualFold(cfg.Host, home.Host) || cfg.Port != home.Port || cfg.TLS != home.TLS || cfg.Auth == nil || cfg.Auth.Method != "sasl-plain" || !strings.EqualFold(cfg.Auth.Account, username) {
		return nil
	}
	cfg.Auth.Password = &password
	_, err := m.updateNetwork(owner, home.ID, cfg)
	return err
}

// ProvisionHome persists the promised network before trying connection admission.
// A full connection pool leaves a saved network that the user can reconnect later.
func (m *Manager) ProvisionHome(owner string, cfg NetworkConfig) error {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return err
	}
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	w.mu.Lock()
	if _, exists := w.configs[cfg.ID]; exists {
		w.mu.Unlock()
		return nil
	}
	if w.deleted || w.storageErr != nil {
		w.mu.Unlock()
		return ErrStorage
	}
	if w.store != nil {
		if err := w.store.SaveNetwork(owner, normalized, true); err != nil {
			w.mu.Unlock()
			return ErrStorage
		}
	}
	n := Network{ID: cfg.ID, Name: cfg.Name, Host: cfg.Host, Port: cfg.Port, TLS: cfg.TLS, Nick: cfg.Nick, Initials: "I", Color: "mint", Status: "disconnected", Capabilities: []string{}}
	if err := w.prepareNetworkProfile(&n, normalized); err != nil {
		w.mu.Unlock()
		return err
	}
	applyConfig(&n, normalized)
	w.configs[cfg.ID], w.networks[cfg.ID] = normalized, n
	w.emit("network.updated", n)
	err = w.storageErr
	w.mu.Unlock()
	if err != nil {
		return ErrStorage
	}
	_, err = m.Connect(owner, normalized)
	if err == ErrBusy {
		return nil
	}
	return err
}
