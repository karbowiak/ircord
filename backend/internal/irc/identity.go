package irc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lrstanley/girc"
)

const defaultNickLength = 32

func validateNickname(nick string, maximum int) error {
	if nick == "" {
		return fmt.Errorf("enter a nickname")
	}
	if strings.ContainsAny(nick, " \t\r\n") {
		return fmt.Errorf("nicknames cannot contain spaces; use the display name for a name with spaces")
	}
	if !girc.IsValidNick(nick) || nick[0] == '?' {
		return fmt.Errorf("use letters, numbers or IRC nickname symbols; nicknames cannot start with a number or hyphen")
	}
	if maximum > 0 && len(nick) > maximum {
		return fmt.Errorf("nickname must be %d characters or fewer", maximum)
	}
	return nil
}

// Called with the workspace lock, alongside network ISUPPORT updates.
func (c *connection) nicknameLimit() int {
	if maximum, err := strconv.Atoi(c.owner.networks[c.config.ID].ISupport["NICKLEN"]); err == nil && maximum > 0 && maximum <= 128 {
		return maximum
	}
	return defaultNickLength
}

func resolveMemberName(v *Member) {
	if v.NameSource == "profile" && strings.TrimSpace(v.DisplayName) != "" {
		return
	}
	if strings.TrimSpace(v.Realname) != "" && v.Realname != "IRCord" {
		v.DisplayName, v.NameSource = v.Realname, "realname"
	} else {
		v.DisplayName, v.NameSource = v.Nick, "nick"
	}
}

// normalizeSelfName gives a stored self profile written before name sources existed an
// explicit source. The automatic "You" placeholder earlier versions seeded is never a
// chosen name, so it falls back to the nickname. Storage migration 5 applies the same rule
// to unsourced rows, and also releases a "You" saved as a profile name.
func normalizeSelfName(v *Member) {
	if v.NameSource != "" {
		return
	}
	if v.DisplayName != "" && v.DisplayName != "You" && v.DisplayName != v.Nick {
		v.NameSource = "profile"
		return
	}
	resolveMemberName(v)
}

func applyRealname(v *Member, name string) {
	// Preserve profile metadata stored by earlier versions without a source field.
	if v.NameSource == "" && v.DisplayName != "" && v.DisplayName != v.Nick && v.DisplayName != name && v.DisplayName != v.Realname {
		v.NameSource = "profile"
	}
	v.Realname = name
	resolveMemberName(v)
}
