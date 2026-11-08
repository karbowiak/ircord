package irc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"ircord/backend/internal/activity"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/model"
	"ircord/backend/internal/notify"
	"ircord/backend/internal/push"
	"ircord/backend/internal/storage"
)

type Options struct {
	NetworkPasswords map[string]string
	Push             *push.Service
	Files            *filehost.Service
	AllowPrivate     bool
	TLSConfig        *tls.Config
	ReconnectMin     time.Duration
	MaxConnections   int
	ConnectTimeout   time.Duration
	Store            *storage.Store
	// Home is the IRCord network (ID, host and port) where channels the user creates are
	// registered to their account.
	Home NetworkConfig
	// Router decides which devices are alerted (#84). Without one, alerts go to every device.
	Router *activity.Router
}
type Manager struct {
	checkSlots      chan struct{}
	listSlots       chan struct{}
	connectionCount int
	serviceSlots    chan struct{}
	deletedOwners   map[string]bool
	mu              sync.Mutex
	owners          map[string]*workspace
	options         Options
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	closed          bool
	// Channel-wide settings keyed by upstream network and folded channel name.
	sharedMu     sync.Mutex
	shared       map[string]ChannelSettings
	sharedLoaded bool
}
type workspace struct {
	readTimes            map[string]time.Time
	unreadFrom           map[string]string
	invitations          map[string]model.Invitation
	systemLogsBackfilled map[string]bool
	ephemeralSubscribers map[chan any]struct{}
	policy               *notify.Policy
	cacheClock           uint64
	cacheTouched         map[string]uint64
	loaded               chan struct{}
	ownerID              string
	store                *storage.Store
	storageErr           error
	deleted              bool
	info                 map[string]ConnectionInfo
	serverLogs           map[string][]ServerLine
	trace                map[string][]ServerLine
	traceSubscribers     map[string]map[chan ServerLine]struct{}
	mu                   sync.Mutex
	configMu             sync.Mutex
	self                 Member
	members              map[string]Member
	networks             map[string]Network
	conversations        map[string]Conversation
	messages             map[string][]Message
	// saved holds bookmarks for a workspace without durable storage.
	saved       map[string]model.SavedMessage
	connections map[string]*connection
	configs     map[string]NetworkConfig
	preferences Preferences
	seq         uint64
	events      []Event
	subscribers map[chan Event]struct{}
	// autoAway is set while no device has been connected for the auto-away delay;
	// autoAwaySent marks the networks IRCord itself set away for it.
	autoAway     bool
	autoAwaySent map[string]bool
	// awayOn is each network's own away state, from RPL_NOWAWAY/RPL_UNAWAY.
	awayOn map[string]bool
	// device is the shared form factor of the device the account is on.
	device string
}
type pending struct {
	input     SendInput
	messageID string
	at        time.Time
}
type multiline struct {
	historyRef string
	historical bool
	first      girc.Event
	text       string
	lines      int
}
type connection struct {
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
	registrationError  string
	listGate           chan struct{}
	listRequest        *listRequest
	listCache          []ChannelListing
	listAt             time.Time
	done               chan struct{}
	// profileCheck collects the server's answers about our own published profile while a
	// check is pending, profileCheckGen tells checks apart and profileCheckDone is set once a
	// check has queued its SETs. profilePublished caches what
	// this account last published on this network (kept in account storage); profileSave
	// serializes its updates. See profile_publish.go.
	profileCheck     map[string]string
	profileCheckGen  int
	profileCheckDone bool
	profilePublished map[string]string
	profileSave      sync.Mutex
}

func New(options Options) *Manager {
	options.NetworkPasswords = clone(options.NetworkPasswords)
	if options.ReconnectMin <= 0 {
		options.ReconnectMin = time.Second
	}
	if options.MaxConnections <= 0 {
		options.MaxConnections = 1000
	}
	if options.ConnectTimeout <= 0 {
		options.ConnectTimeout = 15 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{serviceSlots: make(chan struct{}, 16), checkSlots: make(chan struct{}, 16), listSlots: make(chan struct{}, 4), deletedOwners: map[string]bool{}, owners: map[string]*workspace{}, options: options, ctx: ctx, cancel: cancel}
	m.sharedMu.Lock()
	_ = m.loadSharedChannels() // logged; retried on first use
	m.sharedMu.Unlock()
	return m
}
func (m *Manager) space(owner string) *workspace {
	m.mu.Lock()
	if w := m.owners[owner]; w != nil {
		m.mu.Unlock()
		<-w.loaded
		return w
	}
	self := Member{ID: owner, Nick: "ircord", DisplayName: "ircord", NameSource: "nick", Color: "lavender", Status: "online", Role: "member"}
	w := &workspace{policy: notify.New(notify.Defaults()), cacheTouched: map[string]uint64{}, loaded: make(chan struct{}), info: map[string]ConnectionInfo{}, serverLogs: map[string][]ServerLine{}, trace: map[string][]ServerLine{}, traceSubscribers: map[string]map[chan ServerLine]struct{}{}, ownerID: owner, store: m.options.Store, self: self, members: map[string]Member{owner: self}, networks: map[string]Network{}, conversations: map[string]Conversation{}, messages: map[string][]Message{}, connections: map[string]*connection{}, configs: map[string]NetworkConfig{}, preferences: Preferences{Theme: "dark", ShowMembers: true}, subscribers: map[chan Event]struct{}{}}

	w.deleted = m.deletedOwners[owner]
	w.invitations = map[string]model.Invitation{}
	m.owners[owner] = w
	m.mu.Unlock()
	w.mu.Lock()
	if !w.deleted {
		w.load()
	}
	w.mu.Unlock()
	close(w.loaded)
	return w
}
func (w *workspace) emit(kind string, data any) {
	if w.deleted {
		w.storageErr = ErrNotFound
		return
	}
	if w.storageErr != nil {
		w.commandError("", "Storage is unavailable; restart after restoring storage to resynchronize.")
		return
	}
	if member, ok := data.(Member); ok {
		member.Ignored = member.ID != w.self.ID && w.policy.Ignored(member.Nick, member.Username, member.Host)
		data = member
	}
	var e Event
	if w.store != nil {
		var err error
		e, err = w.store.Commit(w.ownerID, kind, data)
		if err != nil {
			w.storageErr = err
			w.commandError("", "Storage is unavailable; live delivery is paused.")
			return
		}
		w.seq = e.Seq
		w.storageErr = nil
	} else {
		w.seq++
		b, _ := json.Marshal(data)
		e = Event{V: 1, Seq: w.seq, Type: kind, Data: b}
	}
	if kind == "notification-settings.updated" {
		raw, _ := json.Marshal(data)
		w.policy = notify.Compile(raw)
	}
	if w.store == nil {
		w.events = append(w.events, e)
		if len(w.events) > 256 {
			w.events = append([]Event(nil), w.events[len(w.events)-256:]...)
		}
	}
	for ch := range w.subscribers {
		select {
		case ch <- clone(e):
		default:
			delete(w.subscribers, ch)
			close(ch)
		}
	}
}
func (w *workspace) commandError(clientID, message string) {
	if w.store != nil && clientID != "" {
		_ = w.store.FailCommand(w.ownerID, clientID, message)
	}
	w.sendCommandError(clientID, message)
}
func (w *workspace) sendCommandError(clientID, message string) {
	for ch := range w.subscribers {
		select {
		case ch <- Event{V: 1, Type: "command.error", ClientID: clientID, Message: message}:
		default:
			delete(w.subscribers, ch)
			close(ch)
		}
	}
}

// Subscribe atomically snapshots the replay cursor and attaches the live queue.
// A slow consumer is closed and must resynchronize, never stall another owner.
func (m *Manager) Subscribe(owner string, after uint64) ([]Event, <-chan Event, bool, func()) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	ch := make(chan Event, 64)
	if len(w.subscribers) >= 16 {
		close(ch)
		return nil, ch, false, func() {}
	}
	valid := after <= w.seq && (len(w.events) == 0 || after+1 >= w.events[0].Seq)
	replay := []Event{}
	if valid {
		for _, e := range w.events {
			if e.Seq > after {
				replay = append(replay, clone(e))
			}
		}
	}
	if w.store != nil {
		var err error
		replay, valid, err = w.store.Replay(owner, after, 4096)
		if err != nil {
			valid = false
			w.storageErr = err
		}
	}
	// Older durable network events predate the expanded feature DTO. Re-encode
	// through the current model so reconnect replay satisfies the same schema
	// as live events; absent legacy features default to unavailable.
	for i := range replay {
		if replay[i].Type != "network.updated" {
			continue
		}
		var network Network
		if json.Unmarshal(replay[i].Data, &network) != nil {
			valid = false
			break
		}
		replay[i].Data, _ = json.Marshal(network)
	}
	w.subscribers[ch] = struct{}{}
	var once sync.Once
	return replay, ch, valid, func() {
		once.Do(func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			if _, ok := w.subscribers[ch]; ok {
				delete(w.subscribers, ch)
				close(ch)
			}
		})
	}
}
func (m *Manager) Snapshot(owner string) Snapshot {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	s := Snapshot{Version: 1, Demo: false, User: w.self, Members: []Member{}, Networks: []Network{}, Conversations: []Conversation{}, Preferences: w.preferences, Cursor: w.seq}
	for _, v := range w.invitations {
		if !v.Rejected {
			s.Invitations = append(s.Invitations, v)
		}
	}
	sort.Slice(s.Invitations, func(i, j int) bool { return s.Invitations[i].CreatedAt.Before(s.Invitations[j].CreatedAt) })
	for _, v := range w.members {
		s.Members = append(s.Members, v)
	}
	for _, v := range w.networks {
		s.Networks = append(s.Networks, v)
	}
	for _, v := range w.conversations {
		if w.store != nil {
			v = w.refreshUnread(v)
		}
		s.Conversations = append(s.Conversations, v)
	}
	sort.Slice(s.Networks, func(i, j int) bool { return s.Networks[i].ID < s.Networks[j].ID })
	sort.Slice(s.Conversations, func(i, j int) bool { return s.Conversations[i].ID < s.Conversations[j].ID })
	return clone(s)
}
func (m *Manager) Connect(owner string, cfg NetworkConfig) (Network, error) {
	cfg = m.configuredPassword(cfg)
	var normalizeErr error
	cfg, normalizeErr = normalizeConfig(cfg)
	if normalizeErr != nil {
		return Network{}, normalizeErr
	}
	if owner == "" || !clean(owner, 128) {
		return Network{}, fmt.Errorf("owner is required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 || cfg.Host == "" || !clean(cfg.Host, 253) || strings.ContainsAny(cfg.Host, " /\\") || !girc.IsValidNick(cfg.Nick) || !clean(cfg.Name, 100) || !clean(cfg.SASLUser, 100) || !clean(cfg.SASLPassword, 512) || !clean(cfg.ServerPassword, 512) {
		return Network{}, fmt.Errorf("invalid network configuration")
	}
	if !cfg.TLS && !m.options.AllowPrivate {
		return Network{}, fmt.Errorf("TLS is required")
	}
	if cfg.Auth != nil && cfg.Auth.Method != "none" && (cfg.Auth.Password == nil || *cfg.Auth.Password == "") {
		return Network{}, fmt.Errorf("add the authentication password before connecting")
	}
	if (cfg.SASLUser == "") != (cfg.SASLPassword == "") {
		return Network{}, fmt.Errorf("SASL username and password must be provided together")
	}
	if cfg.ID == "" {
		cfg.ID = id()
	}
	if !token(cfg.ID, 100) {
		return Network{}, fmt.Errorf("invalid network ID")
	}
	w := m.space(owner)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Network{}, fmt.Errorf("manager is closed")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.connections[cfg.ID]; ok {
		return Network{}, fmt.Errorf("network already has an owner connection")
	}
	if w.storageErr != nil {
		return Network{}, ErrStorage
	}
	if w.deleted || m.deletedOwners[owner] {
		return Network{}, ErrNotFound
	}
	if m.connectionCount >= m.options.MaxConnections || len(w.connections) >= 20 {
		return Network{}, ErrBusy
	}
	ctx, cancel := context.WithCancel(m.ctx)
	c := &connection{serviceGate: make(chan struct{}, 1), serviceBatches: map[string]string{}, createdAt: time.Now(), manager: m, owner: w, config: cfg, ctx: ctx, cancel: cancel, queue: make(chan []*girc.Event, 64), pending: map[string]pending{}, batches: map[string]*multiline{}, historyBatches: map[string]bool{}, historyGate: make(chan struct{}, 1), historyRequests: map[string]*historyRequest{}, historyResponses: map[string]*historyRequest{}, batchLabels: map[string]string{}, catching: map[string]bool{}, modeListGate: make(chan struct{}, 1), listGate: make(chan struct{}, 1), closedTargets: map[string]bool{}, desired: map[string]string{}, roster: map[string]map[string]bool{}, arrivals: map[string]bool{}, nicks: map[string]string{}, caps: map[string]bool{}, done: make(chan struct{})}
	n := Network{ID: cfg.ID, Name: cfg.Name, Host: cfg.Host, Port: cfg.Port, TLS: cfg.TLS, Nick: cfg.Nick, Initials: strings.ToUpper(string([]rune(cfg.Name + "I")[0])), Color: "mint", Status: "connecting", Capabilities: []string{}}
	applyConfig(&n, cfg)
	if w.store != nil {
		for _, conv := range w.conversations {
			if conv.NetworkID == cfg.ID {
				if e := w.store.BeginGap(owner, conv.ID); e != nil {
					cancel()
					return Network{}, ErrStorage
				}
			}
		}
	}
	for _, entry := range cfg.Channels {
		target := strings.Fields(entry)[0]
		if !girc.IsValidChannel(target) || !clean(target, 100) || strings.ContainsAny(target, " ,:") {
			cancel()
			return Network{}, fmt.Errorf("invalid initial channel")
		}
		c.desired[c.foldUnlocked(target)] = entry
	}
	if w.store != nil {
		if err := w.store.SaveNetwork(owner, cfg, true); err != nil {
			cancel()
			return Network{}, ErrStorage
		}
	}
	c.restoreNicks()
	w.info[n.ID] = newConnectionInfo(n.ID)
	w.networks[n.ID] = n
	w.connections[n.ID] = c
	w.configs[n.ID] = cfg
	w.emit("network.updated", n)
	if w.storageErr != nil {
		delete(w.connections, n.ID)
		cancel()
		return Network{}, ErrStorage
	}
	m.connectionCount++
	m.wg.Add(2)
	go c.run()
	go c.writer()
	return n, nil
}
func (m *Manager) connection(owner, network string) (*connection, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	c := w.connections[network]
	if c == nil || w.deleted {
		return nil, ErrNotFound
	}
	return c, nil
}
func (m *Manager) Disconnect(owner, network string) error {
	c, err := m.connection(owner, network)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.ready = false
	c.mu.Unlock()
	if c.owner.store != nil {
		c.owner.mu.Lock()
		cfg := c.owner.configs[network]
		c.owner.mu.Unlock()
		if err := c.owner.store.SaveNetwork(owner, cfg, false); err != nil {
			return ErrStorage
		}
	}
	c.cancel()
	c.closeTransport()
	<-c.done
	c.workers.Wait()
	c.owner.mu.Lock()
	removed := c.owner.connections[network] == c
	if removed {
		n := c.owner.networks[network]
		n.Status = "disconnected"
		n.StatusDetail = ""
		c.owner.networks[network] = n
		c.owner.emit("network.updated", n)
		delete(c.owner.connections, network)
	}
	c.owner.mu.Unlock()
	if removed {
		m.mu.Lock()
		m.connectionCount--
		m.mu.Unlock()
	}
	return nil
}
func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.cancel()
	var conns []*connection
	for _, w := range m.owners {
		w.mu.Lock()
		for _, c := range w.connections {
			conns = append(conns, c)
		}
		w.mu.Unlock()
	}
	m.mu.Unlock()
	for _, c := range conns {
		c.closeTransport()
	}
	m.wg.Wait()
	m.mu.Lock()
	for _, w := range m.owners {
		w.mu.Lock()
		w.closeTraceSubscribers()
		for ch := range w.subscribers {
			close(ch)
			delete(w.subscribers, ch)
		}
		w.mu.Unlock()
	}
	m.mu.Unlock()
}
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
				c.mu.Unlock()
				if !ready || client == nil {
					break
				}
				c.capture(*event, true)
				client.Send(event)
				c.recordSentProfile(event)
			}
		}
	}
}

var requestedCaps = []string{"echo-message", "labeled-response", "batch", "server-time", "message-tags", "account-tag", "account-notify", "away-notify", "extended-join", "chghost", "setname", "multi-prefix", "draft/chathistory", "draft/multiline", "draft/metadata-3", "draft/message-redaction", "draft/read-marker", "sasl"}

func (c *connection) run() {
	defer c.manager.wg.Done()
	defer close(c.done)
	delay := c.manager.options.ReconnectMin
	everConnected := false
	for c.ctx.Err() == nil {
		c.owner.mu.Lock()
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
		everConnected = everConnected || c.ready
		if !everConnected && c.ctx.Err() == nil && dialErr != nil {
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

type safeDialer struct {
	connected    func(net.Conn)
	ctx          context.Context
	allowPrivate bool
}

func (d safeDialer) Dial(network, address string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(d.ctx, 8*time.Second)
	defer cancel()
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		ip = ip.Unmap()
		if !d.allowPrivate && (!ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || netip.MustParsePrefix("100.64.0.0/10").Contains(ip)) {
			return nil, fmt.Errorf("private IRC destinations are disabled")
		}
	}
	var lastErr error
	for _, ip := range ips {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		lastErr = err
		if err == nil {
			if d.connected != nil {
				d.connected(conn)
			}
			return conn, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("connect to IRC destination: %w", lastErr)
	}
	return nil, fmt.Errorf("IRC destination has no usable addresses")
}

func (c *connection) closeTransport() {
	c.mu.Lock()
	conn := c.transport
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}
