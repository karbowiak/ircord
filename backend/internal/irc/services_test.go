package irc

import (
	"context"
	"errors"
	"fmt"
	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
	"strings"
	"sync"
	"testing"
	"time"
)

func serviceStatus(t *testing.T, err error, want int) {
	t.Helper()
	var e *ServiceError
	if !errors.As(err, &e) || e.Status != want {
		t.Fatalf("service status %d: %v", want, err)
	}
}

func TestServiceRepliesAreCorrelatedPrivateAndLateRepliesCannotCompleteNextRequest(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	w := m.space("alice")
	client := girc.New(girc.Config{Nick: "alice", User: "test", Server: "example.org"})
	c := &connection{manager: m, owner: w, client: client, ctx: context.Background(), config: NetworkConfig{ID: "net"}, serviceBatches: map[string]string{}}
	r := &serviceRequest{label: "chanserv:first", client: client, done: make(chan struct{})}
	c.serviceRequest = r
	receive := func(line string) {
		t.Helper()
		e := girc.ParseEvent(line)
		if e == nil {
			t.Fatal("bad fixture")
		}
		c.receive(client, *e)
	}
	receive("@label=chanserv:first :server.example BATCH +secret labeled-response")
	receive("@batch=secret :ChanServ!ChanServ@localhost NOTICE alice :To confirm, run this command: /CS UNREGISTER #private 123456789")
	receive(":server.example BATCH -secret")
	select {
	case <-r.done:
	default:
		t.Fatal("batch did not complete")
	}
	if len(r.lines) != 1 || !strings.Contains(r.lines[0], "123456789") {
		t.Fatal("reply not collected")
	}
	if len(w.trace["net"]) != 0 || len(w.serverLogs["net"]) != 0 || len(w.messages) != 0 {
		t.Fatal("confirmation entered diagnostics or conversation history")
	}
	next := &serviceRequest{label: "chanserv:next", client: client, done: make(chan struct{})}
	c.serviceRequest = next
	receive("@label=chanserv:first :ChanServ!ChanServ@localhost NOTICE alice :Channel #private is now unregistered")
	select {
	case <-next.done:
		t.Fatal("late reply completed a different request")
	default:
	}
	// A peer cannot impersonate a ChanServ response merely by using a label.
	if c.observeServices(client, *girc.ParseEvent("@label=chanserv:next :someone!user@host NOTICE alice :Insufficient privileges")) {
		t.Fatal("peer notice was consumed")
	}
	if len(next.lines) != 0 {
		t.Fatal("peer spoofed a reply")
	}
	if m.Snapshot("bob").Cursor != 0 {
		t.Fatal("cross-account live delivery")
	}
}

func TestObservedAccountDoesNotComeFromSavedSASL(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	w := m.space("owner")
	client := girc.New(girc.Config{Nick: "alice", User: "test", Server: "example.org"})
	c := &connection{manager: m, owner: w, client: client, ctx: context.Background(), config: NetworkConfig{ID: "net", SASLUser: "configured-only"}}
	w.networks["net"] = Network{ID: "net"}
	for _, v := range []struct{ line, want string }{{":server.example 900 alice alice!u@h actual-account :logged in", "actual-account"}, {"900 alice alice!u@h unprefixed-account :logged in", "unprefixed-account"}, {":server.example 901 alice alice!u@h :logged out", ""}, {":alice!u@h ACCOUNT other-account", "other-account"}, {":alice!u@h ACCOUNT *", ""}} {
		c.observeServices(client, *girc.ParseEvent(v.line))
		if w.networks["net"].Account != v.want {
			t.Fatal("wrong observed account")
		}
	}
	c.observeServices(client, *girc.ParseEvent(":server.example 900 alice alice!u@h actual-account :logged in"))
	c.connectionState("disconnected", client, nil)
	if w.networks["net"].Account != "" {
		t.Fatal("disconnected account survived")
	}
}

func TestErgoChannelServices(t *testing.T) {
	host, port := ergo(t)
	suffix := id()[:8]
	alice, bob := "csa"+suffix, "csb"+suffix
	password := id()
	room := "#cs" + suffix
	registerErgoAccount(t, host, port, alice, password)
	registerErgoAccount(t, host, port, bob, password)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"alice", "bob", "anon"} {
		if _, err := store.CreateAccount(owner, owner); err != nil {
			t.Fatal(err)
		}
	}
	m := New(Options{Store: store, AllowPrivate: true, ReconnectMin: time.Hour})
	defer func() { m.Close(); store.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	for _, v := range []struct{ owner, nick, account string }{{"alice", alice, alice}, {"bob", bob, bob}, {"anon", "csanon" + suffix, ""}} {
		cfg := NetworkConfig{ID: "net", Name: "Ergo", Host: host, Port: port, Nick: v.nick}
		if v.account != "" {
			cfg.SASLUser = v.account
			cfg.SASLPassword = password
		}
		if _, err := m.Connect(v.owner, cfg); err != nil {
			t.Fatal(err)
		}
		if err := m.WaitConnected(ctx, v.owner, "net"); err != nil {
			t.Fatal(err)
		}
	}
	eventually(t, func() bool { return m.Snapshot("alice").Networks[0].Account == alice })
	conv, err := m.Join("alice", "net", room)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		v := findConversation(t, m, "alice", "net", room)
		return v.Joined && channelAt(v, m.Snapshot("alice").Networks[0].ISupport, "alice", 'o')
	})
	if err := m.Raw("alice", "net", "MODE "+room+" +s"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ChannelRegistration(ctx, "bob", conv.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign conversation accepted", err)
	}
	info, err := m.ChannelRegistration(ctx, "alice", conv.ID)
	if err != nil || info.Registered {
		t.Fatal(info, err)
	}
	info, err = m.RegisterChannel(ctx, "alice", conv.ID)
	if err != nil || !info.Registered || info.Founder != alice || info.RegisteredAt == nil {
		t.Fatal(info, err)
	}
	if _, err := m.RegisterChannel(ctx, "alice", conv.ID); err == nil {
		t.Fatal("duplicate registration succeeded")
	} else {
		serviceStatus(t, err, 409)
	}
	for _, owner := range []string{"bob", "anon"} {
		if _, err := m.Join(owner, "net", room); err != nil {
			t.Fatal(err)
		}
		eventually(t, func() bool { return findConversation(t, m, owner, "net", room).Joined })
	}
	if err := m.UnregisterChannel(ctx, "bob", conv.ID); err == nil {
		t.Fatal("non-founder unregistered")
	} else {
		serviceStatus(t, err, 403)
	}
	if _, err := m.RegisterChannel(ctx, "anon", conv.ID); err == nil {
		t.Fatal("anonymous mutation allowed")
	} else {
		serviceStatus(t, err, 403)
	}
	items, err := m.ChannelAccess(ctx, "alice", conv.ID)
	if err != nil || len(items) == 0 {
		t.Fatal(items, err)
	}
	for _, mode := range []string{"v", "o"} {
		items, err = m.SetChannelAccess(ctx, "alice", conv.ID, bob, mode, false)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range items {
			if e.Account == bob && e.Mode == mode {
				found = true
			}
		}
		if !found {
			t.Fatal("role not updated", items)
		}
	}
	if _, err := m.SetChannelAccess(ctx, "alice", conv.ID, alice, "a", false); err == nil {
		t.Fatal("founder changed through access API")
	} else {
		serviceStatus(t, err, 403)
	}
	if _, err := m.SetChannelAccess(ctx, "alice", conv.ID, "missing"+suffix, "v", false); err == nil {
		t.Fatal("missing account accepted")
	} else {
		serviceStatus(t, err, 422)
	}
	items, err = m.SetChannelAccess(ctx, "alice", conv.ID, bob, "", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range items {
		if e.Account == bob {
			t.Fatal("access was not removed")
		}
	}
	// Concurrent reads use exact reply labels and are serialized per connection.
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := m.ChannelRegistration(ctx, "alice", conv.ID)
			if e == nil && v.Founder != alice {
				e = errors.New("wrong founder")
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	transfer, err := m.TransferChannel(ctx, "alice", conv.ID, bob)
	if err != nil || transfer.Status != "pending" {
		t.Fatal(transfer, err)
	}
	if err := m.Raw("bob", "net", "CS TRANSFER accept "+room); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { v, e := m.ChannelRegistration(ctx, "alice", conv.ID); return e == nil && v.Founder == bob })
	if err := m.UnregisterChannel(ctx, "bob", conv.ID); err != nil {
		t.Fatal(err)
	}
	info, err = m.ChannelRegistration(ctx, "bob", conv.ID)
	if err != nil || info.Registered {
		t.Fatal("unregister failed", info, err)
	}
	for _, owner := range []string{"alice", "bob"} {
		c, _ := m.connection(owner, "net")
		c.owner.mu.Lock()
		for _, line := range c.owner.trace["net"] {
			if strings.Contains(line.Text, "To confirm") || line.Command == "CS" && strings.HasPrefix(line.Tags["label"], "chanserv:") {
				t.Error("managed confirmation leaked")
			}
		}
		c.owner.mu.Unlock()
	}
	if err := m.Disconnect("bob", "net"); err != nil {
		t.Fatal(err)
	}
	if m.Snapshot("bob").Networks[0].Account != "" {
		t.Fatal("account survived disconnect")
	}
}

func TestServiceTimeoutKeepsLateRepliesPrivate(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	w := m.space("owner")
	life, cancelLife := context.WithCancel(context.Background())
	defer cancelLife()
	client := girc.New(girc.Config{Nick: "alice", User: "test", Server: "example.org"})
	c := &connection{manager: m, owner: w, client: client, ctx: life, cancel: cancelLife, ready: true, queue: make(chan []*girc.Event, 1), config: NetworkConfig{ID: "net"}, serviceBatches: map[string]string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.serviceExchange(ctx, "UNREGISTER #private"); err == nil {
		t.Fatal("missing reply accepted")
	} else {
		serviceStatus(t, err, 504)
	}
	queued := <-c.queue
	if c.serviceRequest != nil {
		t.Fatal("timed-out request remained active")
	}
	label := queued[0].Tags["label"]
	event := girc.ParseEvent("@label=" + label + " :ChanServ!ChanServ@localhost NOTICE alice :To confirm, run this command: /CS UNREGISTER #private abc23")
	c.receive(client, *event)
	if len(w.trace["net"]) != 0 || len(w.serverLogs["net"]) != 0 || len(w.messages) != 0 {
		t.Fatal("late confirmation was exposed")
	}
}

func TestIncompleteServiceBatchesAreBoundedAndFailClosed(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	w := m.space("owner")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := girc.New(girc.Config{Nick: "alice", User: "test", Server: "example.org"})
	c := &connection{manager: m, owner: w, client: client, ctx: ctx, cancel: cancel, config: NetworkConfig{ID: "net"}, serviceBatches: map[string]string{}}
	for i := 0; i < 128; i++ {
		c.serviceBatches[fmt.Sprint(i)] = "chanserv:stale"
	}
	r := &serviceRequest{label: "chanserv:current", client: client, done: make(chan struct{})}
	c.serviceRequest = r
	c.receive(client, *girc.ParseEvent("@label=chanserv:current :server.example BATCH +overflow labeled-response"))
	if len(c.serviceBatches) != 128 || ctx.Err() == nil {
		t.Fatal("incomplete batches were not bounded")
	}
	serviceStatus(t, r.err, 502)
	c.receive(client, *girc.ParseEvent("@batch=overflow :ChanServ!ChanServ@localhost NOTICE alice :To confirm, run this command: /CS UNREGISTER #private secret"))
	if len(w.trace["net"]) != 0 {
		t.Fatal("untrackable confirmation was exposed")
	}
}
