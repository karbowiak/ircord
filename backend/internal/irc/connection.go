package irc

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

type connection struct {
	partyRequest      *partyIRCReply
	serviceGate       chan struct{}
	serviceRequest    *serviceRequest
	serviceBatches    map[string]string
	ergoServices      bool
	readMarkersSeen   map[string]bool
	realnames         map[string]string
	typingSent        map[string]typingSent
	typingReceived    map[string]typingSent
	typingWindow      time.Time
	typingBurst       int
	attractSent       map[string]time.Time
	attractRecent     []time.Time
	attractReceived   map[string]time.Time
	createdAt         time.Time
	modeListGate      chan struct{}
	modeList          *modeListRequest
	transport         net.Conn
	workers           sync.WaitGroup
	manager           *Manager
	owner             *workspace
	config            NetworkConfig
	ctx               context.Context
	cancel            context.CancelFunc
	mu                sync.Mutex
	client            *girc.Client
	queue             chan []*girc.Event
	pending           map[string]pending
	batches           map[string]*multiline
	historyBatches    map[string]bool
	metadataSyncs     map[string]*metadataSync
	metadataRequested map[string]bool
	metadataRetries   map[string]int
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

var requestedCaps = []string{"echo-message", "labeled-response", "batch", "server-time", "message-tags", "account-tag", "account-notify", "away-notify", "extended-join", "chghost", "setname", "multi-prefix", "draft/chathistory", "draft/multiline", "draft/metadata-3", "draft/message-redaction", "draft/read-marker", "sasl"}

func (c *connection) enqueue(events ...*girc.Event) error {
	c.mu.Lock()
	ready := c.ready
	c.mu.Unlock()
	if !ready {
		return ErrDisconnected
	}
	select {
	case <-c.ctx.Done():
		return ErrDisconnected
	case c.queue <- events:
		return nil
	default:
		return ErrBusy
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
			sent := false
			for _, event := range events {
				select {
				case <-c.ctx.Done():
					return
				case <-ticker.C:
				}
				c.mu.Lock()
				client := c.client
				ready := c.ready
				if strings.HasPrefix(event.Tags["label"], "chanserv:") {
					ready = ready && c.serviceRequest != nil && c.serviceRequest.label == event.Tags["label"] && c.serviceRequest.client == client
				}
				if strings.HasPrefix(event.Tags["label"], "party:") {
					ready = ready && c.partyRequest != nil && c.partyRequest.label == event.Tags["label"] && c.partyRequest.client == client
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
				c.capture(*event, true)
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
		c.registrationError = ""
		c.caps = map[string]bool{}
		c.capOffers = map[string]string{}
		c.capValues = map[string]string{}
		c.lineBytes = 0
		c.ergoServices = false
		c.serviceBatches = map[string]string{}
		c.mu.Unlock()
		client.Handlers.Add(girc.UPDATE_GENERAL, func(client *girc.Client, _ girc.Event) { c.refreshFeatures(client) })
		client.Handlers.Add(girc.CONNECTED, func(client *girc.Client, _ girc.Event) { c.connected(client) })
		// ALL_EVENTS runs before girc's command-specific state handlers. Observe
		// its post-update notification instead of reading a one-event-old roster.
		client.Handlers.Add(girc.UPDATE_STATE, func(client *girc.Client, _ girc.Event) { c.syncMembers(client) })
		client.Handlers.Add(girc.ALL_EVENTS, func(client *girc.Client, e girc.Event) { c.receive(client, e) })
		c.connectionState("connecting", client, nil)
		attemptCtx, cancelAttempt := context.WithCancel(c.ctx)
		timedOut := false // protected by c.mu, alongside ready
		var attemptTransport net.Conn
		registrationTimer := time.AfterFunc(c.manager.options.ConnectTimeout, func() {
			c.mu.Lock()
			if c.ready {
				c.mu.Unlock()
				return
			}
			timedOut = true
			transport := attemptTransport
			c.mu.Unlock()
			cancelAttempt()
			if transport != nil {
				_ = transport.Close()
			}
		})
		dialErr := client.DialerConnect(safeDialer{ctx: attemptCtx, allowPrivate: c.manager.options.AllowPrivate, connected: func(conn net.Conn) {
			c.mu.Lock()
			attemptTransport = conn
			c.transport = conn
			c.mu.Unlock()
			if attemptCtx.Err() != nil {
				_ = conn.Close()
			}
		}})
		registrationTimer.Stop()
		cancelAttempt()
		c.mu.Lock()
		if timedOut {
			dialErr = context.DeadlineExceeded
		}
		c.transport = nil
		c.mu.Unlock()
		c.connectionState("disconnected", client, dialErr)
		c.failHistory()
		c.failServices()
		c.mu.Lock()
		wasReady := c.ready
		stable := wasReady && time.Since(c.readyAt) >= stableConnection
		c.everConnected = c.everConnected || wasReady
		if !c.everConnected && c.ctx.Err() == nil && dialErr != nil {
			c.permanent = true
		}
		c.ready = false
		c.caps = map[string]bool{}
		c.capValues = map[string]string{}
		c.batches = map[string]*multiline{}
		c.historyBatches = map[string]bool{}
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
		if permanent {
			return
		}
	drain:
		for {
			select {
			case <-c.queue:
			default:
				break drain
			}
		}
		if stable {
			// The backoff covers consecutive failures; a connection that stayed up
			// ends them. A server that drops each session at once keeps backing off.
			delay = c.manager.options.ReconnectMin
		}
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(delay + time.Duration(time.Now().UnixNano()%int64(delay/4+1))):
		}
		if delay < 30*time.Second {
			delay *= 2
		}
	}
}

func (c *connection) closeTransport() {
	c.mu.Lock()
	conn := c.transport
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

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
		select {
		case c.queue <- setup:
		case <-c.ctx.Done():
		}
	}
}
