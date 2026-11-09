package irc

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lrstanley/girc"
)

const defaultNickLength = 32

// Self reads the account profile without materializing members or conversations.
func (m *Manager) Self(owner string) Member {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	return clone(w.self)
}

func (m *Manager) Realname(owner, network, value string) error {
	if value == "" || !clean(value, 128) {
		return fmt.Errorf("invalid realname")
	}
	c, err := m.connection(owner, network)
	if err != nil {
		return err
	}
	if !c.has("setname") {
		return ErrUnsupported
	}
	return c.enqueue(&girc.Event{Command: "SETNAME", Params: []string{value}})
}

func (c *connection) currentNick() string {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client != nil {
		return client.GetNick()
	}
	return c.config.Nick
}

func (c *connection) fold(s string) string {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client != nil {
		if mapping, _ := client.GetServerOption("CASEMAPPING"); mapping == "ascii" {
			return strings.ToLower(s)
		}
	}
	return girc.ToRFC1459(s)
}

// foldUnlocked is used only with c.mu held, and does not call the IRC client.
func (c *connection) foldUnlocked(s string) string { return strings.ToLower(s) }

func (m *Manager) SetProfile(owner string, p Profile) (Member, error) {
	p.Bio = strings.ReplaceAll(p.Bio, "\r\n", "\n")
	if p.DisplayName != nil && !clean(*p.DisplayName, 80) {
		return Member{}, fmt.Errorf("invalid profile")
	}
	if !utf8.ValidString(p.Bio) || utf8.RuneCountInString(p.Bio) > 300 || strings.ContainsAny(p.Bio, "\r\x00") {
		return Member{}, fmt.Errorf("About you must contain 300 characters or fewer and no control characters")
	}
	if p.NameColor != nil && *p.NameColor != "" {
		if len(*p.NameColor) != 7 || (*p.NameColor)[0] != '#' {
			return Member{}, fmt.Errorf("invalid name color")
		}
		for _, r := range (*p.NameColor)[1:] {
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return Member{}, fmt.Errorf("invalid name color")
			}
		}
	}
	if p.AvatarURL != nil && *p.AvatarURL != "" {
		u, err := url.Parse(*p.AvatarURL)
		if err != nil || (u.Scheme != "https" && (m.options.Files == nil || !m.options.Files.OwnsURL(owner, *p.AvatarURL))) || u.Host == "" || u.User != nil || !clean(*p.AvatarURL, 300) {
			return Member{}, fmt.Errorf("avatar must be an HTTPS URL")
		}
	}
	w := m.space(owner)
	w.mu.Lock()
	if w.deleted {
		w.mu.Unlock()
		return Member{}, ErrNotFound
	}
	self := w.self
	before := self
	if p.DisplayName != nil {
		if name := strings.TrimSpace(*p.DisplayName); name != "" {
			self.DisplayName, self.NameSource = name, "profile"
		} else {
			self.NameSource = ""
			resolveMemberName(&self)
		}
	}
	self.Bio = p.Bio
	self.NameColor = ""
	if p.NameColor != nil {
		self.NameColor = *p.NameColor
	}
	if p.AvatarURL != nil {
		self.AvatarURL = *p.AvatarURL
	}
	w.self = self
	w.members[self.ID] = self
	w.emit("profile.updated", self)
	if w.storageErr != nil {
		w.mu.Unlock()
		return Member{}, ErrStorage
	}
	// Only the fields this edit changed are published, so an edit made here does not clear
	// values another IRCord instance signed in to the same account has shared. Publishing
	// under the workspace mutex keeps the SETs in the same order as the edits, also against a
	// connect-time publish (profileCheckReply). enqueue never blocks.
	changed := changedProfileKeys(before, self)
	for _, c := range w.connections {
		c.pushProfileKeys(self, changed)
	}
	w.mu.Unlock()
	return clone(self), nil
}

func (m *Manager) SetAway(owner, network, reason string) error {
	if !clean(reason, 200) {
		return fmt.Errorf("invalid away reason")
	}
	c, err := m.connection(owner, network)
	if err != nil {
		return err
	}
	m.manualAway(owner, network)
	return c.enqueue(&girc.Event{Command: "AWAY", Params: []string{reason}})
}

func (m *Manager) Nick(owner, network, nick string) error {
	if err := validateNickname(nick, 0); err != nil {
		return err
	}
	c, err := m.connection(owner, network)
	if err != nil {
		return err
	}
	c.owner.mu.Lock()
	maximum := c.nicknameLimit()
	c.owner.mu.Unlock()
	if err := validateNickname(nick, maximum); err != nil {
		return err
	}
	return c.enqueue(&girc.Event{Command: "NICK", Params: []string{nick}})
}

func (c *connection) receiveUser(client *girc.Client, e girc.Event) {
	switch e.Command {
	case "AWAY":
		if e.Source != nil {
			w := c.owner
			w.mu.Lock()
			v := c.member(e.Source.Name, "")
			v.Status = "online"
			if len(e.Params) > 0 {
				v.Status = "away"
			}
			w.members[v.ID] = v
			if v.ID == w.self.ID {
				w.self = v
				w.emit("profile.updated", v)
			} else {
				w.emit("member.updated", v)
			}
			w.mu.Unlock()
		}
	case "305", "306":
		w := c.owner
		w.mu.Lock()
		self := w.self
		self.Status = "online"
		if e.Command == "306" {
			self.Status = "away"
		}
		w.self = self
		w.members[self.ID] = self
		if w.awayOn == nil {
			w.awayOn = map[string]bool{}
		}
		w.awayOn[c.config.ID] = e.Command == "306"
		w.emit("profile.updated", self)
		w.mu.Unlock()
	case "SETNAME":
		if e.Source != nil && len(e.Params) > 0 && clean(e.Last(), 128) {
			w := c.owner
			w.mu.Lock()
			v := c.member(e.Source.Name, "")
			if c.realnames == nil {
				c.realnames = map[string]string{}
			}
			c.realnames[v.ID] = e.Last()
			if v.ID != w.self.ID {
				applyRealname(&v, e.Last())
				w.members[v.ID] = v
				w.emit("member.updated", v)
				w.mu.Unlock()
				break
			}
			n := w.networks[c.config.ID]
			n.Realname = e.Last()
			w.networks[n.ID] = n
			cfg := w.configs[n.ID]
			cfg.Realname = n.Realname
			w.configs[n.ID] = cfg
			if w.store != nil {
				if err := w.store.SaveNetwork(w.ownerID, cfg, true); err != nil {
					w.storageErr = err
				}
			}
			w.emit("network.updated", n)
			w.mu.Unlock()
		}
	case "NICK":
		if e.Source != nil {
			w := c.owner
			w.mu.Lock()
			old := c.fold(e.Source.Name)
			if memberID, ok := c.nicks[old]; ok {
				delete(c.nicks, old)
				c.nicks[c.fold(e.Last())] = memberID
				member := w.members[memberID]
				member.Nick = e.Last()
				if member.NameSource != "profile" && (member.NameSource != "" || member.DisplayName == e.Source.Name) {
					resolveMemberName(&member)
				}
				w.members[memberID] = member
				if memberID == w.self.ID {
					w.self = member
					w.emit("profile.updated", member)
				} else {
					w.emit("member.updated", member)
				}
			}
			n := w.networks[c.config.ID]
			n.Nick = client.GetNick()
			w.networks[n.ID] = n
			cfg := w.configs[n.ID]
			if cfg.Nick != n.Nick {
				cfg.Nick = n.Nick
				w.configs[n.ID] = cfg
				if w.store != nil {
					if err := w.store.SaveNetwork(w.ownerID, cfg, true); err != nil {
						w.storageErr = err
					}
				}
			}
			w.emit("network.updated", n)
			w.mu.Unlock()
		}
	}
}

func (c *connection) nicknameConflict(client *girc.Client, e girc.Event) {
	c.mu.Lock()
	ready := c.ready
	failed := !ready && (c.config.AltNick == "" || len(e.Params) > 1 && strings.EqualFold(e.Params[1], c.config.AltNick))
	if failed {
		c.permanent = true
		c.registrationError = "Nickname is already in use"
	}
	c.mu.Unlock()
	if ready {
		c.owner.mu.Lock()
		c.owner.sendCommandError("", "Nickname is already in use. Choose another nickname.")
		c.owner.mu.Unlock()
	}
	if failed {
		client.Close()
	}
}

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
