package irc

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

type ChannelCreate struct {
	Name       string            `json:"name" minLength:"1" maxLength:"100"`
	Topic      *string           `json:"topic,omitempty" maxLength:"2000"`
	Flags      string            `json:"flags,omitempty" maxLength:"100"`
	ModeParams map[string]string `json:"modeParams,omitempty"`
}
type ChannelCreateResult struct {
	Conversation Conversation `json:"conversation"`
	Registered   bool         `json:"registered"`
	Configured   bool         `json:"configured"`
	Warnings     []string     `json:"warnings" nullable:"false"`
}
type ChannelAvailability struct {
	Status string `json:"status" enum:"available,unavailable,unknown"`
}

type channelPlan struct {
	target  string
	desired map[byte]bool
	params  map[string]string
	topic   *string
}

type channelClientKey struct{}

func (c *connection) creatingTarget(target string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.explicitCreating[c.foldUnlocked(target)]
}

// Only explicitly requested modes change. In particular, an open preset must
// not clear unrelated server defaults. Keys never enter a DTO, event or trace.
func planChannel(in ChannelCreate, support map[string]string) (channelPlan, error) {
	p := channelPlan{target: strings.TrimSpace(in.Name), desired: map[byte]bool{}, params: in.ModeParams, topic: in.Topic}
	types := support["CHANTYPES"]
	if types == "" {
		types = "#&"
	}
	if p.target != "" && !strings.ContainsRune(types, rune(p.target[0])) {
		p.target = "#" + p.target
	}
	maximum := 100
	if n, err := strconv.Atoi(support["CHANNELLEN"]); err == nil && n > 0 {
		maximum = min(maximum, n)
	}
	if !utf8.ValidString(p.target) || len(p.target) > maximum || !girc.IsValidChannel(p.target) || !clean(p.target, maximum) || strings.ContainsAny(p.target, " ,:\x07") || !strings.ContainsRune(types, rune(p.target[0])) {
		return p, serviceError(422, "Enter a valid channel name within this server's length limit.")
	}
	groups := strings.Split(support["CHANMODES"], ",")
	if len(groups) != 4 {
		return p, serviceError(501, "This server does not advertise its channel settings. Use Join channel instead.")
	}
	allowed := groups[1] + groups[2] + groups[3]
	adding := true
	for i := 0; i < len(in.Flags); i++ {
		mode := in.Flags[i]
		if mode == '+' {
			adding = true
			continue
		}
		if mode == '-' {
			adding = false
			continue
		}
		if !strings.ContainsRune(allowed, rune(mode)) {
			return p, serviceError(422, "A requested channel setting is not supported by this server.")
		}
		if _, exists := p.desired[mode]; exists {
			return p, serviceError(422, "A channel setting was specified more than once.")
		}
		p.desired[mode] = adding
	}
	if len(in.ModeParams) > 16 {
		return p, serviceError(422, "Too many channel settings were requested.")
	}
	for mode, value := range in.ModeParams {
		if len(mode) != 1 || !strings.Contains(groups[1]+groups[2], mode) || !utf8.ValidString(value) || !clean(value, 100) || value == "" || strings.ContainsAny(value, " ,:") || strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
			return p, serviceError(422, "Enter valid values for the supported channel settings.")
		}
		if enabled, exists := p.desired[mode[0]]; exists && !enabled {
			return p, serviceError(422, "A disabled channel setting cannot have a value.")
		}
		p.desired[mode[0]] = true
		if mode == "k" {
			if maximum, err := strconv.Atoi(support["KEYLEN"]); err == nil && maximum > 0 && len(value) > maximum {
				return p, serviceError(422, "Shorten the channel key to fit this server's key limit.")
			}
		}
		if mode == "f" && !girc.IsValidChannel(value) {
			return p, serviceError(422, "Choose a valid channel for forwarding.")
		}
		if mode == "l" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 2147483647 {
				return p, serviceError(422, "Enter a positive channel member limit.")
			}
		}
	}
	for mode, enabled := range p.desired {
		if enabled && strings.ContainsRune(groups[1]+groups[2], rune(mode)) && p.params[string(mode)] == "" {
			return p, serviceError(422, "A channel setting needs a value.")
		}
	}
	if in.Topic != nil {
		maximum := 300
		if n, err := strconv.Atoi(support["TOPICLEN"]); err == nil && n > 0 {
			maximum = min(n, 2000)
		}
		if !utf8.ValidString(*in.Topic) || !clean(*in.Topic, maximum) || len(*in.Topic)+len(p.target) > 400 {
			return p, serviceError(422, "Shorten the topic to fit this server's topic limit.")
		}
	}
	return p, nil
}

func (c *connection) associatedChannels(support map[string]string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.home() && c.manager.options.AssociatedServer != nil && !bouncerRoute(c.config, c.caps, support)
}

func (m *Manager) ChannelAvailability(ctx context.Context, owner, network, name string) (ChannelAvailability, error) {
	c, err := m.connection(owner, network)
	if err != nil {
		return ChannelAvailability{}, err
	}
	c.owner.mu.Lock()
	support := clone(c.owner.networks[network].ISupport)
	c.owner.mu.Unlock()
	p, err := planChannel(ChannelCreate{Name: name}, support)
	if err != nil {
		return ChannelAvailability{}, err
	}
	if c.managedPartyTarget(p.target) {
		return ChannelAvailability{Status: "unavailable"}, nil
	}
	c.owner.mu.Lock()
	joined := c.owner.conversations[c.conversationID(p.target)].Joined
	c.owner.mu.Unlock()
	if joined {
		return ChannelAvailability{Status: "unavailable"}, nil
	}
	if c.associatedChannels(support) {
		select {
		case m.checkSlots <- struct{}{}:
			defer func() { <-m.checkSlots }()
		default:
			return ChannelAvailability{Status: "unknown"}, nil
		}
		ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		rows, err := m.options.AssociatedServer.Channels(ctx)
		if err != nil {
			return ChannelAvailability{Status: "unknown"}, nil
		}
		for _, row := range rows {
			if model.FoldIdentity(row.Name, support["CASEMAPPING"]) == model.FoldIdentity(p.target, support["CASEMAPPING"]) {
				return ChannelAvailability{Status: "unavailable"}, nil
			}
		}
		return ChannelAvailability{Status: "available"}, nil
	}
	// Public discovery remains the requesting user's privacy-filtered IRC LIST.
	return ChannelAvailability{Status: "unknown"}, nil
}

// Creation is a bounded ordinary-IRC workflow, not an atomic reservation.
// Serialize attempts across local accounts for a server/target, and serialize
// labeled exchanges on the user's existing connection with other service work.
func (m *Manager) CreateChannel(ctx context.Context, owner, network string, in ChannelCreate) (ChannelCreateResult, error) {
	out := ChannelCreateResult{Warnings: []string{}}
	c, err := m.connection(owner, network)
	if err != nil {
		return out, err
	}
	w := c.owner
	w.mu.Lock()
	n := clone(w.networks[network])
	w.mu.Unlock()
	p, err := planChannel(in, n.ISupport)
	if err != nil {
		return out, err
	}
	if c.managedPartyTarget(p.target) {
		return out, serviceError(409, "This channel name is unavailable. Choose another name.")
	}
	c.mu.Lock()
	client, ready, labeled, services := c.client, c.ready, c.caps["labeled-response"], c.ergoServices
	c.mu.Unlock()
	if !ready || client == nil {
		return out, ErrDisconnected
	}
	if !labeled {
		return out, serviceError(501, "This network cannot confirm channel creation. Use Join channel instead.")
	}
	if target := p.params["f"]; target != "" {
		w.mu.Lock()
		forward := w.conversations[c.conversationID(target)]
		allowed := forward.Joined && channelAt(forward, n.ISupport, w.self.ID, 'o')
		w.mu.Unlock()
		if !allowed {
			return out, serviceError(422, "Join the forwarding channel as an operator before using it in channel settings.")
		}
	}
	if services && !serviceAccount(n.Account) {
		return out, serviceError(403, "Sign in to your IRC account before creating a registered channel.")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	key := net.JoinHostPort(strings.ToLower(c.config.Host), strconv.Itoa(c.config.Port)) + "/" + strconv.FormatBool(c.config.TLS) + "/" + model.FoldIdentity(p.target, n.ISupport["CASEMAPPING"])
	m.mu.Lock()
	if m.channelCreates == nil {
		m.channelCreates = map[string]bool{}
	}
	if m.channelCreates[key] || len(m.channelCreates) >= 16 {
		m.mu.Unlock()
		return out, ErrBusy
	}
	m.channelCreates[key] = true
	m.mu.Unlock()
	defer func() { m.mu.Lock(); delete(m.channelCreates, key); m.mu.Unlock() }()
	select {
	case c.serviceGate <- struct{}{}:
		defer func() { <-c.serviceGate }()
	case <-ctx.Done():
		return out, ErrBusy
	case <-c.ctx.Done():
		return out, ErrDisconnected
	}
	select {
	case m.serviceSlots <- struct{}{}:
		defer func() { <-m.serviceSlots }()
	case <-ctx.Done():
		return out, ErrBusy
	}
	w.mu.Lock()
	joined := w.conversations[c.conversationID(p.target)].Joined
	w.mu.Unlock()
	if joined {
		return out, serviceError(409, "This channel name is unavailable. Choose another name.")
	}
	availability, err := m.ChannelAvailability(ctx, owner, network, p.target)
	if err != nil {
		return out, err
	}
	if availability.Status == "unavailable" {
		return out, serviceError(409, "This channel name is unavailable. Choose another name.")
	}
	if c.associatedChannels(n.ISupport) && availability.Status != "available" {
		return out, serviceError(503, "Channel availability could not be checked. Try again shortly.")
	}
	if services {
		info, err := c.registrationInfo(ctx, p.target)
		if err != nil {
			return out, err
		}
		if info.Registered {
			return out, serviceError(409, "This channel name is unavailable. Choose another name.")
		}
	} else {
		rows, err := c.channelExchange(ctx, &girc.Event{Command: "LIST", Params: []string{p.target}}, "323")
		if err != nil {
			return out, err
		}
		for _, row := range rows {
			if row.Command == "322" {
				return out, serviceError(409, "This channel name is unavailable. Choose another name.")
			}
		}
	}
	c.mu.Lock()
	if c.client != client || !c.ready {
		c.mu.Unlock()
		return out, ErrDisconnected
	}
	if c.explicitCreating == nil {
		c.explicitCreating = map[string]bool{}
	}
	c.explicitCreating[c.foldUnlocked(p.target)] = true
	delete(c.creating, girc.ToRFC1459(p.target))
	delete(c.closedTargets, c.foldUnlocked(p.target))
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.explicitCreating, c.foldUnlocked(p.target)); c.mu.Unlock() }()
	started := time.Now()
	ctx = context.WithValue(ctx, channelClientKey{}, client)
	joinReplies, joinErr := c.channelExchange(ctx, &girc.Event{Command: "JOIN", Params: []string{p.target}}, "366")
	w.mu.Lock()
	out.Conversation = clone(c.conversation(p.target))
	w.mu.Unlock()
	partial := func(message string) (ChannelCreateResult, error) {
		out.Warnings = append(out.Warnings, message)
		w.mu.Lock()
		out.Conversation = clone(w.conversations[out.Conversation.ID])
		w.mu.Unlock()
		return out, nil
	}
	if joinErr != nil {
		return partial("The server did not confirm the join. Check the channel before retrying; no settings were applied.")
	}
	// A sole operator is the ordinary IRC creation signal. Read the completed
	// labeled NAMES reply and our owned snapshot, never girc's mutable mode cache.
	if !soleChannelCreator(joinReplies, p.target, n.Nick, n.ISupport) || c.awaitChannelControl(ctx, p.target) != nil {
		return out, serviceError(409, "The channel already exists or your control could not be verified. No settings were applied.")
	}
	replies, err := c.channelExchange(ctx, &girc.Event{Command: "MODE", Params: []string{p.target}}, "324")
	if err != nil {
		return partial("The channel was joined, but its current settings could not be verified. No settings were applied.")
	}
	currentModes, validModes := channelModeValues(replies, n.ISupport)
	if !validModes {
		return partial("The channel was joined, but its current settings could not be verified. No settings were applied.")
	}
	for _, reply := range replies {
		if reply.Command == "329" && len(reply.Params) >= 3 {
			seconds, parseErr := strconv.ParseInt(reply.Params[2], 10, 64)
			if parseErr == nil && seconds < started.Unix() {
				return out, serviceError(409, "This channel existed before this request. No settings were applied.")
			}
		}
	}
	// Recheck registration after JOIN: an existing-name race must never turn
	// into configuration of another founder's registered channel.
	if services {
		info, err := c.registrationInfo(ctx, p.target)
		if err != nil {
			return partial("The channel was joined, but registration could not be checked. No settings were applied.")
		}
		if info.Registered {
			return out, serviceError(409, "This channel was already registered. No settings were applied.")
		}
		lines, registerErr := c.serviceExchange(ctx, "REGISTER "+p.target)
		if registerErr == nil && (len(lines) != 1 || !strings.EqualFold(lines[0], "Channel "+p.target+" successfully registered")) {
			registerErr = unexpectedServiceReply()
		}
		if registerErr == nil {
			info, registerErr = c.registrationInfo(ctx, p.target)
		}
		if registerErr == nil && (!info.Registered || !strings.EqualFold(info.Founder, n.Account)) {
			registerErr = unexpectedServiceReply()
		}
		if registerErr != nil {
			var failure *ServiceError
			if errors.As(registerErr, &failure) && failure.Status == 409 && strings.Contains(failure.Detail, "already registered") {
				return out, serviceError(409, "Another account registered this channel. No settings were applied.")
			}
			current, checkErr := c.registrationInfo(ctx, p.target)
			if checkErr != nil || current.Registered && !strings.EqualFold(current.Founder, n.Account) {
				return partial("Registration could not be verified. No settings were applied; refresh the channel before retrying.")
			}
			out.Registered = current.Registered && strings.EqualFold(current.Founder, n.Account)
			// We still control a new native channel. Apply requested protection,
			// then truthfully return the registration failure separately.
			out.Warnings = append(out.Warnings, "The channel was created, but registration failed. Register it from channel settings before leaving.")
		} else {
			out.Registered = true
		}
	} else {
		out.Warnings = append(out.Warnings, "This network has no supported ChanServ. The channel uses native operator ownership and may disappear when empty.")
	}
	c.mu.Lock()
	sameClient := c.client == client && c.ready
	c.desired[c.foldUnlocked(p.target)] = p.target
	c.mu.Unlock()
	if !sameClient {
		return partial("The connection changed. Channel settings were not confirmed; refresh before retrying.")
	}
	if err := c.saveTargets(); err != nil {
		return partial("The channel was created, but automatic rejoin could not be saved.")
	}
	// Apply privacy first; no messages or invitations are sent by this workflow.
	order := "isk" + strings.Join(strings.Split(n.ISupport["CHANMODES"], ",")[1:], "")
	seen := map[byte]bool{}
	for i := 0; i < len(order); i++ {
		mode := order[i]
		if seen[mode] {
			continue
		}
		seen[mode] = true
		enabled, requested := p.desired[mode]
		if !requested {
			continue
		}
		old, exists := currentModes[mode]
		if exists == enabled && (!enabled || p.params[string(mode)] == "" || old == p.params[string(mode)]) {
			continue
		}
		sign := "+"
		if !enabled {
			sign = "-"
		}
		params := []string{p.target, sign + string(mode)}
		groups := strings.Split(n.ISupport["CHANMODES"], ",")
		if strings.ContainsRune(groups[1], rune(mode)) || enabled && strings.ContainsRune(groups[2], rune(mode)) {
			value := p.params[string(mode)]
			if !enabled {
				value = old
			}
			if value == "" {
				return partial("A channel setting needs its current value. Finish the change in channel settings.")
			}
			params = append(params, value)
		}
		if _, err = c.channelExchange(ctx, &girc.Event{Command: "MODE", Params: params}, ""); err != nil {
			return partial("The channel was created, but some settings failed. Review its privacy and channel settings before inviting anyone.")
		}
	}
	replies, err = c.channelExchange(ctx, &girc.Event{Command: "MODE", Params: []string{p.target}}, "324")
	if err != nil || !confirmedChannelModes(replies, p, n.ISupport) {
		return partial("The channel was created, but its requested settings were not confirmed. Review its privacy before inviting anyone.")
	}
	if p.topic != nil {
		if _, err = c.channelExchange(ctx, &girc.Event{Command: "TOPIC", Params: []string{p.target, *p.topic}}, ""); err != nil {
			return partial("The channel was created, but its topic could not be saved.")
		}
		replies, err = c.channelExchange(ctx, &girc.Event{Command: "TOPIC", Params: []string{p.target}}, "")
		found := false
		for _, reply := range replies {
			if reply.Command == "332" && reply.Last() == *p.topic || reply.Command == "331" && *p.topic == "" {
				found = true
			}
		}
		if err != nil || !found {
			return partial("The channel was created, but its topic was not confirmed.")
		}
	}
	out.Configured = true
	w.mu.Lock()
	out.Conversation = clone(w.conversations[out.Conversation.ID])
	w.emit("channel-services.updated", model.ChannelServicesChange{ConversationID: out.Conversation.ID})
	w.mu.Unlock()
	return out, nil
}

func confirmedChannelModes(replies []girc.Event, p channelPlan, support map[string]string) bool {
	values, ok := channelModeValues(replies, support)
	if !ok {
		return false
	}
	for mode, enabled := range p.desired {
		value, exists := values[mode]
		if exists != enabled || enabled && p.params[string(mode)] != "" && value != p.params[string(mode)] {
			return false
		}
	}
	return true
}

func channelModeValues(replies []girc.Event, support map[string]string) (map[byte]string, bool) {
	groups := strings.Split(support["CHANMODES"], ",")
	if len(groups) != 4 {
		return nil, false
	}
	for _, reply := range replies {
		if reply.Command != "324" || len(reply.Params) < 3 {
			continue
		}
		modes, params := reply.Params[2], reply.Params[3:]
		values := map[byte]string{}
		for i := 0; i < len(modes); i++ {
			mode := modes[i]
			if mode == '+' || mode == '-' {
				continue
			}
			values[mode] = ""
			if strings.ContainsRune(groups[1]+groups[2], rune(mode)) {
				if len(params) == 0 {
					return nil, false
				}
				values[mode], params = params[0], params[1:]
			}
		}
		return values, true
	}
	return nil, false
}

func soleChannelCreator(replies []girc.Event, target, nick string, support map[string]string) bool {
	prefix := support["PREFIX"]
	_, symbols, ok := strings.Cut(prefix, ")")
	if !ok {
		symbols = "~&@%+"
	}
	names := []string{}
	for _, reply := range replies {
		if reply.Command == "353" && len(reply.Params) >= 4 && model.FoldIdentity(reply.Params[2], support["CASEMAPPING"]) == model.FoldIdentity(target, support["CASEMAPPING"]) {
			names = append(names, strings.Fields(reply.Last())...)
		}
	}
	if len(names) != 1 {
		return false
	}
	name := strings.TrimLeft(names[0], symbols)
	name, _, _ = strings.Cut(name, "!")
	return model.FoldIdentity(name, support["CASEMAPPING"]) == model.FoldIdentity(nick, support["CASEMAPPING"])
}

func (c *connection) awaitChannelControl(ctx context.Context, target string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		w := c.owner
		w.mu.Lock()
		conv := w.conversations[c.conversationID(target)]
		controlled := conv.Joined && channelAt(conv, w.networks[c.config.ID].ISupport, w.self.ID, 'o')
		w.mu.Unlock()
		if controlled {
			return nil
		}
		select {
		case <-ctx.Done():
			return ErrForbidden
		case <-c.ctx.Done():
			return ErrDisconnected
		case <-ticker.C:
		}
	}
}
