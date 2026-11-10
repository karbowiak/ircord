package irc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

type partyMessageEvent struct {
	party   string
	message Message
	sender  Member
	updated bool
}

// capture runs only for the private controller. Persist before acknowledging a
// history batch, then enqueue bounded account delivery outside workspace locks.
func (p *partyManager) capture(w *workspace, kind string, data any) {
	if w.ownerID != partyControlOwner {
		return
	}
	msg, ok := data.(Message)
	if !ok || msg.UpstreamID == "" {
		return
	}
	v, e := p.store.PartyByConversation(msg.ConversationID)
	if e != nil {
		return
	}
	canonical := clone(msg)
	canonical.Account, canonical.ClientID = "", ""
	canonical.Historical = false
	canonical.Position = 0
	if old, e := p.store.PartyMessage(v.ID, msg.ID); e == nil && equal(old, canonical) {
		return
	}
	if e = p.store.SavePartyMessage(v.ID, canonical); e != nil {
		w.storageErr = e
		return
	}
	p.mu.Lock()
	if p.maintenance == nil {
		p.maintenance = map[string]int{}
	}
	p.maintenance[v.ID]++
	sweep := p.maintenance[v.ID]%100 == 0
	p.mu.Unlock()
	if sweep {
		if e := p.store.PrunePartyCanonical(v.ID); e != nil {
			w.storageErr = e
			return
		}
	}
	if msg.Historical {
		return
	}
	event := partyMessageEvent{party: v.ID, message: clone(msg), sender: clone(w.networkMember(v.NetworkID, msg.SenderID)), updated: kind == "message.updated"}
	select {
	case p.events <- event:
	default:
		// Canonical bytes are durable. Force reconciliation/materialization when
		// delivery falls behind, rather than allocate unbounded goroutines.
		p.mu.Lock()
		if p.dirty == nil {
			p.dirty = map[string]bool{}
		}
		p.dirty[v.ID] = true
		p.mu.Unlock()
	}
}
func (p *partyManager) run() {
	defer p.m.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	// Restore runs from durable records without waiting for an account to log in.
	p.restore()
	for {
		select {
		case <-p.m.ctx.Done():
			return
		case event := <-p.events:
			p.deliver(event)
		case event := <-p.native:
			p.applyNative(event)
		case <-ticker.C:
			p.restore()
		case <-p.wake:
			p.restore()
		}
	}
}
func (p *partyManager) restore() {
	sweep := time.Now().After(p.nextMaintenance)
	if sweep {
		p.nextMaintenance = time.Now().Add(time.Minute)
	}
	all, e := p.store.Parties()
	if e != nil {
		return
	}
	for _, v := range all {
		if sweep {
			_ = p.store.PrunePartyHistory(v.ID)
		}
		if p.m.ctx.Err() != nil {
			return
		}
		if v.State == "archived" {
			continue
		}
		connected := p.ready(v.ID)
		if v.State == "active" && connected {
			if invitations, e := p.store.PartyInvitations(v.ID); e == nil {
				for _, invitation := range invitations {
					if invitation.State == "pending" {
						p.emitInvitation(invitation.Invitee, invitation.Invitation)
					}
				}
			}
			p.mu.Lock()
			dirty := p.dirty[v.ID]
			delete(p.dirty, v.ID)
			p.mu.Unlock()
			if dirty {
				ctx, cancel := context.WithTimeout(p.m.ctx, 10*time.Second)
				unlock, e := p.lock(ctx, v.ID)
				if e == nil {
					if current, e := p.store.Party(v.ID); e == nil && current.State == "active" {
						_ = p.materialize(current)
						_ = p.publish(current)
					}
					unlock()
				}
				cancel()
			}
			p.joinMembers(v)
			continue
		}
		ctx, cancel := context.WithTimeout(p.m.ctx, 90*time.Second)
		unlock, e := p.lock(ctx, v.ID)
		if e != nil {
			cancel()
			return
		}
		current, e := p.store.Party(v.ID)
		if e == nil && current.State == "active" && p.ready(current.ID) {
			unlock()
			cancel()
			p.joinMembers(current)
			continue
		}
		if e == nil {
			if current.State == "active" {
				current.State = "recovering"
				e = p.save(&current)
			}
			if e == nil {
				e = p.reconcile(ctx, &current)
			}
			if e == nil {
				_ = p.publish(current)
			}
		}
		unlock()
		cancel()
	}
}
func (p *partyManager) materialize(v storage.PartyRecord) error {
	if e := p.store.PrunePartyHistory(v.ID); e != nil {
		return ErrStorage
	}
	for _, member := range v.Members {
		local := p.localOwner(member.PrincipalID())
		if local == "" {
			continue
		}
		if e := p.store.MaterializeParty(local, v); e != nil {
			return ErrStorage
		}
	}
	return nil
}
func (p *partyManager) deliver(event partyMessageEvent) {
	ctx, cancel := context.WithTimeout(p.m.ctx, 10*time.Second)
	defer cancel()
	unlock, e := p.lock(ctx, event.party)
	if e != nil {
		return
	}
	defer unlock()
	v, e := p.store.Party(event.party)
	if e != nil || v.State != "active" {
		return
	}
	canonical, revision, e := p.store.PartyMessageVersion(v.ID, event.message.ID)
	if e != nil {
		return
	}
	canonical.Historical = event.message.Historical
	event.message = canonical
	if _, real := p.transport.(*partyIRCController); real && !p.ready(v.ID) {
		return
	}
	for _, member := range v.Members {
		local := p.localOwner(member.PrincipalID())
		if local == "" {
			continue
		}
		w := p.m.space(local)
		w.mu.Lock()
		if w.deleted {
			w.mu.Unlock()
			continue
		}
		conv, ok := w.conversations[v.ConversationID]
		if !ok {
			w.mu.Unlock()
			continue
		}
		msg := clone(event.message)
		msg.Account = ""
		if strings.EqualFold(msg.SenderAccount, member.Username) {
			msg.Account = msg.SenderAccount
		}
		msg.Mention = msg.SenderID != w.self.ID
		existing, lookupErr := p.store.Message(member.LocalAccountID(), v.ConversationID, msg.ID)
		fresh := errors.Is(lookupErr, storage.ErrNotFound)
		if lookupErr != nil && !fresh {
			w.mu.Unlock()
			continue
		}
		// User echo owns its outbox correlation. Canonical copies retain that
		// account's client ID while sharing every content revision.
		if !fresh {
			msg.ClientID = existing.ClientID
		}
		if !fresh && msg.Deleted {
			adapter := &connection{manager: p.m, owner: w, config: NetworkConfig{ID: v.NetworkID}}
			adapter.applyRedaction(conv, existing, false)
			if w.storageErr == nil {
				_ = p.store.MarkPartyCopy(v.ID, member.LocalAccountID(), msg.ID, revision)
			}
			w.mu.Unlock()
			continue
		}
		if !fresh && (!equal(msg.Edits, existing.Edits) || msg.EditedAt != nil) {
			adapter := &connection{manager: p.m, owner: w, config: NetworkConfig{ID: v.NetworkID}}
			adapter.saveEdited(conv, msg, false)
			if w.storageErr == nil {
				_ = p.store.MarkPartyCopy(v.ID, member.LocalAccountID(), msg.ID, revision)
			}
			w.mu.Unlock()
			continue
		}
		sender := event.sender
		if sender.ID == member.AccountID {
			sender = w.networkSelf(v.NetworkID)
		} else {
			w.members[sender.ID] = sender
			w.emit("member.updated", sender)
		}
		for _, participant := range v.Members {
			if participant.AccountID == sender.ID && sender.DisplayName == "" {
				sender.DisplayName = participant.DisplayName
				sender.Nick = participant.Username
			}
		}
		kind := "message.updated"
		if fresh {
			kind = "message.created"
		}
		w.emit(kind, msg)
		if w.storageErr == nil {
			_ = p.store.MarkPartyCopy(v.ID, member.LocalAccountID(), msg.ID, revision)
		}
		if fresh {
			w.messages[msg.ConversationID] = append(w.messages[msg.ConversationID], msg)
		} else {
			for i, old := range w.messages[msg.ConversationID] {
				if old.ID == msg.ID {
					w.messages[msg.ConversationID][i] = msg
				}
			}
		}
		if len(w.messages[msg.ConversationID]) > 500 {
			w.messages[msg.ConversationID] = w.messages[msg.ConversationID][len(w.messages[msg.ConversationID])-500:]
		}
		conv = w.refreshUnread(conv)
		w.conversations[conv.ID] = conv
		w.emitCounts(conv)
		if fresh && !event.updated && !msg.Historical {
			p.m.notifyMessage(w, conv, msg, sender, member.Username)
			p.m.recordInbox(w, conv, msg, sender, member.Username, false)
		}
		w.mu.Unlock()
	}
}
func (p *partyManager) joinMembers(v storage.PartyRecord) {
	if v.State != "active" {
		return
	}
	for _, member := range v.Members {
		local := p.localOwner(member.PrincipalID())
		if local == "" {
			continue
		}
		w := p.m.space(local)
		w.mu.Lock()
		c := w.connections[v.NetworkID]
		network := w.networks[v.NetworkID]
		conv := w.conversations[v.ConversationID]
		w.mu.Unlock()
		if c == nil || network.Status != "connected" || !strings.EqualFold(network.Account, member.Username) || !c.home() {
			continue
		}
		c.mu.Lock()
		client := c.client
		delete(c.closedTargets, c.foldUnlocked(v.Channel))
		c.mu.Unlock()
		if client != nil && client.LookupChannel(v.Channel) != nil {
			continue
		}
		_ = c.enqueue(&girc.Event{Command: "JOIN", Params: []string{v.Channel}})
		_ = conv
	}
}

func (m *Manager) leaveAllParties(owner string) error {
	if m.parties == nil {
		return nil
	}
	all, e := m.parties.store.Parties()
	if e != nil {
		return ErrStorage
	}
	for _, v := range all {
		if v.HasMember(m.options.Store.PartyPrincipal(owner)) {
			ctx, cancel := context.WithTimeout(m.ctx, 90*time.Second)
			e = m.LeaveParty(ctx, owner, v.ID)
			cancel()
			if e != nil {
				return e
			}
		}
	}
	return nil
}

func conversationPolicyKind(conv Conversation) string {
	if conv.Party != nil {
		return "direct"
	}
	return conv.Kind
}

func (p *partyManager) ready(party string) bool {
	p.mu.Lock()
	c := p.control
	protected := p.protected[party]
	p.mu.Unlock()
	if c == nil {
		return false
	}
	c.mu.Lock()
	ready, client := c.ready, c.client
	c.mu.Unlock()
	c.owner.mu.Lock()
	storageOK := c.owner.storageErr == nil
	c.owner.mu.Unlock()
	return ready && storageOK && client != nil && client == protected
}
