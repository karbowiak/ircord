// Package activity owns private device selection and notification deadlines.
// A visible, focused focal tab can keep reading indefinitely by renewing its
// bounded lease. Only genuine engagement selects a different focal device.
package activity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"time"

	"ircord/backend/internal/storage"
)

const (
	DefaultTarget    = 30 * time.Second
	DefaultAll       = 5 * time.Minute
	LeaseTTL         = 60 * time.Second
	MaxTabs          = 64
	MaxCandidates    = 50 // candidate-page size, never a truncation of conversation groups
	alertBuffer      = 16
	sessionRetention = 24 * time.Hour
	Desktop          = "desktop"
	Phone            = "phone"
	Tablet           = "tablet"
	checkpointEvery  = 25 * time.Second
	reconnectGrace   = 5 * time.Second
)
const (
	phaseGrace = iota
	phaseTarget
	phaseAll
)

type Delays struct{ Target, All time.Duration }

func (d Delays) Normalize() Delays {
	if d.Target < 0 {
		d.Target = 0
	}
	if d.All < d.Target {
		d.All = d.Target
	}
	return d
}
func phaseAt(elapsed time.Duration, d Delays) int {
	if elapsed >= d.All {
		return phaseAll
	}
	if elapsed >= d.Target {
		return phaseTarget
	}
	return phaseGrace
}

type Item struct {
	Conversation string    `json:"conversation"`
	Message      string    `json:"message"`
	At           time.Time `json:"at"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	Kind         string    `json:"kind"`
	Badge        int       `json:"badge"`
	SenderNick   string    `json:"senderNick,omitempty"`
	SenderUser   string    `json:"senderUser,omitempty"`
	SenderHost   string    `json:"senderHost,omitempty"`
}

type Alert struct {
	item           Item
	Delivery       string `json:"delivery" enum:"sound,os"`
	Generation     string `json:"generation"`
	AlertID        string `json:"alertId" doc:"Stable device/message/conversation/generation identity shared by page and push."`
	Sound          bool   `json:"sound"`
	Preview        bool   `json:"preview"`
	ConversationID string `json:"conversationId"`
	MessageID      string `json:"messageId"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	Kind           string `json:"kind" enum:"mention,message"`
	Badge          int    `json:"badge"`
	Unread         int    `json:"unread" minimum:"1"`
	Conversations  int    `json:"conversations" minimum:"1"`
	Reminder       bool   `json:"reminder"`
}

type Target struct {
	Only, Except []string
	Fallback     bool // retained for push API compatibility; activity never enables fallback
	Valid        func(string) bool
	Done         func(string, bool)
}

func (t Target) Includes(id string) bool {
	for _, v := range t.Except {
		if v == id {
			return false
		}
	}
	if t.Only == nil {
		return true
	}
	for _, v := range t.Only {
		if v == id {
			return true
		}
	}
	return false
}

type Pusher interface {
	Route(string, Alert, Target, func())
	HasSubscription(string, string) bool
}
type Source interface {
	Delays(string) Delays
	Eligible(string, Item) bool
	AutoAwayDelay(string) time.Duration
	SetAutoAway(string, bool)
	SetDevice(string, string)
}
type Store interface {
	SaveInterval(string, []byte) error
	DeleteInterval(string) error
	LoadIntervals() (map[string][]byte, error)
}

// CandidateStore keeps all live eligible items on disk. The scheduler pages
// complete conversations and then their candidates, bounding work per tick
// without discarding a conversation when a large burst arrives.
type CandidateStore interface {
	PutCandidate(string, uint64, candidate) error
	CandidateGroups(string, uint64, string, int) ([]string, error)
	CandidatePage(string, uint64, string, string, int) ([]candidate, error)
	MarkCandidate(string, uint64, string, string, string) error
	CandidateAlerted(string, uint64, string, string, string) (bool, error)
	ClearCandidates(string, uint64) error
}

type Options struct {
	Push    Pusher
	Source  Source
	Store   Store
	Devices *storage.Store
	Now     func() time.Time
}
type tab struct {
	session             string
	expires, lastActive time.Time
	viewed              string
	incarnation         string
}
type session struct {
	lastActive      time.Time
	formFactor      string
	canAlert        bool
	subs            map[*Subscription]struct{}
	tabs            map[string]time.Time
	seen            time.Time
	revoked         bool
	capabilityAt    time.Time
	permission      string
	capabilityForm  string
	activeWrittenAt time.Time
}
type candidate struct {
	Item
	Alerted []string `json:"alerted,omitempty"`
}
type interval struct {
	Version      uint64      `json:"version,omitempty"`
	Generation   uint64      `json:"generation"`
	T0           time.Time   `json:"t0"`
	Preferred    string      `json:"preferred"`
	Phase        int         `json:"phase"`
	TargetCursor string      `json:"targetCursor,omitempty"`
	AllCursor    string      `json:"allCursor,omitempty"`
	TargetDone   bool        `json:"targetDone,omitempty"`
	AllDone      bool        `json:"allDone,omitempty"`
	Candidates   []candidate `json:"candidates,omitempty"` // only non-SQL test/embedding stores
}
type account struct {
	tabs               map[string]*tab
	sessions           map[string]*session
	interval           *interval
	focal              string
	revision           uint64
	generation         uint64
	lastRenewed        time.Time
	checkpointAt       time.Time
	activeCandidates   []candidate
	queued             map[string]bool
	serial             uint64
	persistenceBlocked bool
	sockets            int
	since              time.Time
	autoAway           bool
	device             string
	appliedAway        bool
	appliedDevice      string
}

type Subscription struct {
	C                                            <-chan Alert
	c                                            chan Alert
	Updates                                      <-chan RoutingSnapshot
	updates                                      chan RoutingSnapshot
	owner, session, tab, incarnation, connection string
	sequence                                     uint64
	seq                                          uint64
}
type Router struct {
	options            Options
	mu                 sync.Mutex
	accounts           map[string]*account
	generation, seq    uint64
	epochBase          uint64
	presenceMu         sync.Mutex
	persistMu, writeMu sync.Mutex
	pending            map[string][]byte
	pendingClear       map[string]uint64
	pendingPrune       map[string]uint64
	wake               chan struct{}
	stop, done         chan struct{}
}

func New(options Options) *Router {
	if options.Now == nil {
		options.Now = time.Now
	}
	r := &Router{options: options, accounts: map[string]*account{}, pending: map[string][]byte{}, pendingClear: map[string]uint64{}, pendingPrune: map[string]uint64{}, wake: make(chan struct{}, 1), epochBase: uint64(options.Now().UnixNano())}
	if options.Store != nil {
		if store, ok := options.Store.(interface {
			NextRoutingEpoch(time.Time) (uint64, error)
		}); ok {
			base, err := store.NextRoutingEpoch(options.Now())
			if err != nil {
				slog.Warn("Notification routing epoch could not be reserved", "error", err)
			} else {
				r.epochBase = base
			}
		}
		saved, err := options.Store.LoadIntervals()
		if err != nil {
			slog.Warn("Saved notification routing could not be loaded", "error", err)
		}
		for owner, raw := range saved {
			r.restore(owner, raw)
		}
	}
	return r
}
func (r *Router) SetSource(s Source)          { r.options.Source = s }
func (r *Router) SetDevices(s *storage.Store) { r.options.Devices = s }
func (r *Router) Start() {
	r.stop, r.done = make(chan struct{}), make(chan struct{})
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.stop:
				r.flush()
				return
			case <-r.wake:
				r.flush()
			case <-ticker.C:
				r.tick(r.options.Now())
				r.flush()
			}
		}
	}()
}
func (r *Router) Close() {
	if r.stop != nil {
		close(r.stop)
		<-r.done
		r.stop = nil
	}
	r.flush()
}
func (r *Router) Track(owner string) { r.mu.Lock(); r.account(owner); r.mu.Unlock() }
func (r *Router) Forget(owner string) {
	r.mu.Lock()
	if a := r.accounts[owner]; a != nil {
		r.clearCandidates(owner, a.generation)
	}
	delete(r.accounts, owner)
	r.queueLocked(owner, nil)
	r.mu.Unlock()
	r.flush()
}

func (r *Router) account(owner string) *account {
	a := r.accounts[owner]
	if a == nil {
		a = &account{tabs: map[string]*tab{}, sessions: map[string]*session{}, since: r.options.Now(), revision: r.epochBase}
		r.accounts[owner] = a
	}
	return a
}
func (a *account) session(id string, now time.Time) *session {
	s := a.sessions[id]
	if s == nil {
		s = &session{subs: map[*Subscription]struct{}{}, tabs: map[string]time.Time{}}
		a.sessions[id] = s
	}
	s.seen = now
	return s
}
func (a *account) active(now time.Time) bool {
	for _, t := range a.tabs {
		if t.session == a.focal && now.Before(t.expires) {
			return true
		}
	}
	return false
}
func (a *account) sessionActive(id string, now time.Time) bool {
	for _, t := range a.tabs {
		if t.session == id && now.Before(t.expires) {
			return true
		}
	}
	return false
}
func (a *account) viewed(conversation string, now time.Time) bool {
	for _, t := range a.tabs {
		if t.session == a.focal && now.Before(t.expires) && t.viewed == conversation {
			return true
		}
	}
	return false
}

func (r *Router) beginLocked(owner string, a *account, at time.Time) {
	if a.interval != nil {
		return
	}
	if a.generation == 0 {
		r.generation++
		a.generation = r.generation
	}
	a.interval = &interval{Generation: a.generation, T0: at, Preferred: a.focal, Phase: phaseGrace, Candidates: a.activeCandidates}
	a.activeCandidates = nil
	r.changedLocked(owner, a)
}
func (r *Router) expireLocked(owner string, a *account, now time.Time) {
	last := time.Time{}
	expired := false
	for key, t := range a.tabs {
		if !now.Before(t.expires) {
			expired = true
			if t.session == a.focal && t.lastActive.After(last) {
				last = t.lastActive
			}
			delete(a.tabs, key)
		}
	}
	if !last.IsZero() && !a.active(now) {
		r.beginLocked(owner, a, last)
	} else if expired {
		r.changedLocked(owner, a)
	}
}

// ReportState accepts only a report belonging to the newest live socket for
// this authenticated session/tab/incarnation. Delayed reports cannot revive it.
func (r *Router) ReportState(owner, id string, in Report) bool {
	now := r.options.Now()
	r.mu.Lock()
	a := r.account(owner)
	r.expireLocked(owner, a, now)
	s := a.sessions[id]
	var matched *Subscription
	if s != nil {
		for sub := range s.subs {
			if sub.tab == in.Tab && (matched == nil || sub.seq > matched.seq) {
				matched = sub
			}
		}
	}
	if matched == nil || s.revoked || matched.connection != in.ConnectionID || matched.incarnation != in.Incarnation || in.Sequence <= matched.sequence || in.Sequence == 0 {
		r.mu.Unlock()
		return false
	}
	if in.Signal != "engagement" && in.Signal != "renewal" && in.Signal != "inactive" {
		r.mu.Unlock()
		return false
	}
	matched.sequence = in.Sequence
	oldGeneration, oldInterval := a.generation, a.interval
	if in.Signal == "renewal" {
		if t := a.tabs[id+"\x00"+in.Tab]; t != nil && t.incarnation != in.Incarnation {
			in.Active = false
		}
	}
	r.reportLocked(owner, a, id, in.Tab, in.Active && in.Signal != "inactive", in.Signal == "engagement", in.ViewedConversationID, in.FormFactor, in.CanAlert, now)
	if t := a.tabs[id+"\x00"+in.Tab]; t != nil {
		t.incarnation = in.Incarnation
	}
	critical := oldGeneration != a.generation || oldInterval != a.interval
	writeCapabilities := r.options.Devices != nil && (s.capabilityAt.IsZero() || s.permission != in.Permission || s.capabilityForm != in.FormFactor || now.Sub(s.capabilityAt) >= 5*time.Minute || in.Signal == "engagement" && now.Sub(s.activeWrittenAt) >= time.Minute)
	if writeCapabilities {
		s.capabilityAt, s.permission, s.capabilityForm = now, in.Permission, in.FormFactor
		if in.Signal == "engagement" {
			s.activeWrittenAt = now
		}
	}
	r.mu.Unlock()
	// Capability diagnostics are not part of routing authorization. Keep this
	// SQL outside the account router lock; storage fences reordered writes by
	// the server's acceptance timestamp and still requires a live session.
	if critical {
		r.flush()
	}
	if writeCapabilities {
		if err := r.options.Devices.ReportDeviceCapabilities(owner, id, in.Permission, in.FormFactor, in.Signal == "engagement", now); err != nil {
			r.mu.Lock()
			if s.capabilityAt.Equal(now) {
				s.capabilityAt = time.Time{}
			}
			r.mu.Unlock()
		}
	}
	r.syncPresence(owner)
	return true
}

// Report is kept for internal callers and focused router tests. HTTP always
// uses ReportState and cannot bypass its socket/sequence fence.
func (r *Router) Report(owner, id, tabID string, active bool, form string, canAlert bool) {
	now := r.options.Now()
	r.mu.Lock()
	a := r.account(owner)
	r.expireLocked(owner, a, now)
	r.reportLocked(owner, a, id, tabID, active, active, "", form, canAlert, now)
	r.mu.Unlock()
	r.syncPresence(owner)
}
func (r *Router) reportLocked(owner string, a *account, id, tabID string, active, engaged bool, viewed, form string, canAlert bool, now time.Time) {
	s := a.session(id, now)
	wasActive := a.active(now)
	wasSessionActive := a.sessionActive(id, now)
	oldFocal, oldForm, oldCanAlert := a.focal, s.formFactor, s.canAlert
	if form == Desktop || form == Phone || form == Tablet {
		s.formFactor = form
	}
	s.canAlert = canAlert
	key := id + "\x00" + tabID
	oldTab := a.tabs[key]
	// A renewal cannot resurrect a suspended lease or cancel its inactivity
	// interval. Focus/open/input must report genuine engagement to do that.
	if active && !engaged && (oldTab == nil || !now.Before(oldTab.expires)) {
		active = false
	}
	if active {
		if engaged {
			newSelection := a.focal != id || !wasActive || a.interval != nil
			a.focal = id
			s.lastActive = now
			if _, ok := s.tabs[tabID]; !ok && len(s.tabs) >= MaxTabs {
				oldest := ""
				for tabID, seen := range s.tabs {
					if oldest == "" || seen.Before(s.tabs[oldest]) {
						oldest = tabID
					}
				}
				delete(s.tabs, oldest)
			}
			s.tabs[tabID] = now
			if newSelection {
				r.clearCandidates(owner, a.generation)
				a.activeCandidates = nil
				a.persistenceBlocked = false
				a.queued = map[string]bool{}
			}
			if a.interval != nil {
				a.interval = nil
			}
			if newSelection {
				r.generation++
				a.generation = r.generation
			}
		}
		if _, exists := a.tabs[key]; !exists && len(a.tabs) >= MaxTabs {
			oldest := ""
			for k, t := range a.tabs {
				if oldest == "" || t.expires.Before(a.tabs[oldest].expires) {
					oldest = k
				}
			}
			delete(a.tabs, oldest)
		}
		a.tabs[key] = &tab{session: id, expires: now.Add(LeaseTTL), lastActive: now, viewed: viewed}
		if id == a.focal {
			a.lastRenewed = now
		}
	} else {
		delete(a.tabs, key)
		if id == a.focal && !a.active(now) {
			r.beginLocked(owner, a, now)
		}
	}
	r.deviceLocked(a)
	changed := oldFocal != a.focal || wasActive != a.active(now) || wasSessionActive != a.sessionActive(id, now) || oldForm != s.formFactor || oldCanAlert != s.canAlert || active && (oldTab == nil || oldTab.viewed != viewed)
	if changed {
		r.changedLocked(owner, a)
	} else if id == a.focal && now.Sub(a.checkpointAt) >= checkpointEvery {
		r.persistLocked(owner, a)
	}
}

func (r *Router) EndSession(owner, id string) {
	now := r.options.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[owner]
	if a == nil {
		return
	}
	for key, t := range a.tabs {
		if t.session == id {
			delete(a.tabs, key)
		}
	}
	if s := a.sessions[id]; s != nil {
		s.revoked = true
		s.canAlert = false
		s.lastActive = time.Time{}
		s.subs = map[*Subscription]struct{}{}
	}
	if a.focal == id && !a.active(now) {
		r.beginLocked(owner, a, now)
	}
	// Keep the focal identity until new genuine engagement. Revocation removes
	// authorization, not the remaining devices' five-minute protection.
	r.changedLocked(owner, a)
}

func single(item Item) Alert {
	return Alert{ConversationID: item.Conversation, MessageID: item.Message, Title: item.Title, Body: item.Body, Kind: item.Kind, Badge: item.Badge, Unread: 1, Conversations: 1}
}

// recordLocked releases the global router mutex during durable I/O and fences
// the result against a handoff, revocation/deletion or newer generation.
func (r *Router) recordLocked(owner string, iv *interval, c candidate) bool {
	if store, ok := r.options.Store.(interface {
		PutCandidateState(string, uint64, candidate, []byte) error
	}); ok {
		a := r.accounts[owner]
		raw, err := r.routingDataLocked(a)
		if err == nil {
			r.mu.Unlock()
			err = store.PutCandidateState(owner, iv.Generation, c, raw)
			r.mu.Lock()
		}
		if r.accounts[owner] != a || a.generation != iv.Generation {
			return false
		}
		if err != nil {
			slog.Warn("Notification candidate and routing checkpoint could not be saved", "error", err)
			a.persistenceBlocked = true
			appendCandidate(iv, c)
		}
		return err == nil
	}
	if store, ok := r.options.Store.(CandidateStore); ok {
		a := r.accounts[owner]
		r.mu.Unlock()
		err := store.PutCandidate(owner, iv.Generation, c)
		r.mu.Lock()
		if r.accounts[owner] != a || a.generation != iv.Generation {
			return false
		}
		if err != nil {
			slog.Warn("Notification candidate could not be saved", "error", err)
			a.persistenceBlocked = true
			appendCandidate(iv, c)
		}
		return err == nil
	}
	appendCandidate(iv, c)
	return true
}
func appendCandidate(iv *interval, c candidate) {
	for _, existing := range iv.Candidates {
		if existing.Conversation == c.Conversation && existing.Message == c.Message {
			return
		}
	}
	iv.Candidates = append(iv.Candidates, c)
}
func (r *Router) clearCandidates(owner string, gen uint64) {
	if _, ok := r.options.Store.(CandidateStore); !ok {
		return
	}
	r.persistMu.Lock()
	if gen > r.pendingClear[owner] {
		r.pendingClear[owner] = gen
	}
	r.persistMu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// Notify is called after the workspace stored a live message. It does not call
// the workspace under the router lock. Quiet/read/policy rechecks happen again
// in scheduled work and in queued push delivery.
func (r *Router) Notify(owner string, item Item, delays Delays) {
	now := r.options.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.account(owner)
	r.expireLocked(owner, a, now)
	alert := single(item)
	if a.active(now) && a.viewed(item.Conversation, now) {
		return
	}
	if !a.active(now) && a.interval == nil {
		r.beginLocked(owner, a, now.Add(-delays.Normalize().All))
		a.interval.Phase = phaseAll
	}
	iv := a.interval
	activeRecord := iv == nil
	if activeRecord {
		iv = &interval{Generation: a.generation, Candidates: a.activeCandidates}
	}
	// A later live item reopens only its durable pending work. Existing receipts
	// keep earlier items quiet when the paced scheduler revisits the groups.
	iv.AllDone = false
	iv.Version++
	iv.AllCursor = ""
	iv.TargetDone = false
	iv.TargetCursor = ""
	saved := r.recordLocked(owner, iv, candidate{Item: item})
	if r.accounts[owner] != a || a.generation != iv.Generation {
		return
	}
	if activeRecord {
		if a.interval == nil {
			a.activeCandidates = iv.Candidates
		} else {
			a.interval.Version++
			for _, c := range iv.Candidates {
				appendCandidate(a.interval, c)
			}
		}
	}
	if !saved || a.persistenceBlocked {
		r.persistLocked(owner, a)
		return
	}
	now = r.options.Now()
	r.expireLocked(owner, a, now)
	if a.active(now) {
		if !a.viewed(item.Conversation, now) {
			r.deliverLocked(owner, a, alert, Target{Only: []string{a.focal}}, "sound", item)
		}
		return
	}
	iv = a.interval
	if iv == nil {
		return
	}
	switch phaseAt(now.Sub(iv.T0), delays.Normalize()) {
	case phaseGrace:
		r.deliverLocked(owner, a, alert, Target{Only: []string{iv.Preferred}}, "sound", item)
	case phaseTarget:
		r.deliverLocked(owner, a, alert, Target{Only: []string{iv.Preferred}}, "os", item)
	case phaseAll:
		r.deliverLocked(owner, a, alert, Target{}, "os", item)
	}
	r.persistLocked(owner, a)
}

func (r *Router) preferences(owner, id string) (storage.DeviceNotifications, bool) {
	if r.options.Devices == nil {
		return storage.DeviceNotifications{Initialized: true, Content: "inherit", Delivery: "push", Sound: "mentions", Preview: true, Permission: "granted"}, true
	}
	if !r.options.Devices.SessionActive(owner, id) {
		return storage.DeviceNotifications{}, false
	}
	d, err := r.options.Devices.NotificationDevice(owner, id)
	return d.Notifications, err == nil
}
func allows(p storage.DeviceNotifications, kind string) bool {
	return p.Initialized && p.Delivery != "off" && p.Content != "none" && (p.Content != "mentions" || kind == "mention")
}
func (r *Router) sessionIDs(owner string, a *account) []string {
	ids := []string{}
	if r.options.Devices != nil {
		sessions, err := r.options.Devices.DeviceSessions(owner)
		if err != nil {
			return ids
		}
		for _, s := range sessions {
			ids = append(ids, s.ID)
		}
		return ids
	}
	for id, s := range a.sessions {
		if s.revoked {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func alerted(c candidate, id string) bool {
	for _, v := range c.Alerted {
		if v == id {
			return true
		}
	}
	return false
}
func (r *Router) markLocked(owner string, a *account, item Item, id string) {
	if a.interval == nil {
		return
	}
	iv := a.interval
	if s, ok := r.options.Store.(CandidateStore); ok {
		if err := s.MarkCandidate(owner, iv.Generation, item.Conversation, item.Message, id); err != nil {
			slog.Warn("Notification receipt could not be saved", "error", err)
		}
		return
	}
	for i, c := range iv.Candidates {
		if c.Conversation == item.Conversation && c.Message == item.Message && !alerted(c, id) {
			iv.Candidates[i].Alerted = append(c.Alerted, id)
		}
	}
}
func alertIdentity(owner, id string, generation uint64, alert Alert) string {
	sum := sha256.Sum256([]byte(owner + "\x00" + id + "\x00" + strconv.FormatUint(generation, 10) + "\x00" + alert.ConversationID + "\x00" + alert.MessageID))
	return hex.EncodeToString(sum[:16])
}
func (r *Router) deliverLocked(owner string, a *account, base Alert, target Target, delivery string, item Item) bool {
	complete := true
	for _, id := range r.sessionIDs(owner, a) {
		if !target.Includes(id) {
			continue
		}
		if delivery == "os" && a.interval != nil {
			if store, ok := r.options.Store.(CandidateStore); ok {
				seen, err := store.CandidateAlerted(owner, a.generation, item.Conversation, item.Message, id)
				if err != nil || seen {
					continue
				}
			} else {
				seen := false
				for _, c := range a.interval.Candidates {
					if c.Conversation == item.Conversation && c.Message == item.Message && alerted(c, id) {
						seen = true
					}
				}
				if seen {
					continue
				}
			}
		}
		p, ok := r.preferences(owner, id)
		if !ok || !allows(p, base.Kind) {
			continue
		}
		s := a.sessions[id]
		var sub *Subscription
		if s != nil {
			sub = pick(s)
		}
		push := delivery == "os" && p.Delivery == "push" && p.Permission == "granted" && r.options.Push != nil && r.options.Push.HasSubscription(owner, id)
		if delivery == "sound" && sub == nil || delivery == "os" && !push && (sub == nil || !s.canAlert || p.Permission != "granted") {
			continue
		}
		alert := base
		alert.item = item
		alert.Delivery = delivery
		alert.Sound = p.Sound == "all" || p.Sound == "mentions" && base.Kind == "mention"
		alert.Preview = p.Preview
		alert.Generation = strconv.FormatUint(a.generation, 10)
		alert.AlertID = alertIdentity(owner, id, a.generation, alert)
		if delivery == "os" && a.queued[alert.AlertID] {
			complete = false
			continue
		}
		if !p.Preview {
			alert.Title = "IRCord"
			alert.Body = "New activity in a conversation."
		}
		if push {
			gen := a.generation
			sessionID := id
			t := Target{Only: []string{id}, Valid: func(check string) bool {
				return check == sessionID && r.validDelivery(owner, check, gen, item, alert.Sound, alert.Preview)
			}, Done: func(check string, delivered bool) {
				if check == sessionID {
					r.completeDelivery(owner, sessionID, gen, alert, item, delivered)
				}
			}}
			accepted := true
			if queue, ok := r.options.Push.(interface {
				TryRoute(string, Alert, Target) bool
			}); ok {
				accepted = queue.TryRoute(owner, alert, t)
			} else {
				r.options.Push.Route(owner, alert, t, nil)
			}
			if accepted {
				if receipts, ok := r.options.Push.(interface{ DeliveryReceipts() bool }); ok && receipts.DeliveryReceipts() {
					if a.queued == nil {
						a.queued = map[string]bool{}
					}
					a.queued[alert.AlertID] = true
					complete = false
				} else {
					r.markLocked(owner, a, item, id)
				}
			} else {
				complete = false
			}
		} else if sub != nil {
			select {
			case sub.c <- alert:
				if delivery == "os" {
					if a.queued == nil {
						a.queued = map[string]bool{}
					}
					a.queued[alert.AlertID] = true
					complete = false
				}
			default:
				if delivery == "os" {
					complete = false
				}
			}
		}
	}
	return complete
}

func (r *Router) completeDelivery(owner, id string, gen uint64, alert Alert, item Item, delivered bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[owner]
	if a == nil || a.generation != gen {
		return
	}
	delete(a.queued, alert.AlertID)
	if delivered {
		r.markLocked(owner, a, item, id)
	}
	if a.interval != nil {
		if a.interval.Phase == phaseAll {
			a.interval.AllDone = false
		} else {
			a.interval.TargetDone = false
		}
		r.persistLocked(owner, a)
	}
}

func (r *Router) CompleteAlert(sub *Subscription, alert Alert, delivered bool) {
	if alert.Delivery != "os" {
		return
	}
	gen, err := strconv.ParseUint(alert.Generation, 10, 64)
	if err == nil {
		r.completeDelivery(sub.owner, sub.session, gen, alert, alert.item, delivered)
	}
}
func (r *Router) validDelivery(owner, id string, gen uint64, item Item, sound, preview bool) bool {
	r.mu.Lock()
	a := r.accounts[owner]
	valid := a != nil && a.generation == gen
	p, ok := r.preferences(owner, id)
	valid = valid && ok && allows(p, item.Kind) && p.Delivery == "push" && p.Permission == "granted"
	valid = valid && p.Preview == preview && (p.Sound == "all" || p.Sound == "mentions" && item.Kind == "mention") == sound
	r.mu.Unlock()
	if !valid {
		return false
	}
	return r.options.Source == nil || r.options.Source.Eligible(owner, item)
}

// PrepareAlert fences a queued page alert at the socket writer too. Remote
// Off, revoked sessions, new engagement, reads and newly viewed conversations
// take effect even if an alert was already waiting in the bounded page queue.
func (r *Router) PrepareAlert(sub *Subscription, alert Alert) (Alert, bool) {
	r.mu.Lock()
	a := r.accounts[sub.owner]
	generation, err := strconv.ParseUint(alert.Generation, 10, 64)
	valid := err == nil && a != nil && a.generation == generation
	if valid {
		s := a.sessions[sub.session]
		valid = s != nil && !s.revoked && pick(s) == sub
	}
	p, ok := r.preferences(sub.owner, sub.session)
	valid = valid && ok && allows(p, alert.Kind)
	if valid && alert.Delivery == "sound" && a.active(r.options.Now()) && a.viewed(alert.ConversationID, r.options.Now()) {
		valid = false
	}
	if alert.Delivery == "os" {
		valid = valid && p.Permission == "granted"
	}
	alert.Sound = p.Sound == "all" || p.Sound == "mentions" && alert.Kind == "mention"
	alert.Preview = p.Preview
	if p.Preview {
		alert.Title, alert.Body = alert.item.Title, alert.item.Body
	} else {
		alert.Title, alert.Body = "IRCord", "New activity in a conversation."
	}
	r.mu.Unlock()
	if !valid {
		return alert, false
	}
	if r.options.Source != nil && !r.options.Source.Eligible(sub.owner, alert.item) {
		return alert, false
	}
	return alert, true
}

func pick(s *session) *Subscription {
	var best *Subscription
	var at time.Time
	newest := map[string]*Subscription{}
	for sub := range s.subs {
		if newest[sub.tab] == nil || sub.seq > newest[sub.tab].seq {
			newest[sub.tab] = sub
		}
	}
	for _, sub := range newest {
		seen := s.tabs[sub.tab]
		if best == nil || seen.After(at) || seen.Equal(at) && sub.seq < best.seq {
			best, at = sub, seen
		}
	}
	return best
}

func (r *Router) Subscribe(owner, id, tabID string) (*Subscription, func()) {
	return r.SubscribeClient(owner, id, tabID, "")
}
func (r *Router) SubscribeClient(owner, id, tabID, incarnation string) (*Subscription, func()) {
	ch := make(chan Alert, alertBuffer)
	updates := make(chan RoutingSnapshot, 1)
	token := make([]byte, 16)
	_, _ = rand.Read(token)
	now := r.options.Now()
	r.mu.Lock()
	r.seq++
	sub := &Subscription{C: ch, c: ch, Updates: updates, updates: updates, owner: owner, session: id, tab: tabID, incarnation: incarnation, connection: hex.EncodeToString(token), seq: r.seq}
	a := r.account(owner)
	a.session(id, now).subs[sub] = struct{}{}
	a.sockets++
	a.autoAway = false
	r.deviceLocked(a)
	r.changedLocked(owner, a)
	r.mu.Unlock()
	r.syncPresence(owner)
	var once sync.Once
	return sub, func() {
		once.Do(func() {
			r.mu.Lock()
			a := r.accounts[owner]
			if a == nil {
				r.mu.Unlock()
				return
			}
			if s := a.sessions[id]; s != nil {
				delete(s.subs, sub)
				s.seen = r.options.Now()
				same := false
				for other := range s.subs {
					if other.tab == tabID {
						same = true
					}
				}
				if !same {
					key := id + "\x00" + tabID
					if t := a.tabs[key]; t != nil {
						until := r.options.Now().Add(reconnectGrace)
						if until.Before(t.expires) {
							t.expires = until
						}
					}
				}
			}
			a.sockets--
			if a.sockets == 0 {
				a.since = r.options.Now()
			}
			r.deviceLocked(a)
			r.changedLocked(owner, a)
			r.mu.Unlock()
			r.syncPresence(owner)
		})
	}
}

func (r *Router) snapshotLocked(a *account, id, connection string) RoutingSnapshot {
	phase := "all"
	var since *time.Time
	if a.active(r.options.Now()) {
		phase = "active"
	} else if a.interval != nil {
		at := a.interval.T0
		since = &at
		switch a.interval.Phase {
		case phaseGrace:
			phase = "sound"
		case phaseTarget:
			phase = "target"
		case phaseAll:
			phase = "all"
		}
	}
	return RoutingSnapshot{Revision: strconv.FormatUint(a.revision, 10), DeviceID: DeviceID(id), FocalDeviceID: DeviceID(a.focal), Phase: phase, InactiveSince: since, ConnectionID: connection}
}
func (r *Router) Snapshot(owner, id string) RoutingSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.account(owner)
	r.expireLocked(owner, a, r.options.Now())
	return r.snapshotLocked(a, id, "")
}
func (r *Router) changedLocked(owner string, a *account) {
	a.revision++
	r.persistLocked(owner, a)
	for id, s := range a.sessions {
		for sub := range s.subs {
			snapshot := r.snapshotLocked(a, id, sub.connection)
			select {
			case <-sub.updates:
			default:
			}
			sub.updates <- snapshot
		}
	}
}
func (r *Router) Changed(owner string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.changedLocked(owner, r.account(owner))
}
func (r *Router) DeviceState(owner, id string) (focal, active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[owner]
	if a == nil {
		return
	}
	for sid := range a.sessions {
		if DeviceID(sid) == id {
			focal = sid == a.focal
			for _, t := range a.tabs {
				if t.session == sid && r.options.Now().Before(t.expires) {
					active = true
				}
			}
			return
		}
	}
	return id == DeviceID(a.focal), false
}
func (r *Router) Active(owner string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[owner]
	return a != nil && a.active(r.options.Now())
}

func (r *Router) groups(owner string, iv *interval, after string, limit int) ([]string, error) {
	if s, ok := r.options.Store.(CandidateStore); ok {
		return s.CandidateGroups(owner, iv.Generation, after, limit)
	}
	set := map[string]bool{}
	for _, c := range iv.Candidates {
		if c.Conversation > after {
			set[c.Conversation] = true
		}
	}
	out := []string{}
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (r *Router) candidatePage(owner string, iv *interval, conv, after string) ([]candidate, error) {
	if s, ok := r.options.Store.(CandidateStore); ok {
		return s.CandidatePage(owner, iv.Generation, conv, after, MaxCandidates)
	}
	out := []candidate{}
	for _, c := range iv.Candidates {
		if c.Conversation == conv && c.Message > after {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	if len(out) > MaxCandidates {
		out = out[:MaxCandidates]
	}
	return out, nil
}
func (r *Router) group(owner string, iv *interval, conv string, mentionsOnly bool) (candidate, int, error) {
	var best candidate
	count := 0
	after := ""
	for {
		page, err := r.candidatePage(owner, iv, conv, after)
		if err != nil {
			return best, count, err
		}
		for _, c := range page {
			after = c.Message
			if mentionsOnly && c.Kind != "mention" {
				continue
			}
			if r.options.Source != nil && !r.options.Source.Eligible(owner, c.Item) {
				continue
			}
			count++
			if best.Message == "" || c.At.After(best.At) || c.At.Equal(best.At) && c.Message > best.Message {
				best = c
			}
		}
		if len(page) < MaxCandidates {
			return best, count, nil
		}
	}
}

// Each tick sends at most eight complete conversation groups per account.
// The durable cursor advances only after its conversation has been processed.
func (r *Router) transition(owner string, delays Delays, now time.Time) {
	r.mu.Lock()
	a := r.accounts[owner]
	if a == nil || a.interval == nil || a.active(now) || a.persistenceBlocked {
		r.mu.Unlock()
		return
	}
	iv := a.interval
	want := phaseAt(now.Sub(iv.T0), delays)
	if want > iv.Phase {
		iv.Phase = want
		r.changedLocked(owner, a)
	}
	phase := iv.Phase
	if phase == phaseGrace || phase == phaseTarget && iv.TargetDone || phase == phaseAll && iv.AllDone {
		r.mu.Unlock()
		return
	}
	copyIV := *iv
	copyIV.Candidates = append([]candidate(nil), iv.Candidates...)
	after := iv.TargetCursor
	if phase == phaseAll {
		after = iv.AllCursor
	}
	r.mu.Unlock()
	groups, err := r.groups(owner, &copyIV, after, 8)
	if err != nil {
		return
	}
	for _, conv := range groups {
		best, count, err := r.group(owner, &copyIV, conv, false)
		if err != nil {
			return
		}
		mention, mentionCount, err := r.group(owner, &copyIV, conv, true)
		if err != nil {
			return
		}
		r.mu.Lock()
		a = r.accounts[owner]
		if a == nil || a.interval == nil || a.interval.Generation != copyIV.Generation || a.interval.Phase != phase || a.interval.Version != copyIV.Version || a.persistenceBlocked || a.active(r.options.Now()) {
			r.mu.Unlock()
			return
		}
		iv = a.interval
		complete := true
		for _, id := range r.sessionIDs(owner, a) {
			if phase == phaseTarget && id != iv.Preferred {
				continue
			}
			p, ok := r.preferences(owner, id)
			if !ok {
				continue
			}
			chosen, n := best, count
			if p.Content == "mentions" {
				chosen, n = mention, mentionCount
			}
			if n == 0 || alerted(chosen, id) {
				continue
			}
			alert := single(chosen.Item)
			alert.Unread = n
			alert.Reminder = n > 1
			if !r.deliverLocked(owner, a, alert, Target{Only: []string{id}}, "os", chosen.Item) {
				complete = false
			}
		}
		if !complete {
			r.persistLocked(owner, a)
			r.mu.Unlock()
			return
		}
		if phase == phaseTarget {
			iv.TargetCursor = conv
		} else {
			iv.AllCursor = conv
		}
		r.persistLocked(owner, a)
		r.mu.Unlock()
	}
	if len(groups) < 8 {
		r.mu.Lock()
		a = r.accounts[owner]
		if a != nil && a.interval != nil && a.interval.Generation == copyIV.Generation && a.interval.Phase == phase && a.interval.Version == copyIV.Version && !a.persistenceBlocked {
			if phase == phaseTarget {
				a.interval.TargetDone = true
			} else {
				a.interval.AllDone = true
				r.persistMu.Lock()
				r.pendingPrune[owner] = a.interval.Generation
				r.persistMu.Unlock()
				a.interval.Candidates = nil
			}
			r.persistLocked(owner, a)
		}
		r.mu.Unlock()
	}
}

func (r *Router) tick(now time.Time) {
	r.mu.Lock()
	owners, away, blocked := []string{}, []string{}, []string{}
	for owner, a := range r.accounts {
		if a.persistenceBlocked {
			blocked = append(blocked, owner)
		}
		r.expireLocked(owner, a, now)
		if a.interval != nil && !a.interval.AllDone {
			owners = append(owners, owner)
		}
		if a.sockets == 0 && !a.autoAway {
			away = append(away, owner)
		}
		for id, s := range a.sessions {
			if id != a.focal && len(s.subs) == 0 && !a.sessionActive(id, now) && now.Sub(s.seen) > sessionRetention {
				delete(a.sessions, id)
			}
		}
	}
	r.mu.Unlock()
	for _, owner := range blocked {
		r.retryCandidates(owner)
	}
	if r.options.Source == nil {
		return
	}
	sort.Strings(owners)
	for _, owner := range owners {
		r.transition(owner, r.options.Source.Delays(owner).Normalize(), now)
	}
	for _, owner := range away {
		delay := r.options.Source.AutoAwayDelay(owner)
		if delay <= 0 {
			continue
		}
		r.mu.Lock()
		a := r.accounts[owner]
		start := a != nil && a.sockets == 0 && !a.autoAway && now.Sub(a.since) >= delay
		if start {
			a.autoAway = true
		}
		r.mu.Unlock()
		if start {
			r.syncPresence(owner)
		}
	}
}

// A failed journal write remains in the checkpoint and prevents dispatch or
// cursor advancement. Retry outside the global mutex, even without a new
// incoming message, and drop the work only if genuine engagement supersedes it.
func (r *Router) retryCandidates(owner string) {
	r.mu.Lock()
	a := r.accounts[owner]
	if a == nil || !a.persistenceBlocked {
		r.mu.Unlock()
		return
	}
	gen := a.generation
	items := append([]candidate(nil), a.activeCandidates...)
	if a.interval != nil {
		items = append(items, a.interval.Candidates...)
	}
	raw, err := r.routingDataLocked(a)
	r.mu.Unlock()
	if err != nil {
		return
	}
	for _, c := range items {
		if store, ok := r.options.Store.(interface {
			PutCandidateState(string, uint64, candidate, []byte) error
		}); ok {
			err = store.PutCandidateState(owner, gen, c, raw)
		} else if store, ok := r.options.Store.(CandidateStore); ok {
			err = store.PutCandidate(owner, gen, c)
		}
		if err != nil {
			return
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.accounts[owner] != a || a.generation != gen {
		return
	}
	checked := map[string]bool{}
	for _, c := range items {
		checked[c.Conversation+"\x00"+c.Message] = true
	}
	keep := func(items []candidate) []candidate {
		out := items[:0:0]
		for _, c := range items {
			if !checked[c.Conversation+"\x00"+c.Message] {
				out = append(out, c)
			}
		}
		return out
	}
	a.activeCandidates = keep(a.activeCandidates)
	remaining := len(a.activeCandidates)
	if a.interval != nil {
		a.interval.Candidates = keep(a.interval.Candidates)
		a.interval.Version++
		remaining += len(a.interval.Candidates)
	}
	a.persistenceBlocked = remaining > 0
	r.persistLocked(owner, a)
}

// Existing public form-factor and automatic-AWAY behavior stays independent of
// private routing. No private names, preferences or identifiers reach IRC.
func (r *Router) deviceLocked(a *account) {
	best := ""
	var found *session
	for id, s := range a.sessions {
		if len(s.subs) == 0 {
			continue
		}
		if found == nil || s.lastActive.After(found.lastActive) || s.lastActive.Equal(found.lastActive) && id < best {
			best, found = id, s
		}
	}
	a.device = ""
	if found != nil && (found.formFactor == Phone || found.formFactor == Tablet) {
		a.device = found.formFactor
	}
}
func (r *Router) syncPresence(owner string) {
	if r.options.Source == nil {
		return
	}
	r.presenceMu.Lock()
	defer r.presenceMu.Unlock()
	r.mu.Lock()
	a := r.accounts[owner]
	if a == nil {
		r.mu.Unlock()
		return
	}
	away, device := a.autoAway, a.device
	setAway, setDevice := away != a.appliedAway, device != a.appliedDevice
	a.appliedAway, a.appliedDevice = away, device
	r.mu.Unlock()
	if setAway {
		r.options.Source.SetAutoAway(owner, away)
	}
	if setDevice {
		r.options.Source.SetDevice(owner, device)
	}
}
