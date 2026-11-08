package irc

import (
	"strings"

	"github.com/lrstanley/girc"
)

// Join notices need to know who was in a channel before each event. Neither
// girc's roster nor conv.MemberIDs can say: a reconnect starts girc's state
// empty, so a JOIN the server replays for a channel we never left looks new,
// and relying on girc running our ALL_EVENTS handler before its own state
// handlers would tie us to an order girc does not document. The connection
// keeps its own roster instead, updated in IRC event order and rebuilt on
// every connection: our JOIN starts a channel's set, NAMES and WHO replies
// fill it, and other members' JOIN, PART, KICK, QUIT and NICK keep it current.
// Its size follows the channels currently joined, never the users ever seen.

// presence applies e to the roster. For a JOIN it reports whether the joiner
// was absent, so the event is a real arrival. For QUIT and NICK it returns the
// folded channels the actor was in before e.
func (c *connection) presence(e girc.Event) (arrived bool, channels []string) {
	switch e.Command {
	case "JOIN", "PART", "KICK", "QUIT", "NICK":
	case "353": // RPL_NAMREPLY: <me> <symbol> <channel> :<names>
		if len(e.Params) >= 4 {
			prefixes := c.namePrefixes()
			nicks := []string{}
			for _, name := range strings.Fields(e.Last()) {
				name, _, _ = strings.Cut(strings.TrimLeft(name, prefixes), "!")
				if name != "" {
					nicks = append(nicks, name)
				}
			}
			c.present(e.Params[2], nicks...)
		}
		return
	case "352": // RPL_WHOREPLY: <me> <channel> <user> <host> <server> <nick> ...
		if len(e.Params) >= 6 {
			c.present(e.Params[1], e.Params[5])
		}
		return
	case "354": // WHOX reply to girc's "%tacuhnr,1": <me> 1 <channel> <user> <host> <nick> ...
		if len(e.Params) >= 6 && e.Params[1] == "1" {
			c.present(e.Params[2], e.Params[5])
		}
		return
	default:
		return
	}
	if e.Source == nil {
		return
	}
	nick, selfNick := c.fold(e.Source.Name), c.fold(c.currentNick())
	self := nick == selfNick
	channel, key := "", ""
	if len(e.Params) > 0 {
		channel, key = c.fold(e.Params[0]), c.foldUnlocked(e.Params[0])
	}
	target := ""
	if e.Command == "KICK" && len(e.Params) > 1 {
		target = c.fold(e.Params[1])
	}
	renamed := ""
	if e.Command == "NICK" && len(e.Params) > 0 {
		renamed = c.fold(e.Last())
	}
	known := false
	if self && e.Command == "JOIN" && channel != "" {
		// The casemapping is known by now, so this ID matches the stored one.
		id := c.conversationID(e.Params[0])
		c.owner.mu.Lock()
		_, known = c.owner.conversations[id]
		c.owner.mu.Unlock()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.roster == nil {
		c.roster = map[string]map[string]bool{}
	}
	switch e.Command {
	case "JOIN":
		if channel == "" {
			return
		}
		if self {
			// A session reattaching to an account the server kept online, or a
			// reconnect rejoining saved channels, replays our JOIN for every
			// channel we never left, including ones another client of the
			// account joined. A channel we already have is a restoration unless
			// we left it or asked to join it while not joined.
			arrived = c.arrivals[key] || !known
			delete(c.arrivals, key)
			c.roster[channel] = map[string]bool{nick: true}
			return
		}
		members := c.roster[channel]
		if members == nil {
			return true, nil
		}
		arrived = !members[nick]
		members[nick] = true
	case "PART", "KICK":
		gone := nick
		if e.Command == "KICK" {
			gone = target
		}
		if gone == selfNick {
			c.leftUnlocked(channel, key)
		} else if members := c.roster[channel]; members != nil {
			delete(members, gone)
		}
	case "QUIT", "NICK":
		for name, members := range c.roster {
			if !members[nick] {
				continue
			}
			channels = append(channels, name)
			delete(members, nick)
			if renamed != "" {
				members[renamed] = true
			}
		}
	}
	return
}

// resetRoster starts a new IRC connection's roster. Arrival marks carry over:
// a channel we left before the drop is still a real arrival when rejoined.
func (c *connection) resetRoster() {
	c.mu.Lock()
	c.roster = map[string]map[string]bool{}
	c.mu.Unlock()
}

// leftUnlocked forgets a channel we left; our next JOIN of it is a real arrival.
// Arrival marks only exist for channels we were in, and PartReason drops the
// mark along with the conversation, so they stay bounded by the conversations.
func (c *connection) leftUnlocked(channel, key string) {
	delete(c.roster, channel)
	c.arriveUnlocked(key)
}

// arriveUnlocked marks our next JOIN of a channel as a real arrival.
func (c *connection) arriveUnlocked(key string) {
	if c.arrivals == nil {
		c.arrivals = map[string]bool{}
	}
	c.arrivals[key] = true
}

// present adds nicks to a channel we are in; replies about other channels are ignored.
func (c *connection) present(channel string, nicks ...string) {
	channel = c.fold(channel)
	folded := make([]string, 0, len(nicks))
	for _, nick := range nicks {
		folded = append(folded, c.fold(nick))
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	members := c.roster[channel]
	if members == nil {
		return
	}
	for _, nick := range folded {
		members[nick] = true
	}
}

// namePrefixes are the membership prefix characters a NAMES reply can put before a nick.
func (c *connection) namePrefixes() string {
	c.owner.mu.Lock()
	prefix := c.owner.networks[c.config.ID].ISupport["PREFIX"]
	c.owner.mu.Unlock()
	if _, symbols, ok := strings.Cut(prefix, ")"); ok && symbols != "" {
		return symbols
	}
	return "~&@%+"
}
