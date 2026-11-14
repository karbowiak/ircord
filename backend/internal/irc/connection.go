package irc

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

type connection struct {
	channelRequest         *channelExchangeReply
	serviceGate            chan struct{}
	serviceRequest         *serviceRequest
	serviceBatches         map[string]string
	ergoServices           bool
	readMarkersSeen        map[string]bool
	realnames              map[string]string
	typingSent             map[string]typingSent
	typingReceived         map[string]typingSent
	typingWindow           time.Time
	typingBurst            int
	attractSent            map[string]time.Time
	attractRecent          []time.Time
	attractReceived        map[string]time.Time
	createdAt              time.Time
	modeListGate           chan struct{}
	modeList               *modeListRequest
	transport              net.Conn
	workers                sync.WaitGroup
	manager                *Manager
	owner                  *workspace
	config                 NetworkConfig
	ctx                    context.Context
	cancel                 context.CancelFunc
	mu                     sync.Mutex
	historyCallbackMu      sync.RWMutex
	client                 *girc.Client
	privateRedaction       atomic.Pointer[[]string]
	attemptDone            chan struct{}
	queue                  chan []*girc.Event
	queuedClients          map[*girc.Event]*girc.Client
	automaticHistoryWrites map[*girc.Event]*historyRequest
	searchWrites           map[*girc.Event]*searchRequest
	searchRequest          *searchRequest
	searchBatches          map[string]*searchBatch
	searchQuarantined      bool
	historyScheduler       *historyScheduler
	pending                map[string]pending
	batches                map[string]*multiline
	historyBatches         map[string]bool
	metadataSyncs          map[string]*metadataSync
	metadataRequested      map[string]bool
	metadataRetries        map[string]int
	// metadataSubscribed is set once METADATA SUB was sent on this connection.
	metadataSubscribed bool
	metadataJoined     map[string]bool
	creating           map[string]time.Time
	explicitCreating   map[string]bool
	metadataJoins      map[string]map[string]string
	metadataJoinTimer  *time.Timer
	historyGate        chan struct{}
	historyRequests    map[string]*historyRequest
	historyResponses   map[string]*historyRequest
	rawHistory         *rawHistoryRequest
	rawHistoryWrites   map[*girc.Event]*rawHistoryRequest
	batchLabels        map[string]string
	redactions         map[string]chan string
	unlabeledHistory   *historyRequest
	catching           map[string]bool
	closedTargets      map[string]bool
	desired            map[string]string
	roster             map[string]map[string]bool // channel members seen on this IRC connection; see presence.go
	arrivals           map[string]bool            // channels whose next JOIN of ours is a real arrival
	nicks              map[string]string
	caps               map[string]bool
	capOffers          map[string]string
	capValues          map[string]string
	lineBytes          int
	ready              bool
	permanent          bool
	restored           bool // working or reconnecting when the backend last stopped; see Manager.connect
	everConnected      bool // restored, or registered at least once in this process
	readyAt            time.Time
	registrationError  string
	listGate           chan struct{}
	listRequest        *listRequest
	listCache          []ChannelListing
	listAt             time.Time
	done               chan struct{}
	// profileCheck collects the server's answers about our own published profile while a
	// check is pending, profileCheckGen tells checks apart and profileCheckDone is set once a
	// check has completed or timed out. profilePublished caches what
	// this account last published on this network (kept in account storage); profileSave
	// serializes its updates. See profile_publish.go.
	profileCheck     map[string]string
	profileCheckGen  int
	profileCheckDone bool
	profilePublished map[string]string
	profileSave      sync.Mutex
}

var requestedCaps = []string{"echo-message", "labeled-response", "batch", "server-time", "message-tags", "account-tag", "account-notify", "away-notify", "extended-join", "chghost", "setname", "multi-prefix", "draft/chathistory", "draft/multiline", "draft/metadata-3", "draft/message-redaction", "draft/read-marker", "soju.im/search", "sasl"}

func (c *connection) enqueue(events ...*girc.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enqueueLocked(events...)
}

func (c *connection) enqueueLocked(events ...*girc.Event) error {
	if !c.ready || (c.attemptDone != nil && !c.transportAliveLocked()) {
		return ErrDisconnected
	}
	select {
	case <-c.ctx.Done():
		return ErrDisconnected
	case c.queue <- events:
		c.trackQueuedClients(events, c.client)
		return nil
	default:
		return ErrBusy
	}
}

// The caller holds c.mu. A dequeued batch retains the client it was queued for.
func (c *connection) trackQueuedClients(events []*girc.Event, client *girc.Client) {
	if client == nil {
		return
	}
	if c.queuedClients == nil {
		c.queuedClients = make(map[*girc.Event]*girc.Client)
	}
	for _, event := range events {
		c.queuedClients[event] = client
	}
}

func (c *connection) writer() {
	defer c.manager.wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	maintenance := time.NewTicker(time.Second)
	defer maintenance.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-maintenance.C:
			c.mu.Lock()
			expired := []string{}
			for id, p := range c.pending {
				if time.Since(p.at) > 15*time.Second {
					delete(c.pending, id)
					expired = append(expired, id)
				}
			}
			c.mu.Unlock()
			c.owner.mu.Lock()
			for _, id := range expired {
				c.owner.commandUncertain(id, "IRC did not acknowledge this message. Check the conversation before retrying.")
			}
			c.owner.mu.Unlock()
		case events := <-c.queue:
			// Remove queue tracking immediately, including when cancellation
			// interrupts the writer before its next send tick.
			rawWrites := make([]*rawHistoryRequest, len(events))
			historyWrites := make([]*historyRequest, len(events))
			searchWrites := make([]*searchRequest, len(events))
			queuedClients := make([]*girc.Client, len(events))
			c.mu.Lock()
			for i, event := range events {
				rawWrites[i] = c.takeRawHistoryWrite(event)
				historyWrites[i] = c.takeHistoryWrite(event)
				searchWrites[i] = c.searchWrites[event]
				delete(c.searchWrites, event)
				queuedClients[i] = c.queuedClients[event]
				delete(c.queuedClients, event)
			}
			c.mu.Unlock()
			sent := false
			for i, event := range events {
				select {
				case <-c.ctx.Done():
					return
				case <-ticker.C:
				}
				c.mu.Lock()
				client := c.client
				ready := c.ready && c.transportAliveLocked() && (queuedClients[i] == nil || queuedClients[i] == client)
				if strings.HasPrefix(event.Tags["label"], "chanserv:") {
					ready = ready && c.serviceRequest != nil && c.serviceRequest.label == event.Tags["label"] && c.serviceRequest.client == client
				}
				if strings.HasPrefix(event.Tags["label"], "party:") {
					ready = ready && c.channelRequest != nil && c.channelRequest.label == event.Tags["label"] && c.channelRequest.client == client
				}
				c.mu.Unlock()
				if !ready || client == nil {
					break
				}
				if c.owner.ownerID != partyControlOwner && len(event.Params) > 0 && c.managedPartyTarget(event.Params[0]) && event.Command != "PART" {
					if c.partyTargetAccess(event.Params[0]) != nil {
						c.rejectPartySend(events, sent)
						break
					}
				}
				c.mu.Lock()
				ready = c.ready && c.client == client && c.transportAliveLocked() &&
					c.startRawHistoryWrite(rawWrites[i], client) && c.startHistoryWrite(historyWrites[i], client) && c.startSearchWrite(searchWrites[i], client)
				c.mu.Unlock()
				if !ready {
					break
				}
				if event.Command != "SEARCH" {
					c.capture(*event, true)
				}
				client.Send(event)
				sent = true
				c.recordSentProfile(event)
			}
		}
	}
}

// stableConnection is how long a connection must stay up before the reconnect
// delay starts again from ReconnectMin. Tests shorten it.
var stableConnection = time.Minute

func (c *connection) run() {
	defer c.manager.wg.Done()
	defer close(c.done)
	defer c.drainQueue()
	delay := c.manager.options.ReconnectMin
	// A restored network worked before, so a server that is briefly unreachable
	// is retried rather than treated as a wrong address.
	c.mu.Lock()
	c.everConnected = c.restored
	c.mu.Unlock()
	for c.ctx.Err() == nil {
		c.owner.mu.Lock()
		// Every transport reconnect can miss messages. Freeze the last durable
		// watermark before the new transport can deliver any live events.
		if c.owner.store != nil {
			for _, conv := range c.owner.conversations {
				if conv.NetworkID == c.config.ID {
					if err := c.owner.store.BeginGap(c.owner.ownerID, conv.ID); err != nil {
						c.owner.storageErr = err
						// This ends the connection without emitting, so log here.
						c.owner.logStorageErr()
						c.owner.mu.Unlock()
						return
					}
				}
			}
		}
		attemptCfg := c.owner.configs[c.config.ID]
		c.realnames = map[string]string{}
		c.readMarkersSeen = map[string]bool{}
		c.owner.mu.Unlock()
		c.resetRoster()
		caps := map[string][]string{}
		for _, cap := range requestedCaps {
			if cap != "sasl" {
				caps[cap] = nil
			}
		}
		cfg := girc.Config{Server: c.config.Host, Port: c.config.Port, Nick: attemptCfg.Nick, User: attemptCfg.Username, Name: attemptCfg.Realname, ServerPass: c.config.ServerPassword, SSL: c.config.TLS, TLSConfig: c.manager.options.TLSConfig, DisableSTS: true, AllowFlood: true, SupportedCaps: caps}
		gateway, gatewayOK := c.manager.options.ServerList.gateway(c.config.Host, c.config.Port, c.config.TLS)
		if gatewayOK && c.manager.options.Origin != nil {
			address, err := c.manager.options.Origin(c.owner.ownerID)
			if err == nil && address != "" {
				ip, parseErr := netip.ParseAddr(address)
				if parseErr != nil || ip.Zone() != "" {
					err = fmt.Errorf("invalid stored client address")
				} else {
					address = ip.Unmap().String()
					wireAddress := address
					if strings.HasPrefix(wireAddress, ":") {
						wireAddress = "0" + wireAddress
					}
					cfg.WebIRC = girc.WebIRC{Password: gateway.Password, Gateway: gateway.Gateway, Hostname: wireAddress, Address: wireAddress}
					secrets := []string{gateway.Password, wireAddress, address}
					c.privateRedaction.Store(&secrets)
				}
			}
			if err != nil {
				c.owner.mu.Lock()
				n := c.owner.networks[c.config.ID]
				n.Status, n.StatusDetail = "connecting", "Could not load the client address. Retrying."
				c.owner.networks[n.ID] = n
				c.owner.emit("network.updated", n)
				c.owner.mu.Unlock()
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(delay + time.Duration(rand.Int64N(int64(delay/4)+1))):
				}
				if delay < 30*time.Second {
					delay = min(delay*2, 30*time.Second)
				}
				continue
			}
		}
		// Keep-alive pings double as latency samples (latency.go).
		cfg.PingDelay = pingInterval
		if c.config.SASLUser != "" {
			cfg.SASL = &girc.SASLPlain{User: c.config.SASLUser, Pass: c.config.SASLPassword}
		}
		cfg.HandleNickCollide = func(old string) string {
			c.mu.Lock()
			ready := c.ready
			c.mu.Unlock()
			if ready {
				return ""
			}
			if attemptCfg.AltNick != "" && old == attemptCfg.Nick {
				return attemptCfg.AltNick
			}
			return ""
		}
		if cfg.TLSConfig != nil {
			cfg.TLSConfig = cfg.TLSConfig.Clone()
			if cfg.TLSConfig.ServerName == "" {
				cfg.TLSConfig.ServerName = c.config.Host
			}
		}
		client := girc.New(cfg)
		c.mu.Lock()
		c.client = client
		attemptDone := make(chan struct{})
		c.attemptDone = attemptDone
		c.registrationError = ""
		c.caps = map[string]bool{}
		c.capOffers = map[string]string{}
		c.capValues = map[string]string{}
		c.lineBytes = 0
		c.ergoServices = false
		c.serviceBatches = map[string]string{}
		c.mu.Unlock()
		attemptCtx, cancelAttempt := context.WithCancel(c.ctx)
		client.Handlers.Add(girc.UPDATE_GENERAL, func(client *girc.Client, _ girc.Event) {
			c.callbackFromAttempt(client, func() { c.refreshFeatures(client) })
		})
		client.Handlers.Add(girc.CONNECTED, func(client *girc.Client, _ girc.Event) {
			c.callbackFromAttempt(client, func() { c.connectedAttempt(attemptCtx, client) })
		})
		// ALL_EVENTS runs before girc's command-specific state handlers. Observe
		// its post-update notification instead of reading a one-event-old roster.
		client.Handlers.Add(girc.UPDATE_STATE, func(client *girc.Client, _ girc.Event) {
			c.callbackFromAttempt(client, func() { c.syncMembers(client) })
		})
		client.Handlers.Add(girc.ALL_EVENTS, c.receiveFromAttempt)
		c.connectionState("connecting", client, nil)
		timedOut := false // protected by c.mu, alongside ready
		var attemptTransport net.Conn
		registrationTimer := time.AfterFunc(c.manager.options.ConnectTimeout, func() {
			c.mu.Lock()
			if c.client != client || c.ready || attemptCtx.Err() != nil {
				c.mu.Unlock()
				return
			}
			timedOut = true
			transport := attemptTransport
			if c.transport == transport {
				c.transport = nil
			}
			c.mu.Unlock()
			cancelAttempt()
			if transport != nil {
				_ = transport.Close()
			}
		})
		dialErr := client.DialerConnect(safeDialer{ctx: attemptCtx, allowPrivate: c.manager.options.AllowPrivate, requirePrivate: cfg.WebIRC.Password != "" && !cfg.SSL, connected: func(raw net.Conn) net.Conn {
			conn := &attemptConn{Conn: raw}
			c.mu.Lock()
			attemptTransport = conn
			active := c.client == client && attemptCtx.Err() == nil && !timedOut
			if active {
				c.transport = conn
			}
			c.mu.Unlock()
			if !active {
				_ = conn.Close()
			}
			return conn
		}})
		cancelAttempt()
		registrationTimer.Stop()
		c.mu.Lock()
		if timedOut {
			dialErr = context.DeadlineExceeded
		}
		c.transport = nil
		c.mu.Unlock()
		c.connectionState("disconnected", client, dialErr)
		c.failServices()
		// Drain history callbacks from this attempt before releasing its gate.
		c.historyCallbackMu.Lock()
		c.mu.Lock()
		wasReady := c.ready
		stable := wasReady && time.Since(c.readyAt) >= stableConnection
		c.everConnected = c.everConnected || wasReady
		var serverError *girc.EventError
		if !wasReady && cfg.WebIRC.Password != "" && errors.As(dialErr, &serverError) && serverError.Event != nil && strings.Contains(strings.ToLower(serverError.Event.Last()), "webirc") {
			c.permanent = true
			c.registrationError = c.redactSecrets(serverError.Event.Last())
		}
		if c.ctx.Err() == nil && terminalConnectionFailure(dialErr) {
			c.permanent = true
		}
		c.ready = false
		c.caps = map[string]bool{}
		c.capValues = map[string]string{}
		c.batches = map[string]*multiline{}
		c.historyBatches = map[string]bool{}
		c.searchBatches = nil
		c.searchQuarantined = false
		c.metadataSyncs = map[string]*metadataSync{}
		c.metadataRequested = map[string]bool{}
		c.metadataRetries = map[string]int{}
		c.metadataSubscribed = false
		c.profileCheck = nil
		c.profileCheckDone = false
		c.metadataJoined = map[string]bool{}
		c.metadataJoins = nil
		if c.metadataJoinTimer != nil {
			c.metadataJoinTimer.Stop()
			c.metadataJoinTimer = nil
		}
		permanent := c.permanent
		registrationError := c.registrationError
		pendingCommands := c.pending
		c.pending = map[string]pending{}
		c.mu.Unlock()
		c.failHistory()
		c.historyCallbackMu.Unlock()
		close(attemptDone)
		c.owner.mu.Lock()
		n := c.owner.networks[c.config.ID]
		n.Features = model.Features{Metadata: []string{}}
		n.Capabilities = []string{}
		n.Account = ""
		p := c.owner.networkSelf(n.ID)
		p.Status, p.Account, p.AwayMessage = "offline", "", ""
		n.Profile = &p
		n.Status = "connecting"
		if c.ctx.Err() != nil {
			n.Status = "disconnected"
		}
		if permanent {
			n.Status = "error"
		}
		if dialErr != nil {
			n.StatusDetail = c.redactSecrets(connectionFailure(dialErr))
		}
		if permanent && registrationError != "" {
			n.StatusDetail = registrationError
		}
		c.owner.networks[n.ID] = n
		c.owner.emit("network.updated", n)
		for clientID := range pendingCommands {
			c.owner.commandUncertain(clientID, "IRC disconnected before acknowledging this message.")
		}
		c.owner.mu.Unlock()
		c.drainQueue()
		if permanent {
			return
		}
		if stable {
			// The backoff covers consecutive failures; a connection that stayed up
			// ends them. A server that drops each session at once keeps backing off.
			delay = c.manager.options.ReconnectMin
		}
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(delay + time.Duration(rand.Int64N(int64(delay/4)+1))):
		}
		if delay < 30*time.Second {
			delay = min(delay*2, 30*time.Second)
		}
	}
}

func (c *connection) drainQueue() {
	for {
		select {
		case events := <-c.queue:
			c.mu.Lock()
			for _, event := range events {
				c.takeRawHistoryWrite(event)
				c.takeHistoryWrite(event)
				delete(c.searchWrites, event)
				delete(c.queuedClients, event)
			}
			c.mu.Unlock()
		default:
			return
		}
	}
}

func (c *connection) closeTransport() {
	c.mu.Lock()
	conn := c.transport
	c.transport = nil
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

// Drain all callbacks before the next attempt can mutate shared session state.
func (c *connection) callbackFromAttempt(client *girc.Client, callback func()) {
	c.historyCallbackMu.RLock()
	defer c.historyCallbackMu.RUnlock()
	c.mu.Lock()
	current := c.client == client && c.transport != nil && c.ctx.Err() == nil
	c.mu.Unlock()
	if current {
		callback()
	}
}

func (c *connection) receiveFromAttempt(client *girc.Client, e girc.Event) {
	c.callbackFromAttempt(client, func() { c.receive(client, e) })
}

func (c *connection) connected(client *girc.Client) {
	c.callbackFromAttempt(client, func() { c.connectedAttempt(c.ctx, client) })
}

func (c *connection) connectedAttempt(attemptCtx context.Context, client *girc.Client) {
	c.mu.Lock()
	current := c.client == client && c.transportAliveLocked() && attemptCtx.Err() == nil && client.IsConnected()
	c.mu.Unlock()
	if !current {
		return
	}
	if c.config.SASLUser != "" && !client.HasCapability("sasl") {
		c.mu.Lock()
		if c.client != client || !c.transportAliveLocked() || attemptCtx.Err() != nil || !client.IsConnected() {
			c.mu.Unlock()
			return
		}
		c.permanent = true
		c.registrationError = "The server does not support SASL authentication."
		c.mu.Unlock()
		client.Close()
		return
	}
	caps := []string{}
	c.mu.Lock()
	if c.client != client || !c.transportAliveLocked() || attemptCtx.Err() != nil || !client.IsConnected() {
		c.mu.Unlock()
		return
	}
	for _, cap := range requestedCaps {
		if c.caps[cap] {
			caps = append(caps, cap)
		}
	}
	c.ready = true
	c.readyAt = time.Now()
	desired := orderedTargets(c.desired, c.config.Channels)
	c.mu.Unlock()
	w := c.owner
	w.mu.Lock()
	c.restoreNicks()
	n := w.networks[c.config.ID]
	n.Status = "connected"
	n.Capabilities = caps
	n.Nick = client.GetNick()
	self := w.networkSelf(n.ID)
	self.Nick, self.Status, self.AwayMessage = n.Nick, "online", ""
	resolveMemberName(&self)
	n.Profile = &self
	c.mu.Lock()
	c.lineBytes = safeMessageLineBytes(client.MaxEventLength())
	n.Features = c.features(n.ISupport)
	c.mu.Unlock()
	w.networks[n.ID] = n
	w.emit("network.updated", n)
	c.nicks[c.fold(n.Nick)] = w.self.ID
	w.mu.Unlock()
	c.connectionState("connected", client, nil)
	if c.manager.parties != nil && c.owner.ownerID != partyControlOwner {
		select {
		case c.manager.parties.wake <- struct{}{}:
		default:
		}
	}
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
		if c.owner.ownerID != partyControlOwner {
			fields := strings.Fields(entry)
			if len(fields) > 0 && c.managedPartyTarget(fields[0]) && !c.pendingPartyMember(fields[0]) {
				continue
			}
		}
		setup = append(setup, &girc.Event{Command: "JOIN", Params: strings.Fields(entry)})
	}
	for _, line := range c.config.Perform {
		if event, err := rawEvent(line); err == nil {
			if c.denyPartyCommand(event) != nil {
				continue
			}
			setup = append(setup, event)
		}
	}
	if len(setup) > 0 {
		c.mu.Lock()
		c.trackQueuedClients(setup, client)
		c.mu.Unlock()
		select {
		case c.queue <- setup:
		case <-attemptCtx.Done():
			c.mu.Lock()
			for _, event := range setup {
				delete(c.queuedClients, event)
			}
			c.mu.Unlock()
		}
	}
}

// The caller holds c.mu. Test transports need not be attemptConn instances.
func (c *connection) transportAliveLocked() bool {
	if c.transport == nil {
		return false
	}
	if conn, ok := c.transport.(*attemptConn); ok {
		return !conn.dead.Load()
	}
	return true
}
