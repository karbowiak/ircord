package irc

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lrstanley/girc"
	ergoapi "ircord/backend/internal/ergo"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

type partyControlIdentity struct {
	Username, Password string
	Registered         bool
}
type partyIRCController struct {
	p          *partyManager
	mu         sync.Mutex
	client     *girc.Client
	generation string
	verified   map[string]*girc.Client
}
type partyIRCReply struct {
	client          *girc.Client
	label, terminal string
	batches         map[string]bool
	events          []girc.Event
	done            chan struct{}
	err             error
}

func (c *connection) partyReply(client *girc.Client, e girc.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.partyRequest
	if r == nil {
		return
	}
	if r.client != nil && (client != r.client || c.client != r.client) {
		r.err = ErrDisconnected
		select {
		case <-r.done:
		default:
			close(r.done)
		}
		return
	}
	label := e.Tags["label"]
	batch := e.Tags["batch"]
	if e.Command == "BATCH" && len(e.Params) > 0 {
		ref := strings.TrimLeft(e.Params[0], "+-")
		if e.Params[0][0] == '+' && (label == r.label || r.batches[batch]) {
			r.batches[ref] = true
			return
		}
		if e.Params[0][0] == '-' && r.batches[ref] {
			delete(r.batches, ref)
			if len(r.batches) == 0 {
				select {
				case <-r.done:
				default:
					close(r.done)
				}
			}
			return
		}
	}
	if label != r.label && !r.batches[batch] {
		return
	}
	if len(r.events) >= 256 {
		r.err = ErrBusy
	} else {
		r.events = append(r.events, *e.Copy())
	}
	if e.Command == "FAIL" || len(e.Command) == 3 && e.Command[0] >= '4' && e.Command[0] <= '5' {
		r.err = serviceError(503, "The IRC server rejected the channel change. Check your permissions and server limits.")
	}
	if batch == "" && (r.terminal == "" || e.Command == r.terminal || e.Command == "ACK" || r.err != nil) {
		select {
		case <-r.done:
		default:
			close(r.done)
		}
	}
}
func (c *connection) partyExchange(ctx context.Context, event *girc.Event, terminal string) ([]girc.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	r := &partyIRCReply{label: "party:" + id(), terminal: terminal, batches: map[string]bool{}, done: make(chan struct{})}
	c.mu.Lock()
	r.client = c.client
	if expected, ok := ctx.Value(channelClientKey{}).(*girc.Client); ok && (expected != r.client || !c.ready) {
		c.mu.Unlock()
		return nil, ErrDisconnected
	}
	if c.partyRequest != nil {
		c.mu.Unlock()
		return nil, ErrBusy
	}
	c.partyRequest = r
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.partyRequest == r {
			c.partyRequest = nil
		}
		c.mu.Unlock()
	}()
	event.Tags = girc.Tags{"label": r.label}
	event.Sensitive = true
	if e := c.enqueue(event); e != nil {
		return nil, e
	}
	select {
	case <-ctx.Done():
		return nil, serviceError(504, "The IRC server did not confirm the change. Refresh the channel before retrying.")
	case <-c.ctx.Done():
		return nil, ErrDisconnected
	case <-r.done:
		c.mu.Lock()
		defer c.mu.Unlock()
		return r.events, r.err
	}
}
func (ctl *partyIRCController) connection(ctx context.Context) (*connection, error) {
	p, m := ctl.p, ctl.p.m
	raw, e := p.store.PartyControl()
	var identity partyControlIdentity
	if errors.Is(e, storage.ErrNotFound) {
		identity = partyControlIdentity{Username: "ircordsvc" + id()[:16], Password: id() + id()}
		raw, _ = json.Marshal(identity)
		if e = p.store.SavePartyControl(raw); e != nil {
			return nil, ErrStorage
		}
	} else if e != nil || json.Unmarshal(raw, &identity) != nil {
		return nil, ErrStorage
	}
	if !identity.Registered {
		e = m.options.PartyAccounts.Register(ctx, identity.Username, identity.Password)
		if errors.Is(e, ergoapi.ErrAccountExists) {
			var matches bool
			matches, e = m.options.PartyAccounts.CheckPassword(ctx, identity.Username, identity.Password)
			if e == nil && !matches {
				return nil, serviceError(503, "The party control identity could not be recovered.")
			}
		}
		if e != nil {
			return nil, serviceError(503, "The party control account could not be provisioned.")
		}
		identity.Registered = true
		raw, _ = json.Marshal(identity)
		if e = p.store.SavePartyControl(raw); e != nil {
			return nil, ErrStorage
		}
	}
	c, e := m.connection(partyControlOwner, m.options.Home.ID)
	if e != nil {
		cfg := m.options.Home
		cfg.Nick = identity.Username
		cfg.Auth = &model.AuthInput{Method: "sasl-plain", Account: identity.Username, Password: &identity.Password}
		cfg.Channels = nil
		cfg.AutoJoin = nil
		cfg.Perform = nil
		if _, e = m.Connect(partyControlOwner, cfg); e != nil {
			return nil, e
		}
		c, e = m.connection(partyControlOwner, cfg.ID)
		if e != nil {
			return nil, e
		}
	}
	if e = m.WaitConnected(ctx, partyControlOwner, m.options.Home.ID); e != nil {
		return nil, serviceError(503, "The party control connection is unavailable.")
	}
	p.mu.Lock()
	p.control = c
	p.mu.Unlock()
	if !c.hasAll("sasl", "labeled-response", "account-tag", "extended-join", "account-notify", "draft/chathistory", "batch", "server-time") {
		return nil, serviceError(501, "The IRC server lacks capabilities required for private parties.")
	}
	w := c.owner
	w.mu.Lock()
	network := w.networks[c.config.ID]
	w.mu.Unlock()
	maximum, e := strconv.Atoi(network.ISupport["CHANNELLEN"])
	if e != nil || maximum < 40 {
		return nil, serviceError(501, "The IRC server channel name limit is too small for private parties.")
	}
	if !strings.EqualFold(network.Account, identity.Username) {
		return nil, serviceError(503, "The party control identity is not authenticated.")
	}
	if _, e := m.ResolvePartyCandidate(ctx, identity.Username, ""); e != nil {
		return nil, e
	}
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if ctl.client != client {
		ctl.client = client
		ctl.generation = id()
	}
	return c, nil
}

// Reconcile is serialized for the single controller connection. Members are
// never channel operators; only this backend account owns the registration.
func (ctl *partyIRCController) Reconcile(ctx context.Context, v storage.PartyRecord) error {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	c, e := ctl.connection(ctx)
	if e != nil {
		return e
	}
	c.mu.Lock()
	auditedClient := c.client
	c.mu.Unlock()
	m := ctl.p.m
	if _, e = c.partyExchange(ctx, &girc.Event{Command: "JOIN", Params: []string{v.Channel}}, "366"); e != nil {
		return e
	}
	c.owner.mu.Lock()
	conv := c.conversation(v.Channel)
	c.owner.mu.Unlock()
	// Protect even failed provisioning attempts before registration can fail
	// on a server limit. No ordinary member has been admitted at this point.
	limit := len(v.Members) + 1
	if channel := auditedClient.LookupChannel(v.Channel); channel != nil {
		limit += len(channel.UserList)
	}
	if _, e = c.partyExchange(ctx, &girc.Event{Command: "MODE", Params: []string{v.Channel, "+isntRl", strconv.Itoa(limit)}}, ""); e != nil {
		return e
	}
	info, e := m.ChannelRegistration(ctx, partyControlOwner, conv.ID)
	if e != nil {
		return e
	}
	if !info.Registered {
		if v.State != "provisioning" {
			return serviceError(503, "The party channel registration is missing. Restore the associated server before reconnecting.")
		}
		if _, e = m.RegisterChannel(ctx, partyControlOwner, conv.ID); e != nil {
			return e
		}
		info, e = m.ChannelRegistration(ctx, partyControlOwner, conv.ID)
		if e != nil {
			return e
		}
	}
	if !info.Registered || !strings.EqualFold(info.Founder, c.config.SASLUser) {
		return serviceError(503, "The party channel is not owned by its control account.")
	}
	limit = len(v.Members) + 1
	if channel := auditedClient.LookupChannel(v.Channel); channel != nil {
		limit += len(channel.UserList)
	}
	if _, e = c.partyExchange(ctx, &girc.Event{Command: "MODE", Params: []string{v.Channel, "+isntRl", strconv.Itoa(limit)}}, ""); e != nil {
		return e
	}
	replies, e := c.partyExchange(ctx, &girc.Event{Command: "MODE", Params: []string{v.Channel}}, "324")
	if e != nil {
		return e
	}
	protected := false
	for _, r := range replies {
		if r.Command == "324" && len(r.Params) >= 4 {
			modes := strings.TrimPrefix(r.Params[2], "+")
			protected = true
			for _, mode := range "isntRl" {
				protected = protected && strings.ContainsRune(modes, mode)
			}
			protected = protected && r.Params[len(r.Params)-1] == strconv.Itoa(limit)
			protected = protected && !strings.ContainsAny(modes, "kfmUu")
		}
	}
	if !protected {
		return serviceError(503, "The party channel protection could not be confirmed.")
	}
	for _, mode := range []string{"b", "e", "I"} {
		entries, e := m.ModeList(ctx, partyControlOwner, conv.ID, mode)
		if e != nil {
			return e
		}
		for _, entry := range entries {
			if _, e = c.partyExchange(ctx, &girc.Event{Command: "MODE", Params: []string{v.Channel, "-" + mode, entry.Mask}}, ""); e != nil {
				return e
			}
		}
		entries, e = m.ModeList(ctx, partyControlOwner, conv.ID, mode)
		if e != nil || len(entries) != 0 {
			return serviceError(503, "The party admission lists could not be confirmed.")
		}
	}
	// Check online accounts through the privileged WHOIS API, never nickname
	// masks. A nick change does not change durable admission.
	if e = ctl.occupants(ctx, c, v); e != nil {
		return e
	}
	entries, e := m.ChannelAccess(ctx, partyControlOwner, conv.ID)
	if e != nil {
		return e
	}
	wanted := map[string]bool{}
	for _, member := range v.Members {
		wanted[strings.ToLower(member.Username)] = true
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Account, c.config.SASLUser) {
			continue
		}
		if entry.Mode != "v" || !wanted[strings.ToLower(entry.Account)] {
			if _, e = m.SetChannelAccess(ctx, partyControlOwner, conv.ID, entry.Account, "", true); e != nil {
				return e
			}
		}
	}
	for _, member := range v.Members {
		if _, e = m.SetChannelAccess(ctx, partyControlOwner, conv.ID, member.Username, "v", false); e != nil {
			return e
		}
	}
	entries, e = m.ChannelAccess(ctx, partyControlOwner, conv.ID)
	if e != nil {
		return e
	}
	confirmed := map[string]bool{}
	for _, entry := range entries {
		if strings.EqualFold(entry.Account, c.config.SASLUser) {
			continue
		}
		if entry.Mode != "v" || !wanted[strings.ToLower(entry.Account)] {
			return serviceError(503, "The party account permissions could not be confirmed.")
		}
		confirmed[strings.ToLower(entry.Account)] = true
	}
	if len(confirmed) != len(wanted) {
		return serviceError(503, "The party account permissions could not be confirmed.")
	}
	if e = ctl.occupants(ctx, c, v); e != nil {
		return e
	}
	// The controller is the founder and can read the full retained history;
	// account-visible copies do not depend on user join-time query cutoffs.
	if _, e = c.serviceExchange(ctx, "SET "+v.Channel+" HISTORY on"); e != nil {
		return e
	}
	history, e := c.serviceExchange(ctx, "GET "+v.Channel+" HISTORY")
	if e != nil {
		return e
	}
	if !slices.Contains(history, "Given current server settings, the channel history setting is: persistent") {
		return serviceError(501, "The IRC server must support retained channel history for private parties.")
	}
	// Recover missed retained history before opening access. Full canonical
	// history is bounded below; no user connection requests private backfill.
	latest, e := ctl.p.store.PartyLatest(v.ID)
	if e != nil {
		return e
	}
	point := time.Now().Add(-storage.AccountRetention)
	if !latest.CreatedAt.IsZero() {
		point = latest.CreatedAt.Add(-time.Millisecond)
	}
	for page := 0; page < 500; page++ {
		r, e := c.syncHistory(ctx, v.Channel, "AFTER", "timestamp="+point.UTC().Format(ircTimeLayout), 100)
		if e != nil {
			return e
		}
		if r.Count < 100 || r.Newest.At.IsZero() || !r.Newest.At.After(point) {
			break
		}
		point = r.Newest.At
		if page == 499 {
			return serviceError(503, "The party history is still synchronizing.")
		}
	}
	if e = ctl.auditHistory(ctx, c, v); e != nil {
		return e
	}
	c.mu.Lock()
	same := c.ready && c.client == auditedClient
	c.mu.Unlock()
	if !same {
		return ErrDisconnected
	}
	c.owner.mu.Lock()
	storageOK := c.owner.storageErr == nil
	c.owner.mu.Unlock()
	if !storageOK {
		return ErrStorage
	}
	if ctl.verified == nil {
		ctl.verified = map[string]*girc.Client{}
	}
	ctl.verified[v.ID] = auditedClient
	return nil
}

func (ctl *partyIRCController) protectedClient(party string) *girc.Client {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	return ctl.verified[party]
}

// Audit all retained canonical windows after each controller reconnect.
// Ergo removes redactions from history; comparing complete time windows also
// removes stale messages and edits which were deleted while disconnected.
// Progress survives a request timeout, and access stays closed until complete.
func (ctl *partyIRCController) auditHistory(ctx context.Context, c *connection, v storage.PartyRecord) error {
	upper, done, e := ctl.p.store.PartyAudit(v.ID, ctl.generation)
	if e != nil {
		return ErrStorage
	}
	if done {
		return nil
	}
	// Server expiry is not a deletion. Audit only windows actually returned by
	// Ergo; its oldest visible row does not prove anything about older rows.
	cutoff := time.Now().Add(-storage.AccountRetention)
	for page := 0; page < 500; page++ {
		r, e := c.syncHistory(ctx, v.Channel, "BEFORE", "timestamp="+upper.UTC().Format(ircTimeLayout), 100)
		if e != nil {
			return e
		}
		lower := r.Oldest.At
		done = r.Count < 100 || lower.IsZero() || !lower.After(cutoff)
		if lower.IsZero() {
			return ctl.p.store.SavePartyAudit(v.ID, ctl.generation, upper, true)
		}
		if lower.Before(cutoff) {
			lower = cutoff
		}
		if !lower.Before(upper) {
			return serviceError(503, "The party history is still synchronizing.")
		}
		stored, e := ctl.p.store.PartyWindow(v.ID, lower, upper)
		if e != nil {
			return ErrStorage
		}
		window := r
		window.Oldest.At, window.Newest.At = lower, upper
		c.owner.mu.Lock()
		conv := c.conversation(v.Channel)
		for _, msg := range stored {
			if msg.Deleted {
				continue
			}
			if msg.CreatedAt.After(lower) && msg.CreatedAt.Before(upper) && !r.IDs[msg.UpstreamID] {
				c.applyRedaction(conv, msg, false)
			} else if edits := keptEdits(msg.Edits, window); len(edits) != len(msg.Edits) {
				msg.Edits = edits
				c.saveEdited(conv, msg, false)
			}
		}
		c.owner.mu.Unlock()
		if e = ctl.p.store.SavePartyAudit(v.ID, ctl.generation, lower, done); e != nil {
			return ErrStorage
		}
		if done {
			return nil
		}
		upper = lower
	}
	return serviceError(503, "The party history is still synchronizing. The server will continue in the background.")
}
func (ctl *partyIRCController) occupants(ctx context.Context, c *connection, v storage.PartyRecord) error {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client == nil {
		return ErrDisconnected
	}
	for attempt := 0; attempt < 3; attempt++ {
		if _, e := c.partyExchange(ctx, &girc.Event{Command: "NAMES", Params: []string{v.Channel}}, "366"); e != nil {
			return e
		}
		channel := client.LookupChannel(v.Channel)
		if channel == nil {
			return ErrDisconnected
		}
		if len(channel.UserList) > 4096 {
			return serviceError(503, "The party channel contains unexpected occupants.")
		}
		kicked := false
		for _, nick := range channel.UserList {
			user, e := ctl.p.m.options.PartyAccounts.User(ctx, nick)
			if errors.Is(e, ergoapi.ErrNotFound) {
				continue
			}
			if e != nil {
				return serviceError(503, "Party account admission could not be verified.")
			}
			allowed := strings.EqualFold(user.Account, c.config.SASLUser)
			for _, member := range v.Members {
				allowed = allowed || strings.EqualFold(member.Username, user.Account)
			}
			if !allowed {
				if _, e = c.partyExchange(ctx, &girc.Event{Command: "KICK", Params: []string{v.Channel, nick, "Party membership required"}}, ""); e != nil {
					return e
				}
				kicked = true
			}
		}
		if !kicked {
			return nil
		}
	}
	return serviceError(503, "The party channel contains unexpected occupants.")
}
