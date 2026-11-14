package irc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

var errSearchUnavailable = errors.New("upstream search unavailable")

const searchWait = 10 * time.Second

type searchRequest struct {
	label         string
	ctx           context.Context
	client        *girc.Client
	transport     net.Conn
	dispatch      chan struct{}
	done          chan searchReply
	abandoned     bool
	items         []girc.Event
	bytes, events int
	limit         int
	invalid       bool
}

func (req *searchRequest) charge(e girc.Event) {
	req.events++
	req.bytes += e.LenOpts(false)
	if req.events > 512 || req.bytes > 256*1024 {
		req.invalid = true
	}
	if label := e.Tags["label"]; label != "" && label != req.label {
		req.invalid = true
	}
}

// searchUpstreamResults never mutates members, history, read state, or replay.
type searchAttempt struct {
	client    *girc.Client
	transport net.Conn
}

func (m *Manager) searchUpstreamResults(ctx context.Context, owner string, in SearchQuery, c *connection, original Conversation, attempt searchAttempt, events []girc.Event) (SearchPage, error) {
	page := SearchPage{Items: []SearchResult{}, Source: &SearchSource{Requested: "upstream", Actual: "upstream", Coverage: "upstream-bounded", Completeness: "unknown"}}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return SearchPage{}, err
	}
	if w.deleted {
		return SearchPage{}, ErrNotFound
	}
	if err := m.partyAccess(owner, original.ID); err != nil {
		return SearchPage{}, err
	}
	conv, ok := w.conversations[original.ID]
	if !ok {
		return SearchPage{}, ErrNotFound
	}
	if w.connections[original.NetworkID] != c || conv.NetworkID != original.NetworkID || c.target(conv) != c.target(original) || conv.Party != nil {
		return SearchPage{}, errSearchUnavailable
	}
	seen := map[string]bool{}
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return SearchPage{}, err
		}
		if event.Source == nil || len(event.Params) != 2 || validateNickname(event.Source.Name, 100) != nil {
			return SearchPage{}, errSearchUnavailable
		}
		target := event.Params[0]
		validTarget := c.fold(target) == c.fold(c.target(conv))
		if conv.Kind == "direct" {
			validTarget = validTarget && c.fold(event.Source.Name) == c.fold(c.currentNick()) || c.fold(target) == c.fold(c.currentNick()) && c.fold(event.Source.Name) == c.fold(c.target(conv))
		}
		if !validTarget {
			return SearchPage{}, errSearchUnavailable
		}
		at, err := time.Parse(time.RFC3339Nano, event.Tags["time"])
		if err != nil {
			return SearchPage{}, errSearchUnavailable
		}
		if event.Tags[editTag] != "" {
			continue
		}
		upstreamID := event.Tags["msgid"]
		if upstreamID != "" && seen[upstreamID] {
			continue
		}
		if upstreamID != "" {
			seen[upstreamID] = true
		}
		var msg Message
		found := false
		if upstreamID != "" {
			for _, candidate := range w.messages[conv.ID] {
				if candidate.UpstreamID == upstreamID {
					msg, found = candidate, true
					break
				}
			}
			if !found && w.store != nil {
				msg, err = w.store.MessageContext(ctx, owner, conv.ID, upstreamID)
				if err == nil {
					found = true
				} else if !errors.Is(err, storage.ErrNotFound) {
					if ctx.Err() != nil {
						return SearchPage{}, ctx.Err()
					}
					return SearchPage{}, ErrStorage
				}
			}
		}
		identity := &SearchContext{Available: true, Identity: "retained"}
		senderNick := ""
		if !found {
			sender := c.knownMember(event.Source.Name, event.Tags["account"])
			if sender == "" {
				sender = "search:sender:" + id()
			}
			senderNick = event.Source.Name
			msg = Message{ID: "search:" + id(), ConversationID: conv.ID, SenderID: sender, Text: event.Last(), CreatedAt: at, Kind: "message", Reactions: []Reaction{}, UpstreamID: upstreamID, Historical: true}
			if event.Command == "NOTICE" {
				msg.Kind = "notice"
			}
			if strings.HasPrefix(msg.Text, "\x01ACTION ") && strings.HasSuffix(msg.Text, "\x01") {
				msg.Kind = "action"
				msg.Text = strings.TrimSuffix(strings.TrimPrefix(msg.Text, "\x01ACTION "), "\x01")
			}
			reason := "not-retained"
			if upstreamID == "" {
				reason = "missing-msgid"
			}
			identity = &SearchContext{Available: false, Reason: reason, Identity: "ephemeral"}
		}
		if !storage.MatchesSearch(msg, storage.SearchFilter{Query: strings.TrimSpace(in.Query)}) {
			continue
		}
		page.Items = append(page.Items, SearchResult{Message: msg, Conversation: conv, Context: identity, SenderNick: senderNick})
	}
	slices.SortFunc(page.Items, func(a, b SearchResult) int {
		if n := b.Message.CreatedAt.Compare(a.Message.CreatedAt); n != 0 {
			return n
		}
		return strings.Compare(b.Message.ID, a.Message.ID)
	})
	c.mu.Lock()
	validAttempt := c.searchUsableLocked() && c.client == attempt.client && c.transport == attempt.transport
	c.mu.Unlock()
	if !validAttempt {
		return SearchPage{}, errSearchUnavailable
	}
	return clone(page), nil
}

type searchReply struct {
	items []girc.Event
	err   error
}

type searchBatch struct {
	request *searchRequest
	root    string
	depth   int
}

func (c *connection) searchUsableLocked() bool {
	return !c.searchQuarantined && c.ready && c.client != nil && c.transportAliveLocked() && c.caps["soju.im/search"] && c.caps["batch"] && c.caps["server-time"] && c.caps["message-tags"] && c.caps["labeled-response"]
}

// Called by the writer with c.mu held, immediately before dispatch.
func (c *connection) startSearchWrite(req *searchRequest, client *girc.Client) bool {
	if req == nil {
		return true
	}
	if req.abandoned || req.ctx.Err() != nil || c.searchRequest != req || c.client != client || !c.searchUsableLocked() {
		return false
	}
	req.client = client
	req.transport = c.transport
	req.dispatch <- struct{}{}
	return true
}

func searchTagValue(value string) string {
	return strings.NewReplacer("\\", "\\\\", ";", "\\:", " ", "\\s", "\r", "\\r", "\n", "\\n").Replace(value)
}

func (c *connection) upstreamSearch(ctx context.Context, target, text string, limit int) ([]girc.Event, searchAttempt, error) {
	return c.upstreamSearchWithBudgets(ctx, target, text, limit, searchWait, searchWait)
}

func (c *connection) upstreamSearchWithBudgets(ctx context.Context, target, text string, limit int, dispatchBudget, responseBudget time.Duration) ([]girc.Event, searchAttempt, error) {
	if err := ctx.Err(); err != nil {
		return nil, searchAttempt{}, err
	}
	attributes := "in=" + searchTagValue(target) + ";text=" + searchTagValue(text) + fmt.Sprintf(";limit=%d", limit)
	req := &searchRequest{label: "search:" + id(), ctx: ctx, limit: limit, dispatch: make(chan struct{}, 1), done: make(chan searchReply, 1)}
	event := &girc.Event{Command: "SEARCH", Params: []string{attributes}, Tags: girc.Tags{"label": req.label}}
	c.mu.Lock()
	lineLimit := safeMessageLineBytes(girc.DefaultMaxLineLength - girc.DefaultMaxPrefixLength)
	if c.lineBytes > 0 {
		lineLimit = min(lineLimit, c.lineBytes)
	}
	if !c.searchUsableLocked() || c.searchRequest != nil || event.LenOpts(false) >= lineLimit {
		c.mu.Unlock()
		return nil, searchAttempt{}, errSearchUnavailable
	}
	if c.searchWrites == nil {
		c.searchWrites = map[*girc.Event]*searchRequest{}
	}
	c.searchRequest = req
	select {
	case c.queue <- []*girc.Event{event}:
		c.searchWrites[event] = req
		c.trackQueuedClients([]*girc.Event{event}, c.client)
	default:
		c.searchRequest = nil
		c.mu.Unlock()
		return nil, searchAttempt{}, errSearchUnavailable
	}
	attempt := c.attemptDone
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		req.abandoned = true
		if c.searchRequest == req {
			c.searchRequest = nil
		}
		c.mu.Unlock()
	}()
	timer := time.NewTimer(dispatchBudget)
	defer timer.Stop()
	select {
	case <-req.dispatch:
	case <-ctx.Done():
		return nil, searchAttempt{}, ctx.Err()
	case <-c.ctx.Done():
		return nil, searchAttempt{}, errSearchUnavailable
	case <-attempt:
		return nil, searchAttempt{}, errSearchUnavailable
	case <-timer.C:
		return nil, searchAttempt{}, errSearchUnavailable
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(responseBudget)
	select {
	case result := <-req.done:
		c.mu.Lock()
		valid := c.searchUsableLocked() && c.client == req.client && c.transport == req.transport
		attempt := searchAttempt{client: req.client, transport: req.transport}
		c.mu.Unlock()
		if !valid {
			return nil, searchAttempt{}, errSearchUnavailable
		}
		return result.items, attempt, result.err
	case <-ctx.Done():
		return nil, searchAttempt{}, ctx.Err()
	case <-c.ctx.Done():
		return nil, searchAttempt{}, errSearchUnavailable
	case <-attempt:
		return nil, searchAttempt{}, errSearchUnavailable
	case <-timer.C:
		return nil, searchAttempt{}, errSearchUnavailable
	}
}

// Search batches, including unknown and late responses, never reach live processing.
func (c *connection) receiveSearch(e girc.Event) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.searchQuarantined {
		return true
	}
	if e.Command == "FAIL" && len(e.Params) > 0 && strings.EqualFold(e.Params[0], "SEARCH") {
		label := e.Tags["label"]
		if label == "" {
			label = c.batchLabels[e.Tags["batch"]]
		}
		if label == "" {
			if b := c.searchBatches[e.Tags["batch"]]; b != nil && b.request != nil {
				label = b.request.label
			}
		}
		if req := c.searchRequest; req != nil && label == req.label {
			select {
			case req.done <- searchReply{err: errSearchUnavailable}:
			default:
			}
		}
		return true
	}
	if e.Command == "BATCH" && len(e.Params) > 0 && len(e.Params[0]) > 1 {
		ref := e.Params[0][1:]
		if e.Params[0][0] == '+' {
			parent := c.searchBatches[e.Tags["batch"]]
			root := len(e.Params) > 1 && e.Params[1] == "soju.im/search"
			if !root && parent == nil {
				return false
			}
			if c.searchBatches == nil {
				c.searchBatches = map[string]*searchBatch{}
			}
			if len(c.searchBatches) >= 128 || len(ref) > 128 || c.searchBatches[ref] != nil || parent != nil && parent.depth >= 8 {
				c.searchQuarantined = true
				if c.transport != nil {
					_ = c.transport.Close()
				}
				return true
			}
			b := &searchBatch{root: ref, depth: 1}
			if parent != nil {
				b.request, b.root, b.depth = parent.request, parent.root, parent.depth+1
				if len(e.Params) > 1 && e.Params[1] == "draft/multiline" && b.request != nil {
					b.request.invalid = true
				}
			}
			if root && parent == nil {
				label := e.Tags["label"]
				if label == "" {
					label = c.batchLabels[e.Tags["batch"]]
				}
				if req := c.searchRequest; req != nil && !req.abandoned && req.client != nil && req.label == label {
					b.request = req
				}
			}
			if b.request != nil {
				b.request.charge(e)
			}
			c.searchBatches[ref] = b
			return true
		}
		if b := c.searchBatches[ref]; b != nil {
			delete(c.searchBatches, ref)
			if b.request != nil {
				b.request.charge(e)
			}
			if ref == b.root && b.request != nil {
				req := b.request
				for _, other := range c.searchBatches {
					if other.root == ref {
						req.invalid = true
					}
				}
				var err error
				if req.invalid {
					err = errSearchUnavailable
				}
				select {
				case req.done <- searchReply{items: req.items, err: err}:
				default:
				}
			}
			return true
		}
	}
	if b := c.searchBatches[e.Tags["batch"]]; b != nil {
		if req := b.request; req != nil && !req.abandoned {
			req.charge(e)
			if e.Command != "PRIVMSG" && e.Command != "NOTICE" || len(req.items) >= req.limit {
				req.invalid = true
			} else if !req.invalid {
				req.items = append(req.items, *e.Copy())
			}
		}
		return true
	}
	return false
}
