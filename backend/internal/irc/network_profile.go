package irc

import (
	"strings"

	"ircord/backend/internal/model"
)

// networkSelf is called with w.mu held. Never borrow another network's profile,
// including the legacy account profile. The ID stays stable for message ownership.
func (w *workspace) networkSelf(network string) Member {
	n := w.networks[network]
	if n.Profile != nil {
		return *n.Profile
	}
	p := Member{ID: w.self.ID, Nick: n.Nick, Realname: n.Realname, Username: n.Username, Color: "lavender", Role: "member", Status: "offline", Device: w.device}
	resolveMemberName(&p)
	return p
}

func (w *workspace) networkMember(network, member string) Member {
	if member == w.self.ID {
		return w.networkSelf(network)
	}
	return w.members[member]
}

func (w *workspace) saveNetworkSelf(network string, p Member) {
	n, ok := w.networks[network]
	if !ok {
		return
	}
	p.ID = w.self.ID
	n.Profile = &p
	w.networks[network] = n
	w.emit("network.updated", n)
}

func (m *Manager) NetworkProfile(owner, network string) (Member, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.configs[network]; !ok || w.deleted {
		return Member{}, ErrNotFound
	}
	if _, ok := w.networks[network]; !ok {
		return Member{}, ErrNotFound
	}
	return clone(w.networkSelf(network)), nil
}

func (m *Manager) SetNetworkProfile(owner, network string, p Profile) (Member, error) {
	return m.setProfile(owner, network, p)
}

// A profile and its publication history belong to one endpoint and services
// account. Nicknames and passwords may change without replacing that identity.
func sameProfileScope(a, b NetworkConfig) bool {
	account := func(c NetworkConfig) string {
		if c.Auth != nil {
			if c.Auth.Method == "" || c.Auth.Method == "none" {
				return "none:"
			}
			return c.Auth.Method + ":" + strings.ToLower(c.Auth.Account)
		}
		if c.SASLUser != "" {
			return "sasl-plain:" + strings.ToLower(c.SASLUser)
		}
		if c.ServerPassword != "" {
			return "server-password:"
		}
		return "none:"
	}
	return strings.EqualFold(a.Host, b.Host) && a.Port == b.Port && a.TLS == b.TLS && account(a) == account(b)
}

// prepareNetworkProfile retains reconnect state, but starts a new server or
// services account with a clean profile. Called with w.mu held before saving cfg.
func (w *workspace) prepareNetworkProfile(n *Network, cfg NetworkConfig) error {
	old, exists := w.configs[cfg.ID]
	if exists && sameProfileScope(old, cfg) {
		n.Profile = w.networks[cfg.ID].Profile
	} else if exists {
		if w.store != nil {
			if err := w.store.ResetProfilePublication(w.ownerID, cfg.ID); err != nil {
				return ErrStorage
			}
		}
		delete(w.profileImports, cfg.ID)
		n.Profile = nil
	}
	if n.Profile == nil {
		p := Member{ID: w.self.ID, Nick: cfg.Nick, Realname: cfg.Realname, Username: cfg.Username, Color: "lavender", Role: "member", Status: "offline", Device: w.device}
		resolveMemberName(&p)
		n.Profile = &p
	}
	return nil
}

// loadNetworkProfiles runs after every entity and configuration has loaded.
// This ordering makes a legacy import independent of entity map iteration order.
func (w *workspace) loadNetworkProfiles() error {
	w.profileImports = map[string]map[string]bool{}
	for network, cfg := range w.configs {
		n, exists := w.networks[network]
		if !exists {
			n = Network{ID: network, Name: cfg.Name, Host: cfg.Host, Port: cfg.Port, TLS: cfg.TLS, Nick: cfg.Nick, Status: "disconnected", Initials: "I", Color: "mint", Capabilities: []string{}}
			applyConfig(&n, cfg)
		}
		imports, err := w.store.ProfileImports(w.ownerID, network)
		if err != nil {
			return err
		}
		// SaveNetwork precedes the network event. If a process stopped between
		// those writes, never attach an old profile to the replacement identity.
		prior := NetworkConfig{Host: n.Host, Port: n.Port, TLS: n.TLS, Auth: &model.AuthInput{Method: n.Auth.Method, Account: n.Auth.Account}}
		if exists && n.Profile != nil && !sameProfileScope(prior, cfg) {
			if err := w.store.ResetProfilePublication(w.ownerID, network); err != nil {
				return err
			}
			p := Member{ID: w.self.ID, Nick: cfg.Nick, Realname: cfg.Realname, Username: cfg.Username, Color: "lavender", Role: "member", Status: "offline"}
			resolveMemberName(&p)
			n.Profile = &p
			imports = map[string]bool{}
			applyConfig(&n, cfg)
			n.Name, n.Host, n.Port, n.TLS, n.Nick = cfg.Name, cfg.Host, cfg.Port, cfg.TLS, cfg.Nick
			w.networks[network] = n
			w.emit("network.updated", n)
			if w.storageErr != nil {
				return w.storageErr
			}
		}
		if n.Profile == nil {
			p := w.self
			p.Nick, p.Realname, p.Username = cfg.Nick, cfg.Realname, cfg.Username
			p.Account, p.Host, p.AwayMessage, p.Device = "", "", "", ""
			p.Status = "offline"
			resolveMemberName(&p)
			n.Profile = &p
			for _, key := range metadataKeys {
				imports[key] = true
			}
			if err := w.store.SaveProfileImports(w.ownerID, network, imports); err != nil {
				return err
			}
			w.networks[network] = n
			w.emit("network.updated", n)
			if w.storageErr != nil {
				return w.storageErr
			}
		} else {
			p := *n.Profile
			p.ID, p.Status, p.AwayMessage, p.Account, p.Device = w.self.ID, "offline", "", "", ""
			n.Profile = &p
			w.networks[network] = n
		}
		w.profileImports[network] = imports
	}
	return nil
}
