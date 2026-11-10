package irc

import (
	"github.com/lrstanley/girc"
	"strings"
)

func (m *Manager) partyServer(cfg NetworkConfig) bool {
	home := m.options.Home
	return home.ID != "" && strings.EqualFold(cfg.Host, home.Host) && cfg.Port == home.Port && cfg.TLS == home.TLS
}
func generatedPartyTarget(target string) bool {
	if len(target) != 40 || !strings.HasPrefix(strings.ToLower(target), "#ircord-") {
		return false
	}
	for _, r := range strings.ToLower(target[8:]) {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
func (c *connection) managedPartyTarget(target string) bool {
	return c.manager != nil && c.manager.partyServer(c.config) && c.manager.managedTarget(c.manager.options.Home.ID, target)
}
func (c *connection) denyPartyCommand(event *girc.Event) error {
	if c.manager == nil {
		return nil
	}
	if c.owner.ownerID == partyControlOwner {
		return nil
	}
	for _, param := range event.Params {
		for _, field := range strings.Fields(param) {
			for _, target := range strings.Split(field, ",") {
				if c.managedPartyTarget(strings.TrimLeft(target, ":@+%~&")) {
					return ErrForbidden
				}
			}
		}
	}
	// JOIN 0 and targetless PART variants can change managed memberships too.
	if event.Command == "JOIN" && len(event.Params) > 0 && event.Params[0] == "0" {
		return ErrForbidden
	}
	return nil
}
func (m *Manager) validatePartyConfig(owner string, cfg NetworkConfig) error {
	if owner == partyControlOwner {
		return nil
	}
	c := &connection{manager: m, owner: &workspace{ownerID: owner}, config: cfg}
	for _, entry := range append(append([]string{}, cfg.Channels...), cfg.AutoJoin...) {
		fields := strings.Fields(entry)
		if len(fields) > 0 && c.managedPartyTarget(fields[0]) {
			return ErrForbidden
		}
	}
	for _, line := range cfg.Perform {
		if event, e := rawEvent(line); e == nil {
			if e = c.denyPartyCommand(event); e != nil {
				return e
			}
		}
	}
	return nil
}

func (c *connection) partyTargetAccess(target string) error {
	if c.owner.ownerID == partyControlOwner {
		return nil
	}
	if c.manager.options.Store == nil || !c.manager.partyServer(c.config) || c.config.ID != c.manager.options.Home.ID {
		return ErrForbidden
	}
	party, e := c.manager.options.Store.PartyByTarget(target)
	if e != nil {
		return ErrForbidden
	}
	return c.manager.partyAccess(c.owner.ownerID, party.ConversationID)
}

// A membership/protection change can invalidate a send after HTTP enqueues it.
// Report a known unsent command immediately; only a partially sent batch is uncertain.
func (c *connection) rejectPartySend(events []*girc.Event, sent bool) {
	for _, event := range events {
		clientID := event.Tags["+ircord/client-id"]
		if clientID == "" {
			clientID = event.Tags["label"]
		}
		c.mu.Lock()
		_, pending := c.pending[clientID]
		delete(c.pending, clientID)
		c.mu.Unlock()
		if !pending {
			continue
		}
		c.owner.mu.Lock()
		if sent {
			c.owner.commandUncertain(clientID, "The party changed while sending. Check the conversation before retrying.")
		} else {
			c.owner.commandError(clientID, "The party changed before this message was sent. Try again when the conversation is available.")
		}
		c.owner.mu.Unlock()
	}
}

// AuthorizeConversation also protects API adapters which fetch derived media.
func (m *Manager) AuthorizeConversation(owner, conversation string) error {
	return m.partyAccess(owner, conversation)
}
