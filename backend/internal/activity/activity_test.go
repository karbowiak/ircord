package activity

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type pushed struct {
	owner    string
	alert    Alert
	target   Target
	fellBack func()
}

type fakePush struct {
	mu   sync.Mutex
	sent []pushed
	subs map[string]bool
}

func (p *fakePush) Route(owner string, alert Alert, target Target, fellBack func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent = append(p.sent, pushed{owner, alert, target, fellBack})
}
func (p *fakePush) HasSubscription(owner, session string) bool { return p.subs[owner+"/"+session] }
func (p *fakePush) take() []pushed {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.sent
	p.sent = nil
	return out
}

type fakeSource struct {
	mu       sync.Mutex
	delays   Delays
	read     map[string]bool
	away     map[string][]bool
	devices  map[string][]string
	awayWait time.Duration
}

func (s *fakeSource) Delays(string) Delays { s.mu.Lock(); defer s.mu.Unlock(); return s.delays }
func (s *fakeSource) Eligible(owner string, item Item) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.read[item.Message]
}
func (s *fakeSource) AutoAwayDelay(string) time.Duration { return s.awayWait }
func (s *fakeSource) SetAutoAway(owner string, away bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.away[owner] = append(s.away[owner], away)
}
func (s *fakeSource) SetDevice(owner, device string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices[owner] = append(s.devices[owner], device)
}

type memStore struct{ rows map[string][]byte }

func (m *memStore) SaveInterval(owner string, data []byte) error {
	m.rows[owner] = append([]byte(nil), data...)
	return nil
}
func (m *memStore) DeleteInterval(owner string) error { delete(m.rows, owner); return nil }
func (m *memStore) LoadIntervals() (map[string][]byte, error) {
	out := map[string][]byte{}
	for k, v := range m.rows {
		out[k] = v
	}
	return out, nil
}

type harness struct {
	t      *testing.T
	now    time.Time
	start  time.Time
	push   *fakePush
	source *fakeSource
	store  *memStore
	r      *Router
	n      int
}

func newHarness(t *testing.T) *harness {
	start := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	h := &harness{t: t, now: start, start: start, push: &fakePush{subs: map[string]bool{}}, store: &memStore{rows: map[string][]byte{}}}
	h.source = &fakeSource{delays: Delays{DefaultTarget, DefaultAll}, read: map[string]bool{}, away: map[string][]bool{}, devices: map[string][]string{}}
	h.r = h.router()
	return h
}

func (h *harness) router() *Router {
	r := New(Options{Push: h.push, Source: h.source, Store: h.store, Now: func() time.Time { return h.now }})
	return r
}

// at moves the clock to start+d and runs the scheduler there.
func (h *harness) at(d time.Duration) {
	h.now = h.start.Add(d)
	h.r.tick(h.now)
}

// message delivers an eligible message in conversation conv at start+d.
func (h *harness) message(d time.Duration, conv string) string {
	h.now = h.start.Add(d)
	h.n++
	id := fmt.Sprintf("m%d", h.n)
	h.r.Notify("alice", Item{Conversation: conv, Message: id, At: h.now, Title: "bob", Body: id, Kind: "mention"}, h.source.Delays("alice"))
	return id
}

func (h *harness) expectNone(context string) {
	h.t.Helper()
	if sent := h.push.take(); len(sent) != 0 {
		h.t.Fatalf("%s: expected no alerts, got %+v", context, sent)
	}
}

func (h *harness) expectOne(context string) pushed {
	h.t.Helper()
	sent := h.push.take()
	if len(sent) != 1 {
		h.t.Fatalf("%s: expected one alert, got %+v", context, sent)
	}
	return sent[0]
}

// activeThenAway makes session A of alice active at -1s and inactive at 0.
func (h *harness) activeThenAway(session string) {
	h.now = h.start.Add(-time.Second)
	h.r.Report("alice", session, "tab", true, Desktop, false)
	h.now = h.start
	h.r.Report("alice", session, "tab", false, "", false)
}

func only(p pushed, session string) bool {
	return len(p.target.Only) == 1 && p.target.Only[0] == session
}

func TestActiveClientSuppressesEveryDevice(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"], h.push.subs["alice/B"] = true, true
	h.r.Report("alice", "A", "tab", true, Desktop, false)
	h.message(time.Second, "dm")
	h.at(10 * time.Minute / 20) // renewals keep it active
	h.expectNone("while A is active")
	if !h.r.Active("alice") {
		t.Fatal("alice should be active")
	}
}

func TestBriefSwitchStaysQuietAndCancels(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"] = true
	h.activeThenAway("A")
	h.message(5*time.Second, "dm")
	h.message(10*time.Second, "dm")
	h.at(19 * time.Second)
	h.expectNone("inside the grace period")
	h.now = h.start.Add(20 * time.Second)
	h.r.Report("alice", "A", "tab", true, Desktop, false)
	h.at(31 * time.Second)
	h.at(6 * time.Minute)
	h.expectNone("after returning, the interval's escalation is cancelled")
}

func TestGraceCatchUpThenTargetedThenReminder(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"], h.push.subs["alice/B"] = true, true
	h.activeThenAway("A")
	h.message(5*time.Second, "dm")
	latest := h.message(10*time.Second, "dm")
	h.at(29*time.Second + 999*time.Millisecond)
	h.expectNone("just before the first threshold")
	h.at(30 * time.Second)
	got := h.expectOne("grace catch-up")
	if !only(got, "A") || got.alert.MessageID != latest || got.alert.Reminder {
		t.Fatalf("catch-up should carry the latest grace item to A only: %+v", got)
	}
	h.at(31 * time.Second)
	h.expectNone("the catch-up happens once")
	// Ten items in the targeted window go to A only.
	var last string
	for i := 0; i < 10; i++ {
		last = h.message(time.Minute+time.Duration(i)*20*time.Second, fmt.Sprintf("c%d", i%3))
		if got := h.expectOne("targeted"); !only(got, "A") {
			t.Fatalf("targeted window must alert A only: %+v", got)
		}
	}
	h.at(5*time.Minute - time.Millisecond)
	h.expectNone("just before the second threshold")
	h.at(5 * time.Minute)
	got = h.expectOne("five-minute reminder")
	if got.target.Only != nil || len(got.target.Except) != 1 || got.target.Except[0] != "A" {
		t.Fatalf("reminder should go to every device but A, which already has its item: %+v", got.target)
	}
	if got.alert.MessageID != last || got.alert.Unread != 12 || got.alert.Conversations != 4 || !got.alert.Reminder {
		t.Fatalf("reminder should carry the latest item and the unread count: %+v", got.alert)
	}
	h.at(10 * time.Minute)
	h.expectNone("one reminder per interval")
	h.message(11*time.Minute, "dm")
	if got := h.expectOne("after escalation"); got.target.Only != nil || got.target.Except != nil {
		t.Fatalf("after escalation alerts go everywhere: %+v", got.target)
	}
}

func TestReminderRechecksReadState(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"] = true
	h.activeThenAway("A")
	first := h.message(5*time.Second, "dm")
	second := h.message(10*time.Second, "dm")
	h.at(30 * time.Second) // A is alerted with the latest grace item only
	h.push.take()
	h.source.read[second] = true
	h.at(5 * time.Minute)
	got := h.expectOne("reminder after the latest was read")
	if got.alert.MessageID != first || got.alert.Unread != 1 || got.alert.Reminder || got.target.Except != nil {
		t.Fatalf("the latest remaining unread item goes to every device, A included: %+v", got)
	}

	h = newHarness(t)
	h.push.subs["alice/A"] = true
	h.activeThenAway("A")
	m := h.message(time.Minute, "dm")
	h.push.take()
	h.source.read[m] = true
	h.at(5 * time.Minute)
	h.expectNone("everything read: no empty reminder")
}

func TestNoCandidatesNoReminder(t *testing.T) {
	h := newHarness(t)
	h.activeThenAway("A")
	h.at(30 * time.Second)
	h.at(5 * time.Minute)
	h.expectNone("an interval without messages")
}

func TestHandoffStartsNewIntervalWithNewPreferred(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"], h.push.subs["alice/B"] = true, true
	h.activeThenAway("A")
	h.message(time.Minute, "dm")
	h.push.take()
	h.now = h.start.Add(2 * time.Minute)
	h.r.Report("alice", "B", "tab", true, Phone, false)
	h.message(2*time.Minute+time.Second, "dm")
	h.expectNone("B is active")
	h.now = h.start.Add(3 * time.Minute)
	h.r.Report("alice", "B", "tab", false, "", false)
	h.at(5 * time.Minute)
	h.expectNone("A's old five-minute deadline cannot fire")
	h.message(3*time.Minute+31*time.Second, "dm")
	if got := h.expectOne("new interval"); !only(got, "B") {
		t.Fatalf("B is now preferred: %+v", got)
	}
}

func TestCustomTimingAndShortenedThreshold(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"] = true
	h.source.delays = Delays{15 * time.Second, 2 * time.Minute}
	h.activeThenAway("A")
	h.message(5*time.Second, "dm")
	h.at(15 * time.Second)
	if got := h.expectOne("15s catch-up"); !only(got, "A") {
		t.Fatalf("catch-up at 15s goes to A: %+v", got)
	}
	h.message(30*time.Second, "dm")
	h.push.take()
	h.at(time.Minute + 59*time.Second)
	h.expectNone("before 2m")
	// Shortening below the elapsed time causes one immediate transition, never repeats.
	h.source.delays = Delays{15 * time.Second, time.Minute}
	h.at(time.Minute + 59*time.Second)
	if got := h.expectOne("shortened threshold"); !got.alert.Reminder {
		t.Fatalf("expected the reminder: %+v", got)
	}
	h.source.delays = Delays{10 * time.Second, 30 * time.Second}
	h.at(2 * time.Minute)
	h.at(3 * time.Minute)
	h.expectNone("repeated saves do not repeat the fan-out")
}

func TestSkippingTheTargetPhaseIsOneTransition(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"] = true
	h.activeThenAway("A")
	h.message(5*time.Second, "dm")
	h.message(6*time.Second, "dm")
	h.source.delays = Delays{0, 0}
	h.at(7 * time.Second)
	got := h.expectOne("straight to every device")
	if got.target.Only != nil || got.alert.Unread != 2 {
		t.Fatalf("one reminder for both grace items: %+v", got)
	}
}

// A live message after the deadline but before the scheduler ran goes to every device as
// itself; the scheduler's recheck then decides the reminder, which never repeats that item and
// never counts items read meanwhile (review L1).
func TestMessageAtTheDeadlineGoesEverywhereOnce(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"] = true
	h.activeThenAway("A")
	read := h.message(5*time.Second, "dm")
	h.message(6*time.Second, "dm")
	h.source.read[read] = true
	last := h.message(5*time.Minute, "dm") // before the scheduler runs at the deadline
	got := h.expectOne("live message at the deadline")
	if got.target.Only != nil || got.alert.MessageID != last || got.alert.Unread != 1 || got.alert.Reminder {
		t.Fatalf("the live message goes everywhere as itself: %+v", got)
	}
	h.at(5 * time.Minute)
	h.expectNone("the reminder's latest item already reached every device")
	h.at(6 * time.Minute)
	h.expectNone("the interval is over")
}

// An item that already reached every device through a fallback is not alerted again by the
// five-minute reminder (review M1), whether the router fell back itself or the push did.
func TestFallbackDeliveryIsNotRepeatedAsTheReminder(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/B"] = true // A can be reached by neither page nor push; B never was active
	h.activeThenAway("A")
	h.message(time.Minute, "dm")
	if got := h.expectOne("fallback"); got.target.Only != nil {
		t.Fatalf("expected every device: %+v", got)
	}
	h.at(5 * time.Minute)
	h.expectNone("the fallback item is not repeated")

	h = newHarness(t)
	h.push.subs["alice/A"] = true
	h.activeThenAway("A")
	h.message(time.Minute, "dm")
	got := h.expectOne("push to A")
	if !only(got, "A") || !got.target.Fallback || got.fellBack == nil {
		t.Fatalf("expected a push to A with a fallback hook: %+v", got)
	}
	got.fellBack() // A's push failed; the other devices were tried
	h.at(5 * time.Minute)
	h.expectNone("the pushed fallback item is not repeated")
}

// An open page that cannot show notifications (off, no permission) is no way to reach a device
// (review M2): the next most recently active device that can be alerted is used, else all.
func TestSilentOpenPageIsNotAReachableDevice(t *testing.T) {
	h := newHarness(t)
	_, stop := h.r.Subscribe("alice", "A", "tab")
	defer stop()
	h.activeThenAway("A") // reports canAlert false
	h.message(time.Minute, "dm")
	if got := h.expectOne("silent page"); got.target.Only != nil {
		t.Fatalf("a page without notifications must not hold the alert back: %+v", got)
	}

	h = newHarness(t)
	_, stopA := h.r.Subscribe("alice", "A", "tab")
	defer stopA()
	h.push.subs["alice/B"] = true
	h.now = h.start.Add(-2 * time.Second)
	h.r.Report("alice", "B", "tab", true, Phone, false)
	h.r.Report("alice", "B", "tab", false, "", false)
	h.now = h.start.Add(-time.Second)
	h.r.Report("alice", "A", "tab", true, Desktop, false)
	h.now = h.start
	h.r.Report("alice", "A", "tab", false, "", false)
	h.message(time.Minute, "dm")
	if got := h.expectOne("next device"); !only(got, "B") {
		t.Fatalf("the next most recently active reachable device is used: %+v", got)
	}
	h.at(5 * time.Minute)
	if got := h.expectOne("reminder"); len(got.target.Except) != 1 || got.target.Except[0] != "B" {
		t.Fatalf("the reminder skips the device that already had the item: %+v", got)
	}

	h = newHarness(t)
	_, stopP := h.r.Subscribe("alice", "A", "tab")
	defer stopP()
	h.now = h.start.Add(-time.Second)
	h.r.Report("alice", "A", "tab", true, Desktop, true)
	h.now = h.start
	h.r.Report("alice", "A", "tab", false, "", true)
	h.message(time.Minute, "dm")
	if got := h.expectOne("page that notifies"); !only(got, "A") || got.target.Fallback {
		t.Fatalf("a page that shows notifications is enough: %+v", got)
	}
}

// Signing a session out stops its lease from holding alerts back and its device from being
// preferred (review L5).
func TestEndedSessionStopsSuppressingAndIsNotPreferred(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"], h.push.subs["alice/B"] = true, true
	h.now = h.start.Add(-2 * time.Second)
	h.r.Report("alice", "B", "tab", true, Phone, false)
	h.r.Report("alice", "B", "tab", false, "", false)
	h.now = h.start.Add(-time.Second)
	h.r.Report("alice", "A", "tab", true, Desktop, false)
	h.now = h.start
	h.r.EndSession("alice", "A")
	if h.r.Active("alice") {
		t.Fatal("a signed-out session must not keep the account active")
	}
	h.message(time.Minute, "dm")
	if got := h.expectOne("after sign-out"); !only(got, "B") {
		t.Fatalf("the remaining device is preferred: %+v", got)
	}
}

// Presence side effects always end at the router's latest desired state, however connects
// and the auto-away deadline interleave (review L2).
func TestPresenceSettlesOnTheLatestState(t *testing.T) {
	h := newHarness(t)
	h.source.awayWait = time.Nanosecond
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, stop := h.r.Subscribe("alice", "A", "tab")
			stop()
		}()
		go func() {
			defer wg.Done()
			h.r.tick(h.start.Add(time.Hour))
		}()
	}
	wg.Wait()
	h.r.mu.Lock()
	want := h.r.accounts["alice"].autoAway
	h.r.mu.Unlock()
	h.source.mu.Lock()
	calls := h.source.away["alice"]
	h.source.mu.Unlock()
	if len(calls) > 0 && calls[len(calls)-1] != want {
		t.Fatalf("the last applied away %v differs from the desired %v", calls[len(calls)-1], want)
	}
	for i := 1; i < len(calls); i++ {
		if calls[i] == calls[i-1] {
			t.Fatalf("the same away state was applied twice in a row: %v", calls)
		}
	}
}

func TestUnavailablePreferredFallsBackToEveryDevice(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/B"] = true // A has neither a push subscription nor an open page
	h.activeThenAway("A")
	h.message(time.Minute, "dm")
	if got := h.expectOne("fallback"); got.target.Only != nil {
		t.Fatalf("an unreachable preferred device falls back to every device: %+v", got)
	}
	// A push-only preferred device asks the sender to fall back if delivery fails.
	h = newHarness(t)
	h.push.subs["alice/A"] = true
	h.activeThenAway("A")
	h.message(time.Minute, "dm")
	if got := h.expectOne("push fallback"); !only(got, "A") || !got.target.Fallback {
		t.Fatalf("push-only preferred device needs a delivery fallback: %+v", got)
	}
}

func TestNoActivityRecordAlertsEverywhere(t *testing.T) {
	h := newHarness(t)
	h.message(time.Second, "dm")
	if got := h.expectOne("no record"); got.target.Only != nil {
		t.Fatalf("with no activity record every device is alerted: %+v", got)
	}
	h.at(10 * time.Minute)
	h.expectNone("receiving a message starts no timer")
}

func TestLeaseExpiryStartsIntervalAtLastReport(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"] = true
	h.r.Report("alice", "A", "tab", true, Desktop, false) // at start, never renewed
	h.message(LeaseTTL-time.Second, "dm")
	h.expectNone("lease still current")
	h.at(LeaseTTL)
	if h.r.Active("alice") {
		t.Fatal("an unrenewed lease must lapse")
	}
	// T0 is the last report, so the 30-second deadline has already passed; the next message
	// goes to the preferred device.
	h.message(LeaseTTL+time.Second, "dm")
	if got := h.expectOne("after expiry"); !only(got, "A") {
		t.Fatalf("expected A after the lease lapsed: %+v", got)
	}
}

func TestBackgroundTabDoesNotReplacePreferred(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"], h.push.subs["alice/B"] = true, true
	h.now = h.start.Add(-time.Second)
	h.r.Report("alice", "A", "tab", true, Desktop, false)
	h.now = h.start
	h.r.Report("alice", "A", "tab", false, "", false)
	// A background tab only ever reports inactive; it reports later than A and its ID sorts
	// first, so neither recency nor the tie-break can make it preferred by accident.
	h.push.subs["alice/0"] = true
	h.now = h.start.Add(time.Second)
	h.r.Report("alice", "0", "tab", false, "", false)
	h.message(time.Minute, "dm")
	if got := h.expectOne("targeted"); !only(got, "A") {
		t.Fatalf("A stays preferred: %+v", got)
	}
}

func TestAccountsAreIsolated(t *testing.T) {
	h := newHarness(t)
	h.r.Report("alice", "A", "tab", true, Desktop, false)
	h.now = h.start.Add(time.Second)
	h.r.Notify("bob", Item{Conversation: "dm", Message: "x", At: h.now, Kind: "mention"}, Delays{DefaultTarget, DefaultAll})
	got := h.expectOne("bob")
	if got.owner != "bob" || got.target.Only != nil {
		t.Fatalf("alice's activity must not suppress bob: %+v", got)
	}
}

func TestOneConnectionPerDeviceAlerts(t *testing.T) {
	h := newHarness(t)
	one, stopOne := h.r.Subscribe("alice", "A", "t1")
	defer stopOne()
	two, stopTwo := h.r.Subscribe("alice", "A", "t2")
	defer stopTwo()
	phone, stopPhone := h.r.Subscribe("alice", "B", "p1")
	defer stopPhone()
	other, stopOther := h.r.Subscribe("bob", "C", "x")
	defer stopOther()
	h.now = h.start.Add(-time.Second)
	h.r.Report("alice", "A", "t2", true, Desktop, true)
	h.now = h.start
	h.r.Report("alice", "A", "t2", false, "", true)
	h.message(time.Minute, "dm")
	if len(one.C) != 0 || len(two.C) != 1 || len(phone.C) != 0 || len(other.C) != 0 {
		t.Fatalf("only A's most recently active tab alerts: %d %d %d %d", len(one.C), len(two.C), len(phone.C), len(other.C))
	}
	<-two.C
	h.at(5 * time.Minute)
	// The reminder's item is the one A already alerted, so only B gets it.
	if len(one.C)+len(two.C) != 0 || len(phone.C) != 1 {
		t.Fatalf("reminder skips A: %d %d %d", len(one.C), len(two.C), len(phone.C))
	}
	if a := <-phone.C; !a.Reminder && a.Unread != 1 {
		t.Fatalf("unexpected reminder %+v", a)
	}
}

func TestIntervalSurvivesRestart(t *testing.T) {
	h := newHarness(t)
	h.push.subs["alice/A"], h.push.subs["alice/B"] = true, true
	h.activeThenAway("A")
	h.message(time.Minute, "dm")
	h.push.take()
	h.r.flush()
	if len(h.store.rows) != 1 {
		t.Fatalf("the open interval should be saved: %v", h.store.rows)
	}
	h.r = h.router() // restart
	h.at(4 * time.Minute)
	h.expectNone("restored interval keeps its original deadline")
	h.at(5 * time.Minute)
	if got := h.expectOne("restored reminder"); got.alert.Unread != 1 {
		t.Fatalf("expected the reminder after restart: %+v", got)
	}
	h.r.flush()
	if len(h.store.rows) != 0 {
		t.Fatalf("a finished interval is removed: %v", h.store.rows)
	}
	h.r = h.router()
	h.at(6 * time.Minute)
	h.expectNone("no second reminder after another restart")
}

func TestAutoAwayAndDevice(t *testing.T) {
	h := newHarness(t)
	h.source.awayWait = 10 * time.Minute
	h.r.Track("alice")
	h.at(10*time.Minute - time.Second)
	if len(h.source.away["alice"]) != 0 {
		t.Fatal("not away before the delay")
	}
	h.at(10 * time.Minute)
	h.at(11 * time.Minute)
	if got := h.source.away["alice"]; len(got) != 1 || !got[0] {
		t.Fatalf("away once after the delay: %v", got)
	}
	_, stop := h.r.Subscribe("alice", "P", "t")
	h.r.Report("alice", "P", "t", true, Phone, false)
	if got := h.source.away["alice"]; len(got) != 2 || got[1] {
		t.Fatalf("connecting ends the automatic away: %v", got)
	}
	if got := h.source.devices["alice"]; len(got) != 1 || got[0] != Phone {
		t.Fatalf("a connected phone is shared: %v", got)
	}
	_, stopDesk := h.r.Subscribe("alice", "D", "t")
	h.now = h.now.Add(time.Second)
	h.r.Report("alice", "D", "t", true, Desktop, false)
	if got := h.source.devices["alice"]; len(got) != 2 || got[1] != "" {
		t.Fatalf("a more recently active desktop clears it: %v", got)
	}
	stopDesk()
	if got := h.source.devices["alice"]; len(got) != 3 || got[2] != Phone {
		t.Fatalf("the phone is the remaining device: %v", got)
	}
	stop()
	if got := h.source.devices["alice"]; len(got) != 4 || got[3] != "" {
		t.Fatalf("no connected device clears it: %v", got)
	}
	h.source.awayWait = 0
	h.at(time.Hour)
	if got := h.source.away["alice"]; len(got) != 2 {
		t.Fatalf("a delay of 0 never marks away: %v", got)
	}
}
