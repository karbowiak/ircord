package irc

import (
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

// Member reads one retained member from the account's current workspace.
func (m *Manager) Member(owner, id string) (Member, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	member, ok := w.members[id]
	if !ok || w.deleted {
		return Member{}, ErrNotFound
	}
	return clone(member), nil
}

// pruneMembers forgets stored members nothing refers to any more: not in a
// channel's member list, not a direct-message partner, and not named by a
// retained message, reaction or inbox item. Every nickname ever seen would
// otherwise stay in startup, the first load and the browser's saved copy.
// Runs while the workspace loads, before any connection starts.
func (w *workspace) pruneMembers() error {
	if w.ownerID == partyControlOwner {
		return nil // its members are local accounts, not nicknames
	}
	keep, err := w.store.ReferencedMembers(w.ownerID)
	if err != nil {
		return err
	}
	keep[w.ownerID] = true
	directs := map[string]bool{}
	for _, conv := range w.conversations {
		for _, id := range conv.MemberIDs {
			keep[id] = true
		}
		for id := range conv.MemberModes {
			keep[id] = true
		}
		if conv.Kicked != nil {
			keep[conv.Kicked.ByID] = true
		}
		if conv.Kind == "direct" {
			directs[conv.NetworkID+"\x00"+strings.ToLower(conv.Name)] = true
		}
	}
	gone := []string{}
	for id, member := range w.members {
		// Only nickname members ("<network>:<id>") are pruned. Party and local
		// account profiles are bounded by their parties.
		network, _, nick := strings.Cut(id, ":")
		if !nick || keep[id] || directs[network+"\x00"+strings.ToLower(member.Nick)] {
			continue
		}
		gone = append(gone, id)
	}
	if len(gone) == 0 {
		return nil
	}
	if err = w.store.DeleteMembers(w.ownerID, gone); err != nil {
		return err
	}
	for _, id := range gone {
		delete(w.members, id)
	}
	return nil
}

// Capture pre-state for events whose old membership/nickname disappears when the
// IRC library updates its state. The subsequent UPDATE_STATE refreshes the roster.
func (c *connection) membership(e girc.Event) {
	kind := strings.ToLower(e.Command)
	c.mu.Lock()
	historical := c.historyBatches[e.Tags["batch"]]
	c.mu.Unlock()
	if historical {
		return
	}
	arrived, before := c.presence(e)
	kicked := kind == "kick" && len(e.Params) > 1 && e.Source != nil && c.fold(e.Params[1]) == c.fold(c.currentNick())
	if kicked {
		c.mu.Lock()
		delete(c.desired, c.foldUnlocked(e.Params[0]))
		c.mu.Unlock()
		if err := c.saveTargets(); err != nil {
			slog.Error("Forgetting a channel you were kicked from failed", "error", err)
		}
	}
	if !slices.Contains([]string{"join", "part", "quit", "nick", "kick", "mode", "topic", "invite"}, kind) || e.Source == nil {
		return
	}
	c.mu.Lock()
	closed := len(e.Params) > 0 && c.closedTargets[c.foldUnlocked(e.Params[0])]
	c.mu.Unlock()
	if closed || kind == "join" && !arrived {
		return
	}
	if kind == "invite" {
		c.receiveInvitation(e)
		return
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	actor := c.member(e.Source.Name, e.Tags["account"])
	targets := []Conversation{}
	if kind == "quit" || kind == "nick" {
		present := map[string]bool{}
		for _, channel := range before {
			present[c.conversationID(channel)] = true
		}
		for _, conv := range w.conversations {
			if conv.NetworkID != c.config.ID || conv.Kind != "channel" {
				continue
			}
			if slices.Contains(conv.MemberIDs, actor.ID) || present[conv.ID] {
				targets = append(targets, conv)
			}
		}
	} else if len(e.Params) > 0 && girc.IsValidChannel(e.Params[0]) {
		v := c.conversation(e.Params[0])
		if kicked {
			v.Kicked = &model.Kick{ByID: actor.ID}
			if len(e.Params) > 2 {
				v.Kicked.Reason = e.Last()
			}
			w.conversations[v.ID] = v
			w.emit("conversation.updated", v)
		}
		targets = append(targets, v)
	}
	for _, conv := range targets {
		if conv.Party != nil {
			continue
		}
		at := e.Timestamp
		if t, err := time.Parse(time.RFC3339Nano, e.Tags["time"]); err == nil {
			at = t
		}
		if at.IsZero() {
			at = time.Now().UTC()
		}
		data := &model.MembershipEvent{Type: kind, ActorID: actor.ID}
		switch kind {
		case "invite":
			data.TargetID = c.member(e.Params[0], "").ID
			data.Text = e.Params[1]
		case "part", "quit", "nick", "topic":
			if len(e.Params) > 1 || kind == "nick" || kind == "quit" {
				data.Text = e.Last()
			}
		case "mode":
			// Already hold the workspace lock; redact keys without relocking it.
			params := append([]string{}, e.Params...)
			if strings.Contains(e.Params[1], "k") {
				params = []string{e.Params[0], e.Params[1], "[redacted]"}
			}
			data.Text = strings.Join(params[1:], " ")
		case "kick":
			if len(e.Params) > 1 {
				data.TargetID = c.member(e.Params[1], "").ID
			}
			if len(e.Params) > 2 {
				data.Text = e.Last()
			}
		}
		messageID := id()
		if upstream := e.Tags["msgid"]; upstream != "" && len(targets) == 1 {
			// A redelivered frame carries the same msgid; store its notice once.
			messageID = c.config.ID + ":" + upstream
			if _, known := c.lookupMessage(conv.ID, messageID); known {
				continue
			}
		}
		msg := Message{ID: messageID, ConversationID: conv.ID, SenderID: actor.ID, Text: data.Text, CreatedAt: at, Kind: "event", Event: data, Reactions: []Reaction{}}
		w.messages[conv.ID] = append(w.messages[conv.ID], msg)
		if len(w.messages[conv.ID]) > 500 {
			w.messages[conv.ID] = append([]Message(nil), w.messages[conv.ID][len(w.messages[conv.ID])-500:]...)
		}
		w.boundCache(conv.ID)
		w.emit("message.created", msg)
	}
}

// member is called with the workspace mutex held. IDs are network-scoped and
// survive nick changes; network accounts enrich identity but never authorize it.
func (c *connection) member(nick, account string) Member {
	w := c.owner
	if w.ownerID == partyControlOwner && account != "" && account != "*" && c.manager.options.Store != nil {
		if identity, err := c.manager.options.Store.PartyIdentity(account); err == nil {
			identityID := identity.PrincipalID()
			// Retained local senders keep their historical canonical actor ID.
			if identity.LocalAccountID() != "" && c.manager.options.Store.PartyPrincipal(identity.LocalAccountID()) == identity.PrincipalID() {
				identityID = identity.LocalAccountID()
			}
			v := w.members[identityID]
			if v.ID == "" {
				v = Member{ID: identityID, DisplayName: identity.DisplayName, AvatarURL: identity.AvatarURL, NameSource: "profile", Role: "member", Status: "online", Color: "lavender"}
			}
			v.Nick, v.Account = nick, account
			w.members[v.ID] = v
			c.nicks[c.fold(nick)] = v.ID
			return v
		}
		if a, _, err := c.manager.options.Store.AccountByUsername(account); err == nil {
			v := w.members[a.ID]
			if v.ID == "" {
				v = Member{ID: a.ID, Nick: nick, DisplayName: nick, Role: "member", Status: "online", Account: account, Color: "lavender"}
			}
			v.Nick, v.Account = nick, account
			w.members[v.ID] = v
			c.nicks[c.fold(nick)] = v.ID
			return v
		}
	}
	if c.fold(nick) == c.fold(c.currentNick()) || (account != "" && account != "*" && c.config.SASLUser != "" && strings.EqualFold(account, c.config.SASLUser)) {
		return w.networkSelf(c.config.ID)
	}
	key := c.fold(nick)
	memberID := c.nicks[key]
	if memberID == w.self.ID {
		return w.networkSelf(c.config.ID)
	}
	if memberID == "" {
		memberID = c.config.ID + ":" + id()
		c.nicks[key] = memberID
	}
	v, exists := w.members[memberID]
	before := v
	if !exists {
		v = Member{ID: memberID, Nick: nick, DisplayName: nick, NameSource: "nick", Color: "lavender", Status: "online", Role: "member"}
	}
	v.Nick = nick
	if account != "" && account != "*" {
		v.Account = account
	}
	if !exists || before.Nick != v.Nick || before.Account != v.Account {
		w.members[v.ID] = v
		w.emit("member.updated", v)
	}
	return v
}

// knownMember resolves a nick to a member id without creating one. Callers hold w.mu.
func (c *connection) knownMember(nick, account string) string {
	if c.fold(nick) == c.fold(c.currentNick()) || (account != "" && account != "*" && c.config.SASLUser != "" && strings.EqualFold(account, c.config.SASLUser)) {
		return c.owner.self.ID
	}
	key := c.fold(nick) // fold takes c.mu itself
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nicks[key]
}

// rosterScope names the users and channels one IRC event can change in the IRC
// library's roster. A refresh then visits only those, instead of every user and
// channel, so a NAMES or WHO reply costs the users it names, not the whole roster.
type rosterScope struct {
	all      bool
	users    []string
	channels map[string]bool // channel name -> also refresh each member's details
}

func (s *rosterScope) channel(name string, details bool) {
	if s.channels == nil {
		s.channels = map[string]bool{}
	}
	s.channels[name] = s.channels[name] || details
}

// rosterCommands are the events whose library state handler changes the roster
// and then announces UPDATE_STATE. Those refresh on that announcement; any other
// event's scope is refreshed as soon as it arrives.
//
// This list and scopeOf mirror the state handlers of girc v1.1.3 (builtin.go
// registerBuiltins, cap.go, modes.go; the account-tag handler in cap_tags.go is
// covered by scopeOf). When upgrading girc, check every handler that calls
// state.notify(c, UPDATE_STATE) against both: a missing command leaves the
// roster stale until the next connection attempt refreshes it in full.
var rosterCommands = map[string]bool{"JOIN": true, "PART": true, "KICK": true, "QUIT": true, "NICK": true, "MODE": true, "TOPIC": true, "CHGHOST": true, "AWAY": true, "ACCOUNT": true, girc.RPL_TOPIC: true, girc.RPL_CHANNELMODEIS: true, girc.RPL_NAMREPLY: true, girc.RPL_WHOREPLY: true, girc.RPL_WHOSPCRPL: true}

// scopeOf runs before the library's own handlers apply e, so it can still read
// the channels a departing or renamed user was in.
func (c *connection) scopeOf(client *girc.Client, e girc.Event) rosterScope {
	s := rosterScope{}
	if e.Source != nil {
		// The library copies an account tag on any event to its sender.
		if _, ok := e.Tags.Get("account"); ok {
			s.users = append(s.users, e.Source.Name)
		}
	}
	param := func(i int) string {
		if len(e.Params) > i {
			return e.Params[i]
		}
		return ""
	}
	userChannels := func(nick string) {
		if user := client.LookupUser(nick); user != nil {
			for _, name := range user.ChannelList {
				s.channel(name, false)
			}
		}
	}
	switch e.Command {
	case "JOIN", "PART", "TOPIC", "MODE":
		if girc.IsValidChannel(param(0)) {
			s.channel(param(0), false)
		}
		if e.Source != nil {
			s.users = append(s.users, e.Source.Name)
		}
	case "KICK":
		s.channel(param(0), false)
		s.users = append(s.users, param(1))
	case girc.RPL_TOPIC, girc.RPL_CHANNELMODEIS:
		s.channel(param(1), false)
	case girc.RPL_ENDOFNAMES:
		// NAMES lines only add users; the channel is rebuilt once at the end.
		s.channel(param(1), true)
	case girc.RPL_WHOREPLY, girc.RPL_WHOSPCRPL:
		s.users = append(s.users, param(5))
	case "NICK":
		if e.Source == nil || c.fold(e.Source.Name) == c.fold(client.GetNick()) {
			s.all = true
			break
		}
		userChannels(e.Source.Name)
		s.users = append(s.users, param(0))
	case "QUIT":
		if e.Source != nil {
			userChannels(e.Source.Name)
		}
	case "CHGHOST", "AWAY", "ACCOUNT":
		if e.Source != nil {
			s.users = append(s.users, e.Source.Name)
		}
	}
	return s
}

// noteRosterEvent records the scope the next UPDATE_STATE refreshes. The
// library runs this handler to completion before its state handler for the same
// event. Its account-tag handler can announce UPDATE_STATE concurrently; that
// refreshes the previous scope, and this scope is refreshed after the event.
func (c *connection) noteRosterEvent(client *girc.Client, e girc.Event) {
	if strings.HasPrefix(e.Command, "CLIENT_") {
		return // the library's own notifications, including UPDATE_STATE
	}
	s := c.scopeOf(client, e)
	c.mu.Lock()
	if c.rosterClient != client {
		// A new attempt starts with an empty roster: refresh everything once so
		// channels from the previous connection read as not joined until rejoined.
		c.rosterClient = client
		s.all = true
	}
	c.rosterEvent = s
	c.mu.Unlock()
	if !rosterCommands[e.Command] && (s.all || len(s.users) > 0 || len(s.channels) > 0) {
		c.syncRoster(client, s)
	}
}

// rosterUpdated refreshes what the latest event named after the library applied it.
func (c *connection) rosterUpdated(client *girc.Client) {
	c.mu.Lock()
	s := c.rosterEvent
	c.mu.Unlock()
	c.syncRoster(client, s)
}

// syncMembers refreshes every user and channel in the library's roster.
func (c *connection) syncMembers(client *girc.Client) {
	c.syncRoster(client, rosterScope{all: true})
}

func (c *connection) syncRoster(client *girc.Client, scope rosterScope) {
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	n := w.networks[c.config.ID]
	if n.Nick != client.GetNick() {
		n.Nick = client.GetNick()
		p := w.networkSelf(n.ID)
		p.Nick = n.Nick
		resolveMemberName(&p)
		n.Profile = &p
		// Keep the configured nickname when the server assigned one IRCord
		// rejects; see the NICK handler in user.go.
		if validateNickname(n.Nick, defaultNickLength) == nil {
			cfg := w.configs[n.ID]
			cfg.Nick = n.Nick
			w.configs[n.ID] = cfg
			if w.store != nil {
				if err := w.store.SaveNetwork(w.ownerID, cfg, true); err != nil {
					w.storageErr = err
				}
			}
		}
		w.networks[n.ID] = n
		w.emit("network.updated", n)
	}
	var users []*girc.User
	var channels []*girc.Channel
	gone := []string{}
	if scope.all {
		users, channels = client.Users(), client.Channels()
	} else {
		seen := map[string]bool{}
		add := func(user *girc.User) {
			if key := c.fold(user.Nick); !seen[key] {
				seen[key] = true
				users = append(users, user)
			}
		}
		for name, details := range scope.channels {
			channel := client.LookupChannel(name)
			if channel == nil {
				gone = append(gone, name)
				continue
			}
			channels = append(channels, channel)
			if details {
				for _, user := range channel.Users(client) {
					add(user)
				}
			}
		}
		for _, nick := range scope.users {
			if user := client.LookupUser(nick); user != nil {
				add(user)
			}
		}
	}
	for _, user := range users {
		v := c.member(user.Nick, user.Extras.Account)
		// Only string fields change below, so a shallow copy is a safe baseline.
		before := v
		v.Status, v.AwayMessage = "online", ""
		if user.Extras.Away != "" {
			v.Status = "away"
			if clean(user.Extras.Away, 300) {
				v.AwayMessage = user.Extras.Away
			}
		}
		v.Account = user.Extras.Account
		v.Username = user.Ident
		v.Host = c.memberHostForDisplay(v.ID == w.self.ID, user.Host)
		name := user.Extras.Name
		if override, ok := c.realnames[v.ID]; ok {
			name = override
		}
		if name != "" {
			applyRealname(&v, name)
		}
		if v.ID == w.self.ID {
			if !reflect.DeepEqual(before, v) {
				w.saveNetworkSelf(c.config.ID, v)
			}
		} else {
			w.members[v.ID] = v
			if !reflect.DeepEqual(before, v) {
				w.emit("member.updated", v)
			}
		}
	}
	joined := map[string]bool{}
	for _, channel := range channels {
		if c.owner.ownerID != partyControlOwner && c.managedPartyTarget(channel.Name) && c.partyTargetAccess(channel.Name) != nil {
			gone = append(gone, channel.Name)
			continue
		}
		c.mu.Lock()
		closed := c.closedTargets[c.foldUnlocked(channel.Name)]
		c.mu.Unlock()
		if closed {
			gone = append(gone, channel.Name)
			continue
		}
		v := c.conversation(channel.Name)
		v.Joined = true
		v.Kicked = nil
		w.acceptInvitation(c.config.ID, c.fold(channel.Name))
		v.Topic = channel.Topic
		v.MemberIDs = []string{}
		v.MemberModes = map[string]string{}
		modes := strings.Fields(channel.Modes.String())
		v.Modes = ""
		if len(modes) > 0 {
			v.Modes = strings.TrimPrefix(modes[0], "+")
		}
		v.ModeParams = map[string]string{}
		groups := strings.Split(w.networks[c.config.ID].ISupport["CHANMODES"], ",")
		if len(groups) == 4 {
			for _, mode := range groups[1] + groups[2] {
				if mode == 'k' {
					continue
				} // Channel keys are write-only.
				if value, ok := channel.Modes.Get(string(mode)); ok {
					v.ModeParams[string(mode)] = value
				}
			}
		}
		for _, u := range channel.Users(client) {
			member := c.member(u.Nick, u.Extras.Account)
			v.MemberIDs = append(v.MemberIDs, member.ID)
			if perms, ok := u.Perms.Lookup(channel.Name); ok && perms.Prefixes != "" {
				v.MemberModes[member.ID] = perms.Prefixes
			}
		}
		slices.Sort(v.MemberIDs)
		legacyPresentation := v.Party != nil
		if legacyPresentation && c.manager != nil && c.manager.options.Store != nil {
			if party, err := c.manager.options.Store.Party(v.Party.ID); err == nil && (party.ConversionPending || party.State == "converted") {
				legacyPresentation = false
			}
		}
		if legacyPresentation {
			v.MemberIDs = []string{}
			for _, member := range v.Party.Members {
				v.MemberIDs = append(v.MemberIDs, member.AccountID)
			}
			v.MemberModes = map[string]string{}
			v.Modes = ""
			v.ModeParams = nil
			v.Topic = v.Party.Description
			v.ChannelSettings = nil
		}
		c.applyChannelSettings(&v)
		joined[v.ID] = true
		before := w.conversations[v.ID]
		w.conversations[v.ID] = v
		if !equal(before, v) || !equal(before.ChannelIdentity, v.ChannelIdentity) {
			w.emit("conversation.updated", v)
		}
	}
	left := func(cid string) {
		v, ok := w.conversations[cid]
		if ok && v.NetworkID == c.config.ID && v.Kind == "channel" && v.Joined && !joined[cid] {
			v.Joined = false
			v.MemberIDs = []string{}
			clearChannelSettings(&v)
			w.conversations[cid] = v
			w.emit("conversation.updated", v)
		}
	}
	if scope.all {
		for cid := range w.conversations {
			left(cid)
		}
		return
	}
	// A partial refresh only learns about the channels its event named.
	for _, name := range gone {
		left(c.conversationID(name))
	}
}

func (c *connection) receiveQuit(e girc.Event) {
	if e.Command == "QUIT" && e.Source != nil {
		w := c.owner
		w.mu.Lock()
		v := c.member(e.Source.Name, "")
		v.Status = "offline"
		if v.ID == w.self.ID {
			w.saveNetworkSelf(c.config.ID, v)
		} else {
			w.members[v.ID] = v
			w.emit("member.updated", v)
		}
		w.mu.Unlock()
	}
}

func (c *connection) receiveMemberSync(client *girc.Client, e girc.Event) {
	if e.Command == "JOIN" && e.Source != nil && len(e.Params) > 0 && c.fold(e.Source.Name) == c.fold(client.GetNick()) {
		c.owner.mu.Lock()
		delete(c.readMarkersSeen, c.conversationID(e.Params[0]))
		c.owner.mu.Unlock()
		c.requestBotFlags(e.Params[0])
	}
	if e.Source != nil && (e.Command == "QUIT" || e.Command == "JOIN" && len(e.Params) >= 3) {
		w := c.owner
		w.mu.Lock()
		if memberID := c.nicks[c.fold(e.Source.Name)]; memberID != "" {
			// Extended JOIN is fresh server state; a reused nick must not keep
			// an earlier session's SETNAME override.
			delete(c.realnames, memberID)
		}
		w.mu.Unlock()
	}
	if e.Command == "JOIN" && e.Source != nil && c.fold(e.Source.Name) == c.fold(client.GetNick()) && len(e.Params) > 0 && c.hasAll("draft/chathistory", "batch", "server-time") {
		c.catchup(e.Params[0])
	}
	c.metadataMembership(client, e)
	c.createdChannel(client, e)
}
