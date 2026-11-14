package irc

import "time"

const profileExpiryRetry = 5 * time.Second

// One manager-owned loop handles all deadlines. It is joined by Close and
// keeps no separate references to removed workspaces.
func (w *workspace) wakeProfileExpiry() {
	if w.manager == nil {
		return
	}
	select {
	case w.manager.profileExpiryWake <- struct{}{}:
	default:
	}
}

func (m *Manager) runProfileExpiry() {
	defer m.wg.Done()
	select {
	case <-m.ctx.Done():
		return
	case <-m.profileExpiryWake:
	}
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		m.mu.Lock()
		spaces := make([]*workspace, 0, len(m.owners))
		for _, w := range m.owners {
			spaces = append(spaces, w)
		}
		m.mu.Unlock()
		var next time.Time
		for _, w := range spaces {
			w.mu.Lock()
			if !w.deleted && m.ctx.Err() == nil {
				if deadline := w.expireProfiles(time.Now()); !deadline.IsZero() && (next.IsZero() || deadline.Before(next)) {
					next = deadline
				}
			}
			w.mu.Unlock()
		}
		// Do not retain removed workspaces while the loop is idle.
		spaces = nil
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		var timerC <-chan time.Time
		if !next.IsZero() {
			delay := time.Until(next)
			if delay < 0 {
				delay = 0
			}
			timer.Reset(delay)
			timerC = timer.C
		}
		select {
		case <-m.ctx.Done():
			return
		case <-m.profileExpiryWake:
		case <-timerC:
		}
	}
}

// expireProfiles requires w.mu. A failed expiration restores the prior
// in-memory state and retries its durable commit; it never publishes a clear.
func (w *workspace) expireProfiles(now time.Time) time.Time {
	var next time.Time
	consider := func(deadline *time.Time) bool {
		if deadline == nil {
			return false
		}
		if deadline.After(now) {
			if next.IsZero() || deadline.Before(next) {
				next = *deadline
			}
			return false
		}
		return true
	}
	retry := func() {
		candidate := now.Add(profileExpiryRetry)
		if next.IsZero() || candidate.Before(next) {
			next = candidate
		}
	}
	due := false
	if w.self.StatusExpiresAt != nil && consider(w.self.StatusExpiresAt) {
		due = true
	}
	for id := range w.configs {
		n := w.networks[id]
		if n.Profile != nil && n.Profile.StatusExpiresAt != nil && consider(n.Profile.StatusExpiresAt) {
			due = true
		}
	}
	if !due {
		return next
	}
	if w.profileExpiryStorageErr {
		w.storageErr = nil
		w.profileExpiryStorageErr = false
	}
	if w.storageErr != nil {
		retry()
		return next
	}
	if consider(w.self.StatusExpiresAt) {
		before := w.self
		self := before
		self.StatusText, self.StatusEmoji, self.StatusExpiresAt = "", "", nil
		w.self, w.members[self.ID] = self, self
		w.emit("profile.updated", self)
		if w.storageErr != nil {
			w.self, w.members[self.ID] = before, before
			w.profileExpiryStorageErr = true
			retry()
			return next
		}
	}
	for id := range w.configs {
		n, exists := w.networks[id]
		if !exists || n.Profile == nil || !consider(n.Profile.StatusExpiresAt) {
			continue
		}
		before := w.networkSelf(id)
		self := before
		self.StatusText, self.StatusEmoji, self.StatusExpiresAt = "", "", nil
		changed := changedProfileKeys(before, self)
		w.saveNetworkSelf(id, self)
		if w.storageErr != nil {
			w.networks[id] = n
			w.profileExpiryStorageErr = true
			retry()
			return next
		}
		// An offline edit reconciles on reconnect. A live clear is sent only
		// for values still matching this backend's publication history.
		if c := w.connections[id]; c != nil {
			c.mu.Lock()
			ready := c.ready && c.profileCheckDone && c.profileCheck == nil
			c.mu.Unlock()
			if ready {
				published, values := c.publishedProfile(), profileValues(before)
				var safe []string
				for _, key := range changed {
					if last, ok := published[key]; ok && last == values[key] {
						safe = append(safe, key)
					}
				}
				c.pushProfileKeys(self, safe)
			}
		}
	}
	return next
}
