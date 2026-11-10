package irc

import (
	"strings"

	"github.com/lrstanley/girc"
)

func (c *connection) metadata(e girc.Event) {
	if len(e.Params) < 3 {
		return
	}
	// METADATA <target> <key> <visibility> [:<value>]; without a value the key was removed.
	nick, key, value := e.Params[0], e.Params[1], ""
	if e.Command == "761" {
		// RPL_KEYVALUE carries our own nick first.
		if len(e.Params) < 4 {
			return
		}
		nick, key = e.Params[1], e.Params[2]
		if len(e.Params) >= 5 {
			value = e.Params[4]
		}
	} else if len(e.Params) >= 4 {
		value = e.Params[3]
	}
	if girc.IsValidChannel(nick) {
		return
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	v := c.member(nick, "")
	own := v.ID == w.self.ID
	if own {
		// Reconcile GET answers together before applying them. Otherwise an old
		// server value could overwrite an offline edit before the check sees it.
		c.mu.Lock()
		checking := !c.ready || !c.profileCheckDone || c.profileCheck != nil
		c.mu.Unlock()
		if checking || e.Command == "761" && e.Params[0] != "*" {
			return
		}
		if isMetadataKey(key) {
			ours := profileValues(v)[key]
			if last, known := c.publishedProfile()[key]; known && ours != last && value == last {
				// A late echo of the previous value cannot cancel a local edit
				// that is still waiting to be sent or reconciled.
				return
			}
		}
		if isMetadataKey(key) && profileValues(v)[key] == value {
			c.recordPublished(map[string]string{key: value})
			return
		}
	}
	if value != "" {
		c.sawMetadata(e, v.ID, key)
	}
	before := v
	if !applyProfileMetadata(&v, key, value) || equal(before, v) {
		return
	}
	if own {
		w.saveNetworkSelf(c.config.ID, v)
		if isMetadataKey(key) {
			c.recordPublished(map[string]string{key: value})
		}
	} else {
		w.members[v.ID] = v
		w.emit("member.updated", v)
	}
}

func applyProfileMetadata(v *Member, key, value string) bool {
	switch key {
	case "display-name":
		if value == "" {
			// Another IRCord user cleared their profile name.
			clearMetadata(v, key)
		} else if clean(value, 80) {
			v.DisplayName = value
			v.NameSource = "profile"
			resolveMemberName(v)
		} else {
			return false
		}
	case "avatar":
		if value == "" || strings.HasPrefix(value, "https://") && clean(value, 300) {
			v.AvatarURL = value
		} else {
			return false
		}
	case "color":
		if value == "" || len(value) == 7 && value[0] == '#' {
			v.NameColor = value
		} else {
			return false
		}
	case "bio":
		if clean(value, 300) {
			v.Bio = value
		} else {
			return false
		}
	case deviceKey:
		if value == "" || value == "phone" || value == "tablet" {
			v.Device = value
		} else {
			return false
		}
	default:
		return false
	}
	return true
}

func (c *connection) receiveMetadataCleared(e girc.Event) {
	c.profileCheckReply(e)
	if len(e.Params) >= 3 {
		nick, key := e.Params[1], e.Params[2]
		if c.fold(nick) == c.fold(c.currentNick()) && e.Params[0] != "*" {
			// The full connect-time check has already reconciled this answer.
			return
		}
		c.metadata(girc.Event{Command: "METADATA", Params: []string{nick, key, "*"}})
	}
}
