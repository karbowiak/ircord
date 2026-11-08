package irc

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

// features requires c.mu. CAP values become effective only after ACK, never LS
// or NEW. Core IRC actions such as AWAY do not require an IRCv3 capability.
func (c *connection) features(isupport map[string]string) model.Features {
	f := model.Features{Metadata: []string{}}
	if !c.ready {
		return f
	}
	f.MessageLineBytes = c.lineBytes
	if f.MessageLineBytes <= 0 {
		f.MessageLineBytes = safeMessageLineBytes(girc.DefaultMaxLineLength - girc.DefaultMaxPrefixLength)
	}
	f.Setname, f.Away, f.Redact = c.caps["setname"], true, c.caps["draft/message-redaction"]
	policy := isupport["CLIENTTAGDENY"]
	f.Replies = c.caps["message-tags"] && tagAllowed(policy, "reply")
	f.Reactions = f.Replies && tagAllowed(policy, "draft/react") && tagAllowed(policy, "draft/unreact")
	f.Typing = c.caps["message-tags"] && tagAllowed(policy, "typing")
	f.ReadMarkers = c.caps["draft/read-marker"] && c.caps["server-time"]
	f.History = c.caps["draft/chathistory"] && c.caps["batch"] && c.caps["server-time"]
	if c.caps["draft/metadata-3"] {
		f.Metadata = []string{"display-name", "avatar", "color", "bio"}
	}
	if c.caps["draft/multiline"] && c.caps["batch"] && c.caps["message-tags"] {
		params := map[string]int{}
		valid := true
		for _, param := range strings.Split(c.capValues["draft/multiline"], ",") {
			key, value, _ := strings.Cut(param, "=")
			if key != "max-bytes" && key != "max-lines" {
				continue
			}
			if n, err := strconv.Atoi(value); err == nil && n > 0 {
				params[key] = n
			} else {
				valid = false
			}
		}
		// max-bytes is required by the draft. Malformed/missing limits must not
		// enable a send path whose bounds we cannot honor.
		if valid && params["max-bytes"] > 0 {
			lines := params["max-lines"]
			if lines == 0 {
				lines = 100
			}
			f.Multiline = &model.MultilineLimits{MaxBytes: min(params["max-bytes"], 4096), MaxLines: min(lines, 100)}
		}
	}
	return f
}

// girc splits events at equality as well as above its threshold. Stay below it
// so its word-based splitter cannot rewrite whitespace or multiline fragments.
func safeMessageLineBytes(threshold int) int {
	return max(0, min(threshold-1, 510))
}

// Observe the protocol's ACK and DEL directly in ALL_EVENTS. girc executes
// internal and external handlers for a command concurrently, so polling its
// HasCapability from a command handler can race its own CAP state update.
func (c *connection) capabilityEvent(client *girc.Client, e girc.Event) {
	if len(e.Params) < 3 {
		return
	}
	c.mu.Lock()
	if c.client != client || c.ctx.Err() != nil {
		c.mu.Unlock()
		return
	}
	if c.capOffers == nil {
		c.capOffers = map[string]string{}
	}
	if c.capValues == nil {
		c.capValues = map[string]string{}
	}
	for _, token := range strings.Fields(e.Last()) {
		name, value, hasValue := strings.Cut(token, "=")
		switch e.Params[1] {
		case "LS", "NEW":
			if slices.Contains(requestedCaps, name) {
				c.capOffers[name] = value
			}
		case "ACK":
			if strings.HasPrefix(name, "-") {
				delete(c.capValues, name[1:])
				delete(c.caps, name[1:])
				continue
			}
			if !hasValue {
				value = c.capOffers[name]
			}
			if slices.Contains(requestedCaps, name) {
				c.capValues[name] = value
				c.caps[name] = true
			}
		case "DEL":
			delete(c.caps, name)
			delete(c.capOffers, name)
			delete(c.capValues, name)
		}
	}
	c.mu.Unlock()
	c.refreshFeatures(client)
}

func (c *connection) refreshFeatures(client *girc.Client) {
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	c.mu.Lock()
	if !c.ready || c.client != client {
		c.mu.Unlock()
		return
	}
	caps := []string{}
	for _, cap := range requestedCaps {
		if c.caps[cap] {
			caps = append(caps, cap)
		}
	}
	c.lineBytes = safeMessageLineBytes(client.MaxEventLength())
	n := w.networks[c.config.ID]
	features := c.features(n.ISupport)
	c.mu.Unlock()
	if slices.Equal(n.Capabilities, caps) && reflect.DeepEqual(n.Features, features) {
		return
	}
	metadataAdded := len(n.Features.Metadata) == 0 && len(features.Metadata) != 0
	n.Capabilities, n.Features = caps, features
	w.networks[n.ID] = n
	w.emit("network.updated", n)
	info := w.info[n.ID]
	info.NegotiatedCapabilities = append([]string{}, caps...)
	w.info[n.ID] = info
	w.emit("connection.updated", info)
	if metadataAdded {
		_ = c.enqueue(&girc.Event{Command: "METADATA", Params: []string{"*", "SUB", "display-name", "avatar", "color", "bio"}})
		c.pushProfile(w.self)
	}
}

// A local fallback or synthetic HistServ event cannot be addressed upstream.
func addressable(msg Message) bool {
	return msg.UpstreamID != "" && !strings.HasPrefix(msg.UpstreamID, "_") && !msg.Deleted && (msg.Kind == "message" || msg.Kind == "action" || msg.Kind == "notice")
}

func (c *connection) hasAll(caps ...string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, cap := range caps {
		if !c.caps[cap] {
			return false
		}
	}
	return true
}

func channelAt(conv Conversation, support map[string]string, self string, mode byte) bool {
	prefix := support["PREFIX"]
	modes, prefixes := "qaohv", "~&@%+"
	if strings.HasPrefix(prefix, "(") {
		a, b, ok := strings.Cut(prefix[1:], ")")
		if ok && len(a) == len(b) && len(a) > 0 {
			modes, prefixes = a, b
		}
	}
	rank := strings.IndexByte(modes, mode)
	if rank < 0 {
		return false
	}
	for _, own := range conv.MemberModes[self] {
		if r := strings.IndexRune(prefixes, own); r >= 0 && r <= rank {
			return true
		}
	}
	return false
}
func canSpeak(conv Conversation, support map[string]string, self string) bool {
	return conv.Kind != "channel" || !strings.Contains(conv.Modes, "m") || channelAt(conv, support, self, 'v')
}

// Validate managed channel mutations against current membership and PREFIX.
// Servers remain authoritative (ACLs can be stricter than channel modes).
// Read-only MODE/TOPIC queries and arbitrary service text remain available.
func (c *connection) validateChannelCommand(event *girc.Event) error {
	index := 0
	switch event.Command {
	case "KICK":
		if len(event.Params) < 2 {
			return fmt.Errorf("a kick needs a channel and nickname")
		}
	case "INVITE":
		if len(event.Params) < 2 {
			return fmt.Errorf("an invite needs a nickname and channel")
		}
		index = 1
	case "TOPIC":
		if len(event.Params) < 2 {
			return nil
		}
	case "MODE":
		if len(event.Params) < 2 || !strings.ContainsAny(event.Params[1], "+-") {
			return nil
		}
	default:
		return nil
	}
	target := event.Params[index]
	if !girc.IsValidChannel(target) {
		return nil
	}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	conv, exists := w.conversations[c.conversationID(target)]
	if !exists || !conv.Joined {
		return ErrDisconnected
	}
	support := w.networks[c.config.ID].ISupport
	if event.Command == "MODE" && len(event.Params) == 2 {
		// Group A modes are lists: without a parameter these are read-only
		// queries, including any server-specific list modes.
		lists, _, _ := strings.Cut(support["CHANMODES"], ",")
		if lists == "" {
			lists = "b"
		}
		modes := strings.ReplaceAll(strings.ReplaceAll(event.Params[1], "+", ""), "-", "")
		if modes != "" && strings.Trim(modes, lists) == "" {
			return nil
		}
	}
	at := func(mode byte) bool { return channelAt(conv, support, w.self.ID, mode) }
	op, halfop := at('o'), at('h') || at('o')
	allowed := halfop
	switch event.Command {
	case "TOPIC":
		allowed = !strings.Contains(conv.Modes, "t") || halfop
	case "INVITE":
		allowed = !strings.Contains(conv.Modes, "i") || halfop
	case "MODE":
		for _, mode := range event.Params[1] {
			if mode != '+' && mode != '-' && !strings.ContainsRune("vbeIq", mode) {
				allowed = op
			}
		}
	}
	if !allowed {
		return fmt.Errorf("insufficient channel privileges for %s", event.Command)
	}
	return nil
}

// messageEvents validates before queuing or persisting a command. In particular,
// girc must never silently split a PRIVMSG into unrelated messages.
func messageEvents(f model.Features, target, text string, tags girc.Tags) ([]*girc.Event, error) {
	budget := f.MessageLineBytes - len("PRIVMSG  :") - len(target)
	if budget < utf8.UTFMax {
		return nil, fmt.Errorf("this network's message line limit is too small")
	}
	if len(text) <= budget && !strings.Contains(text, "\n") {
		return []*girc.Event{{Command: "PRIVMSG", Params: []string{target, text}, Tags: tags}}, nil
	}
	if f.Multiline == nil {
		return nil, fmt.Errorf("%w: this network does not support multiline messages; use one line of at most %d UTF-8 bytes", ErrUnsupported, budget)
	}
	if len(text) > f.Multiline.MaxBytes {
		return nil, fmt.Errorf("message exceeds this network's multiline limit of %d UTF-8 bytes", f.Multiline.MaxBytes)
	}
	batch := id()
	events := []*girc.Event{{Command: "BATCH", Params: []string{"+" + batch, "draft/multiline", target}, Tags: tags}}
	for _, line := range strings.Split(text, "\n") {
		first := true
		for {
			n := min(len(line), budget)
			for n < len(line) && !utf8.RuneStart(line[n]) {
				n--
			}
			t := girc.Tags{"batch": batch}
			if !first {
				t["draft/multiline-concat"] = ""
			}
			events = append(events, &girc.Event{Command: "PRIVMSG", Params: []string{target, line[:n]}, Tags: t})
			if len(events)-1 > f.Multiline.MaxLines {
				return nil, fmt.Errorf("message exceeds this network's multiline limit of %d wire lines", f.Multiline.MaxLines)
			}
			line = line[n:]
			if line == "" {
				break
			}
			first = false
		}
	}
	return append(events, &girc.Event{Command: "BATCH", Params: []string{"-" + batch}}), nil
}
