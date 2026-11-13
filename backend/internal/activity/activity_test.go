package activity

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"ircord/backend/internal/storage"
)

type pushed struct {
	owner  string
	alert  Alert
	target Target
}
type fakePush struct {
	mu   sync.Mutex
	sent []pushed
	subs map[string]bool
}

type pacedPush struct {
	*fakePush
	busy bool
}

type receiptPush struct{ *fakePush }

func (*receiptPush) DeliveryReceipts() bool { return true }

type failingCandidateStore struct {
	*SQLStore
	fail bool
}

func (s *failingCandidateStore) PutCandidate(owner string, gen uint64, c candidate) error {
	if s.fail {
		return fmt.Errorf("injected candidate write failure")
	}
	return s.SQLStore.PutCandidate(owner, gen, c)
}
func (s *failingCandidateStore) PutCandidateState(owner string, gen uint64, c candidate, raw []byte) error {
	if s.fail {
		return fmt.Errorf("injected candidate write failure")
	}
	return s.SQLStore.PutCandidateState(owner, gen, c, raw)
}

type blockedCandidateStore struct {
	*SQLStore
	entered, release chan struct{}
}

func (s *blockedCandidateStore) PutCandidateState(owner string, gen uint64, c candidate, raw []byte) error {
	close(s.entered)
	<-s.release
	return s.SQLStore.PutCandidateState(owner, gen, c, raw)
}

func (p *pacedPush) TryRoute(owner string, a Alert, target Target) bool {
	if p.busy {
		return false
	}
	p.Route(owner, a, target, nil)
	return true
}

func (p *fakePush) Route(owner string, a Alert, target Target, _ func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent = append(p.sent, pushed{owner, a, target})
}
func (p *fakePush) HasSubscription(owner, id string) bool { return p.subs[owner+"/"+id] }
func (p *fakePush) take() []pushed {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.sent
	p.sent = nil
	return out
}

type fakeSource struct {
	delays    Delays
	awayDelay time.Duration
	read      map[string]bool
	away      map[string][]bool
	devices   map[string][]string
}

type blockedPresenceSource struct {
	*fakeSource
	entered, release chan struct{}
}

func (s *blockedPresenceSource) SetAutoAway(owner string, away bool) {
	if away {
		close(s.entered)
		<-s.release
	}
	s.fakeSource.SetAutoAway(owner, away)
}

func (s *fakeSource) Delays(string) Delays               { return s.delays }
func (s *fakeSource) Eligible(owner string, i Item) bool { return !s.read[owner+"/"+i.Message] }
func (s *fakeSource) AutoAwayDelay(string) time.Duration { return s.awayDelay }
func (s *fakeSource) SetAutoAway(owner string, v bool)   { s.away[owner] = append(s.away[owner], v) }
func (s *fakeSource) SetDevice(owner, v string)          { s.devices[owner] = append(s.devices[owner], v) }

type memStore struct {
	rows   map[string][]byte
	writes int
}

func (s *memStore) SaveInterval(owner string, v []byte) error {
	s.rows[owner] = append([]byte(nil), v...)
	s.writes++
	return nil
}
func (s *memStore) DeleteInterval(owner string) error         { delete(s.rows, owner); return nil }
func (s *memStore) LoadIntervals() (map[string][]byte, error) { return s.rows, nil }

type harness struct {
	t          *testing.T
	start, now time.Time
	r          *Router
	push       *fakePush
	source     *fakeSource
	store      Store
	n          int
	sequence   map[*Subscription]uint64
}

func newHarness(t *testing.T) *harness {
	h := &harness{t: t, start: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), push: &fakePush{subs: map[string]bool{}}, source: &fakeSource{delays: Delays{DefaultTarget, DefaultAll}, awayDelay: 10 * time.Minute, read: map[string]bool{}, away: map[string][]bool{}, devices: map[string][]string{}}, store: &memStore{rows: map[string][]byte{}}, sequence: map[*Subscription]uint64{}}
	h.now = h.start
	h.restart()
	return h
}
func (h *harness) restart() {
	h.r = New(Options{Push: h.push, Source: h.source, Store: h.store, Now: func() time.Time { return h.now }})
}
func (h *harness) connect(owner, id, tabID string) *Subscription {
	h.t.Helper()
	sub, _ := h.r.SubscribeClient(owner, id, tabID, "page-"+tabID)
	h.push.subs[owner+"/"+id] = true
	return sub
}
func (h *harness) report(sub *Subscription, signal string, active bool, viewed string) {
	h.t.Helper()
	h.sequence[sub]++
	if !h.r.ReportState(sub.owner, sub.session, Report{Tab: sub.tab, Incarnation: sub.incarnation, ConnectionID: sub.connection, Sequence: h.sequence[sub], Signal: signal, Active: active, ViewedConversationID: viewed, FormFactor: Desktop, CanAlert: true, Permission: "granted"}) {
		h.t.Fatal("fresh report rejected")
	}
}
func (h *harness) at(d time.Duration) { h.now = h.start.Add(d); h.r.tick(h.now) }
func (h *harness) message(conv string) string {
	h.t.Helper()
	h.n++
	id := fmt.Sprintf("m%05d", h.n)
	h.r.Notify("alice", Item{Conversation: conv, Message: id, At: h.now, Title: "Bob", Body: id, Kind: "mention"}, h.source.delays)
	return id
}
func (h *harness) none() {
	h.t.Helper()
	if got := h.push.take(); len(got) > 0 {
		h.t.Fatalf("unexpected OS push: %+v", got)
	}
}
func drainAlerts(sub *Subscription) []Alert {
	out := []Alert{}
	for {
		select {
		case a := <-sub.C:
			out = append(out, a)
		default:
			return out
		}
	}
}

func TestFocalSoundAndThresholdCatchUpByConversation(t *testing.T) {
	h := newHarness(t)
	a := h.connect("alice", "A", "desktop")
	b := h.connect("alice", "B", "phone")
	h.report(a, "engagement", true, "viewed")
	h.message("viewed")
	h.message("elsewhere")
	h.message("room-two")
	got := drainAlerts(a)
	if len(got) != 2 || got[0].Delivery != "sound" || got[0].ConversationID != "elsewhere" || !got[0].Sound {
		t.Fatalf("active focal outside-view sound: %+v", got)
	}
	if len(drainAlerts(b)) > 0 {
		t.Fatal("other device interrupted")
	}
	h.none()
	h.report(a, "inactive", false, "")
	h.now = h.start.Add(10 * time.Second)
	h.message("dm")
	h.message("dm")
	for _, alert := range drainAlerts(a) {
		if alert.Delivery != "sound" {
			t.Fatal("grace emitted OS", alert)
		}
	}
	h.at(30*time.Second - time.Nanosecond)
	h.none()
	h.at(30 * time.Second)
	sent := h.push.take()
	if len(sent) != 3 {
		t.Fatalf("three conversation groups at target: %+v", sent)
	}
	for _, p := range sent {
		if len(p.target.Only) != 1 || p.target.Only[0] != "A" || p.target.Fallback || p.alert.Delivery != "os" || p.alert.Conversations != 1 {
			t.Fatalf("target routing: %+v", p)
		}
	}
	h.at(5*time.Minute - time.Nanosecond)
	h.none()
	h.at(5 * time.Minute)
	sent = h.push.take()
	if len(sent) != 3 {
		t.Fatalf("three catch-ups on other device: %+v", sent)
	}
	for _, p := range sent {
		if p.target.Only[0] != "B" {
			t.Fatalf("focal already OS-alerted: %+v", p)
		}
	}
	h.at(6 * time.Minute)
	h.none()
}

func TestRenewalNeverStealsFocalOrCancelsItsInactiveInterval(t *testing.T) {
	h := newHarness(t)
	desktop := h.connect("alice", "A", "desktop")
	phone := h.connect("alice", "B", "phone")
	h.report(desktop, "engagement", true, "old")
	h.now = h.start.Add(time.Second)
	h.report(phone, "engagement", true, "phone")
	h.report(desktop, "renewal", true, "old")
	if got := h.r.Snapshot("alice", "A"); got.FocalDeviceID != DeviceID("B") {
		t.Fatal("renewal stole selection", got)
	}
	h.now = h.start.Add(2 * time.Second)
	h.report(phone, "inactive", false, "")
	h.now = h.start.Add(10 * time.Second)
	h.report(desktop, "renewal", true, "old")
	h.message("pending")
	h.at(32 * time.Second)
	sent := h.push.take()
	if len(sent) != 1 || sent[0].target.Only[0] != "B" {
		t.Fatalf("visible desktop suppressed phone deadline: %+v", sent)
	}
	h.now = h.start.Add(40 * time.Second)
	h.report(desktop, "engagement", true, "new")
	generation := h.r.accounts["alice"].generation
	h.at(302 * time.Second)
	h.none()
	if h.r.accounts["alice"].generation != generation {
		t.Fatal("old escalation changed new generation")
	}
}

func TestConfiguredThresholdsUseOneInactivityOrigin(t *testing.T) {
	h := newHarness(t)
	h.source.delays = Delays{10 * time.Second, 40 * time.Second}
	a := h.connect("alice", "A", "tab")
	h.connect("alice", "B", "phone")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	h.now = h.start.Add(time.Second)
	h.message("pending")
	h.at(9 * time.Second)
	h.none()
	h.at(10 * time.Second)
	target := h.push.take()
	if len(target) != 1 || target[0].target.Only[0] != "A" {
		t.Fatal("configured target threshold", target)
	}
	h.at(39 * time.Second)
	h.none()
	h.at(40 * time.Second)
	all := h.push.take()
	if len(all) != 1 || all[0].target.Only[0] != "B" {
		t.Fatal("configured all threshold added an extra target delay", all)
	}
}

func TestPresenceStillSettlesOnTheLatestState(t *testing.T) {
	h := newHarness(t)
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, stop := h.r.Subscribe("alice", "A", "tab"); stop() }()
		go func() { defer wg.Done(); h.r.tick(h.start.Add(time.Hour)) }()
	}
	wg.Wait()
	h.r.mu.Lock()
	want := h.r.accounts["alice"].autoAway
	h.r.mu.Unlock()
	calls := h.source.away["alice"]
	if len(calls) > 0 && calls[len(calls)-1] != want {
		t.Fatal("existing presence ordering changed", calls, want)
	}
	for i := 1; i < len(calls); i++ {
		if calls[i] == calls[i-1] {
			t.Fatal("duplicate presence application", calls)
		}
	}
}

func TestSocketIncarnationSequenceAndAccountFences(t *testing.T) {
	h := newHarness(t)
	old := h.connect("alice", "A", "tab")
	h.report(old, "engagement", true, "one")
	current := h.connect("alice", "A", "tab")
	stale := Report{Tab: "tab", Incarnation: old.incarnation, ConnectionID: old.connection, Sequence: 100, Signal: "inactive", Permission: "granted"}
	if h.r.ReportState("alice", "A", stale) {
		t.Fatal("old socket accepted")
	}
	h.report(current, "engagement", true, "two")
	stale.ConnectionID = current.connection
	stale.Incarnation = current.incarnation
	stale.Sequence = 1
	if h.r.ReportState("alice", "A", stale) {
		t.Fatal("reordered report accepted")
	}
	stale.Sequence = 2
	if h.r.ReportState("bob", "A", stale) {
		t.Fatal("cross-account capability accepted")
	}
	stale.Incarnation = "retired-page"
	if h.r.ReportState("alice", "A", stale) {
		t.Fatal("old page accepted")
	}
	h.message("outside-view")
	if len(drainAlerts(old)) != 0 || len(drainAlerts(current)) != 1 {
		t.Fatal("superseded socket received an alert")
	}
}

func TestQueuedPushGenerationAndReadRecheck(t *testing.T) {
	h := newHarness(t)
	a := h.connect("alice", "A", "tab")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	h.now = h.start.Add(time.Minute)
	message := h.message("pending")
	sent := h.push.take()
	if len(sent) != 1 || !sent[0].target.Valid("A") {
		t.Fatal("new push not eligible", sent)
	}
	h.source.read["alice/"+message] = true
	if sent[0].target.Valid("A") {
		t.Fatal("queued push ignored cross-device read")
	}
	delete(h.source.read, "alice/"+message)
	h.report(a, "engagement", true, "")
	if sent[0].target.Valid("A") {
		t.Fatal("new engagement did not cancel queued generation")
	}
}

func TestPageQueueRechecksReadViewedConversationAndGeneration(t *testing.T) {
	h := newHarness(t)
	a := h.connect("alice", "A", "tab")
	h.report(a, "engagement", true, "viewed")
	id := h.message("other")
	pending := drainAlerts(a)
	if len(pending) != 1 {
		t.Fatal(pending)
	}
	h.source.read["alice/"+id] = true
	if _, valid := h.r.PrepareAlert(a, pending[0]); valid {
		t.Fatal("queued page alert ignored a read")
	}
	h.message("other")
	pending = drainAlerts(a)
	h.report(a, "engagement", true, "other")
	if _, valid := h.r.PrepareAlert(a, pending[0]); valid {
		t.Fatal("queued sound interrupted the newly viewed conversation")
	}
	h.message("third")
	pending = drainAlerts(a)
	h.report(a, "inactive", false, "")
	h.report(a, "engagement", true, "viewed")
	if _, valid := h.r.PrepareAlert(a, pending[0]); valid {
		t.Fatal("old page alert survived new engagement generation")
	}
}

func TestBackpressureKeepsWholePendingGroupUntilRetry(t *testing.T) {
	h := newHarness(t)
	queue := &pacedPush{fakePush: h.push, busy: true}
	h.r.options.Push = queue
	a := h.connect("alice", "A", "tab")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	h.now = h.start.Add(time.Second)
	h.message("one")
	h.message("one")
	h.message("two")
	h.at(30 * time.Second)
	h.none()
	if h.r.accounts["alice"].interval.TargetCursor != "" {
		t.Fatal("full queue advanced durable group cursor")
	}
	queue.busy = false
	h.at(31 * time.Second)
	sent := h.push.take()
	if len(sent) != 2 {
		t.Fatal("queue backpressure dropped a conversation", sent)
	}
	for _, p := range sent {
		if p.alert.ConversationID == "one" && p.alert.Unread != 2 {
			t.Fatal("partial conversation group", p)
		}
	}
}

func TestReadingRenewsWithoutInputAndWritesAreBounded(t *testing.T) {
	h := newHarness(t)
	a := h.connect("alice", "A", "tab")
	h.report(a, "engagement", true, "reading")
	h.r.flush()
	saved := h.store.(*memStore).writes
	generation := h.r.accounts["alice"].generation
	for i := 1; i <= 20; i++ {
		h.now = h.start.Add(time.Duration(i) * time.Second)
		h.report(a, "engagement", true, "reading")
		h.r.flush()
	}
	if h.store.(*memStore).writes != saved || h.r.accounts["alice"].generation != generation {
		t.Fatal("same-device input rewrote routing or generation")
	}
	for i := 1; i <= 60; i++ {
		h.now = h.start.Add(time.Duration(i) * 25 * time.Second)
		h.report(a, "renewal", true, "reading")
		h.r.tick(h.now)
	}
	if !h.r.Active("alice") {
		t.Fatal("passive reading expired despite renewals")
	}
	h.r.flush()
	h.at(26 * time.Minute)
	if h.r.Active("alice") {
		t.Fatal("suspended lease stayed active")
	}
	if h.r.accounts["alice"].interval.T0 != h.start.Add(25*time.Minute) {
		t.Fatal("expiry did not use last renewal")
	}
}

func TestGroupedReadRecheckPreservesOtherConversationsAndPaces(t *testing.T) {
	h := newHarness(t)
	a := h.connect("alice", "A", "tab")
	h.connect("alice", "B", "phone")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	h.now = h.start.Add(time.Second)
	read := h.message("read")
	for i := 0; i < 70; i++ {
		h.message(fmt.Sprintf("c%03d", i))
	}
	h.source.read["alice/"+read] = true
	total := 0
	for i := 0; i < 10; i++ {
		h.at(30*time.Second + time.Duration(i)*time.Second)
		sent := h.push.take()
		if len(sent) > 8 {
			t.Fatal("unpaced group burst", len(sent))
		}
		for _, p := range sent {
			if p.alert.ConversationID == "read" {
				t.Fatal("read conversation escalated")
			}
		}
		total += len(sent)
	}
	if total != 70 {
		t.Fatalf("conversation groups were truncated: %d", total)
	}
	total = 0
	for i := 0; i < 10; i++ {
		h.at(5*time.Minute + time.Duration(i)*time.Second)
		for _, p := range h.push.take() {
			if p.target.Only[0] != "B" {
				t.Fatal("duplicate focal OS", p)
			}
			total++
		}
	}
	if total != 70 {
		t.Fatalf("all-device groups were truncated: %d", total)
	}
}

func TestDurableRestartDeadlineAndDeduplication(t *testing.T) {
	h := newHarness(t)
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.CreateUser("alice", "alice", "hash"); err != nil {
		t.Fatal(err)
	}
	store, err := NewSQLStore(db.DB)
	if err != nil {
		t.Fatal(err)
	}
	h.store = store
	h.restart()
	a := h.connect("alice", "A", "tab")
	h.connect("alice", "B", "phone")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	h.now = h.start.Add(time.Second)
	first := h.message("one")
	latest := h.message("one")
	h.message("two")
	h.at(30 * time.Second)
	sent := h.push.take()
	if len(sent) != 2 {
		t.Fatal("target groups", sent)
	}
	h.source.read["alice/"+latest] = true
	h.r.flush()
	h.restart()
	h.connect("alice", "A", "tab2")
	h.connect("alice", "B", "phone2")
	h.at(299 * time.Second)
	h.none()
	h.at(300 * time.Second)
	sent = h.push.take()
	if len(sent) != 3 {
		t.Fatalf("older unread to both, unchanged group to B: %+v", sent)
	}
	for _, p := range sent {
		if p.alert.ConversationID == "one" && p.alert.MessageID != first {
			t.Fatal("read latest replaced unread", p)
		}
	}
	h.r.flush()
	h.restart()
	h.connect("alice", "A", "tab3")
	h.connect("alice", "B", "phone3")
	h.at(301 * time.Second)
	h.none()
}

func TestQueuedDeliverySurvivesRestartBeforeTransportReceipt(t *testing.T) {
	h := newHarness(t)
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.CreateUser("alice", "alice", "hash"); err != nil {
		t.Fatal(err)
	}
	store, err := NewSQLStore(db.DB)
	if err != nil {
		t.Fatal(err)
	}
	h.store = store
	h.restart()
	h.r.options.Push = &receiptPush{h.push}
	a := h.connect("alice", "A", "tab")
	h.connect("alice", "B", "phone")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	h.now = h.start.Add(time.Second)
	h.message("pending")
	h.at(30 * time.Second)
	first := h.push.take()
	if len(first) != 1 {
		t.Fatal(first)
	}
	// No flush or transport callback: the candidate and its routing checkpoint
	// were committed together before it entered the process-local queue.
	h.restart()
	h.r.options.Push = &receiptPush{h.push}
	h.connect("alice", "A", "new-tab")
	h.connect("alice", "B", "new-phone")
	h.at(31 * time.Second)
	retry := h.push.take()
	if len(retry) != 1 || retry[0].alert.AlertID != first[0].alert.AlertID {
		t.Fatal("queued alert lost or identity changed on restart", retry)
	}
	retry[0].target.Done("A", false)
	h.at(32 * time.Second)
	retry = h.push.take()
	if len(retry) != 1 {
		t.Fatal("failed transport was treated as delivered", retry)
	}
	retry[0].target.Done("A", true)
	h.at(33 * time.Second)
	h.none()
	h.at(5 * time.Minute)
	other := h.push.take()
	if len(other) != 1 || other[0].target.Only[0] != "B" {
		t.Fatal("catch-up did not skip confirmed focal receipt", other)
	}
	other[0].target.Done("B", true)
	h.at(301 * time.Second)
	h.r.flush()
	h.restart()
	h.connect("alice", "A", "last-tab")
	h.connect("alice", "B", "last-phone")
	h.at(302 * time.Second)
	h.none()
}

func TestCandidateWriteFailureRetainsGroupAndRetriesAfterRestart(t *testing.T) {
	h := newHarness(t)
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.CreateUser("alice", "alice", "hash"); err != nil {
		t.Fatal(err)
	}
	sqlStore, err := NewSQLStore(db.DB)
	if err != nil {
		t.Fatal(err)
	}
	store := &failingCandidateStore{SQLStore: sqlStore}
	h.store = store
	h.restart()
	a := h.connect("alice", "A", "tab")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	store.fail = true
	h.now = h.start.Add(time.Second)
	h.message("pending")
	h.at(30 * time.Second)
	h.none()
	h.r.flush()
	h.restart()
	h.connect("alice", "A", "new-tab")
	h.at(31 * time.Second)
	h.none()
	if !h.r.accounts["alice"].persistenceBlocked || h.r.accounts["alice"].interval.TargetCursor != "" {
		t.Fatal("failed persistence advanced the group")
	}
	store.fail = false
	h.at(32 * time.Second)
	sent := h.push.take()
	if len(sent) != 1 || sent[0].alert.ConversationID != "pending" {
		t.Fatal("retained group did not retry", sent)
	}
}

func TestCandidateTransactionDoesNotBlockRouterAndFencesHandoff(t *testing.T) {
	h := newHarness(t)
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.CreateUser("alice", "alice", "hash"); err != nil {
		t.Fatal(err)
	}
	sqlStore, err := NewSQLStore(db.DB)
	if err != nil {
		t.Fatal(err)
	}
	store := &blockedCandidateStore{SQLStore: sqlStore, entered: make(chan struct{}), release: make(chan struct{})}
	h.store = store
	h.restart()
	a := h.connect("alice", "A", "tab")
	b := h.connect("alice", "B", "phone")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	done := make(chan struct{})
	go func() {
		h.r.Notify("alice", Item{Conversation: "pending", Message: "blocked", At: h.now, Kind: "mention"}, h.source.delays)
		close(done)
	}()
	<-store.entered
	other := make(chan struct{})
	go func() { h.r.Snapshot("bob", "C"); close(other) }()
	select {
	case <-other:
	case <-time.After(2 * time.Second):
		t.Fatal("candidate SQL held global router mutex")
	}
	h.report(b, "engagement", true, "phone")
	close(store.release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("candidate transaction did not finish")
	}
	h.none()
	var count int
	if err = db.DB.QueryRow(`SELECT count(*) FROM notification_candidates WHERE owner='alice'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("old transaction resurrected a cancelled candidate", count, err)
	}
}

func TestRoutingRevisionEpochSurvivesLostCheckpointAndClockRollback(t *testing.T) {
	h := newHarness(t)
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewSQLStore(db.DB)
	if err != nil {
		t.Fatal(err)
	}
	h.store = store
	h.restart()
	h.connect("alice", "A", "tab")
	for i := 0; i < 100; i++ {
		h.r.Changed("alice")
	}
	before, _ := strconv.ParseUint(h.r.Snapshot("alice", "A").Revision, 10, 64)
	// Do not flush the latest revisions, and move the wall clock backwards.
	h.now = h.start.Add(-time.Hour)
	h.restart()
	after, _ := strconv.ParseUint(h.r.Snapshot("alice", "A").Revision, 10, 64)
	if after <= before {
		t.Fatal("restart reused an already published revision", before, after)
	}
}

func TestUnavailableOrRevokedFocalNeverFallsBackEarly(t *testing.T) {
	h := newHarness(t)
	a := h.connect("alice", "A", "tab")
	h.connect("alice", "B", "phone")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	h.push.subs["alice/A"] = false
	h.r.accounts["alice"].sessions["A"].canAlert = false
	h.now = h.start.Add(time.Minute)
	h.message("pending")
	h.at(2 * time.Minute)
	h.none()
	h.r.EndSession("alice", "A")
	h.at(4 * time.Minute)
	h.none()
	h.at(5 * time.Minute)
	sent := h.push.take()
	if len(sent) != 1 || sent[0].target.Only[0] != "B" {
		t.Fatal("revocation/fallback", sent)
	}
	bob := h.connect("bob", "C", "other")
	h.report(bob, "engagement", true, "dm")
	h.r.Notify("bob", Item{Conversation: "other", Message: "b1", At: h.now, Kind: "mention"}, h.source.delays)
	if len(drainAlerts(bob)) != 1 {
		t.Fatal("account routing isolation")
	}
}

func TestClosingOneTabKeepsOtherTabLeaseAndPresence(t *testing.T) {
	h := newHarness(t)
	a, stopA := h.r.SubscribeClient("alice", "A", "one", "page-one")
	b, stopB := h.r.SubscribeClient("alice", "A", "two", "page-two")
	h.report(a, "engagement", true, "one")
	h.report(b, "engagement", true, "two")
	stopA()
	if !h.r.Active("alice") || h.r.accounts["alice"].sockets != 1 {
		t.Fatal("closing one tab disconnected device")
	}
	stopB()
	h.at(reconnectGrace)
	if h.r.Active("alice") {
		t.Fatal("last socket kept activity past reconnect grace")
	}
	h.at(30 * time.Second)
	if h.r.accounts["alice"].interval.Phase != phaseTarget {
		t.Fatal("disconnect skipped interval")
	}
	h.at(10*time.Minute + reconnectGrace)
	if got := h.source.away["alice"]; len(got) != 1 || !got[0] {
		t.Fatal("existing auto-away changed", got)
	}
}

func TestReconnectRenewalPreservesLeaseWithoutSelection(t *testing.T) {
	h := newHarness(t)
	desktop, stop := h.r.SubscribeClient("alice", "A", "desktop", "same-page")
	h.push.subs["alice/A"] = true
	phone := h.connect("alice", "B", "phone")
	h.report(desktop, "engagement", true, "desktop")
	h.now = h.start.Add(time.Second)
	h.report(phone, "engagement", true, "phone")
	stop()
	h.now = h.start.Add(2 * time.Second)
	reconnected, _ := h.r.SubscribeClient("alice", "A", "desktop", "same-page")
	h.report(reconnected, "renewal", true, "desktop")
	if got := h.r.Snapshot("alice", "A"); got.FocalDeviceID != DeviceID("B") {
		t.Fatal("reconnect stole focal", got)
	}
	h.report(phone, "inactive", false, "")
	h.at(32 * time.Second)
	if h.r.Active("alice") {
		t.Fatal("nonfocal reconnect suppressed inactive focal")
	}
}

func TestDeviceContentNarrowingAndRemoteOffWhileAsleep(t *testing.T) {
	h := newHarness(t)
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.CreateUser("alice", "alice", "hash"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"A", "B"} {
		if err = db.CreateVerifiedSession(id, "alice", []byte(id), time.Now().Add(time.Hour), "hash", id); err != nil {
			t.Fatal(err)
		}
		if err = db.SetDevicePreferences("alice", id, storage.DevicePreferences{Content: "inherit", Delivery: "push", Sound: "mentions", Preview: true}); err != nil {
			t.Fatal(err)
		}
		if err = db.ReportDeviceCapabilities("alice", id, "granted", Desktop, false, h.now); err != nil {
			t.Fatal(err)
		}
	}
	h.r.SetDevices(db)
	a := h.connect("alice", "A", "tab")
	h.connect("alice", "B", "phone")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	if err = db.SetDevicePreferences("alice", "A", storage.DevicePreferences{Revision: 1, Content: "mentions", Delivery: "push", Sound: "mentions", Preview: true}); err != nil {
		t.Fatal(err)
	}
	h.now = h.start.Add(time.Second)
	mention := h.message("channel")
	h.now = h.start.Add(2 * time.Second)
	h.r.Notify("alice", Item{Conversation: "channel", Message: "plain", At: h.now, Title: "Later plain", Kind: "message"}, h.source.delays)
	h.at(30 * time.Second)
	sent := h.push.take()
	if len(sent) != 1 || sent[0].alert.MessageID != mention || sent[0].alert.Unread != 1 {
		t.Fatal("later plain replaced qualifying device mention", sent)
	}
	if !sent[0].target.Valid("A") {
		t.Fatal("fresh queue target invalid")
	}
	if err = db.SetDevicePreferences("alice", "B", storage.DevicePreferences{Revision: 1, Content: "inherit", Delivery: "off", Sound: "mentions", Preview: true}); err != nil {
		t.Fatal(err)
	}
	h.at(5 * time.Minute)
	h.none()
	if err = db.SetDevicePreferences("alice", "A", storage.DevicePreferences{Revision: 2, Content: "mentions", Delivery: "off", Sound: "mentions", Preview: true}); err != nil {
		t.Fatal(err)
	}
	if sent[0].target.Valid("A") {
		t.Fatal("remote Off did not cancel queued push")
	}
	pending := drainAlerts(a)
	if len(pending) == 0 {
		t.Fatal("missing queued grace sound")
	}
	if _, valid := h.r.PrepareAlert(a, pending[0]); valid {
		t.Fatal("remote Off did not cancel a queued page alert")
	}
}

func TestCapabilityDiagnosticsSkipUnchangedReports(t *testing.T) {
	h := newHarness(t)
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.CreateUser("alice", "alice", "hash"); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateVerifiedSession("A", "alice", []byte("A"), time.Now().Add(time.Hour), "hash", "A"); err != nil {
		t.Fatal(err)
	}
	h.r.SetDevices(db)
	a := h.connect("alice", "A", "tab")
	h.report(a, "engagement", true, "reading")
	var before, after int
	if err = db.DB.QueryRow("SELECT total_changes()").Scan(&before); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 20; i++ {
		h.now = h.start.Add(time.Duration(i) * time.Second)
		h.report(a, "engagement", true, "reading")
	}
	if err = db.DB.QueryRow("SELECT total_changes()").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("unchanged input caused %d database row changes", after-before)
	}
	h.now = h.start.Add(time.Minute)
	h.report(a, "engagement", true, "reading")
	d, err := db.NotificationDevice("alice", "A")
	if err != nil || d.LastActiveAt == nil || !d.LastActiveAt.Equal(h.now) {
		t.Fatal("bounded active timestamp", d, err)
	}
}

func TestAutoAwayFollowsAllActiveLeases(t *testing.T) {
	h := newHarness(t)
	h.source.awayDelay = 30 * time.Second
	a := h.connect("alice", "A", "desktop")
	b := h.connect("alice", "B", "phone")
	h.report(a, "engagement", true, "")
	h.report(b, "engagement", true, "")
	// The non-focal desktop remains active while the focal phone is inactive.
	h.report(b, "inactive", false, "")
	for second := 10; second <= 100; second += 10 {
		h.now = h.start.Add(time.Duration(second) * time.Second)
		h.report(a, "renewal", true, "")
		h.r.tick(h.now)
	}
	if got := h.source.away["alice"]; len(got) != 0 {
		t.Fatal("active reader did not prevent away", got)
	}
	h.report(a, "inactive", false, "")
	h.at(129 * time.Second)
	if got := h.source.away["alice"]; len(got) != 0 {
		t.Fatal("away started before the grace elapsed", got)
	}
	h.at(130 * time.Second)
	if got := h.source.away["alice"]; len(got) != 1 || !got[0] {
		t.Fatal("inactive sockets did not become away", got)
	}
	newTab := h.connect("alice", "C", "new")
	h.report(newTab, "inactive", false, "")
	if got := h.source.away["alice"]; len(got) != 1 {
		t.Fatal("inactive reconnect cleared away", got)
	}
	h.report(newTab, "engagement", true, "")
	if got := h.source.away["alice"]; len(got) != 2 || got[1] {
		t.Fatal("returning active client did not clear away immediately", got)
	}
}

func TestAutoAwayStartsAtLeaseExpiryWithOpenSocket(t *testing.T) {
	h := newHarness(t)
	h.source.awayDelay = 30 * time.Second
	a := h.connect("alice", "A", "phone")
	h.report(a, "engagement", true, "")
	h.at(89 * time.Second)
	if got := h.source.away["alice"]; len(got) != 0 {
		t.Fatal("expiry grace began before lease expiry", got)
	}
	h.at(90 * time.Second)
	if got := h.source.away["alice"]; len(got) != 1 || !got[0] {
		t.Fatal("expired open socket did not become away", got)
	}
	// A renewal cannot revive an expired lease.
	h.report(a, "renewal", true, "")
	if got := h.source.away["alice"]; len(got) != 1 {
		t.Fatal("expired renewal cleared away", got)
	}
}

func TestAutoAwaySessionRevocationAndAccountIsolation(t *testing.T) {
	h := newHarness(t)
	h.source.awayDelay = 30 * time.Second
	a := h.connect("alice", "A", "tab")
	b := h.connect("bob", "B", "tab")
	h.report(a, "engagement", true, "")
	h.report(b, "engagement", true, "")
	h.now = h.start.Add(5 * time.Second)
	h.r.EndSession("alice", "A")
	h.at(34 * time.Second)
	if len(h.source.away["alice"]) != 0 {
		t.Fatal("session revocation skipped grace")
	}
	h.at(35 * time.Second)
	if got := h.source.away["alice"]; len(got) != 1 || !got[0] {
		t.Fatal("revoked last session did not become away", got)
	}
	if got := h.source.away["bob"]; len(got) != 0 {
		t.Fatal("another account's revocation affected an active client", got)
	}
}

func TestAutoAwayExplicitDelaysAndObsoleteReports(t *testing.T) {
	for _, delay := range []time.Duration{0, 10 * time.Minute} {
		t.Run(delay.String(), func(t *testing.T) {
			h := newHarness(t)
			h.source.awayDelay = delay
			old := h.connect("alice", "A", "tab")
			h.report(old, "engagement", true, "")
			h.report(old, "inactive", false, "")
			current := h.connect("alice", "A", "tab")
			h.at(599 * time.Second)
			if got := h.source.away["alice"]; len(got) != 0 {
				t.Fatal("explicit delay ignored", got)
			}
			h.at(600 * time.Second)
			if got := h.source.away["alice"]; (len(got) == 1 && got[0]) != (delay > 0) {
				t.Fatal("explicit delay or never ignored", got)
			}
			if h.r.ReportState("alice", "A", Report{Tab: old.tab, Incarnation: old.incarnation, ConnectionID: old.connection, Sequence: 100, Signal: "engagement", Active: true}) {
				t.Fatal("obsolete connection report accepted")
			}
			if delay > 0 && len(h.source.away["alice"]) != 1 {
				t.Fatal("obsolete report cleared away", h.source.away["alice"])
			}
			h.report(current, "engagement", true, "")
		})
	}
}

func TestAutoAwayDelayedWriteCannotOverrideNewActivity(t *testing.T) {
	h := newHarness(t)
	h.source.awayDelay = 30 * time.Second
	source := &blockedPresenceSource{fakeSource: h.source, entered: make(chan struct{}), release: make(chan struct{})}
	h.r.options.Source = source
	a := h.connect("alice", "A", "tab")
	h.report(a, "engagement", true, "")
	h.report(a, "inactive", false, "")
	h.now = h.start.Add(30 * time.Second)
	tickDone := make(chan struct{})
	go func() { h.r.tick(h.now); close(tickDone) }()
	<-source.entered
	reportDone := make(chan struct{})
	go func() { h.r.Report("alice", "A", "tab", true, Desktop, true); close(reportDone) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		h.r.mu.Lock()
		active := !h.r.accounts["alice"].autoAway
		h.r.mu.Unlock()
		if active {
			break
		}
		if time.Now().After(deadline) {
			close(source.release)
			<-tickDone
			<-reportDone
			t.Fatal("activity was blocked by a presence write")
		}
		time.Sleep(time.Millisecond)
	}
	close(source.release)
	<-tickDone
	<-reportDone
	if got := h.source.away["alice"]; len(got) != 2 || !got[0] || got[1] {
		t.Fatal("delayed away write overrode newer activity", got)
	}
}
