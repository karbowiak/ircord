package irc

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

type ChannelRegistration struct {
	Registered   bool       `json:"registered"`
	Founder      string     `json:"founder,omitempty"`
	RegisteredAt *time.Time `json:"registeredAt,omitempty"`
	// Automatic is set when IRCord registered the channel because you created it.
	Automatic bool `json:"automatic,omitempty"`
}
type AccessEntry struct {
	Account string `json:"account"`
	Mode    string `json:"mode" enum:"q,a,o,h,v"`
}
type TransferResult struct {
	Status string `json:"status" enum:"pending,done"`
}

// ServiceError contains a bounded public explanation, never raw service replies or codes.
type ServiceError struct {
	Status int
	Detail string
}

func (e *ServiceError) Error() string              { return e.Detail }
func serviceError(status int, detail string) error { return &ServiceError{status, detail} }

type serviceRequest struct {
	label  string
	client *girc.Client
	lines  []string
	done   chan struct{}
	err    error
}

func (r *serviceRequest) finish(err error) {
	select {
	case <-r.done:
		return
	default:
	}
	r.err = err
	close(r.done)
}

// Managed service exchanges have unique labels, including late replies after a
// timeout. Serialize workflows per connection and bound global concurrent work.
func (m *Manager) withServices(ctx context.Context, owner, conversation string, mutation bool, run func(context.Context, *connection, string) error) error {
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	w.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	if owner != partyControlOwner && (conv.Party != nil || c != nil && c.managedPartyTarget(c.target(conv))) {
		return ErrForbidden
	}
	if conv.Kind != "channel" {
		return serviceError(400, "Only channels have ChanServ registration.")
	}
	if c == nil {
		return ErrDisconnected
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	select {
	case c.serviceGate <- struct{}{}:
		defer func() { <-c.serviceGate }()
	case <-ctx.Done():
		return serviceError(504, "ChanServ is busy. Please try again.")
	case <-c.ctx.Done():
		return ErrDisconnected
	}
	select {
	case m.serviceSlots <- struct{}{}:
		defer func() { <-m.serviceSlots }()
	case <-ctx.Done():
		return serviceError(504, "ChanServ is busy. Please try again.")
	case <-c.ctx.Done():
		return ErrDisconnected
	}
	c.mu.Lock()
	ready, supported := c.ready, c.ergoServices && c.caps["labeled-response"]
	c.mu.Unlock()
	if !ready {
		return ErrDisconnected
	}
	if !supported {
		return serviceError(501, "This network has no ChanServ that IRCord supports.")
	}
	w.mu.Lock()
	account := w.networks[conv.NetworkID].Account
	w.mu.Unlock()
	if mutation && account == "" {
		return serviceError(403, "Log in to an IRC account before managing channel registration.")
	}
	target := conv.Target
	if target == "" {
		target = "#" + conv.Name
	}
	if !girc.IsValidChannel(target) || !clean(target, 100) || strings.ContainsAny(target, " ,:") {
		return serviceError(400, "Invalid channel name.")
	}
	err := run(ctx, c, target)
	if err == nil && mutation {
		w.mu.Lock()
		w.emit("channel-services.updated", model.ChannelServicesChange{ConversationID: conversation})
		if w.storageErr != nil {
			err = ErrStorage
		}
		w.mu.Unlock()
	}
	return err
}

func (c *connection) serviceExchange(ctx context.Context, command string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	c.mu.Lock()
	r := &serviceRequest{label: "chanserv:" + id(), client: c.client, done: make(chan struct{})}
	if expected, ok := ctx.Value(channelClientKey{}).(*girc.Client); ok && (expected != r.client || !c.ready) {
		c.mu.Unlock()
		return nil, ErrDisconnected
	}
	c.serviceRequest = r
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.serviceRequest == r {
			c.serviceRequest = nil
		}
		c.mu.Unlock()
	}()
	e := &girc.Event{Command: "CS", Params: strings.Fields(command), Tags: girc.Tags{"label": r.label}, Sensitive: true}
	if err := c.enqueue(e); err != nil {
		return nil, err
	}
	select {
	case <-r.done:
		c.mu.Lock()
		lines, err := append([]string{}, r.lines...), r.err
		c.mu.Unlock()
		if err != nil {
			return nil, err
		}
		if err = serviceFailure(lines); err != nil {
			return nil, err
		}
		return lines, nil
	case <-ctx.Done():
		return nil, serviceError(504, "ChanServ did not answer in time. Refresh before retrying a change.")
	case <-c.ctx.Done():
		return nil, ErrDisconnected
	}
}

var serviceColor = regexp.MustCompile(`\x03[0-9]{0,2}(?:,[0-9]{1,2})?`)

func cleanServiceText(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, serviceColor.ReplaceAllString(text, ""))
}

// Observe before diagnostics: managed confirmations and responses never enter
// server logs, conversation history, raw traces, or WebSocket events.
func (c *connection) observeServices(client *girc.Client, e girc.Event) bool {
	c.mu.Lock()
	if c.client != client {
		c.mu.Unlock()
		return true
	}
	if e.Command == "004" && len(e.Params) >= 3 && e.Source != nil && e.Source.Ident == "" {
		c.ergoServices = strings.HasPrefix(strings.ToLower(e.Params[2]), "ergo-")
	}
	c.mu.Unlock()
	account, changed := "", false
	if (e.Command == "900" || e.Command == "901") && len(e.Params) >= 2 && (e.Source == nil || e.Source.Ident == "") && (strings.EqualFold(e.Params[0], client.GetNick()) || e.Params[0] == "*") {
		changed = true
		if e.Command == "900" && len(e.Params) >= 3 {
			account = e.Params[2]
		}
	}
	if e.Command == "ACCOUNT" && e.Source != nil && strings.EqualFold(e.Source.Name, client.GetNick()) && len(e.Params) == 1 {
		changed = true
		account = e.Params[0]
		if account == "*" {
			account = ""
		}
	}
	if changed && clean(account, 100) {
		w := c.owner
		w.mu.Lock()
		n := w.networks[c.config.ID]
		if n.Account != account {
			n.Account = account
			p := w.networkSelf(n.ID)
			p.Account = account
			n.Profile = &p
			w.networks[n.ID] = n
			w.emit("network.updated", n)
		}
		w.mu.Unlock()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	label := e.Tags["label"]
	if e.Command == "BATCH" && len(e.Params) > 0 {
		batch := e.Params[0]
		if strings.HasPrefix(batch, "+") && len(e.Params) > 1 && e.Params[1] == "labeled-response" && strings.HasPrefix(label, "chanserv:") {
			if c.serviceBatches == nil {
				c.serviceBatches = map[string]string{}
			}
			if len(c.serviceBatches) >= 128 {
				if r := c.serviceRequest; r != nil {
					r.finish(serviceError(502, "ChanServ returned too many incomplete responses. Reconnect the network."))
				}
				// Stop receiving before diagnostics can observe an untrackable confirmation.
				c.cancel()
				client.Close()
				return true
			}
			c.serviceBatches[batch[1:]] = label
			return true
		}
		if strings.HasPrefix(batch, "-") {
			if prior, ok := c.serviceBatches[batch[1:]]; ok {
				delete(c.serviceBatches, batch[1:])
				if r := c.serviceRequest; r != nil && r.label == prior && r.client == client {
					r.finish(nil)
				}
				return true
			}
		}
	}
	batch := e.Tags["batch"]
	if batch != "" {
		if prior, ok := c.serviceBatches[batch]; ok {
			label = prior
		}
	}
	if !strings.HasPrefix(label, "chanserv:") {
		return false
	}
	r := c.serviceRequest
	if e.Command == "NOTICE" && e.Source != nil && strings.EqualFold(e.Source.Name, "ChanServ") {
		if r != nil && r.label == label && r.client == client {
			if len(r.lines) >= 1024 || len(e.Last()) > 4096 {
				r.finish(serviceError(502, "ChanServ returned too much data."))
			} else {
				r.lines = append(r.lines, cleanServiceText(e.Last()))
				if batch == "" {
					r.finish(nil)
				}
			}
		}
		return true
	}
	if e.Command == "401" || e.Command == "421" || e.Command == "FAIL" || e.Command == "ACK" {
		if r != nil && r.label == label && r.client == client {
			r.finish(serviceError(501, "This network has no ChanServ that IRCord supports."))
		}
		return true
	}
	return false
}
func (c *connection) failServices() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if r := c.serviceRequest; r != nil {
		r.finish(ErrDisconnected)
	}
}
func serviceFailure(lines []string) error {
	for _, line := range lines {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "not logged into an account"):
			return serviceError(403, "Log in to an IRC account before managing this channel.")
		case strings.Contains(lower, "insufficient privileges") || strings.Contains(lower, "only the channel founder") || strings.Contains(lower, "must be an oper") || strings.Contains(lower, "don't own") || strings.Contains(lower, "restricted to server operators"):
			return serviceError(403, "Insufficient privileges to manage this channel.")
		// Before "already registered": the limit reply says "You have already registered the
		// maximum number of channels".
		case strings.Contains(lower, "maximum number of channels"):
			return serviceError(409, "You have reached this network's channel registration limit.")
		case strings.Contains(lower, "already registered"):
			return serviceError(409, "This channel is already registered.")
		case strings.Contains(lower, "channel is not registered"):
			return serviceError(404, "This channel is not registered.")
		case strings.Contains(lower, "account does not exist"):
			return serviceError(422, "Account does not exist.")
		case strings.Contains(lower, "no such channel") || lower == "channel does not exist":
			return serviceError(404, "This channel no longer exists.")
		case strings.Contains(lower, "disabled") || strings.Contains(lower, "unknown command"):
			return serviceError(501, "Channel registration is not enabled on this network.")
		}
	}
	return nil
}
func unexpectedServiceReply() error {
	return serviceError(502, "ChanServ returned an unexpected reply. Refresh or use /msg ChanServ help.")
}
func serviceAccount(account string) bool {
	return account != "" && clean(account, 64) && !strings.ContainsAny(account, " \t,:#") && !strings.ContainsFunc(account, unicode.IsControl)
}

func (c *connection) registrationInfo(ctx context.Context, target string) (ChannelRegistration, error) {
	lines, err := c.serviceExchange(ctx, "INFO "+target)
	if err != nil {
		return ChannelRegistration{}, err
	}
	out := ChannelRegistration{}
	for _, line := range lines {
		if strings.EqualFold(line, "Channel "+target+" is not registered") {
			return out, nil
		}
		if strings.EqualFold(line, "Channel "+target+" is registered") {
			out.Registered = true
		}
		if value, ok := strings.CutPrefix(line, "Founder: "); ok {
			out.Founder = value
		}
		if value, ok := strings.CutPrefix(line, "Registered at: "); ok {
			at, err := time.Parse(time.RFC1123, value)
			if err == nil {
				at = at.UTC()
				out.RegisteredAt = &at
			}
		}
	}
	if !out.Registered || out.Founder == "" || out.RegisteredAt == nil {
		return out, unexpectedServiceReply()
	}
	return out, nil
}
func (m *Manager) ChannelRegistration(ctx context.Context, owner, conversation string) (ChannelRegistration, error) {
	var out ChannelRegistration
	err := m.withServices(ctx, owner, conversation, false, func(ctx context.Context, c *connection, target string) error {
		var err error
		out, err = c.registrationInfo(ctx, target)
		return err
	})
	if err == nil && out.Registered {
		out.Automatic = m.autoRegistered(owner, conversation, out.Founder)
	}
	return out, err
}

// autoRegistered reports whether IRCord registered the channel for you and you still own it.
func (m *Manager) autoRegistered(owner, conversation, founder string) bool {
	w := m.space(owner)
	w.mu.Lock()
	account := w.networks[w.conversations[conversation].NetworkID].Account
	w.mu.Unlock()
	if w.store == nil || account == "" || !strings.EqualFold(account, founder) {
		return false
	}
	found, _ := w.store.AutoRegistered(owner, conversation)
	return found
}
func (m *Manager) RegisterChannel(ctx context.Context, owner, conversation string) (ChannelRegistration, error) {
	var out ChannelRegistration
	err := m.withServices(ctx, owner, conversation, true, func(ctx context.Context, c *connection, target string) error {
		lines, err := c.serviceExchange(ctx, "REGISTER "+target)
		if err != nil {
			return err
		}
		if len(lines) != 1 || !strings.EqualFold(lines[0], "Channel "+target+" successfully registered") {
			return unexpectedServiceReply()
		}
		out, err = c.registrationInfo(ctx, target)
		return err
	})
	return out, err
}
func (c *connection) channelAccess(ctx context.Context, target string) ([]AccessEntry, error) {
	lines, err := c.serviceExchange(ctx, "AMODE "+target)
	if err != nil {
		return nil, err
	}
	out := []AccessEntry{}
	expected := -1
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "channel is not registered") {
			return nil, serviceError(404, "This channel is not registered.")
		}
		fields := strings.Fields(line)
		if len(fields) == 7 && fields[0] == "Channel" && strings.EqualFold(fields[1], target) && strings.Join(fields[4:], " ") == "persistent modes set" {
			expected, _ = strconv.Atoi(fields[3])
		}
		if len(fields) == 5 && fields[0] == "Account" && fields[2] == "receives" && fields[3] == "mode" && len(fields[4]) == 2 && fields[4][0] == '+' && strings.ContainsRune("qaohv", rune(fields[4][1])) {
			out = append(out, AccessEntry{fields[1], fields[4][1:]})
		}
	}
	if expected != len(out) {
		return nil, unexpectedServiceReply()
	}
	return out, nil
}
func (m *Manager) ChannelAccess(ctx context.Context, owner, conversation string) ([]AccessEntry, error) {
	var out []AccessEntry
	err := m.withServices(ctx, owner, conversation, false, func(ctx context.Context, c *connection, target string) error {
		var err error
		out, err = c.channelAccess(ctx, target)
		return err
	})
	return out, err
}
func (m *Manager) SetChannelAccess(ctx context.Context, owner, conversation, account, mode string, remove bool) ([]AccessEntry, error) {
	if !serviceAccount(account) || (!remove && (len(mode) != 1 || !strings.Contains("aohv", mode))) {
		return nil, serviceError(422, "Enter a valid account and channel role.")
	}
	var out []AccessEntry
	err := m.withServices(ctx, owner, conversation, true, func(ctx context.Context, c *connection, target string) error {
		info, err := c.registrationInfo(ctx, target)
		if err != nil {
			return err
		}
		if !info.Registered {
			return serviceError(404, "This channel is not registered.")
		}
		if strings.EqualFold(info.Founder, account) {
			return serviceError(403, "Use ownership transfer to change the founder.")
		}
		sign := "+"
		if remove {
			sign = "-"
			mode = ""
			entries, err := c.channelAccess(ctx, target)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if strings.EqualFold(entry.Account, account) {
					mode = entry.Mode
				}
			}
			if mode == "" {
				out = entries
				return nil
			}
		}
		lines, err := c.serviceExchange(ctx, "AMODE "+target+" "+sign+mode+" "+account)
		if err != nil {
			return err
		}
		if len(lines) != 1 || !(strings.HasPrefix(lines[0], "Successfully set persistent mode ") || lines[0] == "No changes were made") {
			return unexpectedServiceReply()
		}
		out, err = c.channelAccess(ctx, target)
		return err
	})
	return out, err
}
func confirmation(lines []string, command, target, account string) (string, error) {
	prefix := "/CS " + command + " " + target + " "
	if account != "" {
		prefix += account + " "
	}
	for _, line := range lines {
		i := strings.Index(strings.ToLower(line), strings.ToLower(prefix))
		if i < 0 {
			continue
		}
		code := line[i+len(prefix):]
		if len(code) > 0 && len(code) <= 64 && !strings.ContainsAny(code, " \t\r\n") {
			for _, r := range code {
				if !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '=') {
					return "", unexpectedServiceReply()
				}
			}
			return code, nil
		}
	}
	return "", unexpectedServiceReply()
}
func (m *Manager) UnregisterChannel(ctx context.Context, owner, conversation string) error {
	return m.withServices(ctx, owner, conversation, true, func(ctx context.Context, c *connection, target string) error {
		lines, err := c.serviceExchange(ctx, "UNREGISTER "+target)
		if err != nil {
			return err
		}
		code, err := confirmation(lines, "UNREGISTER", target, "")
		if err != nil {
			return err
		}
		lines, err = c.serviceExchange(ctx, "UNREGISTER "+target+" "+code)
		if err != nil {
			return err
		}
		if len(lines) != 1 || !strings.EqualFold(lines[0], "Channel "+target+" is now unregistered") {
			return unexpectedServiceReply()
		}
		if store := m.space(owner).store; store != nil {
			_ = store.SetAutoRegistered(owner, conversation, false)
		}
		return nil
	})
}
func (m *Manager) TransferChannel(ctx context.Context, owner, conversation, account string) (TransferResult, error) {
	if !serviceAccount(account) {
		return TransferResult{}, serviceError(422, "Enter a valid account name.")
	}
	out := TransferResult{}
	err := m.withServices(ctx, owner, conversation, true, func(ctx context.Context, c *connection, target string) error {
		lines, err := c.serviceExchange(ctx, "TRANSFER "+target+" "+account)
		if err != nil {
			return err
		}
		if code, e := confirmation(lines, "TRANSFER", target, account); e == nil {
			lines, err = c.serviceExchange(ctx, "TRANSFER "+target+" "+account+" "+code)
			if err != nil {
				return err
			}
		}
		for _, line := range lines {
			if strings.HasPrefix(line, "Transfer of channel ") && strings.HasSuffix(line, "succeeded, pending acceptance") {
				out.Status = "pending"
				return nil
			}
			if strings.HasPrefix(line, "Successfully transferred channel ") || strings.HasPrefix(line, "Cancelled pending transfer of channel ") {
				out.Status = "done"
				return nil
			}
		}
		return unexpectedServiceReply()
	})
	return out, err
}
