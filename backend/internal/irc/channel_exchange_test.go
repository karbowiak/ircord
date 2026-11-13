package irc

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
)

type channelExchangeWriteObserver chan string

func (w channelExchangeWriteObserver) Write(p []byte) (int, error) {
	select {
	case w <- string(p):
	default:
	}
	return len(p), nil
}

func TestChannelExchangeIgnoresMalformedBatch(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "labeled", true: "nested"}[nested], func(t *testing.T) {
			c := &connection{}
			r := &channelExchangeReply{label: "party:fixture", batches: map[string]bool{}, done: make(chan struct{})}
			c.channelRequest = r
			if nested {
				c.handleChannelExchangeReply(nil, *girc.ParseEvent("@label=party:fixture :server BATCH +parent labeled-response"))
				c.handleChannelExchangeReply(nil, *girc.ParseEvent("@batch=parent :server BATCH +child nested"))
			}
			for _, line := range []string{
				":server BATCH :",
				"@label=party:fixture :server BATCH",
				"@label=party:fixture :server BATCH :",
				"@label=party:fixture :server BATCH +",
				"@label=party:fixture :server BATCH -",
				"@label=party:fixture :server BATCH +-- nested",
				"@label=party:fixture :server BATCH missing-sign",
				"@label=party:fixture :server BATCH +missing-type",
				"@batch=child :server BATCH -",
			} {
				event := girc.ParseEvent(line)
				if event == nil {
					t.Fatalf("girc did not parse %q", line)
				}
				c.handleChannelExchangeReply(nil, *event)
				if len(r.events) != 0 || r.err != nil || len(r.batches) != map[bool]int{false: 0, true: 2}[nested] || nested && (!r.batches["parent"] || !r.batches["child"]) {
					t.Fatalf("malformed BATCH changed exchange for %q: %+v", line, r)
				}
				select {
				case <-r.done:
					t.Fatalf("malformed BATCH completed exchange for %q", line)
				default:
				}
			}
			if nested {
				c.handleChannelExchangeReply(nil, *girc.ParseEvent("@batch=child :server 324 alice #fixture +n"))
				c.handleChannelExchangeReply(nil, *girc.ParseEvent(":server BATCH -child"))
				select {
				case <-r.done:
					t.Fatal("nested batch completed before parent")
				default:
				}
				c.handleChannelExchangeReply(nil, *girc.ParseEvent(":server BATCH -parent"))
			} else {
				c.handleChannelExchangeReply(nil, *girc.ParseEvent("@label=party:fixture :server ACK"))
			}
			select {
			case <-r.done:
			default:
				t.Fatal("valid response did not finish the exchange")
			}
			if len(r.events) != 1 || r.err != nil {
				t.Fatalf("valid reply lost after malformed BATCH: %+v", r)
			}
		})
	}
}

func TestReceiveIgnoresEmptyBatchParameter(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	client := girc.New(girc.Config{Server: "fixture", Nick: "alice", User: "alice"})
	r := &channelExchangeReply{client: client, label: "party:fixture", batches: map[string]bool{}, done: make(chan struct{})}
	c := &connection{manager: m, owner: m.space("alice"), ctx: context.Background(), client: client, config: NetworkConfig{ID: "net"}, channelRequest: r}
	c.receive(client, *girc.ParseEvent(":server BATCH :"))
	if len(r.events) != 0 || len(r.batches) != 0 || r.err != nil {
		t.Fatal("empty BATCH modified the active exchange")
	}
	select {
	case <-r.done:
		t.Fatal("empty BATCH completed the active exchange")
	default:
	}
}

func TestMalformedBatchDoesNotCompleteStaleExchange(t *testing.T) {
	old := girc.New(girc.Config{Server: "old", Nick: "alice", User: "alice"})
	current := girc.New(girc.Config{Server: "current", Nick: "alice", User: "alice"})
	r := &channelExchangeReply{client: old, label: "party:fixture", batches: map[string]bool{}, done: make(chan struct{})}
	c := &connection{client: current, channelRequest: r}
	c.handleChannelExchangeReply(current, *girc.ParseEvent(":server BATCH :"))
	if r.err != nil {
		t.Fatalf("malformed BATCH changed stale exchange: %v", r.err)
	}
	select {
	case <-r.done:
		t.Fatal("malformed BATCH completed the stale exchange")
	default:
	}
	c.handleChannelExchangeReply(current, *girc.ParseEvent("@label=party:fixture :server ACK"))
	if !errors.Is(r.err, ErrDisconnected) {
		t.Fatalf("valid response did not fence stale exchange: %v", r.err)
	}
	select {
	case <-r.done:
	default:
		t.Fatal("stale exchange did not complete after a valid response")
	}
}

func TestChannelExchangeBoundsBatchReplies(t *testing.T) {
	c := &connection{}
	r := &channelExchangeReply{label: "party:fixture", batches: map[string]bool{}, done: make(chan struct{})}
	c.channelRequest = r
	c.handleChannelExchangeReply(nil, *girc.ParseEvent("@label=party:fixture :server BATCH +parent labeled-response"))
	c.handleChannelExchangeReply(nil, *girc.ParseEvent("@batch=parent :server BATCH +child nested"))
	for range 257 {
		c.handleChannelExchangeReply(nil, *girc.ParseEvent("@batch=child :server 322 me #fixture 1 :topic"))
	}
	if len(r.events) != 256 || !errors.Is(r.err, ErrBusy) {
		t.Fatalf("batch limit not enforced: %d events, %v", len(r.events), r.err)
	}
	c.handleChannelExchangeReply(nil, *girc.ParseEvent(":server BATCH -child"))
	select {
	case <-r.done:
		t.Fatal("nested batch closed the exchange early")
	default:
	}
	c.handleChannelExchangeReply(nil, *girc.ParseEvent(":server BATCH -parent"))
	select {
	case <-r.done:
	default:
		t.Fatal("outer batch did not finish the exchange")
	}
}

func TestChannelExchangeCancellationDropsQueuedRequest(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	connectionCtx, stopConnection := context.WithCancel(m.ctx)
	defer stopConnection()
	writes := make(channelExchangeWriteObserver, 1)
	client := girc.New(girc.Config{Server: "fixture", Nick: "alice", User: "alice", Debug: writes})
	select {
	case <-writes:
	default:
	}
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	c := &connection{manager: m, owner: m.space("alice"), ctx: connectionCtx, config: NetworkConfig{ID: "net"}, client: client, transport: conn, ready: true, queue: make(chan []*girc.Event, 2)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	event := &girc.Event{Command: "MODE", Params: []string{"#fixture", "+k", "fixture-secret"}}
	go func() {
		_, err := c.channelExchange(ctx, event, "")
		done <- err
	}()
	eventually(t, func() bool { return len(c.queue) == 1 })
	cancel()
	select {
	case err := <-done:
		serviceStatus(t, err, 504)
	case <-time.After(time.Second):
		t.Fatal("canceled exchange did not return")
	}
	if !event.Sensitive {
		t.Fatal("channel exchange did not mark its request sensitive")
	}
	if err := c.enqueue(&girc.Event{Command: "PING", Params: []string{"fence-sentinel"}}); err != nil {
		t.Fatal(err)
	}
	m.wg.Add(1)
	go c.writer()
	select {
	case line := <-writes:
		if !strings.Contains(line, "fence-sentinel") {
			t.Fatalf("canceled queued exchange reached the transport: %s", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("writer did not reach the sentinel after the canceled request")
	}
	stopConnection()
}

func TestChannelExchangeRequestsStayOutOfDiagnostics(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	w := m.space("alice")
	c := &connection{manager: m, owner: w, config: NetworkConfig{ID: "net"}}
	c.capture(girc.Event{Command: "MODE", Params: []string{"#fixture", "+k", "fixture-secret"}, Tags: girc.Tags{"label": "party:fixture"}, Sensitive: true}, true)
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.trace["net"]) != 0 {
		t.Fatal("sensitive channel exchange appeared in diagnostics")
	}
}

func TestMatchesHomeEndpointKeepsNetworkIDSeparate(t *testing.T) {
	m := New(Options{Home: NetworkConfig{ID: "home", Host: "IRC.Example", Port: 6697, TLS: true}})
	defer m.Close()
	for _, tc := range []struct {
		cfg         NetworkConfig
		match, home bool
	}{
		{NetworkConfig{ID: "home", Host: "irc.example", Port: 6697, TLS: true}, true, true},
		{NetworkConfig{ID: "alias", Host: "irc.example", Port: 6697, TLS: true}, true, false},
		{NetworkConfig{ID: "home", Host: "other", Port: 6697, TLS: true}, false, false},
		{NetworkConfig{ID: "home", Host: "irc.example", Port: 6667, TLS: true}, false, false},
		{NetworkConfig{ID: "home", Host: "irc.example", Port: 6697}, false, false},
	} {
		c := &connection{manager: m, config: tc.cfg}
		if got := m.matchesHomeEndpoint(tc.cfg); got != tc.match {
			t.Fatalf("endpoint match = %t, want %t for %+v", got, tc.match, tc.cfg)
		}
		if got := c.home(); got != tc.home {
			t.Fatalf("home = %t, want %t for %+v", got, tc.home, tc.cfg)
		}
	}
	m.options.Home.ID = ""
	if m.matchesHomeEndpoint(NetworkConfig{Host: "irc.example", Port: 6697, TLS: true}) {
		t.Fatal("unconfigured home matched an endpoint")
	}
}
