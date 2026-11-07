package irc

import (
	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"strings"
	"time"
)

func (c *connection) receiveInvitation(e girc.Event) {
	if len(e.Params) < 2 || e.Source == nil || !girc.IsValidChannel(e.Params[1]) || !clean(e.Params[1], 100) || c.fold(e.Params[0]) != c.fold(c.currentNick()) {
		return
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	key := c.fold(e.Params[1])
	if v, ok := w.conversations[c.conversationID(e.Params[1])]; ok && v.Joined {
		return
	}
	pending := 0
	for _, v := range w.invitations {
		if v.NetworkID == c.config.ID && v.ChannelKey == key {
			return
		}
		if !v.Rejected {
			pending++
		}
	}
	// Bound pending UI work and reject-mask storage per account. Existing masks are never evicted.
	if pending >= 20 || len(w.invitations) >= 10000 {
		return
	}
	if w.policy.Ignored(e.Source.Name, e.Source.Ident, e.Source.Host) {
		return
	}
	v := model.Invitation{ID: id(), NetworkID: c.config.ID, Channel: e.Params[1], ChannelKey: key, InviterID: c.config.ID + ":" + c.fold(e.Source.Name), InviterNick: e.Source.Name, CreatedAt: time.Now().UTC()}
	w.emit("invitation.received", v)
	if w.storageErr == nil {
		w.invitations[v.ID] = v
	}
}

func (m *Manager) RejectInvitation(owner, invitation string) error {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	v, ok := w.invitations[invitation]
	if !ok {
		return ErrNotFound
	}
	if v.Rejected {
		return nil
	}
	w.emit("invitation.removed", model.InvitationRemoval{ID: v.ID, Rejected: true})
	if w.storageErr != nil {
		return ErrStorage
	}
	v.Rejected = true
	w.invitations[v.ID] = v
	return nil
}

// Called with the workspace lock only after the IRC roster confirms our JOIN.
func (w *workspace) acceptInvitation(network, key string) {
	for id, v := range w.invitations {
		if !v.Rejected && v.NetworkID == network && strings.EqualFold(v.ChannelKey, key) {
			w.emit("invitation.removed", model.InvitationRemoval{ID: id})
			if w.storageErr == nil {
				delete(w.invitations, id)
			}
		}
	}
}
