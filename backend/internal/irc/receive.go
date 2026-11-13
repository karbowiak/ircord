package irc

import (
	"strings"
	"time"

	"github.com/lrstanley/girc"
)

// receive preserves protocol ordering while domain files own the state changes.
func (c *connection) receive(client *girc.Client, e girc.Event) {
	if c.ctx.Err() != nil {
		return
	}
	c.partyReply(client, e)
	c.observePartyAdmission(client, e)
	if c.observeServices(client, e) {
		return
	}
	c.capture(e, false)
	c.recordLatency(e, time.Now())
	if e.Command == "CAP" {
		c.capabilityEvent(client, e)
	}
	c.listItem(e)
	c.modeListItem(e)
	if c.closedEvent(e) {
		return
	}
	c.membership(e)
	c.receiveQuit(e)
	if e.Command == "BATCH" {
		c.batch(e)
		c.historyBatch(e)
		return
	}
	if c.receiveMultiline(e) {
		return
	}
	c.historyItem(e.Tags["batch"], e)
	switch e.Command {
	case "ERROR":
		if client != nil && client.Config.WebIRC.Password != "" && strings.Contains(strings.ToLower(e.Last()), "webirc") {
			c.mu.Lock()
			if !c.ready {
				c.permanent, c.registrationError = true, c.redactSecrets(e.Last())
			}
			c.mu.Unlock()
		}
	case "902", "904", "905", "906", "432", "463", "464", "465":
		c.mu.Lock()
		terminal := !c.ready || e.Command == "465"
		if terminal {
			c.permanent = true
			c.registrationError = c.redactSecrets(e.Last())
		}
		c.mu.Unlock()
		if terminal {
			client.Close()
		} else {
			c.receiveCommandFailure(e)
		}
	case "CHATHISTORY":
		// TARGETS replies are consumed by historyItem above.
	case "PRIVMSG", "NOTICE":
		if e.Source != nil && e.Source.Ident == "" && strings.Contains(e.Source.Name, ".") && len(e.Params) > 0 && !girc.IsValidChannel(e.Params[0]) {
			break
		}
		c.message(e)
	case "TAGMSG":
		c.typing(e)
		c.attract(e)
		c.reaction(e)
	case "AWAY", "305", "306", "SETNAME", "NICK":
		c.receiveUser(client, e)
	case "MARKREAD":
		c.receiveReadMarker(e)
	case "333", "329", "328", "TOPIC", "332":
		c.receiveConversation(e)
	case "JOIN", "PART", "KICK", "QUIT", "353", "366", "352", "354", "MODE", "ACCOUNT", "CHGHOST":
		c.receiveMemberSync(client, e)
	case "761", "METADATA":
		c.profileCheckReply(e)
		c.metadata(e)
	case "774":
		c.retryMetadataSync(e)
	case "766":
		c.receiveMetadataCleared(e)
	case "REDACT":
		c.redact(e)
	case "ACK":
		c.redactionResult(c.eventLabel(e), "")
	case "433":
		c.nicknameConflict(client, e)
	case "FAIL", "400", "401", "403", "404", "405", "407", "411", "412", "417", "421", "442", "443", "471", "472", "473", "474", "475", "477", "478", "482":
		c.receiveCommandFailure(e)
	}
}
