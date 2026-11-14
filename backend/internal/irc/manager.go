package irc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/activity"
	ergoapi "ircord/backend/internal/ergo"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/push"
	"ircord/backend/internal/storage"
)

type Options struct {
	DisableAutoRegistration bool
	AssociatedServer        *ergoapi.Client
	NetworkPasswords        map[string]string
	ServerList              ServerList
	Origin                  func(string) (string, error)
	Push                    *push.Service
	Files                   *filehost.Service
	AllowPrivate            bool
	TLSConfig               *tls.Config
	ReconnectMin            time.Duration
	MaxConnections          int
	ConnectTimeout          time.Duration
	Store                   *storage.Store
	// Home is the IRCord network (ID, host and port) where channels the user creates are
	// registered to their account.
	Home NetworkConfig
	// Router decides which devices are alerted (#84). Without one, alerts go to every device.
	Router *activity.Router
	// Moderation is the operator login on the associated server that carries out
	// server owners' deletions. Without it, server owners delete like anyone else.
	Moderation *ModerationLogin
}

// Presets returns only public operator-provided connection hints.
func (m *Manager) Presets() (bool, []NetworkPreset) {
	return m.options.ServerList.Configured, clone(m.options.ServerList.Presets)
}

type Manager struct {
	profileExpiryWake chan struct{}
	people            peopleCache
	partyPeople       peopleCache
	parties           *partyManager
	moderator         *moderator
	purges            purgeJobs
	checkSlots        chan struct{}
	listSlots         chan struct{}
	connectionCount   int
	serviceSlots      chan struct{}
	channelCreates    map[string]bool
	deletedOwners     map[string]bool
	mu                sync.Mutex
	owners            map[string]*workspace
	options           Options
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	closed            bool
	// Channel-wide settings keyed by upstream network and folded channel name.
	sharedMu     sync.Mutex
	shared       map[string]ChannelSettings
	sharedLoaded bool
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
	m := &Manager{profileExpiryWake: make(chan struct{}, 1), serviceSlots: make(chan struct{}, 16), checkSlots: make(chan struct{}, 16), listSlots: make(chan struct{}, 4), deletedOwners: map[string]bool{}, owners: map[string]*workspace{}, options: options, ctx: ctx, cancel: cancel}
	m.wg.Add(1)
	go m.runProfileExpiry()
	m.partyPeople.limit = 128
	m.sharedMu.Lock()
	_ = m.loadSharedChannels() // logged; retried on first use
	m.sharedMu.Unlock()
	m.moderator = newModerator(m)
	m.parties = newPartyManager(m)
	if m.parties != nil {
		m.wg.Add(1)
		go m.parties.run()
	}
	return m
}

func (m *Manager) Connect(owner string, cfg NetworkConfig) (Network, error) {
	return m.connect(owner, cfg, false)
}

// connect starts a network connection. A restored network was working or
// reconnecting when the backend last stopped, so a failed first attempt retries
// instead of stopping.
func (m *Manager) connect(owner string, cfg NetworkConfig, restored bool) (Network, error) {
	cfg = m.configuredPassword(cfg)
	var normalizeErr error
	cfg, normalizeErr = normalizeConfig(cfg)
	if normalizeErr != nil {
		return Network{}, normalizeErr
	}
	if err := m.validatePartyConfig(owner, cfg); err != nil {
		return Network{}, err
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
	c := &connection{restored: restored, serviceGate: make(chan struct{}, 1), serviceBatches: map[string]string{}, createdAt: time.Now(), manager: m, owner: w, config: cfg, ctx: ctx, cancel: cancel, queue: make(chan []*girc.Event, 64), pending: map[string]pending{}, batches: map[string]*multiline{}, historyBatches: map[string]bool{}, historyGate: make(chan struct{}, 1), historyRequests: map[string]*historyRequest{}, historyResponses: map[string]*historyRequest{}, batchLabels: map[string]string{}, catching: map[string]bool{}, modeListGate: make(chan struct{}, 1), listGate: make(chan struct{}, 1), closedTargets: map[string]bool{}, desired: map[string]string{}, roster: map[string]map[string]bool{}, arrivals: map[string]bool{}, nicks: map[string]string{}, caps: map[string]bool{}, done: make(chan struct{})}
	n := Network{ID: cfg.ID, Name: cfg.Name, Host: cfg.Host, Port: cfg.Port, TLS: cfg.TLS, Nick: cfg.Nick, Initials: strings.ToUpper(string([]rune(cfg.Name + "I")[0])), Color: "mint", Status: "connecting", Capabilities: []string{}}
	if err := w.prepareNetworkProfile(&n, cfg); err != nil {
		cancel()
		return Network{}, err
	}
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
	delete(w.signedOut, n.ID)
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
	return clone(n), nil
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

func (m *Manager) Reconnect(owner, network string) error {
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	w.mu.Lock()
	cfg, ok := w.configs[network]
	active := w.connections[network]
	w.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	if active != nil {
		if err := m.Disconnect(owner, network); err != nil {
			return err
		}
	}
	_, err := m.Connect(owner, cfg)
	return err
}

func (m *Manager) Disconnect(owner, network string) error {
	return m.disconnect(owner, network, false)
}

// SignOut turns off every network the owner has on, because they signed out of their last
// device, and disconnects the connected ones. Those networks stay off, across restarts too,
// until SignIn; a dropped connection still reconnects by itself, a deliberate sign-out
// doesn't (#237). A network that fails to disconnect doesn't stop the others.
func (m *Manager) SignOut(owner string) error {
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	// Networks can be on without a connection, such as one saved while the connection pool
	// was full; the next restart would connect them, so mark every network that is on.
	if w.store != nil {
		if err := w.store.SignOutEnabledNetworks(owner); err != nil {
			return ErrStorage
		}
	}
	w.mu.Lock()
	ids := make([]string, 0, len(w.connections))
	for id := range w.connections {
		ids = append(ids, id)
	}
	w.mu.Unlock()
	sort.Strings(ids)
	var errs []error
	for _, id := range ids {
		if err := m.disconnect(owner, id, true); err != nil && err != ErrNotFound {
			errs = append(errs, fmt.Errorf("sign out of network %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// SignIn reconnects the networks SignOut turned off. Networks the user disconnected
// themselves stay disconnected. A network that can't connect keeps its mark for the next
// sign-in and doesn't stop the others.
func (m *Manager) SignIn(owner string) error {
	w := m.space(owner)
	w.configMu.Lock()
	defer w.configMu.Unlock()
	ids := []string{}
	if w.store != nil {
		var err error
		if ids, err = w.store.SignedOutNetworks(owner); err != nil {
			return ErrStorage
		}
	}
	w.mu.Lock()
	for id := range w.signedOut {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	w.mu.Unlock()
	sort.Strings(ids)
	var errs []error
	for _, id := range ids {
		w.mu.Lock()
		cfg, ok := w.configs[id]
		active := w.connections[id] != nil
		if !ok || active {
			delete(w.signedOut, id)
		}
		w.mu.Unlock()
		if !ok || active {
			continue
		}
		// Connect clears the mark once it succeeds; a failure, including a full connection
		// pool, leaves it so the next sign-in tries again.
		if _, err := m.Connect(owner, cfg); err != nil && err != ErrBusy {
			errs = append(errs, fmt.Errorf("reconnect network %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) disconnect(owner, network string, signedOut bool) error {
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
		save := func() error { return c.owner.store.SaveNetwork(owner, cfg, false) }
		if signedOut {
			save = func() error { return c.owner.store.SignOutNetwork(owner, network) }
		}
		if err := save(); err != nil {
			return ErrStorage
		}
	}
	if signedOut {
		c.owner.mu.Lock()
		if c.owner.signedOut == nil {
			c.owner.signedOut = map[string]bool{}
		}
		c.owner.signedOut[network] = true
		c.owner.mu.Unlock()
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
	if m.moderator != nil {
		m.moderator.close()
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

// WaitConnected is a convenience for integration tests and service startup.
func (m *Manager) WaitConnected(ctx context.Context, owner, network string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if n, err := m.Network(owner, network); err == nil && n.Status == "connected" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
