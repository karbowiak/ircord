// Package activity routes an account's notifications by where its user is.
//
// Clients report activity over authenticated HTTP: a tab that is visible and focused holds a
// short lease and renews it while it stays that way; losing focus, hiding, closing or going
// idle ends the lease. An open socket is not activity, and neither are reconnects, incoming
// messages or push receipts. Activity is aggregated per account: while any lease is current,
// interrupting alerts (page notifications, sounds, Web Push) are held back on every device,
// while messages, unread counts and badges keep updating.
//
// When the last lease ends, an inactivity interval starts at T0. Within it:
//
//	elapsed < Target        grace: nothing interrupts; eligible items are kept as candidates.
//	Target <= elapsed < All the most recently active device alone is alerted. On entering this
//	                        phase it gets the latest still-eligible grace item once.
//	elapsed >= All          every device is alerted. On entering this phase, one reminder carries
//	                        the latest still-unread candidate and how many are unread.
//
// Transitions run on the backend's own clock (a ticker), so they happen without a new message
// or an open browser. Candidates are rechecked (read, deleted, muted, quiet hours) right
// before a transition alerts anyone. Each interval has a generation; any lease starting again
// ends the interval, so its pending transitions can never fire. An account with no interval
// (never active since start, or past the All phase) is alerted everywhere straight away.
//
// The same reports drive presence for #136: the account goes away on IRC once no device has
// had an event connection for its auto-away delay and comes back when one connects; and the
// form factor of the most recently active connected device (phone or tablet) is shared.
package activity

import (
	"sort"
	"sync"
	"time"
)

const (
	// DefaultTarget and DefaultAll are the account-wide escalation delays when none are set.
	DefaultTarget = 30 * time.Second
	DefaultAll    = 5 * time.Minute
	// LeaseTTL is how long an activity report keeps a tab active. Clients renew well inside
	// it; a crashed or suspended tab stops suppressing alerts within this time.
	LeaseTTL = 60 * time.Second
	// MaxTabs bounds the tabs tracked per account; MaxCandidates the items an interval keeps.
	MaxTabs       = 64
	MaxCandidates = 50
	// alertBuffer bounds each socket's queued page alerts; alerts beyond it are dropped.
	alertBuffer = 8
)

// Form factors a client may report.
const (
	Desktop = "desktop"
	Phone   = "phone"
	Tablet  = "tablet"
)

// Phases of an inactivity interval.
const (
	phaseGrace = iota
	phaseTarget
	phaseAll
)

// Delays are an account's effective escalation thresholds. All is never below Target.
type Delays struct{ Target, All time.Duration }

// Item is one notification-eligible message.
type Item struct {
	Conversation string    `json:"conversation"`
	Message      string    `json:"message"`
	At           time.Time `json:"at"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	// Kind is the notification kind from the account's rules: "mention" or "message".
	Kind  string `json:"kind"`
	Badge int    `json:"badge"`
}

// Alert is what a device is asked to show: one item, or a reminder carrying the latest of
// several unread items.
type Alert struct {
	ConversationID string `json:"conversationId"`
	MessageID      string `json:"messageId"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	Kind           string `json:"kind" enum:"mention,message"`
	Badge          int    `json:"badge"`
	// Unread is how many eligible items the reminder stands for; 1 for a single alert.
	Unread int `json:"unread" minimum:"1"`
	// Conversations is how many conversations those items span.
	Conversations int  `json:"conversations" minimum:"1"`
	Reminder      bool `json:"reminder"`
}

// Target selects sessions. Only nil means every session of the account.
type Target struct {
	Only   []string
	Except []string
	// Fallback asks the push sender to try every other session when none of Only succeeded.
	Fallback bool
}

// Pusher delivers Web Push and reports whether a session has a subscription.
type Pusher interface {
	Route(owner string, alert Alert, target Target)
	HasSubscription(owner, session string) bool
}

// Source answers questions only the IRC workspace knows. Its methods take workspace locks,
// so the router never calls them while holding its own.
type Source interface {
	Delays(owner string) Delays
	// Eligible reports whether an item still warrants an alert: unread, not deleted, and still
	// allowed by the conversation's level and quiet hours.
	Eligible(owner string, item Item) bool
	AutoAwayDelay(owner string) time.Duration
	SetAutoAway(owner string, away bool)
	SetDevice(owner, formFactor string)
}

// Store keeps interval state across restarts.
type Store interface {
	SaveInterval(owner string, data []byte) error
	DeleteInterval(owner string) error
	LoadIntervals() (map[string][]byte, error)
}

type Options struct {
	Push   Pusher
	Source Source
	Store  Store
	Now    func() time.Time
}

type tab struct {
	session    string
	expires    time.Time
	lastActive time.Time
}

type session struct {
	lastActive time.Time
	formFactor string
	subs       map[*Subscription]struct{}
	// tabs remembers when each tab of this browser was last active, also after its lease.
	tabs map[string]time.Time
}

type candidate struct {
	Item
	// Preferred marks an item the preferred device was already alerted about.
	Preferred bool `json:"preferred,omitempty"`
}

type interval struct {
	Generation uint64      `json:"generation"`
	T0         time.Time   `json:"t0"`
	Preferred  string      `json:"preferred"`
	Phase      int         `json:"phase"`
	Candidates []candidate `json:"candidates,omitempty"`
}

type account struct {
	tabs     map[string]*tab
	sessions map[string]*session
	interval *interval
	// sockets counts open event connections; since is when the count last became zero.
	sockets  int
	since    time.Time
	autoAway bool
	device   string
}

// Subscription receives the page alerts addressed to one event connection.
type Subscription struct {
	C       <-chan Alert
	c       chan Alert
	owner   string
	session string
	tab     string
	seq     uint64
}

type Router struct {
	options    Options
	mu         sync.Mutex
	accounts   map[string]*account
	generation uint64
	seq        uint64
	stop       chan struct{}
	done       chan struct{}
}

// New restores saved intervals. Start runs the scheduler; tests drive tick directly.
func New(options Options) *Router {
	if options.Now == nil {
		options.Now = time.Now
	}
	r := &Router{options: options, accounts: map[string]*account{}}
	if options.Store != nil {
		if saved, err := options.Store.LoadIntervals(); err == nil {
			for owner, raw := range saved {
				iv := decodeInterval(raw)
				if iv == nil {
					_ = options.Store.DeleteInterval(owner)
					continue
				}
				if iv.Generation > r.generation {
					r.generation = iv.Generation
				}
				r.account(owner).interval = iv
			}
		}
	}
	return r
}

// SetSource attaches the workspace source; call it before Start.
func (r *Router) SetSource(source Source) { r.options.Source = source }

// Start runs transitions once a second until Close.
func (r *Router) Start() {
	r.stop, r.done = make(chan struct{}), make(chan struct{})
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.stop:
				return
			case <-ticker.C:
				r.tick(r.options.Now())
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
}

// Track starts auto-away timing for an account that has no connection yet (after a restart).
func (r *Router) Track(owner string) {
	r.mu.Lock()
	r.account(owner)
	r.mu.Unlock()
}

// Forget drops an account's state, for account deletion.
func (r *Router) Forget(owner string) {
	r.mu.Lock()
	delete(r.accounts, owner)
	r.mu.Unlock()
	if r.options.Store != nil {
		_ = r.options.Store.DeleteInterval(owner)
	}
}

func (r *Router) account(owner string) *account {
	a := r.accounts[owner]
	if a == nil {
		a = &account{tabs: map[string]*tab{}, sessions: map[string]*session{}, since: r.options.Now()}
		r.accounts[owner] = a
	}
	return a
}

func (a *account) session(id string) *session {
	s := a.sessions[id]
	if s == nil {
		s = &session{subs: map[*Subscription]struct{}{}, tabs: map[string]time.Time{}}
		a.sessions[id] = s
	}
	return s
}

// active reports whether any lease is current at now.
func (a *account) active(now time.Time) bool {
	for _, t := range a.tabs {
		if now.Before(t.expires) {
			return true
		}
	}
	return false
}

// Report records a tab's activity. active renews its lease; false ends it. formFactor is a
// client hint ("phone", "tablet" or "desktop"); anything else is ignored.
func (r *Router) Report(owner, sessionID, tabID string, active bool, formFactor string) {
	now := r.options.Now()
	r.mu.Lock()
	a := r.account(owner)
	r.expireLocked(owner, a, now)
	s := a.session(sessionID)
	if formFactor == Desktop || formFactor == Phone || formFactor == Tablet {
		s.formFactor = formFactor
	}
	key := sessionID + "\x00" + tabID
	t := a.tabs[key]
	if active {
		if t == nil {
			if len(a.tabs) >= MaxTabs {
				r.dropOldestTabLocked(a)
			}
			t = &tab{session: sessionID}
			a.tabs[key] = t
		}
		t.expires, t.lastActive, s.lastActive = now.Add(LeaseTTL), now, now
		s.touchTab(tabID, now)
		if a.interval != nil {
			// Any client becoming active quiets every device and ends the interval.
			a.interval = nil
			r.persistLocked(owner, a)
		}
	} else if t != nil {
		delete(a.tabs, key)
		if len(a.tabs) == 0 {
			r.beginLocked(owner, a, now)
		}
	}
	device, changed := r.deviceLocked(a)
	r.mu.Unlock()
	if changed && r.options.Source != nil {
		r.options.Source.SetDevice(owner, device)
	}
}

func (s *session) touchTab(id string, at time.Time) {
	if _, ok := s.tabs[id]; !ok && len(s.tabs) >= MaxTabs {
		oldest := ""
		for key, seen := range s.tabs {
			if oldest == "" || seen.Before(s.tabs[oldest]) {
				oldest = key
			}
		}
		delete(s.tabs, oldest)
	}
	s.tabs[id] = at
}

func (r *Router) dropOldestTabLocked(a *account) {
	oldest := ""
	for key, t := range a.tabs {
		if oldest == "" || t.expires.Before(a.tabs[oldest].expires) {
			oldest = key
		}
	}
	delete(a.tabs, oldest)
}

// expireLocked removes ended leases and starts an interval when the last one ends. T0 is the
// last report of the tab that ended, so a client that vanished does not get extra grace.
func (r *Router) expireLocked(owner string, a *account, now time.Time) {
	var last time.Time
	for key, t := range a.tabs {
		if now.Before(t.expires) {
			continue
		}
		if t.lastActive.After(last) {
			last = t.lastActive
		}
		delete(a.tabs, key)
	}
	if !last.IsZero() && len(a.tabs) == 0 {
		r.beginLocked(owner, a, last)
	}
}

// beginLocked starts an inactivity interval at t0 with a fresh generation.
func (r *Router) beginLocked(owner string, a *account, t0 time.Time) {
	if a.interval != nil {
		return
	}
	r.generation++
	a.interval = &interval{Generation: r.generation, T0: t0, Preferred: preferred(a), Phase: phaseGrace}
	r.persistLocked(owner, a)
}

// preferred is the session most recently active; ties go to the smaller ID so the choice is
// deterministic.
func preferred(a *account) string {
	best := ""
	for id, s := range a.sessions {
		if s.lastActive.IsZero() {
			continue
		}
		if best == "" || s.lastActive.After(a.sessions[best].lastActive) || s.lastActive.Equal(a.sessions[best].lastActive) && id < best {
			best = id
		}
	}
	return best
}

func phaseAt(elapsed time.Duration, d Delays) int {
	switch {
	case elapsed >= d.All:
		return phaseAll
	case elapsed >= d.Target:
		return phaseTarget
	}
	return phaseGrace
}

// Normalize keeps delays non-negative with All at least Target.
func (d Delays) Normalize() Delays {
	if d.Target < 0 {
		d.Target = 0
	}
	if d.All < d.Target {
		d.All = d.Target
	}
	return d
}

type dispatch struct {
	alert  Alert
	target Target
}

// Notify routes one eligible message. It may be called with workspace locks held: it never
// calls back into the Source, and push and page deliveries only enqueue.
func (r *Router) Notify(owner string, item Item, delays Delays) {
	delays = delays.Normalize()
	now := r.options.Now()
	r.mu.Lock()
	a := r.account(owner)
	r.expireLocked(owner, a, now)
	var out []dispatch
	switch {
	case a.active(now):
		// Someone is using IRCord: nothing interrupts.
	case a.interval == nil:
		out = append(out, dispatch{single(item), Target{}})
	default:
		iv := a.interval
		want := phaseAt(now.Sub(iv.T0), delays)
		if want < iv.Phase {
			// A lengthened threshold never retracts what was already escalated.
			want = iv.Phase
		}
		switch want {
		case phaseGrace:
			iv.add(candidate{Item: item})
		case phaseTarget:
			// Entering the phase with a new message: it is the latest item, so it replaces the
			// grace catch-up instead of following it.
			iv.Phase = phaseTarget
			d, page := r.targetLocked(owner, a, single(item))
			out = append(out, d)
			iv.add(candidate{Item: item, Preferred: page || d.target.Only != nil})
		case phaseAll:
			alert := single(item)
			if iv.Phase < phaseAll {
				// The live message is the reminder: it is the latest unread item, and counting the
				// interval's candidates with it means the deadline cannot alert twice.
				alert = reminder(append(iv.Candidates, candidate{Item: item}))
			}
			iv.Phase = phaseAll
			out = append(out, dispatch{alert, Target{}})
			a.interval = nil
		}
		r.persistLocked(owner, a)
	}
	r.deliverLocked(owner, a, out)
	r.mu.Unlock()
}

func (iv *interval) add(c candidate) {
	iv.Candidates = append(iv.Candidates, c)
	if len(iv.Candidates) > MaxCandidates {
		iv.Candidates = append([]candidate(nil), iv.Candidates[len(iv.Candidates)-MaxCandidates:]...)
	}
}

func single(item Item) Alert {
	return Alert{ConversationID: item.Conversation, MessageID: item.Message, Title: item.Title, Body: item.Body, Kind: item.Kind, Badge: item.Badge, Unread: 1, Conversations: 1}
}

// reminder carries the latest of items, with their count and how many conversations they span.
func reminder(items []candidate) Alert {
	latest := items[0]
	conversations := map[string]bool{}
	for _, c := range items {
		conversations[c.Conversation] = true
		if !c.At.Before(latest.At) {
			latest = c
		}
	}
	alert := single(latest.Item)
	alert.Unread, alert.Conversations, alert.Reminder = len(items), len(conversations), len(items) > 1
	return alert
}

// targetLocked addresses the preferred device, or every device when it cannot be reached: no
// live page and no push subscription. The fallback never restarts the interval.
func (r *Router) targetLocked(owner string, a *account, alert Alert) (dispatch, bool) {
	pref := a.interval.Preferred
	s := a.sessions[pref]
	page := s != nil && len(s.subs) > 0
	if pref == "" && s == nil {
		return dispatch{alert, Target{}}, false
	}
	if !page && (r.options.Push == nil || !r.options.Push.HasSubscription(owner, pref)) {
		return dispatch{alert, Target{}}, false
	}
	return dispatch{alert, Target{Only: []string{pref}, Fallback: !page}}, page
}

// deliverLocked hands alerts to push and to one event connection per targeted session.
func (r *Router) deliverLocked(owner string, a *account, out []dispatch) {
	for _, d := range out {
		if r.options.Push != nil {
			r.options.Push.Route(owner, d.alert, d.target)
		}
		for id, s := range a.sessions {
			if !d.target.Includes(id) {
				continue
			}
			if sub := pick(s); sub != nil {
				select {
				case sub.c <- d.alert:
				default:
				}
			}
		}
	}
}

// Includes reports whether the target selects a session.
func (t Target) Includes(session string) bool {
	for _, id := range t.Except {
		if id == session {
			return false
		}
	}
	if t.Only == nil {
		return true
	}
	for _, id := range t.Only {
		if id == session {
			return true
		}
	}
	return false
}

// pick chooses the one connection of a browser that alerts, so its tabs make one sound: the
// most recently active tab with a connection, else the oldest connection.
func pick(s *session) *Subscription {
	var best *Subscription
	var bestActive time.Time
	for sub := range s.subs {
		at := s.tabs[sub.tab]
		if best == nil || at.After(bestActive) || at.Equal(bestActive) && sub.seq < best.seq {
			best, bestActive = sub, at
		}
	}
	return best
}

// Subscribe attaches an event connection. Connections are what auto-away watches: the first
// one ends an automatic away.
func (r *Router) Subscribe(owner, sessionID, tabID string) (*Subscription, func()) {
	ch := make(chan Alert, alertBuffer)
	r.mu.Lock()
	r.seq++
	sub := &Subscription{C: ch, c: ch, owner: owner, session: sessionID, tab: tabID, seq: r.seq}
	a := r.account(owner)
	a.session(sessionID).subs[sub] = struct{}{}
	a.sockets++
	back := a.autoAway
	a.autoAway = false
	device, changed := r.deviceLocked(a)
	r.mu.Unlock()
	if r.options.Source != nil {
		if back {
			r.options.Source.SetAutoAway(owner, false)
		}
		if changed {
			r.options.Source.SetDevice(owner, device)
		}
	}
	var once sync.Once
	return sub, func() {
		once.Do(func() {
			r.mu.Lock()
			a := r.accounts[owner]
			if a == nil {
				r.mu.Unlock()
				return
			}
			if s := a.sessions[sessionID]; s != nil {
				delete(s.subs, sub)
			}
			a.sockets--
			if a.sockets == 0 {
				a.since = r.options.Now()
			}
			device, changed := r.deviceLocked(a)
			r.mu.Unlock()
			if changed && r.options.Source != nil {
				r.options.Source.SetDevice(owner, device)
			}
		})
	}
}

// deviceLocked is the form factor to share: that of the most recently active session with a
// connection, when it is a phone or tablet. It reports whether that changed.
func (r *Router) deviceLocked(a *account) (string, bool) {
	best := ""
	var bestSession *session
	for id, s := range a.sessions {
		if len(s.subs) == 0 {
			continue
		}
		if bestSession == nil || s.lastActive.After(bestSession.lastActive) || s.lastActive.Equal(bestSession.lastActive) && id < best {
			best, bestSession = id, s
		}
	}
	device := ""
	if bestSession != nil && (bestSession.formFactor == Phone || bestSession.formFactor == Tablet) {
		device = bestSession.formFactor
	}
	if device == a.device {
		return device, false
	}
	a.device = device
	return device, true
}

type due struct {
	owner      string
	generation uint64
	phase      int
	candidates []candidate
}

// tick ends expired leases, runs due transitions and auto-away. Source calls happen without
// the router lock; work is revalidated against the interval generation before it is applied.
func (r *Router) tick(now time.Time) {
	r.mu.Lock()
	owners := []string{}
	awayCheck := []string{}
	for owner, a := range r.accounts {
		r.expireLocked(owner, a, now)
		if a.interval != nil && a.interval.Phase < phaseAll {
			owners = append(owners, owner)
		}
		if a.sockets == 0 && !a.autoAway {
			awayCheck = append(awayCheck, owner)
		}
	}
	r.mu.Unlock()
	sort.Strings(owners)
	if r.options.Source == nil {
		return
	}
	for _, owner := range owners {
		r.transition(owner, r.options.Source.Delays(owner).Normalize(), now)
	}
	for _, owner := range awayCheck {
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
			r.options.Source.SetAutoAway(owner, true)
		}
	}
}

func (r *Router) transition(owner string, delays Delays, now time.Time) {
	r.mu.Lock()
	a := r.accounts[owner]
	if a == nil || a.interval == nil {
		r.mu.Unlock()
		return
	}
	iv := a.interval
	want := phaseAt(now.Sub(iv.T0), delays)
	if want <= iv.Phase {
		r.mu.Unlock()
		return
	}
	work := due{owner, iv.Generation, iv.Phase, append([]candidate(nil), iv.Candidates...)}
	r.mu.Unlock()
	eligible := work.candidates[:0:0]
	for _, c := range work.candidates {
		if r.options.Source.Eligible(owner, c.Item) {
			eligible = append(eligible, c)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	a = r.accounts[owner]
	if a == nil || a.interval == nil || a.interval.Generation != work.generation || a.interval.Phase != work.phase || a.active(now) {
		return // Activity, a handoff or a live message moved the interval on meanwhile.
	}
	iv = a.interval
	// Keep only candidates that are still eligible, plus any that arrived during the check.
	kept := map[string]bool{}
	for _, c := range eligible {
		kept[c.Conversation+"\x00"+c.Message] = true
	}
	checked := map[string]bool{}
	for _, c := range work.candidates {
		checked[c.Conversation+"\x00"+c.Message] = true
	}
	current := iv.Candidates[:0:0]
	for _, c := range iv.Candidates {
		key := c.Conversation + "\x00" + c.Message
		if kept[key] || !checked[key] {
			current = append(current, c)
		}
	}
	iv.Candidates = current
	var out []dispatch
	if want == phaseAll {
		// One transition, whichever phase was skipped: one reminder or nothing.
		if len(current) > 0 {
			alert := reminder(current)
			target := Target{}
			for _, c := range current {
				if c.Conversation == alert.ConversationID && c.Message == alert.MessageID && c.Preferred {
					// The preferred device already alerted this exact item.
					target.Except = []string{iv.Preferred}
				}
			}
			out = append(out, dispatch{alert, target})
		}
		a.interval = nil
	} else {
		iv.Phase = phaseTarget
		if len(current) > 0 {
			latest := current[0]
			for _, c := range current {
				if !c.At.Before(latest.At) {
					latest = c
				}
			}
			d, page := r.targetLocked(owner, a, single(latest.Item))
			out = append(out, d)
			for i := range iv.Candidates {
				if iv.Candidates[i].Conversation == latest.Conversation && iv.Candidates[i].Message == latest.Message {
					iv.Candidates[i].Preferred = page || d.target.Only != nil
				}
			}
		}
	}
	r.persistLocked(owner, a)
	r.deliverLocked(owner, a, out)
}

// Active reports whether any client of the account currently holds an activity lease.
func (r *Router) Active(owner string) bool {
	now := r.options.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[owner]
	return a != nil && a.active(now)
}
