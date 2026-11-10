package irc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lrstanley/girc"
)

type ChannelListEntry struct {
	Mask  string     `json:"mask"`
	SetBy string     `json:"setBy,omitempty"`
	SetAt *time.Time `json:"setAt,omitempty"`
}
type modeListRequest struct {
	target, mode, label string
	items               []ChannelListEntry
	done                chan struct{}
	err                 error
}

func (c *connection) modeListItem(e girc.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	req := c.modeList
	if req == nil {
		return
	}
	label := e.Tags["label"]
	if label == "" {
		label = c.batchLabels[e.Tags["batch"]]
	}
	if c.caps["labeled-response"] && label != req.label {
		return
	}
	entry, end := "367", "368"
	switch req.mode {
	case "e":
		entry, end = "348", "349"
	case "I":
		entry, end = "346", "347"
	case "q":
		entry, end = "728", "729"
	}
	finish := func() {
		select {
		case <-req.done:
		default:
			close(req.done)
		}
	}
	if e.Command == end {
		finish()
		return
	}
	if e.Command == "FAIL" || e.Command == "403" || e.Command == "442" || e.Command == "482" {
		req.err = fmt.Errorf("IRC rejected the channel list request")
		finish()
		return
	}
	if e.Command != entry || len(e.Params) < 3 || !strings.EqualFold(e.Params[1], req.target) {
		return
	}
	offset := 2
	if req.mode == "q" && e.Params[2] == "q" {
		offset++
	}
	if len(e.Params) <= offset || len(req.items) >= 10000 {
		return
	}
	item := ChannelListEntry{Mask: e.Params[offset]}
	if len(e.Params) > offset+1 {
		item.SetBy = e.Params[offset+1]
	}
	if len(e.Params) > offset+2 {
		if seconds, err := strconv.ParseInt(e.Params[offset+2], 10, 64); err == nil {
			at := time.Unix(seconds, 0).UTC()
			item.SetAt = &at
		}
	}
	req.items = append(req.items, item)
}
func (m *Manager) ModeList(ctx context.Context, owner, conversation, mode string) ([]ChannelListEntry, error) {
	if e := m.partyAccess(owner, conversation); owner != partyControlOwner && e != nil {
		return nil, e
	}
	w := m.space(owner)
	w.mu.Lock()
	conv, ok := w.conversations[conversation]
	c := w.connections[conv.NetworkID]
	network := w.networks[conv.NetworkID]
	w.mu.Unlock()
	if !ok || c == nil {
		return nil, ErrNotFound
	}
	if conv.Party != nil {
		return nil, ErrForbidden
	}
	groups := strings.Split(network.ISupport["CHANMODES"], ",")
	if conv.Kind != "channel" || len(mode) != 1 || len(groups) == 0 || !strings.Contains(groups[0], mode) {
		return nil, fmt.Errorf("not a list mode on this network")
	}
	if !strings.Contains("beIq", mode) {
		return nil, ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	select {
	case c.modeListGate <- struct{}{}:
		defer func() { <-c.modeListGate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, ErrDisconnected
	}
	req := &modeListRequest{target: c.target(conv), mode: mode, label: id(), items: []ChannelListEntry{}, done: make(chan struct{})}
	c.mu.Lock()
	c.modeList = req
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.modeList == req {
			c.modeList = nil
		}
		c.mu.Unlock()
	}()
	event := &girc.Event{Command: "MODE", Params: []string{req.target, "+" + mode}}
	if c.has("labeled-response") {
		event.Tags = girc.Tags{"label": req.label}
	}
	if err := c.enqueue(event); err != nil {
		return nil, err
	}
	select {
	case <-req.done:
		c.mu.Lock()
		items, err := clone(req.items), req.err
		c.mu.Unlock()
		return items, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, ErrDisconnected
	case <-c.done:
		return nil, ErrDisconnected
	}
}
