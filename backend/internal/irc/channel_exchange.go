package irc

import (
	"context"
	"strings"
	"time"

	"github.com/lrstanley/girc"
)

type channelExchangeReply struct {
	client          *girc.Client
	label, terminal string
	batches         map[string]bool
	events          []girc.Event
	done            chan struct{}
	err             error
}

func (c *connection) handleChannelExchangeReply(client *girc.Client, e girc.Event) {
	if e.Command == "BATCH" {
		if len(e.Params) == 0 || len(e.Params[0]) < 2 || (e.Params[0][0] != '+' && e.Params[0][0] != '-') || strings.TrimLeft(e.Params[0], "+-") == "" {
			return
		}
		if e.Params[0][0] == '+' && (len(e.Params) < 2 || e.Params[1] == "") {
			return
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.channelRequest
	if r == nil {
		return
	}
	if r.client != nil && (client != r.client || c.client != r.client) {
		r.err = ErrDisconnected
		select {
		case <-r.done:
		default:
			close(r.done)
		}
		return
	}
	label := e.Tags["label"]
	batch := e.Tags["batch"]
	if e.Command == "BATCH" {
		ref := strings.TrimLeft(e.Params[0], "+-")
		if e.Params[0][0] == '+' && (label == r.label || r.batches[batch]) {
			r.batches[ref] = true
			return
		}
		if e.Params[0][0] == '-' && r.batches[ref] {
			delete(r.batches, ref)
			if len(r.batches) == 0 {
				select {
				case <-r.done:
				default:
					close(r.done)
				}
			}
			return
		}
	}
	if label != r.label && !r.batches[batch] {
		return
	}
	if len(r.events) >= 256 {
		r.err = ErrBusy
	} else {
		r.events = append(r.events, *e.Copy())
	}
	if e.Command == "FAIL" || len(e.Command) == 3 && e.Command[0] >= '4' && e.Command[0] <= '5' {
		r.err = serviceError(503, "The IRC server rejected the channel change. Check your permissions and server limits.")
	}
	if batch == "" && (r.terminal == "" || e.Command == r.terminal || e.Command == "ACK" || r.err != nil) {
		select {
		case <-r.done:
		default:
			close(r.done)
		}
	}
}
func (c *connection) channelExchange(ctx context.Context, event *girc.Event, terminal string) ([]girc.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	// Keep the internal label prefix shared by the writer fence and trace filter.
	r := &channelExchangeReply{label: "party:" + id(), terminal: terminal, batches: map[string]bool{}, done: make(chan struct{})}
	c.mu.Lock()
	r.client = c.client
	if expected, ok := ctx.Value(channelClientKey{}).(*girc.Client); ok && (expected != r.client || !c.ready) {
		c.mu.Unlock()
		return nil, ErrDisconnected
	}
	if c.channelRequest != nil {
		c.mu.Unlock()
		return nil, ErrBusy
	}
	c.channelRequest = r
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.channelRequest == r {
			c.channelRequest = nil
		}
		c.mu.Unlock()
	}()
	event.Tags = girc.Tags{"label": r.label}
	event.Sensitive = true
	if e := c.enqueue(event); e != nil {
		return nil, e
	}
	select {
	case <-ctx.Done():
		return nil, serviceError(504, "The IRC server did not confirm the change. Refresh the channel before retrying.")
	case <-c.ctx.Done():
		return nil, ErrDisconnected
	case <-r.done:
		c.mu.Lock()
		defer c.mu.Unlock()
		return r.events, r.err
	}
}
