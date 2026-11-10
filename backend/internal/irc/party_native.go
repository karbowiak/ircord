package irc

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

type partyNativeEvent struct {
	party, nick, account string
	join                 bool
	client               *girc.Client
}
type partyNativeInvite struct {
	nick, channel string
	client        *girc.Client
	offeredAt     time.Time
}

// The controller uses ordinary IRC INVITE; the recipient's explicit JOIN is
// acceptance. Offline invitations remain in the existing durable invite ledger.
func (p *partyManager) inviteNative(principal string, invitation model.Invitation) {
	if invitation.Party == nil || p.m.options.PartyAccounts == nil {
		return
	}
	p.mu.Lock()
	c := p.control
	p.mu.Unlock()
	if c == nil {
		return
	}
	c.mu.Lock()
	client, ready := c.client, c.ready
	c.mu.Unlock()
	if !ready || client == nil {
		return
	}
	v, e := p.store.Party(invitation.Party.ID)
	if e != nil || v.State != "active" {
		return
	}
	candidate, e := p.store.PartyCandidate(principal)
	if e != nil {
		return
	}
	account := candidate.ServicesAccount
	if account == "" {
		account = candidate.Username
	}
	if account == "" {
		return
	}
	hints := []string{account}
	for _, owner := range p.m.Owners() {
		w := p.m.space(owner)
		w.mu.Lock()
		network := w.networks[p.m.options.Home.ID]
		if strings.EqualFold(network.Account, account) && network.Nick != "" {
			hints = append(hints, network.Nick)
		}
		for _, member := range w.members {
			if strings.EqualFold(member.Account, account) && member.Nick != "" {
				hints = append(hints, member.Nick)
				if len(hints) >= 8 {
					break
				}
			}
		}
		w.mu.Unlock()
		if len(hints) >= 8 {
			break
		}
	}
	slices.Sort(hints)
	hints = slices.Compact(hints)
	ctx, cancel := context.WithTimeout(p.m.ctx, 5*time.Second)
	defer cancel()
	for _, nick := range hints {
		p.mu.Lock()
		prior := p.nativeInvites[invitation.ID]
		p.mu.Unlock()
		user, e := p.m.options.PartyAccounts.User(ctx, nick)
		if e != nil || !strings.EqualFold(user.Account, account) {
			continue
		}
		// An IRC invitation belongs to a live IRC client, so offer it again
		// occasionally when an unaccepted recipient disconnects and reconnects.
		if prior.client == client && strings.EqualFold(prior.nick, nick) && time.Since(prior.offeredAt) < time.Minute {
			return
		}
		if e = c.enqueue(&girc.Event{Command: "INVITE", Params: []string{user.Nickname, v.Channel}}); e != nil {
			return
		}
		p.mu.Lock()
		if p.nativeInvites == nil {
			p.nativeInvites = map[string]partyNativeInvite{}
		}
		p.nativeInvites[invitation.ID] = partyNativeInvite{nick: user.Nickname, channel: v.Channel, client: client, offeredAt: time.Now()}
		p.mu.Unlock()
		return
	}
}

func (p *partyManager) cancelNativeInvite(id string) {
	p.mu.Lock()
	prior := p.nativeInvites[id]
	delete(p.nativeInvites, id)
	c := p.control
	p.mu.Unlock()
	if c == nil || prior.nick == "" {
		return
	}
	c.mu.Lock()
	current := c.ready && c.client == prior.client
	c.mu.Unlock()
	if current {
		_ = c.enqueue(&girc.Event{Command: "UNINVITE", Params: []string{prior.nick, prior.channel}})
	}
}

func (p *partyManager) applyNative(event partyNativeEvent) {
	p.mu.Lock()
	c := p.control
	p.mu.Unlock()
	if c == nil {
		return
	}
	c.mu.Lock()
	current := c.ready && c.client == event.client
	c.mu.Unlock()
	if !current {
		return
	}
	ctx, cancel := context.WithTimeout(p.m.ctx, 90*time.Second)
	defer cancel()
	actor, e := p.m.ResolvePartyCandidate(ctx, event.account, "")
	if e != nil {
		return
	}
	v, e := p.store.Party(event.party)
	if e != nil || v.State == "archived" {
		return
	}
	ctx = context.WithValue(ctx, partyActorKey{}, actor)
	if !event.join {
		if v.HasMember(actor.ID) {
			_ = p.m.LeaveParty(ctx, actor.ID, v.ID)
		}
		return
	}
	if v.HasMember(actor.ID) {
		return
	}
	for _, invitation := range func() []model.Invitation { items, _ := p.store.PendingPartyInvitations(actor.ID); return items }() {
		if invitation.Party != nil && invitation.Party.ID == v.ID {
			record, e := p.store.PartyInvitation(actor.ID, invitation.ID)
			if e == nil {
				_, e = p.m.acceptPartyInvitation(ctx, actor.ID, record)
			}
			if e == nil {
				p.cancelNativeInvite(invitation.ID)
				return
			}
			break
		}
	}
	_ = c.enqueue(&girc.Event{Command: "KICK", Params: []string{v.Channel, event.nick, "Party invitation required"}})
}
