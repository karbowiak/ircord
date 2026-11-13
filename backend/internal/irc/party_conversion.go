package irc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

type PartyConversionResult struct {
	Status         string   `json:"status" enum:"converted,pending"`
	ConversationID string   `json:"conversationId"`
	Prerequisite   string   `json:"prerequisite,omitempty"`
	Warnings       []string `json:"warnings" nullable:"false"`
}
type partyConversionKey struct{}

// ConvertParty is deliberately explicit: restore and development reloads must
// never transfer real server channels. Both service connections are existing
// account connections; only this bounded compatibility path uses the old one.
func (m *Manager) ConvertParty(ctx context.Context, owner, id string) (PartyConversionResult, error) {
	out := PartyConversionResult{Status: "pending", Warnings: []string{}}
	p := m.parties
	if p == nil {
		return out, ErrNotFound
	}
	ctx, cancel := context.WithTimeout(context.WithValue(ctx, partyConversionKey{}, true), 45*time.Second)
	defer cancel()
	unlock, err := p.lock(ctx, id)
	if err != nil {
		return out, err
	}
	defer unlock()
	v, err := p.store.Party(id)
	if err != nil || !v.HasMember(p.store.PartyPrincipal(owner)) {
		return out, ErrNotFound
	}
	out.ConversationID = v.ConversationID
	if v.OwnerID != p.store.PartyPrincipal(owner) {
		return out, ErrForbidden
	}
	if v.State == "converted" {
		p.publishConversion(v)
		if err := p.stopRetiredController(); err != nil {
			out.Warnings = append(out.Warnings, "The former control connection could not be stopped. Restart IRCord after reviewing its connection state.")
		}
		out.Status = "converted"
		return out, nil
	}
	if v.State == "archived" {
		return out, serviceError(409, "An archived group cannot be converted. Restore its intended ownership first.")
	}
	pending := func(text string) (PartyConversionResult, error) { out.Prerequisite = text; return out, nil }
	user, err := m.connection(owner, v.NetworkID)
	if err != nil || !user.home() {
		return pending("Connect the group owner's account to the associated IRC server, then retry conversion.")
	}
	actor, err := p.actor(ctx, owner)
	if err != nil {
		return pending("Sign in as the group's recorded IRC owner on the associated server, then retry conversion.")
	}
	if actor.PrincipalID() != v.OwnerID {
		return out, ErrForbidden
	}
	// Verify all durable account incarnations without pruning or changing grants.
	for _, member := range v.Members {
		if _, err := p.candidate(ctx, member.PrincipalID()); err != nil {
			return pending("A retained participant's IRC identity needs verification. Restore that account before retrying conversion.")
		}
	}
	ctl, ok := p.transport.(*partyIRCController)
	if !ok {
		return pending("The legacy channel control connection is unavailable. Restore it before retrying conversion.")
	}
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	raw, err := p.store.PartyControl()
	var identity partyControlIdentity
	if err != nil || json.Unmarshal(raw, &identity) != nil || !identity.Registered {
		return pending("Restore the existing legacy control account. Conversion cannot create a replacement owner.")
	}
	control, err := ctl.connection(ctx)
	if err != nil {
		return pending("Reconnect the existing legacy control account to the associated IRC server, then retry conversion.")
	}
	control.mu.Lock()
	controlClient := control.client
	control.mu.Unlock()
	controlCtx := context.WithValue(ctx, channelClientKey{}, controlClient)
	for _, c := range []*connection{control, user} {
		select {
		case c.serviceGate <- struct{}{}:
			defer func(c *connection) { <-c.serviceGate }(c)
		case <-ctx.Done():
			return pending("Channel services are busy. Retry conversion after current changes finish.")
		case <-c.ctx.Done():
			return pending("Reconnect both channel accounts, then retry conversion.")
		}
	}
	user.mu.Lock()
	client, ready, labeled := user.client, user.ready, user.caps["labeled-response"]
	user.mu.Unlock()
	if client == nil || !ready || !labeled {
		return pending("The owner's connection must support labeled replies. Reconnect it before retrying conversion.")
	}
	info, err := control.registrationInfo(controlCtx, v.Channel)
	if err != nil || !info.Registered {
		return pending("Restore the existing channel registration before retrying conversion. No channel settings were changed.")
	}
	if !strings.EqualFold(info.Founder, identity.Username) && !strings.EqualFold(info.Founder, actor.ServicesAccount) {
		return pending("The channel has a different native founder. Ask that founder to transfer it to the recorded group owner.")
	}
	// Freeze legacy reconciliation before any ownership effect. Native modes,
	// AMODE grants, invitations and the upstream channel are left intact.
	if !v.ConversionPending {
		v.ConversionPending = true
		if err = p.save(&v); err != nil {
			return out, err
		}
	}
	if err = p.prepareConversion(v); err != nil {
		return out, err
	}
	if strings.EqualFold(info.Founder, identity.Username) {
		lines, err := control.serviceExchange(controlCtx, "TRANSFER "+v.Channel+" "+actor.ServicesAccount)
		if err != nil {
			return pending("The ownership offer was not confirmed. Retry conversion to check the current founder and offer.")
		}
		if code, err := confirmation(lines, "TRANSFER", v.Channel, actor.ServicesAccount); err == nil {
			if _, err = control.serviceExchange(controlCtx, "TRANSFER "+v.Channel+" "+actor.ServicesAccount+" "+code); err != nil {
				return pending("The ownership offer was not confirmed. Retry conversion; the channel and access remain intact.")
			}
		}
		// Stock ChanServ requires the recipient's authenticated acceptance.
		if _, err := user.serviceExchange(ctx, "TRANSFER ACCEPT "+v.Channel); err != nil {
			return pending("Accept the ownership offer as the recorded owner with /CS TRANSFER ACCEPT " + v.Channel + ", then retry conversion.")
		}
	}
	info, err = user.registrationInfo(ctx, v.Channel)
	if err != nil || !info.Registered || !strings.EqualFold(info.Founder, actor.ServicesAccount) {
		return pending("Ownership acceptance was not confirmed. Retry conversion to check the founder; no channel was deleted.")
	}
	// Existing +v grants still admit participants to +i on the associated
	// server. Grant no new access, including to pending invitees.
	ctx = context.WithValue(ctx, channelClientKey{}, client)
	user.owner.mu.Lock()
	joined := user.owner.conversations[v.ConversationID].Joined
	user.owner.mu.Unlock()
	if !joined {
		if _, err = user.channelExchange(ctx, &girc.Event{Command: "JOIN", Params: []string{v.Channel}}, "366"); err != nil {
			return pending("Ownership transferred. Join the existing channel as its owner, then retry conversion.")
		}
	}
	if _, err = user.serviceExchange(ctx, "OP "+v.Channel); err != nil {
		return pending("Ownership transferred. Request operator status with /CS OP " + v.Channel + ", then retry conversion.")
	}
	if _, err = user.channelExchange(ctx, &girc.Event{Command: "NAMES", Params: []string{v.Channel}}, "366"); err != nil {
		return pending("Ownership transferred. Refresh the channel roster, then retry conversion.")
	}
	if user.awaitChannelControl(ctx, v.Channel) != nil {
		return pending("Ownership transferred. Confirm operator access as the recorded owner, then retry conversion.")
	}
	if _, err = user.channelExchange(ctx, &girc.Event{Command: "MODE", Params: []string{v.Channel}}, "324"); err != nil {
		return pending("Ownership transferred. Refresh channel settings, then retry conversion.")
	}
	converted, err := p.store.CompletePartyConversion(v)
	if err != nil {
		return out, ErrStorage
	}
	p.publishConversion(converted)
	// Pending invitations become ordinary invitations; an offline recipient
	// may require a fresh native INVITE after the server's invite expires.
	invitations, err := p.store.PartyInvitations(v.ID)
	if err != nil {
		out.Warnings = append(out.Warnings, "Pending invitations could not be refreshed. Invite those accounts from channel settings.")
	}
	for _, invitation := range invitations {
		if invitation.State != "pending" {
			continue
		}
		local := p.localOwner(invitation.Invitee)
		if local != "" {
			w := m.space(local)
			native := invitation.Invitation
			native.Party, native.Channel = nil, v.Channel
			native.ChannelKey = user.fold(v.Channel)
			w.mu.Lock()
			w.invitations[native.ID] = native
			w.emit("invitation.received", native)
			w.mu.Unlock()
		}
		candidate, lookupErr := p.candidate(ctx, invitation.Invitee)
		if lookupErr != nil {
			out.Warnings = append(out.Warnings, "A pending invitation needs a new native invitation from the channel owner.")
			continue
		}
		online, lookupErr := m.options.AssociatedServer.User(ctx, candidate.Username)
		if lookupErr != nil || !strings.EqualFold(online.Account, candidate.Username) {
			out.Warnings = append(out.Warnings, "An offline invitee keeps their invitation. Invite them again if the server invitation expires.")
			continue
		}
		if _, err := user.channelExchange(ctx, &girc.Event{Command: "INVITE", Params: []string{online.Nickname, v.Channel}}, "341"); err != nil {
			out.Warnings = append(out.Warnings, "A pending invitation needs a new native invitation from the channel owner.")
		}
	}
	// PART only the old controller. Never kick participants, remove grants,
	// unregister, reset privacy or delete upstream/local history.
	if _, err := control.channelExchange(controlCtx, &girc.Event{Command: "PART", Params: []string{v.Channel}}, ""); err != nil {
		out.Warnings = append(out.Warnings, "The former control connection has not confirmed leaving the channel.")
	}
	p.mu.Lock()
	delete(p.protected, v.ID)
	p.mu.Unlock()
	delete(ctl.verified, v.ID)
	if err := p.stopRetiredController(); err != nil {
		out.Warnings = append(out.Warnings, "The former control connection could not be stopped. Restart IRCord after reviewing its connection state.")
	}
	out.Status = "converted"
	return out, nil
}

func (p *partyManager) stopRetiredController() error {
	all, err := p.store.Parties()
	if err != nil {
		return ErrStorage
	}
	for _, legacy := range all {
		if legacy.State != "converted" && legacy.State != "archived" {
			return nil
		}
	}
	p.mu.Lock()
	p.control = nil
	p.mu.Unlock()
	if _, err := p.m.connection(partyControlOwner, p.m.options.Home.ID); err == ErrNotFound {
		return nil
	}
	return p.m.Disconnect(partyControlOwner, p.m.options.Home.ID)
}

func (p *partyManager) prepareConversion(v storage.PartyRecord) error {
	for _, member := range v.Members {
		local := p.localOwner(member.PrincipalID())
		if local == "" {
			continue
		}
		if err := p.store.MaterializeParty(local, v); err != nil {
			return ErrStorage
		}
		w := p.m.space(local)
		w.mu.Lock()
		c := w.connections[v.NetworkID]
		w.mu.Unlock()
		if c != nil {
			c.mu.Lock()
			c.desired[c.foldUnlocked(v.Channel)] = v.Channel
			c.mu.Unlock()
			if err := c.saveTargets(); err != nil {
				return ErrStorage
			}
		}
	}
	return nil
}

func (p *partyManager) publishConversion(v storage.PartyRecord) {
	for _, member := range v.Members {
		local := p.localOwner(member.PrincipalID())
		if local == "" {
			continue
		}
		w := p.m.space(local)
		w.mu.Lock()
		conv, ok := w.conversations[v.ConversationID]
		if !ok {
			conv = p.conversation(v)
		}
		conv.Party, conv.Name, conv.Target = nil, strings.TrimPrefix(v.Channel, "#"), v.Channel
		w.conversations[conv.ID] = conv
		// A converted revision is a presentation tombstone, not a removal: all
		// retained/offline history and navigation IDs stay valid.
		w.emit("party.updated", v.Party)
		w.emit("conversation.updated", conv)
		c := w.connections[v.NetworkID]
		w.mu.Unlock()
		if c != nil {
			c.mu.Lock()
			client := c.client
			c.mu.Unlock()
			if client != nil {
				c.syncMembers(client)
			}
		}
	}
}
