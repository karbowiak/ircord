package irc

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lrstanley/girc"
)

type listRequest struct {
	items    []ChannelListing
	done     chan struct{}
	overflow bool
}

func (c *connection) listItem(e girc.Event) {
	if e.Command != "322" && e.Command != "323" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.listRequest
	if r == nil {
		return
	}
	if e.Command == "322" && len(e.Params) >= 4 {
		if len(r.items) < 100000 {
			n, _ := strconv.Atoi(e.Params[2])
			r.items = append(r.items, ChannelListing{Name: e.Params[1], Users: n, Topic: e.Last()})
		} else {
			r.overflow = true
		}
	}
	if e.Command == "323" {
		select {
		case <-r.done:
		default:
			close(r.done)
		}
	}
}

func (m *Manager) Channels(ctx context.Context, owner, network, q string, limit int) ([]ChannelListing, error) {
	if limit < 1 || limit > 100 || !clean(q, 200) {
		return nil, fmt.Errorf("invalid channel query")
	}
	c, err := m.connection(owner, network)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	select {
	case c.listGate <- struct{}{}:
		defer func() { <-c.listGate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, ErrDisconnected
	}
	c.mu.Lock()
	fresh := time.Since(c.listAt) < 3*time.Minute
	items := clone(c.listCache)
	c.mu.Unlock()
	if c.owner.store != nil {
		cached, ok, err := c.owner.store.CachedChannels(owner, network, q, limit, c.createdAt)
		if err != nil {
			return nil, ErrStorage
		}
		if ok {
			return cached, nil
		}
		fresh = false
	}
	if !fresh {
		select {
		case m.listSlots <- struct{}{}:
			defer func() { <-m.listSlots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.ctx.Done():
			return nil, ErrDisconnected
		}
		requestStarted := time.Now()
		req := &listRequest{done: make(chan struct{}), items: []ChannelListing{}}
		c.mu.Lock()
		c.listRequest = req
		c.mu.Unlock()
		defer func() {
			c.mu.Lock()
			if c.listRequest == req {
				c.listRequest = nil
			}
			c.mu.Unlock()
		}()
		if err = c.enqueue(&girc.Event{Command: "LIST"}); err != nil {
			return nil, err
		}
		select {
		case <-req.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.done:
			return nil, ErrDisconnected
		case <-c.ctx.Done():
			return nil, ErrDisconnected
		}
		c.mu.Lock()
		items = req.items
		overflow := req.overflow
		if !overflow && c.owner.store == nil {
			c.listCache = items
			c.listAt = time.Now()
		}
		c.mu.Unlock()
		if overflow {
			return nil, fmt.Errorf("network channel directory exceeds the 100000-channel limit")
		}
		if c.owner.store != nil {
			if err = c.owner.store.SaveChannels(owner, network, items, requestStarted); err != nil {
				return nil, ErrStorage
			}
			result, _, err := c.owner.store.CachedChannels(owner, network, q, limit, c.createdAt)
			return result, err
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Users == items[j].Users {
			return items[i].Name < items[j].Name
		}
		return items[i].Users > items[j].Users
	})
	result := []ChannelListing{}
	q = strings.ToLower(q)
	for _, v := range items {
		if strings.Contains(strings.ToLower(v.Name+" "+v.Topic), q) {
			result = append(result, v)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}
