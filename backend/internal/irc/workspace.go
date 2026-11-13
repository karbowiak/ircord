package irc

import (
	"sync"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/notify"
	"ircord/backend/internal/storage"
)

type workspace struct {
	manager              *Manager
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
	storageErrLogged     bool
	failedBeforeRestart  map[string]bool // networks that had failed for good when the backend last stopped
	deleted              bool
	info                 map[string]ConnectionInfo
	latency              map[string][]model.LatencySample
	serverLogs           map[string][]ServerLine
	trace                map[string][]ServerLine
	traceSubscribers     map[string]map[chan ServerLine]struct{}
	mu                   sync.Mutex
	configMu             sync.Mutex
	self                 Member
	profileImports       map[string]map[string]bool
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
	// autoAway is set while no client has been active for the auto-away delay;
	// autoAwaySent marks the networks IRCord itself set away for it.
	autoAway     bool
	autoAwaySent map[string]bool
	// awayOn is each network's own away state, from RPL_NOWAWAY/RPL_UNAWAY.
	awayOn map[string]bool
	// Away text and brief-away state are tracked per network.
	awayText  map[string]string
	briefAway map[string]bool
	// signedOut remembers networks disabled by the last session signing out.
	signedOut map[string]bool
	// device is the shared form factor of the device the account is on.
	device string
}

func (m *Manager) space(owner string) *workspace {
	m.mu.Lock()
	if w := m.owners[owner]; w != nil {
		m.mu.Unlock()
		<-w.loaded
		return w
	}
	self := Member{ID: owner, Nick: "ircord", DisplayName: "ircord", NameSource: "nick", Color: "lavender", Status: "online", Role: "member"}
	w := &workspace{policy: notify.New(notify.Defaults()), cacheTouched: map[string]uint64{}, failedBeforeRestart: map[string]bool{}, loaded: make(chan struct{}), info: map[string]ConnectionInfo{}, latency: map[string][]model.LatencySample{}, serverLogs: map[string][]ServerLine{}, trace: map[string][]ServerLine{}, traceSubscribers: map[string]map[chan ServerLine]struct{}{}, ownerID: owner, store: m.options.Store, self: self, members: map[string]Member{owner: self}, networks: map[string]Network{}, conversations: map[string]Conversation{}, messages: map[string][]Message{}, connections: map[string]*connection{}, configs: map[string]NetworkConfig{}, preferences: Preferences{Theme: "dark", ShowMembers: true}, subscribers: map[chan Event]struct{}{}}
	w.deleted = m.deletedOwners[owner]
	w.manager = m
	if owner == partyControlOwner {
		w.store = nil
	}
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

func (m *Manager) Preferences(owner string, p Preferences) Preferences {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if p.Theme != "light" {
		p.Theme = "dark"
	}
	w.preferences = p
	w.emit("preferences.updated", p)
	return p
}
