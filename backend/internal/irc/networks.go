package irc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

func normalizeConfig(cfg NetworkConfig) (NetworkConfig, error) {
	cfg = clone(cfg)
	if err := validateNickname(cfg.Nick, defaultNickLength); err != nil {
		return cfg, err
	}
	if cfg.AltNick != "" {
		if err := validateNickname(cfg.AltNick, defaultNickLength); err != nil {
			return cfg, fmt.Errorf("alternate nickname: %w", err)
		}
	}
	if cfg.Username == "" {
		cfg.Username = "ircord"
	}
	if cfg.Realname == "" {
		cfg.Realname = "IRCord"
	}
	if !clean(cfg.Username, 32) || strings.ContainsAny(cfg.Username, " :@") || !clean(cfg.Realname, 128) || cfg.AltNick != "" && !girc.IsValidNick(cfg.AltNick) {
		return cfg, fmt.Errorf("invalid network identity")
	}
	if len(cfg.AutoJoin) > 100 || len(cfg.Channels) > 100 || len(cfg.Perform) > 20 {
		return cfg, fmt.Errorf("too many auto-join channels or perform lines")
	}
	if cfg.AutoJoin != nil {
		cfg.Channels = append([]string{}, cfg.AutoJoin...)
	}
	for _, entry := range cfg.Channels {
		fields := strings.Fields(entry)
		if len(fields) < 1 || len(fields) > 2 || !girc.IsValidChannel(fields[0]) || !clean(entry, 200) || strings.ContainsAny(fields[0], " ,:") {
			return cfg, fmt.Errorf("invalid auto-join channel")
		}
	}
	for _, line := range cfg.Perform {
		event, err := rawEvent(line)
		if event != nil && (event.Command == "OPER" || event.Command == "PRIVMSG" && len(event.Params) > 0 && strings.EqualFold(event.Params[0], "NickServ")) {
			return cfg, fmt.Errorf("use authentication settings for credentials, rather than perform lines")
		}
		if err != nil {
			return cfg, fmt.Errorf("invalid perform line: %w", err)
		}
	}
	if cfg.Auth != nil {
		cfg.SASLUser = ""
		cfg.SASLPassword = ""
		cfg.ServerPassword = ""
		password := ""
		if cfg.Auth.Password != nil {
			password = *cfg.Auth.Password
		}
		if !clean(password, 512) || !clean(cfg.Auth.Account, 100) {
			return cfg, fmt.Errorf("invalid authentication settings")
		}
		switch cfg.Auth.Method {
		case "none":
			cfg.Auth.Account = ""
			cfg.Auth.Password = nil
		case "sasl-plain":
			if cfg.Auth.Account == "" {
				return cfg, fmt.Errorf("SASL requires an account and password")
			}
			cfg.SASLUser = cfg.Auth.Account
			cfg.SASLPassword = password
		case "server-password":

			cfg.ServerPassword = password
		case "nickserv":

		default:
			return cfg, fmt.Errorf("unsupported authentication method")
		}
	}
	return cfg, nil
}
func applyConfig(n *Network, cfg NetworkConfig) {
	n.AltNick = cfg.AltNick
	n.Username = cfg.Username
	n.Realname = cfg.Realname
	n.AutoJoin = append([]string{}, cfg.Channels...)
	n.Perform = append([]string{}, cfg.Perform...)
	n.Auth = model.NetworkAuth{Method: "none"}
	if cfg.Auth != nil {
		n.Auth.Method = cfg.Auth.Method
		n.Auth.Account = cfg.Auth.Account
		n.Auth.PasswordSet = cfg.Auth.Password != nil && *cfg.Auth.Password != ""
	} else if cfg.SASLUser != "" {
		n.Auth = model.NetworkAuth{Method: "sasl-plain", Account: cfg.SASLUser, PasswordSet: cfg.SASLPassword != ""}
	} else if cfg.ServerPassword != "" {
		n.Auth = model.NetworkAuth{Method: "server-password", PasswordSet: true}
	}
	if n.ISupport == nil {
		n.ISupport = map[string]string{}
	}
	if n.Features.Metadata == nil {
		n.Features.Metadata = []string{}
	}
}
func (m *Manager) UpdateNetwork(owner, network string, cfg NetworkConfig) (Network, error) {
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	return m.updateNetwork(owner, network, cfg)
}

func (m *Manager) updateNetwork(owner, network string, cfg NetworkConfig) (Network, error) {
	w := m.space(owner)
	w.mu.Lock()
	old, ok := w.configs[network]
	active := w.connections[network] != nil
	w.mu.Unlock()
	if !ok {
		return Network{}, ErrNotFound
	}
	cfg = clone(cfg)
	// A retained write-only credential belongs to its original endpoint. In
	// particular an operator-provided default must not be redirected elsewhere.
	if !strings.EqualFold(cfg.Host, old.Host) || cfg.Port != old.Port || cfg.TLS != old.TLS {
		if cfg.Auth != nil && old.Auth != nil && cfg.Auth.Password != nil && old.Auth.Password != nil && *cfg.Auth.Password == *old.Auth.Password {
			cfg.Auth.Password = nil
		}
		if cfg.SASLPassword == old.SASLPassword {
			cfg.SASLPassword = ""
			cfg.SASLUser = ""
		}
		if cfg.ServerPassword == old.ServerPassword {
			cfg.ServerPassword = ""
		}
		old.Auth = nil
		old.SASLPassword = ""
		old.ServerPassword = ""
	}
	cfg.ID = network
	if cfg.Auth != nil && cfg.Auth.Password == nil && old.Auth != nil && old.Auth.Method == cfg.Auth.Method {
		cfg = clone(cfg)
		cfg.Auth.Password = old.Auth.Password
	}
	// Support previously stored legacy configurations without exposing their secrets.
	if cfg.Auth != nil && cfg.Auth.Password == nil {
		switch cfg.Auth.Method {
		case "sasl-plain":
			if old.SASLPassword != "" {
				p := old.SASLPassword
				cfg.Auth.Password = &p
			}
		case "server-password":
			if old.ServerPassword != "" {
				p := old.ServerPassword
				cfg.Auth.Password = &p
			}
		}
	}
	if w.store != nil {
		if err := w.store.InvalidateChannels(owner, network); err != nil {
			return Network{}, ErrStorage
		}
	}
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return Network{}, err
	}
	// Validate all configuration before disrupting the running connection.
	if normalized.Host == "" || !clean(normalized.Host, 253) || strings.ContainsAny(normalized.Host, " /\\") || normalized.Port < 1 || normalized.Port > 65535 || !girc.IsValidNick(normalized.Nick) || !clean(normalized.Name, 100) || (!normalized.TLS && !m.options.AllowPrivate) {
		return Network{}, fmt.Errorf("invalid network configuration")
	}
	missingSecret := normalized.Auth != nil && normalized.Auth.Method != "none" && (normalized.Auth.Password == nil || *normalized.Auth.Password == "")
	if active && missingSecret {
		if err = m.Disconnect(owner, network); err != nil {
			return Network{}, err
		}
		active = false
	}
	if active {
		if err = m.Disconnect(owner, network); err != nil {
			return Network{}, err
		}
		return m.Connect(owner, normalized)
	}
	if w.store != nil {
		if err = w.store.SaveNetwork(owner, normalized, false); err != nil {
			return Network{}, ErrStorage
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	n := w.networks[network]
	n.Name = normalized.Name
	n.Host = normalized.Host
	n.Port = normalized.Port
	n.TLS = normalized.TLS
	n.Nick = normalized.Nick
	applyConfig(&n, normalized)
	w.configs[network] = normalized
	w.networks[network] = n
	w.emit("network.updated", n)
	if w.storageErr != nil {
		return Network{}, ErrStorage
	}
	return clone(n), nil
}
func (m *Manager) DeleteNetwork(owner, network string) error {
	w := m.space(owner)
	w.mu.Lock()
	_, ok := w.networks[network]
	active := w.connections[network] != nil
	w.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	if active {
		if err := m.Disconnect(owner, network); err != nil {
			return err
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emit("network.removed", model.Removal{ID: network})
	if w.storageErr != nil {
		return ErrStorage
	}
	for id, v := range w.invitations {
		if v.NetworkID == network {
			delete(w.invitations, id)
		}
	}
	for id, v := range w.saved {
		if v.NetworkID == network {
			delete(w.saved, id)
		}
	}
	delete(w.networks, network)
	delete(w.configs, network)
	delete(w.info, network)
	delete(w.serverLogs, network)
	delete(w.trace, network)
	for ch := range w.traceSubscribers[network] {
		close(ch)
	}
	delete(w.traceSubscribers, network)
	for key, v := range w.conversations {
		if v.NetworkID == network {
			delete(w.conversations, key)
			delete(w.messages, key)
		}
	}
	for key := range w.members {
		if strings.HasPrefix(key, network+":") {
			delete(w.members, key)
		}
	}
	return nil
}
func rawEvent(line string) (*girc.Event, error) {
	if !clean(line, 510) || strings.TrimSpace(line) == "" || line[0] == ':' || line[0] == '@' {
		return nil, fmt.Errorf("invalid IRC line (maximum 510 UTF-8 bytes)")
	}
	event := girc.ParseEvent(line)
	if event == nil || event.Command == "" {
		return nil, fmt.Errorf("invalid IRC command")
	}
	event.Command = strings.ToUpper(event.Command)
	for _, r := range event.Command {
		if r < 'A' || r > 'Z' {
			return nil, fmt.Errorf("IRC command must contain letters")
		}
	}
	if slices.Contains([]string{"QUIT", "USER", "PASS", "CAP", "AUTHENTICATE", "WEBIRC", "STARTTLS", "BATCH"}, event.Command) {
		return nil, fmt.Errorf("this command is managed by IRCord")
	}
	if event.Command == "NICK" {
		if len(event.Params) != 1 {
			return nil, fmt.Errorf("NICK takes one nickname without spaces")
		}
		if err := validateNickname(event.Params[0], 0); err != nil {
			return nil, err
		}
	}
	return event, nil
}
func (m *Manager) Raw(owner, network, line string) error {
	return m.RawCommand(owner, network, id(), line)
}
func (m *Manager) RawCommand(owner, network, clientID, line string) error {
	if !token(clientID, 100) {
		return fmt.Errorf("invalid clientId")
	}
	event, err := rawEvent(line)
	if err != nil {
		return err
	}
	c, err := m.connection(owner, network)
	if err != nil {
		return err
	}
	if event.Command == "AWAY" {
		m.manualAway(owner, network)
	}
	if event.Command == "NICK" {
		c.owner.mu.Lock()
		maximum := c.nicknameLimit()
		c.owner.mu.Unlock()
		if err := validateNickname(event.Params[0], maximum); err != nil {
			return err
		}
	}
	if event.Command == "SETNAME" && !c.has("setname") || event.Command == "MARKREAD" && !c.hasAll("draft/read-marker", "server-time") || event.Command == "CHATHISTORY" && !c.hasAll("draft/chathistory", "batch", "server-time") {
		return ErrUnsupported
	}
	if err := c.validateChannelCommand(event); err != nil {
		return err
	}
	if event.Command == "PRIVMSG" || event.Command == "NOTICE" {
		c.mu.Lock()
		limit := c.lineBytes
		c.mu.Unlock()
		if limit <= 0 {
			limit = safeMessageLineBytes(girc.DefaultMaxLineLength - girc.DefaultMaxPrefixLength)
		}
		if event.LenOpts(false) > limit {
			return fmt.Errorf("command exceeds this network's line limit; use the message composer")
		}
	}
	if c.has("labeled-response") {
		event.Tags = girc.Tags{"label": "raw:" + clientID}
	}
	return c.enqueue(event)
}

type listRequest struct {
	items    []ChannelListing
	done     chan struct{}
	overflow bool
}

func (c *connection) listItem(e girc.Event) {
	if e.Command != "322" && e.Command != "323" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.listRequest
	if r == nil {
		return
	}
	if e.Command == "322" && len(e.Params) >= 4 {
		if len(r.items) < 100000 {
			n, _ := strconv.Atoi(e.Params[2])
			r.items = append(r.items, ChannelListing{Name: e.Params[1], Users: n, Topic: e.Last()})
		} else {
			r.overflow = true
		}
	}
	if e.Command == "323" {
		select {
		case <-r.done:
		default:
			close(r.done)
		}
	}
}
func (m *Manager) Channels(ctx context.Context, owner, network, q string, limit int) ([]ChannelListing, error) {
	if limit < 1 || limit > 100 || !clean(q, 200) {
		return nil, fmt.Errorf("invalid channel query")
	}
	c, err := m.connection(owner, network)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	select {
	case c.listGate <- struct{}{}:
		defer func() { <-c.listGate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, ErrDisconnected
	}
	c.mu.Lock()
	fresh := time.Since(c.listAt) < 3*time.Minute
	items := clone(c.listCache)
	c.mu.Unlock()
	if c.owner.store != nil {
		cached, ok, err := c.owner.store.CachedChannels(owner, network, q, limit, c.createdAt)
		if err != nil {
			return nil, ErrStorage
		}
		if ok {
			return cached, nil
		}
		fresh = false
	}
	if !fresh {
		select {
		case m.listSlots <- struct{}{}:
			defer func() { <-m.listSlots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.ctx.Done():
			return nil, ErrDisconnected
		}
		requestStarted := time.Now()
		req := &listRequest{done: make(chan struct{}), items: []ChannelListing{}}
		c.mu.Lock()
		c.listRequest = req
		c.mu.Unlock()
		defer func() {
			c.mu.Lock()
			if c.listRequest == req {
				c.listRequest = nil
			}
			c.mu.Unlock()
		}()
		if err = c.enqueue(&girc.Event{Command: "LIST"}); err != nil {
			return nil, err
		}
		select {
		case <-req.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.done:
			return nil, ErrDisconnected
		case <-c.ctx.Done():
			return nil, ErrDisconnected
		}
		c.mu.Lock()
		items = req.items
		overflow := req.overflow
		if !overflow && c.owner.store == nil {
			c.listCache = items
			c.listAt = time.Now()
		}
		c.mu.Unlock()
		if overflow {
			return nil, fmt.Errorf("network channel directory exceeds the 100000-channel limit")
		}
		if c.owner.store != nil {
			if err = c.owner.store.SaveChannels(owner, network, items, requestStarted); err != nil {
				return nil, ErrStorage
			}
			result, _, err := c.owner.store.CachedChannels(owner, network, q, limit, c.createdAt)
			return result, err
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Users == items[j].Users {
			return items[i].Name < items[j].Name
		}
		return items[i].Users > items[j].Users
	})
	result := []ChannelListing{}
	q = strings.ToLower(q)
	for _, v := range items {
		if strings.Contains(strings.ToLower(v.Name+" "+v.Topic), q) {
			result = append(result, v)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
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
		targets = append(targets, v)
	}
	for _, conv := range targets {
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
			data.Text = strings.Join(e.Params[1:], " ")
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

func validAttachment(a model.Attachment) bool {
	u, err := url.Parse(a.URL)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && clean(a.Name, 200) && len(a.URL) < 2048 && a.Width >= 0 && a.Height >= 0 && a.Width <= 16384 && a.Height <= 16384 && a.Size >= 0 && a.Size <= 100<<20
}
func (m *Manager) FilehostConfig(owner, network string) (host, advertised, user, password string, err error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	cfg, ok := w.configs[network]
	if !ok {
		err = ErrNotFound
		return
	}
	host = cfg.Host
	advertised = w.info[network].FilehostURL
	user = cfg.SASLUser
	password = cfg.SASLPassword
	return
}
func HTTPClient(allowPrivate bool) *http.Client {
	return &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (safeDialer{ctx: ctx, allowPrivate: allowPrivate}).Dial(network, address)
	}, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, MaxIdleConns: 100, MaxIdleConnsPerHost: 4}}
}

func (m *Manager) PatchNetwork(owner, network string, fields map[string]json.RawMessage) (Network, error) {
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	w.mu.Lock()
	cfg, ok := w.configs[network]
	cfg = clone(cfg)
	w.mu.Unlock()
	if !ok {
		return Network{}, ErrNotFound
	}
	allowed := []string{"name", "host", "port", "tls", "nick", "altNick", "username", "realname", "auth", "autoJoin", "perform", "channels"}
	if len(fields) == 0 {
		return Network{}, fmt.Errorf("provide a network field to update")
	}
	for key := range fields {
		if !slices.Contains(allowed, key) {
			return Network{}, fmt.Errorf("unknown network field")
		}
	}
	if raw, ok := fields["auth"]; ok {
		var next model.AuthInput
		if string(raw) == "null" || json.Unmarshal(raw, &next) != nil {
			return Network{}, fmt.Errorf("invalid authentication settings")
		}
		if cfg.Auth != nil && cfg.Auth.Method == next.Method && next.Account == "" {
			next.Account = cfg.Auth.Account
		}
		cfg.Auth = &next
		delete(fields, "auth")
	}
	raw, _ := json.Marshal(fields)
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Network{}, err
	}
	return m.updateNetwork(owner, network, cfg)
}

// UpdateHomePassword never installs an account password on a repointed network.
// Hold the configuration lock across validation, persistence and reconnection.
func (m *Manager) UpdateHomePassword(owner string, home NetworkConfig, username, password string) error {
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	w.mu.Lock()
	cfg, ok := w.configs[home.ID]
	cfg = clone(cfg)
	w.mu.Unlock()
	if !ok {
		return nil
	}
	if !strings.EqualFold(cfg.Host, home.Host) || cfg.Port != home.Port || cfg.TLS != home.TLS || cfg.Auth == nil || cfg.Auth.Method != "sasl-plain" || !strings.EqualFold(cfg.Auth.Account, username) {
		return nil
	}
	cfg.Auth.Password = &password
	_, err := m.updateNetwork(owner, home.ID, cfg)
	return err
}

// ProvisionHome persists the promised network before trying connection admission.
// A full connection pool leaves a saved network that the user can reconnect later.
func (m *Manager) ProvisionHome(owner string, cfg NetworkConfig) error {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return err
	}
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	w.mu.Lock()
	if _, exists := w.configs[cfg.ID]; exists {
		w.mu.Unlock()
		return nil
	}
	if w.deleted || w.storageErr != nil {
		w.mu.Unlock()
		return ErrStorage
	}
	if w.store != nil {
		if err := w.store.SaveNetwork(owner, normalized, true); err != nil {
			w.mu.Unlock()
			return ErrStorage
		}
	}
	n := Network{ID: cfg.ID, Name: cfg.Name, Host: cfg.Host, Port: cfg.Port, TLS: cfg.TLS, Nick: cfg.Nick, Initials: "I", Color: "mint", Status: "disconnected", Capabilities: []string{}}
	applyConfig(&n, normalized)
	w.configs[cfg.ID], w.networks[cfg.ID] = normalized, n
	w.emit("network.updated", n)
	err = w.storageErr
	w.mu.Unlock()
	if err != nil {
		return ErrStorage
	}
	_, err = m.Connect(owner, normalized)
	if err == ErrBusy {
		return nil
	}
	return err
}
