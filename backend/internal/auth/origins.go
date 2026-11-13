package auth

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"ircord/backend/internal/storage"
)

const originFlushInterval = 30 * time.Second
const originCacheLimit = 10000

type originEntry struct {
	address string
	seen    time.Time
	dirty   bool
}

type originTracker struct {
	mu       sync.Mutex
	entries  map[string]originEntry
	store    *storage.Store
	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

func newOriginTracker(store *storage.Store) *originTracker {
	t := &originTracker{entries: map[string]originEntry{}, store: store, stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(t.done)
		tick := time.NewTicker(originFlushInterval)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				t.flush()
			case <-t.stop:
				t.flush()
				return
			}
		}
	}()
	return t
}

func (t *originTracker) flush() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for owner, entry := range t.entries {
		if !entry.dirty {
			continue
		}
		if err := t.store.SaveOrigin(owner, entry.address); err != nil {
			slog.Error("Saving a client address failed")
			continue
		}
		entry.dirty = false
		t.entries[owner] = entry
	}
}

func (t *originTracker) observe(owner, address string) error {
	if t == nil || address == "" {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, exists := t.entries[owner]
	if !exists {
		if len(t.entries) >= originCacheLimit {
			oldest := ""
			for key, value := range t.entries {
				if oldest == "" || value.seen.Before(t.entries[oldest].seen) {
					oldest = key
				}
			}
			old := t.entries[oldest]
			if old.dirty {
				if err := t.store.SaveOrigin(oldest, old.address); err != nil {
					return err
				}
			}
			delete(t.entries, oldest)
		}
		// The first verified address is durable before a home connection starts.
		if err := t.store.SaveOrigin(owner, address); err != nil {
			return err
		}
		entry.address = address
	} else if entry.address != address {
		entry.address, entry.dirty = address, true
	}
	entry.seen = time.Now()
	t.entries[owner] = entry
	return nil
}

// Origin returns the newest verified address, including a coalesced pending update.
func (s *Service) Origin(owner string) (string, error) {
	if s.origins == nil {
		return "", nil
	}
	s.origins.mu.Lock()
	defer s.origins.mu.Unlock()
	if entry, ok := s.origins.entries[owner]; ok {
		return entry.address, nil
	}
	return s.store.Origin(owner)
}

func (s *Service) observeOrigin(owner string, r *http.Request) error {
	if s.origins == nil {
		return nil
	}
	address, verified := s.resolvedClientIP(r)
	if !verified {
		return nil
	}
	return s.origins.observe(owner, address)
}

type signupOriginKey struct{}

func (s *Service) withSignupOrigin(ctx context.Context, r *http.Request) context.Context {
	if s.origins == nil {
		return ctx
	}
	address, verified := s.resolvedClientIP(r)
	if !verified {
		return ctx
	}
	return context.WithValue(ctx, signupOriginKey{}, address)
}
