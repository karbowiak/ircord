package irc

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func (c *connection) connected(client *girc.Client) {
	c.mu.Lock()
	current := c.client == client && c.ctx.Err() == nil
	c.mu.Unlock()
	if !current {
		return
	}
	if c.config.SASLUser != "" && !client.HasCapability("sasl") {
		c.mu.Lock()
		c.permanent = true
		c.mu.Unlock()
		client.Close()
		return
	}
	caps := []string{}
	c.mu.Lock()
	for _, cap := range requestedCaps {
		if c.caps[cap] {
			caps = append(caps, cap)
		}
	}
	c.ready = true
	desired := orderedTargets(c.desired, c.config.Channels)
	c.mu.Unlock()
	w := c.owner
	w.mu.Lock()
	c.restoreNicks()
	n := w.networks[c.config.ID]
	n.Status = "connected"
	n.Capabilities = caps
	n.Nick = client.GetNick()
	c.mu.Lock()
	c.lineBytes = safeMessageLineBytes(client.MaxEventLength())
	n.Features = c.features(n.ISupport)
	c.mu.Unlock()
	w.networks[n.ID] = n
	w.emit("network.updated", n)
	w.self.Nick = n.Nick
	if w.self.NameSource != "profile" {
		resolveMemberName(&w.self)
	}
	w.members[w.self.ID] = w.self
	c.nicks[c.fold(n.Nick)] = w.self.ID
	w.emit("profile.updated", w.self)
	w.mu.Unlock()
	c.connectionState("connected", client, nil)
	if c.has("draft/metadata-3") {
		c.subscribeMetadata(client)
	}
	c.resumePresence()
	c.recoverDirects()
	// Ergo can restore an account's existing channel memberships before girc
	// announces CONNECTED. Those JOINs arrived before our capability snapshot;
	// recover their history and marker now instead of relying on another JOIN.
	for _, channel := range client.Channels() {
		c.requestReadMarker(channel.Name)
		if c.hasAll("draft/chathistory", "batch", "server-time") {
			c.catchup(channel.Name)
		}
	}
	if c.config.Auth != nil && c.config.Auth.Method == "nickserv" && c.config.Auth.Password != nil {
		_ = c.enqueue(&girc.Event{Command: "PRIVMSG", Params: []string{"NickServ", "IDENTIFY " + *c.config.Auth.Password}, Sensitive: true})
	}
	setup := []*girc.Event{}
	for _, entry := range desired {
		setup = append(setup, &girc.Event{Command: "JOIN", Params: strings.Fields(entry)})
	}
	for _, line := range c.config.Perform {
		if event, err := rawEvent(line); err == nil {
			setup = append(setup, event)
		}
	}
	if len(setup) > 0 {
		select {
		case c.queue <- setup:
		case <-c.ctx.Done():
		}
	}
}
func (c *connection) receive(client *girc.Client, e girc.Event) {
	if c.ctx.Err() != nil {
		return
	}
	if c.observeServices(client, e) {
		return
	}
	c.capture(e, false)
	if e.Command == "CAP" {
		c.capabilityEvent(client, e)
	}
	c.listItem(e)
	c.modeListItem(e)
	if c.closedEvent(e) {
		return
	}
	c.membership(e)
	if e.Command == "QUIT" && e.Source != nil {
		w := c.owner
		w.mu.Lock()
		v := c.member(e.Source.Name, "")
		v.Status = "offline"
		w.members[v.ID] = v
		w.emit("member.updated", v)
		w.mu.Unlock()
	}
	if e.Command == "BATCH" {
		c.batch(e)
		c.historyBatch(e)
		return
	}
	if batch := e.Tags["batch"]; batch != "" {
		c.mu.Lock()
		b := c.batches[batch]
		if b != nil {
			if b.lines == 0 {
				opening := b.first
				b.first = *e.Copy()
				if b.first.Tags == nil {
					b.first.Tags = girc.Tags{}
				}
				for key, value := range opening.Tags {
					if _, ok := b.first.Tags[key]; !ok {
						b.first.Tags[key] = value
					}
				}
			} else if _, concat := e.Tags["draft/multiline-concat"]; !concat {
				b.text += "\n"
			}
			b.lines++
			b.text += e.Last()
			if len(b.text) > 16384 || b.lines > 100 {
				delete(c.batches, batch)
			}
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
	}
	if e.Command != "BATCH" {
		c.historyItem(e.Tags["batch"], e)
	}
	switch e.Command {
	case "902", "904", "905", "906", "432", "463", "464", "465":
		c.mu.Lock()
		c.permanent = true
		c.registrationError = c.redactSecrets(e.Last())
		c.mu.Unlock()
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
	case "MARKREAD":
		c.receiveReadMarker(e)
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
	case "333", "329":
		if len(e.Params) >= 3 {
			w := c.owner
			w.mu.Lock()
			v := c.conversation(e.Params[1])
			index := 2
			if e.Command == "333" && len(e.Params) > 3 {
				v.TopicSetBy = e.Params[2]
				index = 3
			}
			if seconds, err := strconv.ParseInt(e.Params[index], 10, 64); err == nil {
				at := time.Unix(seconds, 0).UTC()
				if e.Command == "333" {
					v.TopicSetAt = &at
				} else {
					v.CreatedAt = &at
				}
			}
			w.conversations[v.ID] = v
			w.emit("conversation.updated", v)
			w.mu.Unlock()
		}
	case "328":
		if len(e.Params) >= 3 {
			w := c.owner
			w.mu.Lock()
			v := c.conversation(e.Params[1])
			v.URL = e.Last()
			w.conversations[v.ID] = v
			w.emit("conversation.updated", v)
			w.mu.Unlock()
		}
	case "JOIN", "PART", "KICK", "QUIT", "353", "366", "352", "354", "MODE", "ACCOUNT", "CHGHOST":
		if e.Command == "JOIN" && e.Source != nil && len(e.Params) > 0 && c.fold(e.Source.Name) == c.fold(client.GetNick()) {
			c.owner.mu.Lock()
			delete(c.readMarkersSeen, c.conversationID(e.Params[0]))
			c.owner.mu.Unlock()
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
	case "TOPIC", "332":
		if len(e.Params) >= 2 {
			target := e.Params[0]
			if e.Command == "332" && len(e.Params) >= 3 {
				target = e.Params[1]
			}
			w := c.owner
			w.mu.Lock()
			v := c.conversation(target)
			v.Topic = e.Last()
			if e.Command == "TOPIC" && e.Source != nil {
				v.TopicSetBy = e.Source.String()
				at := time.Now().UTC()
				v.TopicSetAt = &at
			}
			w.conversations[v.ID] = v
			w.emit("conversation.updated", v)
			w.mu.Unlock()
		}
	case "761", "METADATA":
		c.profileCheckReply(e)
		c.metadata(e)
	case "774":
		c.retryMetadataSync(e)
	case "766":
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
	case "REDACT":
		c.redact(e)
	case "ACK":
		c.redactionResult(c.eventLabel(e), "")
	case "433":
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
	case "FAIL", "400", "401", "403", "404", "405", "407", "411", "412", "417", "421", "442", "443", "471", "472", "473", "474", "475", "477", "478", "482":
		c.historyFailure(e)
		label := e.Tags["label"]
		c.mu.Lock()
		if label == "" {
			label = c.batchLabels[e.Tags["batch"]]
		}
		_, pending := c.pending[label]
		if pending {
			delete(c.pending, label)
		}
		c.mu.Unlock()
		if c.redactionResult(label, e.Last()) {
			return
		}
		w := c.owner
		w.mu.Lock()
		if strings.HasPrefix(label, "raw:") {
			w.sendCommandError(strings.TrimPrefix(label, "raw:"), e.Last())
		} else {
			w.commandError(label, e.Last())
		}
		w.mu.Unlock()
	}
}
func (c *connection) batch(e girc.Event) {
	if len(e.Params) == 0 || len(e.Params[0]) < 2 {
		return
	}
	ref := e.Params[0][1:]
	c.mu.Lock()
	if e.Params[0][0] == '+' {
		if len(e.Params) >= 2 && e.Params[1] == "draft/multiline" && len(c.batches) < 32 {
			parent := e.Tags["batch"]
			c.batches[ref] = &multiline{first: e, historical: c.historyBatches[parent], historyRef: parent}
		}
		if len(e.Params) >= 2 && e.Params[1] == "chathistory" {
			c.historyBatches[ref] = true
		}
		c.openMetadataSync(ref, e)
		c.mu.Unlock()
		return
	}
	b := c.batches[ref]
	delete(c.batches, ref)
	delete(c.historyBatches, ref)
	sync := c.metadataSyncs[ref]
	delete(c.metadataSyncs, ref)
	c.mu.Unlock()
	if sync != nil {
		c.finishMetadataSync(sync)
	}
	if b != nil && b.lines > 0 {
		b.first.Params[len(b.first.Params)-1] = b.text
		delete(b.first.Tags, "batch")
		c.historyItem(b.historyRef, b.first)
		c.messageWithHistory(b.first, b.historical, b.historyRef)
	}
}

// member is called with the workspace mutex held. IDs are network-scoped and
// survive nick changes; network accounts enrich identity but never authorize it.
func (c *connection) member(nick, account string) Member {
	w := c.owner
	if c.fold(nick) == c.fold(c.currentNick()) || (account != "" && account != "*" && c.config.SASLUser != "" && strings.EqualFold(account, c.config.SASLUser)) {
		return w.self
	}
	key := c.fold(nick)
	memberID := c.nicks[key]
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
	if !exists || !equal(before, v) {
		w.members[v.ID] = v
		w.emit("member.updated", v)
	}
	return v
}
func (c *connection) syncMembers(client *girc.Client) {
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	n := w.networks[c.config.ID]
	if n.Nick != client.GetNick() {
		n.Nick = client.GetNick()
		cfg := w.configs[n.ID]
		cfg.Nick = n.Nick
		w.configs[n.ID] = cfg
		if w.store != nil {
			if err := w.store.SaveNetwork(w.ownerID, cfg, true); err != nil {
				w.storageErr = err
			}
		}
		w.networks[n.ID] = n
		w.emit("network.updated", n)
	}
	for _, user := range client.Users() {
		v := c.member(user.Nick, user.Extras.Account)
		if v.ID == w.self.ID {
			continue
		}
		before := clone(v)
		v.Status = "online"
		if user.Extras.Away != "" {
			v.Status = "away"
		}
		v.Account = user.Extras.Account
		v.Username = user.Ident
		v.Host = user.Host
		name := user.Extras.Name
		if override, ok := c.realnames[v.ID]; ok {
			name = override
		}
		if name != "" {
			applyRealname(&v, name)
		}
		w.members[v.ID] = v
		if !equal(before, v) {
			w.emit("member.updated", v)
		}
	}
	joined := map[string]bool{}
	for _, channel := range client.Channels() {
		c.mu.Lock()
		closed := c.closedTargets[c.foldUnlocked(channel.Name)]
		c.mu.Unlock()
		if closed {
			continue
		}
		v := c.conversation(channel.Name)
		v.Joined = true
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
		c.applyChannelSettings(&v)
		joined[v.ID] = true
		before := w.conversations[v.ID]
		w.conversations[v.ID] = v
		if !equal(before, v) {
			w.emit("conversation.updated", v)
		}
	}
	for cid, v := range w.conversations {
		if v.NetworkID == c.config.ID && v.Kind == "channel" && v.Joined && !joined[cid] {
			v.Joined = false
			v.MemberIDs = []string{}
			clearChannelSettings(&v)
			w.conversations[cid] = v
			w.emit("conversation.updated", v)
		}
	}
}
func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
func (c *connection) message(e girc.Event) { c.messageWithHistory(e, false) }
func (c *connection) messageWithHistory(e girc.Event, forceHistory bool, refs ...string) {
	if e.Source == nil || len(e.Params) < 2 {
		return
	}
	c.mu.Lock()
	historical := forceHistory || c.historyBatches[e.Tags["batch"]]
	ref := e.Tags["batch"]
	if len(refs) > 0 {
		ref = refs[0]
	}
	position := int64(0)
	req := c.historyResponses[ref]
	if req != nil && req.sub == "BEFORE" && req.orderBase != 0 {
		position = req.orderBase + int64(req.result.Count)
	}
	// Catch-up after a disconnect or restart (CHATHISTORY AFTER the stored
	// watermark) recovers messages that were missed, unlike scrolling back
	// (BEFORE) or a new conversation's first fill (LATEST).
	recovered := historical && req != nil && req.sub == "AFTER"
	c.mu.Unlock()
	target := e.Params[0]
	if !girc.IsValidChannel(target) && c.fold(target) == c.fold(c.currentNick()) {
		target = e.Source.Name
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	conv := c.conversation(target)
	upstream := e.Tags["msgid"]
	messageID := id()
	if upstream != "" {
		messageID = c.config.ID + ":" + upstream
	}
	lookupKey := messageID
	if upstream != "" {
		lookupKey = upstream
	}
	existing, known := c.lookupMessage(conv.ID, lookupKey)
	if known && existing.ClientID != "" {
		return
	}

	sender := c.member(e.Source.Name, e.Tags["account"])
	if e.Source.Ident != "" && (sender.Username != e.Source.Ident || sender.Host != e.Source.Host) {
		sender.Username = e.Source.Ident
		sender.Host = e.Source.Host
		w.members[sender.ID] = sender
		w.emit("member.updated", sender)
	}
	at := e.Timestamp
	if parsed, err := time.Parse(time.RFC3339Nano, e.Tags["time"]); err == nil {
		at = parsed
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	msg := Message{Position: position, ID: messageID, ConversationID: conv.ID, SenderID: sender.ID, Text: e.Last(), CreatedAt: at, Kind: "message", Reactions: []Reaction{}, UpstreamID: upstream}
	if encoded := e.Tags["+ircord/gif"]; len(encoded) > 0 && len(encoded) <= 2500 {
		if raw, err := base64.RawURLEncoding.DecodeString(encoded); err == nil {
			var gif GIF
			if json.Unmarshal(raw, &gif) == nil && validGIF(&gif) && gif.URL == msg.Text {
				msg.GIF = &gif
			}
		}
	}
	if strings.HasPrefix(msg.Text, "\x01ACTION ") && strings.HasSuffix(msg.Text, "\x01") {
		msg.Kind = "action"
		msg.Text = strings.TrimSuffix(strings.TrimPrefix(msg.Text, "\x01ACTION "), "\x01")
	}
	if encoded := e.Tags["+ircord/attachment"]; len(encoded) > 0 && len(encoded) <= 1800 {
		if raw, err := base64.RawURLEncoding.DecodeString(encoded); err == nil {
			var a model.Attachment
			if json.Unmarshal(raw, &a) == nil && a.URL == msg.Text && validAttachment(a) {
				msg.Attachment = &a
			}
		}
	}
	if e.Command == "NOTICE" {
		msg.Kind = "notice"
	}
	if historical && strings.EqualFold(e.Source.Name, "HistServ") && strings.HasPrefix(upstream, "_") {
		msg.Kind = "system"
	}
	msg.SenderAccount = senderAccount(e)
	if original := e.Tags[editTag]; original != "" {
		switch c.incomingEdit(conv, sender, e, msg, historical) {
		case editConsumed:
			return
		case editPending:
			msg.EditOf = original
		}
	}
	needsAcknowledgment := false
	if sender.ID == w.self.ID {
		if account := e.Tags["account"]; account != "" && account != "*" {
			msg.Account = account
		}
		clientID := e.Tags["+ircord/client-id"]
		c.mu.Lock()
		p, exists := c.pending[clientID]
		if !exists && !historical {
			for key, candidate := range c.pending {
				if candidate.input.ConversationID == conv.ID && candidate.input.Text == msg.Text {
					clientID = key
					p = candidate
					exists = true
					break
				}
			}
		}
		if exists {
			delete(c.pending, clientID)
		}
		c.mu.Unlock()
		if !exists && clientID != "" && w.store != nil {
			saved, err := w.store.Command(w.ownerID, clientID)
			if err == nil && saved.ConversationID == conv.ID && saved.Input.Text == msg.Text {
				p = pending{input: saved.Input, messageID: saved.MessageID}
				exists = true
				needsAcknowledgment = saved.State != "sent"
			}
		}
		if exists {
			msg.ClientID = clientID
			msg.GIF = p.input.GIF
			if len(p.input.Attachments) == 1 && c.manager.options.Files != nil {
				msg.Attachment, _ = c.manager.options.Files.Attachment(w.ownerID, c.config.ID, p.input.Attachments[0])
			}
			if upstream == "" || p.messageID != "" && w.store != nil {
				if p.messageID != "" {
					msg.ID = p.messageID
				}
			}
			if p.input.ReplyToID != "" {
				if prior, ok := c.lookupMessage(conv.ID, p.input.ReplyToID); ok {
					msg.ReplyTo = &Reply{ID: prior.ID, Nick: w.members[prior.SenderID].Nick, Text: prior.Text}
				}
			}
		}
	}
	if known && !needsAcknowledgment {
		return
	}
	msg.Historical = historical
	msg.Mention = sender.ID != w.self.ID && msg.Kind != "system" && msg.Kind != "event" && (conv.Kind == "direct" || w.policy.Highlight(msg.Text, c.currentNick()))

	reply := e.Tags["+reply"]
	if reply == "" {
		reply = e.Tags["+draft/reply"]
	}
	// Only a reply tag naming a message IRCord has stored counts as a reply to you.
	replyToSelf := false
	if reply != "" && msg.ReplyTo == nil {
		if prior, ok := c.lookupMessage(conv.ID, reply); ok {
			msg.ReplyTo = &Reply{ID: prior.ID, Nick: w.members[prior.SenderID].Nick, Text: prior.Text}
			replyToSelf = prior.SenderID == w.self.ID && !prior.Deleted
		}
	}
	if w.store != nil && upstream != "" {
		if reactions, err := w.store.Reactions(w.ownerID, conv.ID, upstream); err == nil {
			msg.Reactions = reactions
		} else {
			w.storageErr = err
			return
		}
	}

	msg = c.manager.prepareMedia(w, msg)
	replaced := false
	for i, old := range w.messages[conv.ID] {
		if old.ID == msg.ID {
			w.messages[conv.ID][i] = msg
			replaced = true
			break
		}
	}
	if !replaced {
		w.messages[conv.ID] = append(w.messages[conv.ID], msg)
	}
	sort.SliceStable(w.messages[conv.ID], func(i, j int) bool { return w.messages[conv.ID][i].CreatedAt.Before(w.messages[conv.ID][j].CreatedAt) })
	if len(w.messages[conv.ID]) > 500 {
		w.messages[conv.ID] = append([]Message(nil), w.messages[conv.ID][len(w.messages[conv.ID])-500:]...)
	}
	w.boundCache(conv.ID)
	if !historical && sender.ID != w.self.ID && !w.policy.Ignored(sender.Nick, sender.Username, sender.Host) {
		conv.UnreadCount++
		if msg.Mention {
			conv.MentionCount++
		}
		w.conversations[conv.ID] = conv
		w.emitCounts(conv)
	}
	if historical && w.store != nil && !needsAcknowledgment {
		inserted, err := w.store.HistoryInsert(w.ownerID, msg)
		if err != nil {
			w.storageErr = err
		} else if inserted && recovered {
			c.manager.recordInbox(w, conv, msg, sender, c.currentNick(), replyToSelf)
		}
	} else {
		w.emit("message.created", msg)
		if !historical {
			c.manager.schedulePreview(w, conv, msg)
			c.manager.notifyMessage(w, conv, msg, sender, c.currentNick())
			c.manager.recordInbox(w, conv, msg, sender, c.currentNick(), replyToSelf)
		}
	}
	if w.storageErr == nil {
		c.applyPendingEdits(conv, msg, historical)
	}
}
func (c *connection) reaction(e girc.Event) {
	if e.Source == nil || len(e.Params) == 0 {
		return
	}
	reply := e.Tags["+reply"]
	if reply == "" {
		reply = e.Tags["+draft/reply"]
	}
	emoji, add := e.Tags["+draft/react"]
	if !add {
		emoji = e.Tags["+draft/unreact"]
	}
	if reply == "" || emoji == "" || len(emoji) > 64 {
		return
	}
	target := e.Params[0]
	if !girc.IsValidChannel(target) && c.fold(target) == c.fold(c.currentNick()) {
		target = e.Source.Name
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	conv := c.conversation(target)
	member := c.member(e.Source.Name, e.Tags["account"])
	if w.store != nil {
		at, err := time.Parse(time.RFC3339Nano, e.Tags["time"])
		if err != nil {
			at = time.Now().UTC()
		}
		c.mu.Lock()
		historical := c.historyBatches[e.Tags["batch"]]
		c.mu.Unlock()
		if err = w.store.SetReaction(w.ownerID, conv.ID, reply, member.ID, emoji, add, at, !historical); err != nil {
			w.storageErr = err
			return
		}
		msg, ok := c.lookupMessage(conv.ID, reply)
		if !ok || msg.Deleted {
			return
		}
		before := clone(msg)
		msg.Reactions, err = w.store.Reactions(w.ownerID, conv.ID, reply)
		if err != nil {
			w.storageErr = err
			return
		}
		for i, old := range w.messages[conv.ID] {
			if old.ID == msg.ID {
				w.messages[conv.ID][i] = msg
			}
		}
		if !equal(before, msg) {
			if historical {
				if err = w.store.SaveHistory(w.ownerID, msg); err != nil {
					w.storageErr = err
				}
			} else {
				w.emit("message.updated", msg)
			}
		}
		return
	}

	for i, msg := range w.messages[conv.ID] {
		if msg.UpstreamID != reply {
			continue
		}
		before := clone(msg)
		found := false
		for j, r := range msg.Reactions {
			if r.Emoji != emoji {
				continue
			}
			found = true
			if add && !slices.Contains(r.MemberIDs, member.ID) {
				r.MemberIDs = append(r.MemberIDs, member.ID)
			}
			if !add {
				r.MemberIDs = slices.DeleteFunc(r.MemberIDs, func(id string) bool { return id == member.ID })
			}
			msg.Reactions[j] = r
		}
		if add && !found {
			msg.Reactions = append(msg.Reactions, Reaction{Emoji: emoji, MemberIDs: []string{member.ID}})
		}
		msg.Reactions = slices.DeleteFunc(msg.Reactions, func(r Reaction) bool { return len(r.MemberIDs) == 0 })
		w.messages[conv.ID][i] = msg
		if !equal(before, msg) {
			w.emit("message.updated", msg)
		}
		return
	}
}
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
func (c *connection) redact(e girc.Event) {
	if e.Source == nil || len(e.Params) < 2 {
		return
	}
	label := c.eventLabel(e)
	c.mu.Lock()
	historical := c.historyBatches[e.Tags["batch"]]
	c.mu.Unlock()
	target := e.Params[0]
	if !girc.IsValidChannel(target) && c.fold(target) == c.fold(c.currentNick()) {
		target = e.Source.Name
	}
	w := c.owner
	w.mu.Lock()
	// Only an existing conversation and a stored message can be redacted; an
	// unsolicited REDACT must not create a conversation or member as a side effect.
	if conv, ok := w.conversations[c.conversationID(target)]; ok {
		if msg, found := c.lookupMessage(conv.ID, e.Params[1]); found {
			if c.mayRedact(conv, msg, e) {
				c.applyRedaction(conv, msg, historical)
			}
		} else {
			c.revertEdit(conv, e.Params[1], e, historical)
		}
	}
	w.mu.Unlock()
	c.redactionResult(label, "")
}

// mayRedact accepts a REDACT only from the message author, a channel operator, or
// a server. The server already enforces its rules; this keeps a lax or hostile
// peer from erasing other people's messages in our stored history.
func (c *connection) mayRedact(conv Conversation, msg Message, e girc.Event) bool {
	if e.Source.Ident == "" && e.Source.Host == "" && strings.Contains(e.Source.Name, ".") {
		return true
	}
	source := c.knownMember(e.Source.Name, e.Tags["account"])
	if source == "" {
		return false
	}
	if source == msg.SenderID {
		return true
	}
	return strings.ContainsAny(conv.MemberModes[source], "~&@%")
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

// applyRedaction replaces a message with a deletion marker and drops everything
// derived from its content: reactions, the mention marker and badge, reply
// excerpts that quote it, and copies of its text in the replay log and server
// window. It is idempotent. Callers hold w.mu; on a storage failure it sets
// w.storageErr and the caller reports it.
func (c *connection) applyRedaction(conv Conversation, msg Message, historical bool) Message {
	w := c.owner
	if msg.Deleted {
		return msg
	}
	clientID := msg.ClientID
	msg.Text = "Message deleted"
	c.manager.removeMessageMedia(w, msg)
	msg.Media = nil
	msg.Deleted = true
	msg.Mention = false
	msg.GIF = nil
	msg.Attachment = nil
	msg.ReplyTo = nil
	msg.Preview = nil
	msg.PreviewSource = ""
	msg.Reactions = []Reaction{}
	editIDs := make([]string, 0, len(msg.Edits))
	for _, v := range msg.Edits {
		editIDs = append(editIDs, v.UpstreamID)
	}
	msg.Edits, msg.EditedAt, msg.OriginalText = nil, nil, ""
	children := map[string]Message{}
	for _, child := range w.messages[conv.ID] {
		if child.ReplyTo != nil && child.ReplyTo.ID == msg.ID {
			children[child.ID] = clone(child)
		}
	}
	if w.store != nil {
		if err := w.store.ClearRedacted(w.ownerID, conv.ID, msg.ID, msg.UpstreamID); err != nil {
			w.storageErr = err
			return msg
		}
		stored, err := w.store.Replies(w.ownerID, conv.ID, msg.ID)
		if err != nil {
			w.storageErr = err
			return msg
		}
		for _, child := range stored {
			if _, ok := children[child.ID]; !ok {
				children[child.ID] = child
			}
		}
	}
	for i, old := range w.messages[conv.ID] {
		if old.ID == msg.ID {
			w.messages[conv.ID][i] = msg
		}
	}
	if !c.saveRedaction(msg, historical) {
		return msg
	}
	for _, child := range children {
		if child.ReplyTo.Text == msg.Text {
			continue
		}
		child.ReplyTo = &Reply{ID: msg.ID, Nick: child.ReplyTo.Nick, Text: msg.Text}
		for i, old := range w.messages[conv.ID] {
			if old.ID == child.ID {
				w.messages[conv.ID][i] = child
			}
		}
		if !c.saveRedaction(child, historical) {
			return msg
		}
	}
	c.scrubLines(msg, clientID)
	for _, edit := range editIDs {
		c.scrubLines(Message{UpstreamID: edit, Text: msg.Text}, "")
	}
	if w.store != nil && w.storageErr == nil {
		if err := w.store.ScrubRedacted(w.ownerID, c.config.ID, msg, clientID, editIDs...); err != nil {
			w.storageErr = err
			return msg
		}
	}
	// Message.Mention does not survive clone/storage, so recount from the store.
	if w.store != nil && w.storageErr == nil {
		if current, ok := w.conversations[conv.ID]; ok {
			count, err := w.store.Mentions(w.ownerID, conv.ID)
			if err != nil {
				w.storageErr = err
				return msg
			}
			if count != current.MentionCount {
				current.MentionCount = count
				w.conversations[conv.ID] = current
				w.emitCounts(current)
			}
		}
	}
	return msg
}

// scrubLines removes the deleted text from the in-memory server window and raw
// trace. Callers hold w.mu.
func (c *connection) scrubLines(msg Message, clientID string) {
	w := c.owner
	storage.ScrubServerLines(w.serverLogs[c.config.ID], msg.UpstreamID, clientID, msg.Text)
	storage.ScrubServerLines(w.trace[c.config.ID], msg.UpstreamID, clientID, msg.Text)
}
func (c *connection) saveRedaction(msg Message, historical bool) bool {
	w := c.owner
	if !historical {
		w.emit("message.updated", msg)
		return w.storageErr == nil
	}
	if w.store != nil {
		if err := w.store.SaveHistory(w.ownerID, msg); err != nil {
			w.storageErr = err
			return false
		}
	}
	return true
}
func (c *connection) eventLabel(e girc.Event) string {
	if label := e.Tags["label"]; label != "" {
		return label
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.batchLabels[e.Tags["batch"]]
}

// redactionResult completes a waiting outgoing REDACT or edit. reason is empty
// on success.
func (c *connection) redactionResult(label, reason string) bool {
	if !strings.HasPrefix(label, "redact:") && !strings.HasPrefix(label, "edit:") {
		return false
	}
	c.mu.Lock()
	wait := c.redactions[label]
	delete(c.redactions, label)
	c.mu.Unlock()
	if wait != nil {
		wait <- reason
	}
	return true
}
func (c *connection) closedEvent(e girc.Event) bool {
	index := 0
	switch e.Command {
	case "PRIVMSG", "NOTICE", "TAGMSG", "REDACT", "JOIN", "PART", "KICK", "TOPIC", "MODE":
	case "332", "333", "329", "328", "324":
		index = 1
	default:
		return false
	}
	if len(e.Params) <= index || !girc.IsValidChannel(e.Params[index]) {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closedTargets[c.foldUnlocked(e.Params[index])]
}
