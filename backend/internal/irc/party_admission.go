package irc

import (
	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"slices"
	"strings"
)

// A native JOIN accepts its pending invitation; PART leaves the account's
// party. QUIT is only a disconnect and preserves durable membership.
func (c *connection) observePartyAdmission(client *girc.Client, e girc.Event) {
	if e.Tags["batch"] != "" {
		return
	}
	if c.owner.ownerID != partyControlOwner || c.manager.parties == nil || e.Source == nil {
		return
	}
	if strings.EqualFold(e.Source.Name, client.GetNick()) {
		if e.Command == "ACCOUNT" && len(e.Params) > 0 && !strings.EqualFold(e.Params[0], c.config.SASLUser) {
			c.closeTransport()
		}
		return
	}
	p := c.manager.parties
	if (e.Command == "JOIN" || e.Command == "PART") && len(e.Params) > 0 {
		party, err := p.store.PartyByTarget(e.Params[0])
		if err != nil {
			return
		}
		account := e.Tags["account"]
		if e.Command == "JOIN" && len(e.Params) > 1 {
			account = e.Params[1]
		}
		if account == "" || account == "*" {
			if e.Command == "JOIN" {
				_ = c.enqueue(&girc.Event{Command: "KICK", Params: []string{party.Channel, e.Source.Name, "Registered account required"}})
			}
			return
		}
		select {
		case p.native <- partyNativeEvent{party: party.ID, nick: e.Source.Name, account: account, join: e.Command == "JOIN", client: client}:
		default:
			if e.Command == "JOIN" {
				_ = c.enqueue(&girc.Event{Command: "KICK", Params: []string{party.Channel, e.Source.Name, "Party control is busy. Join again shortly."}})
			}
		}
		return
	}
	if e.Command != "ACCOUNT" || len(e.Params) == 0 {
		return
	}
	for _, channel := range client.Channels() {
		if !slices.ContainsFunc(channel.UserList, func(nick string) bool { return strings.EqualFold(nick, e.Source.Name) }) {
			continue
		}
		party, err := p.store.PartyByTarget(channel.Name)
		if err != nil {
			continue
		}
		allowed := slices.ContainsFunc(party.Members, func(member model.PartyMember) bool { return strings.EqualFold(member.Username, e.Params[0]) })
		if !allowed {
			p.mu.Lock()
			delete(p.protected, party.ID)
			p.mu.Unlock()
			_ = c.enqueue(&girc.Event{Command: "KICK", Params: []string{party.Channel, e.Source.Name, "Party membership required"}})
			select {
			case p.wake <- struct{}{}:
			default:
			}
		}
	}
}

func (c *connection) partyMessageAllowed(target, account string, historical bool) bool {
	if c.owner.ownerID != partyControlOwner || !girc.IsValidChannel(target) {
		return true
	}
	if account == "" || account == "*" {
		return false
	}
	party, e := c.manager.options.Store.PartyByTarget(target)
	if e != nil {
		return false
	}
	if !historical {
		return slices.ContainsFunc(party.Members, func(member model.PartyMember) bool { return strings.EqualFold(member.Username, account) })
	}
	var accepted bool
	e = c.manager.options.Store.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM party_participants WHERE party=? AND username=? COLLATE NOCASE)", party.ID, account).Scan(&accepted)
	return e == nil && accepted
}
