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
		p := e.Params[len(e.Params)-4:]
		nick, key, value = p[0], p[1], p[3]
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
	if v.ID == w.self.ID {
		return
	}
	if value != "" {
		c.sawMetadata(e, v.ID, key)
	}
	switch key {
	case "display-name":
		if value == "" {
			// Another IRCord user cleared their profile name.
			clearMetadata(&v, key)
		} else if clean(value, 80) {
			v.DisplayName = value
			v.NameSource = "profile"
			resolveMemberName(&v)
		}
	case "avatar":
		if value == "" || strings.HasPrefix(value, "https://") && clean(value, 300) {
			v.AvatarURL = value
		}
	case "color":
		if value == "" || len(value) == 7 && value[0] == '#' {
			v.NameColor = value
		}
	case "bio":
		if clean(value, 300) {
			v.Bio = value
		}
	case deviceKey:
		if value == "" || value == "phone" || value == "tablet" {
			v.Device = value
		}
	default:
		return
	}
	w.members[v.ID] = v
	w.emit("member.updated", v)
}

func (c *connection) receiveMetadataCleared(e girc.Event) {
	c.profileCheckReply(e)
	if len(e.Params) >= 3 {
		p := e.Params[len(e.Params)-3:]
		w := c.owner
		w.mu.Lock()
		v := c.member(p[0], "")
		if v.ID != w.self.ID && clearMetadata(&v, p[1]) {
			w.members[v.ID] = v
			w.emit("member.updated", v)
		}
		w.mu.Unlock()
	}
}
