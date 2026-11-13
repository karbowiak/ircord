package irc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/lrstanley/girc"
)

func TestHistoryQueueEachRequestGetsResponseBudget(t *testing.T) {
	f := historyFixture(t)
	f.c.historyGate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	done := make(chan error, 3)
	for i := 0; i < cap(done); i++ {
		go func() {
			_, err := f.c.syncHistoryWithBudgets(ctx, "#test", "AFTER", "msgid=anchor", 100, 300*time.Millisecond, 300*time.Millisecond)
			done <- err
		}()
	}
	eventually(t, func() bool { return len(f.c.historyScheduling().slots) == 3 })
	// This exceeds each operation's response budget, but no request has yet
	// acquired the gate or entered the writer queue.
	time.Sleep(400 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("gate wait consumed operation budget: %v", err)
	default:
	}
	<-f.c.historyGate
	for i := 0; i < cap(done); i++ {
		request := historyCommand(t, f.c, "AFTER", "msgid=anchor", nil)
		time.Sleep(40 * time.Millisecond)
		historyReply(f.c, request, true)
		if err := <-done; err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
}

func TestHistoryQueueWaitingCallerCancellation(t *testing.T) {
	f := historyFixture(t)
	f.c.historyGate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := f.c.syncHistory(ctx, "#test", "AFTER", "msgid=anchor", 100); done <- err }()
	eventually(t, func() bool { return len(f.c.historyScheduling().slots) == 1 })
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued caller did not cancel")
	}
	<-f.c.historyGate
	if len(f.c.historyScheduling().slots) != 0 || len(f.c.queue) != 0 {
		t.Fatal("cancelled waiter retained admission or queued a command")
	}
}

func TestHistoryQueueCallerDeadlineIncludesGateWait(t *testing.T) {
	f := historyFixture(t)
	f.c.historyGate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	_, err := f.c.syncHistory(ctx, "#test", "AFTER", "msgid=anchor", 100)
	<-f.c.historyGate
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("caller deadline was not respected while queued: %v", err)
	}
}

func TestHistoryQueueGateWaitIsBounded(t *testing.T) {
	f := historyFixture(t)
	f.c.historyGate <- struct{}{}
	_, err := f.c.syncHistoryWithBudgets(context.Background(), "#test", "AFTER", "msgid=anchor", 100, 5*time.Millisecond, 5*time.Millisecond)
	<-f.c.historyGate
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("gate wait was not bounded: %v", err)
	}
	if len(f.c.historyScheduling().slots) != 0 || len(f.c.queue) != 0 {
		t.Fatal("timed-out waiter retained admission or queued a command")
	}
}

func TestHistoryQueueConnectionShutdownCancelsWaiter(t *testing.T) {
	f := historyFixture(t)
	ctx, stop := context.WithCancel(f.m.ctx)
	f.c.ctx = ctx
	f.c.historyGate <- struct{}{}
	done := make(chan error, 1)
	go func() {
		_, err := f.c.syncHistory(context.Background(), "#test", "AFTER", "msgid=anchor", 100)
		done <- err
	}()
	eventually(t, func() bool { return len(f.c.historyScheduling().slots) == 1 })
	stop()
	select {
	case err := <-done:
		if !errors.Is(err, ErrDisconnected) {
			t.Fatalf("shutdown waiter: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("connection shutdown did not release queued waiter")
	}
	<-f.c.historyGate
	if len(f.c.historyScheduling().slots) != 0 {
		t.Fatal("shutdown retained history admission")
	}
}

func TestHistoryQueueWriterTimeoutFencesQueuedRequest(t *testing.T) {
	f := historyFixture(t)
	done := make(chan error, 1)
	go func() {
		_, err := f.c.syncHistoryWithBudgets(context.Background(), "#test", "AFTER", "msgid=anchor", 100, 60*time.Millisecond, time.Second)
		done <- err
	}()
	events := <-f.c.queue
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("writer timeout: %v", err)
	}
	f.c.mu.Lock()
	req := f.c.takeHistoryWrite(events[0])
	allowed := req != nil && f.c.startHistoryWrite(req, f.c.client)
	tracking := len(f.c.automaticHistoryWrites)
	f.c.mu.Unlock()
	if allowed || tracking != 0 {
		t.Fatalf("expired queued request could be written: allowed=%v tracking=%d", allowed, tracking)
	}
}

func TestHistoryQueueWriterDelayDoesNotConsumeResponseBudget(t *testing.T) {
	f := historyFixture(t)
	done := make(chan error, 1)
	go func() {
		_, err := f.c.syncHistoryWithBudgets(context.Background(), "#test", "AFTER", "msgid=anchor", 100, 500*time.Millisecond, 500*time.Millisecond)
		done <- err
	}()
	events := <-f.c.queue
	time.Sleep(300 * time.Millisecond)
	f.c.mu.Lock()
	req := f.c.takeHistoryWrite(events[0])
	started := req != nil && f.c.startHistoryWrite(req, f.c.client)
	f.c.mu.Unlock()
	if !started {
		t.Fatal("request expired before writer budget")
	}
	time.Sleep(300 * time.Millisecond)
	historyReply(f.c, events[0], true)
	if err := <-done; err != nil {
		t.Fatalf("writer delay consumed response budget: %v", err)
	}
}

func TestHistoryQueueUnlabeledWrittenCancellationWaitsForTeardown(t *testing.T) {
	f := historyFixture(t)
	delete(f.c.caps, "labeled-response")
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	attemptDone := make(chan struct{})
	f.c.mu.Lock()
	f.c.transport = conn
	f.c.attemptDone = attemptDone
	f.c.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := f.c.syncHistory(ctx, "#test", "AFTER", "msgid=anchor", 100); done <- err }()
	historyCommand(t, f.c, "AFTER", "msgid=anchor", nil)
	cancel()
	eventually(t, func() bool {
		f.c.mu.Lock()
		defer f.c.mu.Unlock()
		return f.c.unlabeledHistory != nil && f.c.unlabeledHistory.abandoned
	})
	_ = peer.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := peer.Write([]byte("late")); err == nil {
		t.Fatal("abandoned written request did not close its transport")
	}
	select {
	case err := <-done:
		t.Fatalf("gate released before attempt teardown: %v", err)
	default:
	}
	if len(f.c.historyGate) != 1 {
		t.Fatal("history gate was released before attempt teardown")
	}
	f.c.failHistory()
	close(attemptDone)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("written cancellation: %v", err)
	}
	if len(f.c.historyGate) != 0 {
		t.Fatal("teardown did not release history gate")
	}
}

func TestHistoryQueueUnlabeledResponseTimeoutWaitsForTeardown(t *testing.T) {
	f := historyFixture(t)
	delete(f.c.caps, "labeled-response")
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	attemptDone := make(chan struct{})
	f.c.mu.Lock()
	f.c.transport = conn
	f.c.attemptDone = attemptDone
	f.c.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		_, err := f.c.syncHistoryWithBudgets(context.Background(), "#test", "AFTER", "msgid=anchor", 100, time.Second, 60*time.Millisecond)
		done <- err
	}()
	historyCommand(t, f.c, "AFTER", "msgid=anchor", nil)
	eventually(t, func() bool {
		f.c.mu.Lock()
		defer f.c.mu.Unlock()
		return f.c.unlabeledHistory != nil && f.c.unlabeledHistory.abandoned
	})
	_ = peer.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := peer.Write([]byte("late")); err == nil {
		t.Fatal("timed-out request did not close its transport")
	}
	select {
	case err := <-done:
		t.Fatalf("timed-out request released the gate before teardown: %v", err)
	default:
	}
	f.c.failHistory()
	close(attemptDone)
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("response timeout: %v", err)
	}
}

func TestHistoryQueueUnlabeledTimeoutReconnectFencesLateReply(t *testing.T) {
	port, peers := capabilityServer(t, "message-tags batch server-time draft/chathistory", "")
	m := New(Options{AllowPrivate: true, ReconnectMin: 10 * time.Millisecond})
	t.Cleanup(m.Close)
	if _, err := m.Connect("alice", NetworkConfig{ID: "net", Name: "test", Host: "127.0.0.1", Port: port, Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	<-peers
	c, err := m.connection("alice", "net")
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		_, err := c.syncHistoryWithBudgets(ctx, "#test", "AFTER", "msgid=anchor", 100, time.Second, 150*time.Millisecond)
		finished <- err
	}()
	var oldClient *girc.Client
	eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.unlabeledHistory == nil || c.unlabeledHistory.client == nil {
			return false
		}
		oldClient = c.unlabeledHistory.client
		return true
	})
	if err := <-finished; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed-out request: %v", err)
	}
	var replacement *capabilityPeer
	select {
	case replacement = <-peers:
	case <-ctx.Done():
		t.Fatal("timed-out internal request did not reconnect")
	}
	if err := m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	go func() { _, err := c.syncHistory(ctx, "#test", "AFTER", "msgid=anchor", 100); finished <- err }()
	eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.unlabeledHistory != nil && c.unlabeledHistory.client != nil && c.unlabeledHistory.client != oldClient
	})
	c.receiveFromAttempt(oldClient, girc.Event{Command: "FAIL", Params: []string{"CHATHISTORY", "INVALID_PARAMS", "late old failure"}})
	select {
	case err := <-finished:
		t.Fatalf("old reply completed replacement request: %v", err)
	default:
	}
	replacement.send(":test BATCH +history chathistory #test")
	replacement.send(":test BATCH -history")
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("replacement request: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("replacement history did not complete")
	}
}

func TestHistoryQueueAdmissionIsBounded(t *testing.T) {
	f := historyFixture(t)
	f.c.historyGate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, historyWaiterLimit+4)
	for i := 0; i < cap(done); i++ {
		go func() { _, err := f.c.syncHistory(ctx, "#test", "AFTER", "msgid=anchor", 100); done <- err }()
	}
	eventually(t, func() bool { return len(f.c.historyScheduling().slots) == historyWaiterLimit })
	for i := 0; i < 4; i++ {
		if err := <-done; !errors.Is(err, ErrBusy) {
			t.Fatalf("excess request was not rejected: %v", err)
		}
	}
	cancel()
	for i := 0; i < historyWaiterLimit; i++ {
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting request: %v", err)
		}
	}
	<-f.c.historyGate
	if len(f.c.historyScheduling().slots) != 0 {
		t.Fatal("cancelled waiters retained admission")
	}
}

func TestHistoryQueueOverflowRescansDurableGaps(t *testing.T) {
	f := historyFixture(t)
	const targets = historyCatchupLimit + 6
	names := make([]string, 0, targets)
	for i := 0; i < targets; i++ {
		name := fmt.Sprintf("peer%03d", i)
		if i >= historyCatchupLimit {
			name = "#" + name
			f.c.owner.mu.Lock()
			conv := f.c.conversation(name)
			conv.Joined = true
			f.c.owner.conversations[conv.ID] = conv
			f.c.owner.mu.Unlock()
		}
		names = append(names, name)
		f.c.catchup(name)
	}
	s := f.c.historyScheduling()
	f.c.mu.Lock()
	pending, scanning := len(s.pending), s.scan
	f.c.mu.Unlock()
	if pending > historyCatchupLimit || !scanning {
		t.Fatalf("scheduler not bounded or overflow not recorded: pending=%d scan=%v", pending, scanning)
	}
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	completed := map[string]bool{}
	for len(completed) < targets {
		select {
		case events := <-f.c.queue:
			if len(events) != 1 || events[0].Command != "CHATHISTORY" {
				t.Fatalf("unexpected queued command: %+v", events)
			}
			request := events[0]
			f.c.mu.Lock()
			req := f.c.takeHistoryWrite(request)
			ok := req != nil && f.c.startHistoryWrite(req, f.c.client)
			f.c.mu.Unlock()
			if !ok {
				t.Fatal("queued catch-up could not dispatch")
			}
			historyReply(f.c, request, true)
			completed[request.Params[1]] = true
		case <-deadline.C:
			t.Fatalf("overflowed gaps not retried: completed=%d/%d", len(completed), targets)
		}
	}
	for _, name := range names {
		id := f.c.conversationID(name)
		eventually(t, func() bool {
			mark, err := f.store.Watermark("alice", id)
			return err == nil && !mark.Gap
		})
	}
	eventually(t, func() bool {
		f.c.mu.Lock()
		defer f.c.mu.Unlock()
		return len(s.pending) == 0 && !s.scan
	})
}

func TestHistoryQueueRetriesRotateBetweenTargets(t *testing.T) {
	f := historyFixture(t)
	f.c.catchup("peerfirst")
	f.c.catchup("peersecond")
	first := historyCommand(t, f.c, "LATEST", "*", nil)
	if first.Params[1] != "peerfirst" {
		t.Fatalf("unexpected first target: %s", first.Params[1])
	}
	f.c.receiveCommandFailure(historyFailureEvent(first, "temporarily unavailable"))
	second := historyCommand(t, f.c, "LATEST", "*", nil)
	if second.Params[1] != "peersecond" {
		t.Fatalf("failed target monopolized scheduler: %s", second.Params[1])
	}
	historyReply(f.c, second, true)
}
