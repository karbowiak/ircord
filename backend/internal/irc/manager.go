package irc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
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
	"ircord/backend/internal/preview"
	"ircord/backend/internal/push"
	"ircord/backend/internal/storage"
)

type Options struct {
	NetworkPasswords map[string]string
	Push             *push.Service
	Previews         *preview.Service
	Files            *filehost.Service
	AllowPrivate     bool
	TLSConfig        *tls.Config
	ReconnectMin     time.Duration
	MaxConnections   int
	Store            *storage.Store
}
type Manager struct {
	listSlots       chan struct{}
	previewJobs     chan previewJob
	connectionCount int
	deletedOwners   map[string]bool
	mu              sync.Mutex
	owners          map[string]*workspace
	options         Options
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	closed          bool
}
type workspace struct {
	invitations          map[string]model.Invitation
	systemLogsBackfilled map[string]bool
	typingSubscribers    map[chan model.TypingEvent]struct{}
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
	self                 Member
	members              map[string]Member
	networks             map[string]Network
	conversations        map[string]Conversation
	messages             map[string][]Message
	connections          map[string]*connection
	configs              map[string]NetworkConfig
	preferences          Preferences
	seq                  uint64
	events               []Event
	subscribers          map[chan Event]struct{}
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
	realnames         map[string]string
	typingSent        map[string]typingSent
	typingReceived    map[string]typingSent
	typingWindow      time.Time
	typingBurst       int
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
	historyGate       chan struct{}
	historyRequests   map[string]*historyRequest
	historyResponses  map[string]*historyRequest
	batchLabels       map[string]string
	unlabeledHistory  *historyRequest
	catching          map[string]bool
	closedTargets     map[string]bool
	desired           map[string]string
	nicks             map[string]string
	caps              map[string]bool
	ready             bool
	permanent         bool
	registrationError string
	listGate          chan struct{}
	listRequest       *listRequest
	listCache         []ChannelListing
	listAt            time.Time
	done              chan struct{}
}

func New(options Options) *Manager {
	options.NetworkPasswords = clone(options.NetworkPasswords)
	if options.ReconnectMin <= 0 {
		options.ReconnectMin = time.Second
	}
	if options.MaxConnections <= 0 {
		options.MaxConnections = 1000
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{listSlots: make(chan struct{}, 4), deletedOwners: map[string]bool{}, owners: map[string]*workspace{}, options: options, ctx: ctx, cancel: cancel}
	if options.Previews != nil {
		m.previewJobs = make(chan previewJob, 64)
		for i := 0; i < 4; i++ {
			m.wg.Add(1)
			go m.previewWorker()
		}
	}
	return m
}
func (m *Manager) space(owner string) *workspace {
	m.mu.Lock()
	if w := m.owners[owner]; w != nil {
		m.mu.Unlock()
		<-w.loaded
		return w
	}
	self := Member{ID: owner, Nick: "ircord", DisplayName: "You", Color: "lavender", Status: "online", Role: "member"}
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
	s.Capabilities.ServerPreviews = m.options.Previews != nil
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
			v.UnreadCount, _ = w.store.Unread(owner, v.ID)
			v.MentionCount, _ = w.store.Mentions(owner, v.ID)
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
	c := &connection{createdAt: time.Now(), manager: m, owner: w, config: cfg, ctx: ctx, cancel: cancel, queue: make(chan []*girc.Event, 64), pending: map[string]pending{}, batches: map[string]*multiline{}, historyBatches: map[string]bool{}, historyGate: make(chan struct{}, 1), historyRequests: map[string]*historyRequest{}, historyResponses: map[string]*historyRequest{}, batchLabels: map[string]string{}, catching: map[string]bool{}, modeListGate: make(chan struct{}, 1), listGate: make(chan struct{}, 1), closedTargets: map[string]bool{}, desired: map[string]string{}, nicks: map[string]string{}, caps: map[string]bool{}, done: make(chan struct{})}
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
				c.mu.Unlock()
				if !ready || client == nil {
					break
				}
				c.capture(*event, true)
				client.Send(event)
			}
		}
	}
}

var requestedCaps = []string{"echo-message", "labeled-response", "batch", "server-time", "message-tags", "account-tag", "account-notify", "away-notify", "extended-join", "chghost", "setname", "multi-prefix", "draft/chathistory", "draft/multiline", "draft/metadata-3", "draft/message-redaction", "draft/read-marker", "sasl"}

func (c *connection) run() {
	defer c.manager.wg.Done()
	defer close(c.done)
	delay := c.manager.options.ReconnectMin
	for c.ctx.Err() == nil {
		c.owner.mu.Lock()
		attemptCfg := c.owner.configs[c.config.ID]
		c.realnames = map[string]string{}
		c.owner.mu.Unlock()
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
		c.mu.Unlock()
		client.Handlers.Add(girc.CONNECTED, func(client *girc.Client, _ girc.Event) { c.connected(client) })
		// ALL_EVENTS runs before girc's command-specific state handlers. Observe
		// its post-update notification instead of reading a one-event-old roster.
		client.Handlers.Add(girc.UPDATE_STATE, func(client *girc.Client, _ girc.Event) { c.syncMembers(client) })
		client.Handlers.Add(girc.ALL_EVENTS, func(client *girc.Client, e girc.Event) { c.receive(client, e) })
		c.connectionState("connecting", client, nil)
		dialErr := client.DialerConnect(safeDialer{ctx: c.ctx, allowPrivate: c.manager.options.AllowPrivate, connected: func(conn net.Conn) {
			c.mu.Lock()
			c.transport = conn
			c.mu.Unlock()
			if c.ctx.Err() != nil {
				_ = conn.Close()
			}
		}})
		c.mu.Lock()
		c.transport = nil
		c.mu.Unlock()
		c.connectionState("disconnected", client, dialErr)
		c.failHistory()
		c.mu.Lock()
		c.ready = false
		c.batches = map[string]*multiline{}
		c.historyBatches = map[string]bool{}
		permanent := c.permanent
		registrationError := c.registrationError
		pendingCommands := c.pending
		c.pending = map[string]pending{}
		c.mu.Unlock()
		c.owner.mu.Lock()
		n := c.owner.networks[c.config.ID]
		n.Status = "connecting"
		if c.ctx.Err() != nil {
			n.Status = "disconnected"
		}
		if permanent {
			n.Status = "error"
		}
		if dialErr != nil {
			n.StatusDetail = c.redactSecrets(dialErr.Error())
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
