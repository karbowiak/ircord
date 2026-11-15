package irc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lrstanley/girc"
)

// Bots are people whose IRC client set the network's bot user mode (BOT in
// ISUPPORT, +B on Ergo). A member is a bot while:
//
//   - their live messages, notices, TAGMSGs and JOINs carry the IRCv3 bot tag,
//     and stops being one when a live message arrives without it (once the
//     server has shown that it sends the tag at all),
//   - the flags of a WHO reply about them contain the bot mode letter, or
//   - WHOIS answers with RPL_WHOISBOT (335).
//
// Networks without BOT in ISUPPORT never mark anyone. History replays are
// ignored: their tags describe the sender when the message was sent.

const (
	roleBot    = "bot"
	roleMember = "member"
	// botWhoToken marks the WHOX request for channel members' flags. girc's own
	// WHO uses token 1 and leaves the flags out.
	botWhoToken = "28"
)

var (
	// ErrBotForbidden rejects a bot mark by someone who isn't a server owner on
	// the associated server, or where the moderation login isn't configured.
	ErrBotForbidden error = botForbidden{}
	// ErrBotRefused carries the IRC server's reason for refusing a bot mark.
	ErrBotRefused = errors.New("the IRC server refused to change the bot mark")
	// ErrBotUnconfirmed reports a bot mark the server did not answer in time.
	ErrBotUnconfirmed = errors.New("the IRC server did not confirm the bot mark in time; check the profile again")
)

type botForbidden struct{}

func (botForbidden) Error() string {
	return "only server owners can mark bots, on the associated server"
}
func (botForbidden) Is(target error) bool { return target == ErrForbidden }

// botMode is the network's bot user mode letter, or "" when it has none.
func (c *connection) botMode() string {
	mode, _ := c.botSupport()
	return mode
}

// botSupport reads the network's bot mode letter and whether it offers WHOX.
func (c *connection) botSupport() (mode string, whox bool) {
	c.owner.mu.Lock()
	support := c.owner.networks[c.config.ID].ISupport
	mode = support["BOT"]
	_, whox = support["WHOX"]
	c.owner.mu.Unlock()
	if len(mode) != 1 {
		return "", whox
	}
	return mode, whox
}

// observeBot reads bot status from one incoming line.
func (c *connection) observeBot(e girc.Event) {
	switch e.Command {
	case "PRIVMSG", "NOTICE", "TAGMSG", "JOIN", "352", "354", "335":
	default:
		return
	}
	mode := c.botMode()
	if mode == "" {
		return
	}
	switch e.Command {
	case "352": // RPL_WHOREPLY: <me> <channel> <user> <host> <server> <nick> <flags> :<hops> <real name>
		if len(e.Params) >= 8 {
			c.setBot(e.Params[5], "", strings.Contains(e.Params[6], mode))
		}
		return
	case "354": // WHOX reply to "%tnf,28": <me> 28 <nick> <flags>
		if len(e.Params) == 4 && e.Params[1] == botWhoToken {
			c.setBot(e.Params[2], "", strings.Contains(e.Params[3], mode))
		}
		return
	case "335": // RPL_WHOISBOT: <me> <nick> :is a bot
		if len(e.Params) >= 2 {
			c.setBot(e.Params[1], "", true)
		}
		return
	}
	if e.Source == nil || e.Source.Ident == "" && strings.Contains(e.Source.Name, ".") || !c.has("message-tags") {
		return
	}
	_, tagged := e.Tags["bot"]
	if _, draft := e.Tags["draft/bot"]; draft {
		tagged = true
	}
	c.mu.Lock()
	historical := c.historyBatches[e.Tags["batch"]]
	c.botTagged = c.botTagged || tagged
	sendsTags := c.botTagged
	c.mu.Unlock()
	// A server that never sends the tag says nothing by leaving it out.
	if historical || !tagged && !sendsTags {
		return
	}
	c.setBot(e.Source.Name, e.Tags["account"], tagged)
}

// requestBotFlags asks for the flags of everyone in a channel we just joined,
// so bots there are known before they speak. Best effort.
func (c *connection) requestBotFlags(channel string) {
	// Without WHOX, girc's WHO is answered with 352 lines, which carry the flags.
	if mode, whox := c.botSupport(); mode == "" || !whox {
		return
	}
	_ = c.enqueue(&girc.Event{Command: "WHO", Params: []string{channel, "%tnf," + botWhoToken}})
}

// setBot records whether nick is a bot and publishes the change. It creates the
// member when unknown, as every other line about a person does.
func (c *connection) setBot(nick, account string, bot bool) {
	if nick == "" || nick == "*" {
		return
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	c.setBotLocked(nick, account, bot)
}

// setBotLocked requires w.mu.
func (c *connection) setBotLocked(nick, account string, bot bool) Member {
	w := c.owner
	v := c.member(nick, account)
	if (v.Role == roleBot) == bot {
		return v
	}
	v.Role = roleMember
	if bot {
		v.Role = roleBot
	}
	if v.ID == w.self.ID {
		w.saveNetworkSelf(c.config.ID, v)
	} else {
		w.members[v.ID] = v
		w.emit("member.updated", v)
	}
	return v
}

// BotMark is the outcome of a server owner's bot mark, for the audit log.
type BotMark struct {
	Member  Member
	Nick    string
	Account string
}

// SetBot marks or unmarks memberID of networkID as a bot on the associated server: the
// moderation login sends SAMODE <nick> +B or -B. Only server owners may, and
// only where that login is configured. Every IRCord account connected to the
// associated server sees the change at once; other clients see it on their
// next WHO or message.
func (m *Manager) SetBot(ctx context.Context, owner, networkID, memberID string, bot bool) (BotMark, error) {
	w := m.space(owner)
	w.mu.Lock()
	_, ok := w.networks[networkID]
	c := w.connections[networkID]
	member, known := w.members[memberID]
	self, deleted := w.self.ID, w.deleted
	// Member IDs belong to one network: someone known only elsewhere is unknown here.
	known = known && c != nil && member.Nick != "" && c.knownMember(member.Nick, "") == memberID
	w.mu.Unlock()
	if !ok || deleted {
		return BotMark{}, ErrNotFound
	}
	if m.moderationNetwork() != networkID || !m.isServerOwner(owner) || c != nil && !c.home() {
		return BotMark{}, ErrBotForbidden
	}
	if c == nil {
		return BotMark{}, ErrDisconnected
	}
	if !known || memberID == self {
		return BotMark{}, ErrNotFound
	}
	account := member.Account
	if account == "*" {
		account = ""
	}
	out := BotMark{Member: member, Nick: member.Nick, Account: account}
	mode := c.botMode()
	if mode == "" {
		mode = "B"
	}
	change := "-" + mode
	if bot {
		change = "+" + mode
	}
	refusal, err := m.moderator.request(ctx, "SAMODE", member.Nick, change)
	switch {
	case errors.Is(err, errModerationUnconfirmed):
		return out, ErrBotUnconfirmed
	case err != nil:
		return out, err
	case refusal != "":
		return out, fmt.Errorf("%w: %s", ErrBotRefused, refusal)
	}
	out.Member = m.applyBotMark(owner, member.Nick, bot, out.Member)
	return out, nil
}

// applyBotMark shows a confirmed bot mark in every workspace connected to the
// associated server that knows the nick, and returns the caller's member.
func (m *Manager) applyBotMark(owner, nick string, bot bool, fallback Member) Member {
	home := m.options.Home.ID
	m.mu.Lock()
	spaces := make(map[string]*workspace, len(m.owners))
	for id, w := range m.owners {
		spaces[id] = w
	}
	m.mu.Unlock()
	out := fallback
	for id, w := range spaces {
		w.mu.Lock()
		if c := w.connections[home]; c != nil && !w.deleted && c.knownMember(nick, "") != "" {
			v := c.setBotLocked(nick, "", bot)
			if id == owner {
				out = v
			}
		}
		w.mu.Unlock()
	}
	return out
}
