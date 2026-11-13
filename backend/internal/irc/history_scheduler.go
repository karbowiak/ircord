package irc

import (
	"time"

	"github.com/lrstanley/girc"
)

// Admission bounds both API waiters and background work per connection.
const historyWaiterLimit = 16
const historyCatchupLimit = 64

type historyJob struct {
	target  string
	due     time.Time
	backoff time.Duration
}

type historyScheduler struct {
	slots   chan struct{}
	wake    chan struct{}
	queue   []historyJob
	pending map[string]bool
	started bool
	scan    bool
	cursor  string
	scanGen uint64
	passGen uint64
}

func (c *connection) historyScheduling() *historyScheduler {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.historyScheduler == nil {
		c.historyScheduler = &historyScheduler{slots: make(chan struct{}, historyWaiterLimit), wake: make(chan struct{}, 1), pending: map[string]bool{}}
	}
	return c.historyScheduler
}

func (c *connection) scheduleHistory(target string) {
	s := c.historyScheduling()
	c.mu.Lock()
	if s.pending[target] {
		c.mu.Unlock()
		return
	}
	if len(s.pending) < historyCatchupLimit {
		s.pending[target] = true
		s.queue = append(s.queue, historyJob{target: target})
	} else {
		// BeginGap already recorded the work; rescan durable gaps as capacity
		// becomes available instead of retaining an unbounded target list.
		s.scan = true
		s.scanGen++
	}
	if !s.started {
		s.started = true
		c.workers.Add(1)
		c.manager.wg.Add(1)
		go c.runHistoryScheduler(s)
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	c.mu.Unlock()
}

func (c *connection) runHistoryScheduler(s *historyScheduler) {
	defer c.workers.Done()
	defer c.manager.wg.Done()
	for c.ctx.Err() == nil {
		c.mu.Lock()
		now := time.Now()
		index := -1
		wait := time.Second
		for i, job := range s.queue {
			if !job.due.After(now) {
				index = i
				break
			}
			if d := time.Until(job.due); d < wait {
				wait = d
			}
		}
		var job historyJob
		if index >= 0 {
			job = s.queue[index]
			copy(s.queue[index:], s.queue[index+1:])
			s.queue = s.queue[:len(s.queue)-1]
		}
		scan := s.scan && len(s.pending) < historyCatchupLimit
		c.mu.Unlock()
		added := false
		if scan {
			// A bounded pass advances past already queued or closed gaps without
			// making every overflowed target wait for a separate timer tick.
			for i := 0; i < 32; i++ {
				if c.scanHistoryGap(s) {
					added = true
				}
				c.mu.Lock()
				stopScan := len(s.pending) >= historyCatchupLimit || !s.scan
				c.mu.Unlock()
				if stopScan {
					break
				}
			}
		}
		if index >= 0 {
			retry, failed := c.catchupOnce(job.target)
			c.mu.Lock()
			if retry && c.ctx.Err() == nil {
				if failed {
					if job.backoff == 0 {
						job.backoff = time.Second
					} else {
						job.backoff = min(job.backoff*2, 30*time.Second)
					}
					job.due = time.Now().Add(job.backoff)
				} else {
					job.backoff = 0
					job.due = time.Time{}
				}
				s.queue = append(s.queue, job)
			} else {
				delete(s.pending, job.target)
			}
			c.mu.Unlock()
			continue
		}
		if added {
			continue
		}
		timer := time.NewTimer(wait)
		select {
		case <-c.ctx.Done():
			timer.Stop()
			return
		case <-s.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

// A lexicographic cursor bounds each scan step without collecting all targets.
func (c *connection) scanHistoryGap(s *historyScheduler) bool {
	if s.cursor == "" {
		c.mu.Lock()
		s.passGen = s.scanGen
		c.mu.Unlock()
	}
	w := c.owner
	w.mu.Lock()
	var candidate Conversation
	for _, conv := range w.conversations {
		if conv.NetworkID != c.config.ID || conv.ID <= s.cursor || candidate.ID != "" && conv.ID >= candidate.ID {
			continue
		}
		if conv.Kind != "direct" && (conv.Kind != "channel" || !conv.Joined) {
			continue
		}
		candidate = conv
	}
	w.mu.Unlock()
	if candidate.ID == "" {
		s.cursor = ""
		c.mu.Lock()
		if s.passGen == s.scanGen {
			s.scan = false
		}
		c.mu.Unlock()
		return false
	}
	s.cursor = candidate.ID
	target := c.target(candidate)
	if c.owner.ownerID != partyControlOwner && c.managedPartyTarget(target) {
		return false
	}
	mark, err := w.store.Watermark(w.ownerID, candidate.ID)
	if err != nil || !mark.Gap {
		return false
	}
	c.mu.Lock()
	if !s.pending[target] && len(s.pending) < historyCatchupLimit {
		s.pending[target] = true
		s.queue = append(s.queue, historyJob{target: target})
		c.mu.Unlock()
		return true
	}
	c.mu.Unlock()
	return false
}

func (c *connection) catchup(target string) {
	if c.owner.ownerID != partyControlOwner && c.managedPartyTarget(target) {
		return
	}
	if c.owner.store == nil {
		_ = c.enqueue(&girc.Event{Command: "CHATHISTORY", Params: []string{"LATEST", target, "*", "100"}})
		return
	}
	w := c.owner
	w.mu.Lock()
	conv := c.conversation(target)
	err := w.store.BeginGap(w.ownerID, conv.ID)
	if err != nil {
		w.storageErr = err
		w.logStorageErr()
	}
	w.mu.Unlock()
	if err == nil {
		c.scheduleHistory(target)
	}
}
