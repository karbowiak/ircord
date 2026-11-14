package api

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

const (
	// offlineCacheBytes caps the fetched originals kept for sharing between viewers.
	offlineCacheBytes = 128 << 20
	// offlineCacheEntries caps how many originals the cache keeps at once.
	offlineCacheEntries = 64
	// offlineCacheTime is how long a fetched original can be shared.
	offlineCacheTime = 5 * time.Minute
)

// offlineFillSlots is how many third-party downloads can run at once.
const offlineFillSlots = 4

var (
	errOfflineTooLarge = errors.New("file is too large for offline storage")
	errOfflineBusy     = errors.New("offline download capacity reached")
)

// offlineCache keeps recently fetched third-party originals on disk, keyed by
// URL. Concurrent requests for one URL share a single fetch, and later viewers
// within the lifetime read the saved copy. Entries hold an unlinked temporary
// file, so nothing is left behind when the process stops.
//
// The cache is keyed by URL only. Callers must authorize each viewer for the
// URL before reading an entry.
type offlineCache struct {
	maxFile    int64
	maxBytes   int64
	maxEntries int
	ttl        time.Duration
	now        func() time.Time

	// fills bounds running fetches, which continue after their first viewer
	// leaves. With maxFile it also bounds disk used by unfinished downloads.
	fills chan struct{}

	mu      sync.Mutex
	entries map[string]*offlineEntry
	total   int64
}

type offlineEntry struct {
	key     string
	done    chan struct{}
	file    *os.File
	size    int64
	err     error
	expires time.Time
	cached  bool // listed in entries; guarded by the cache mutex
	refs    int  // viewers reading the file; guarded by the cache mutex
	closed  bool
}

func newOfflineCache(maxFile, maxBytes int64, maxEntries int, ttl time.Duration) *offlineCache {
	return &offlineCache{maxFile: maxFile, maxBytes: maxBytes, maxEntries: maxEntries, ttl: ttl, now: time.Now, entries: map[string]*offlineEntry{}, fills: make(chan struct{}, offlineFillSlots)}
}

// offlineFetch writes the original for key to dst. It must stop when ctx ends.
type offlineFetch func(ctx context.Context, dst io.Writer) error

// get returns a reader for the original at key, fetching it when no live copy
// or in-flight fetch exists. The fetch runs under its own deadline, so one
// viewer leaving does not fail the others sharing it. The caller must call
// release when finished with the reader.
func (c *offlineCache) get(ctx context.Context, key string, fetchTime time.Duration, fetch offlineFetch) (reader *io.SectionReader, release func(), err error) {
	c.mu.Lock()
	c.sweepLocked(nil)
	entry := c.entries[key]
	if entry == nil {
		// Fetches outlive the viewer that started them, so they have their
		// own bound: at most cap(fills) run at once.
		select {
		case c.fills <- struct{}{}:
		default:
			c.mu.Unlock()
			return nil, nil, errOfflineBusy
		}
		entry = &offlineEntry{key: key, done: make(chan struct{}), cached: true}
		c.entries[key] = entry
		go c.fill(entry, fetchTime, fetch)
	}
	entry.refs++
	c.mu.Unlock()

	select {
	case <-entry.done:
	case <-ctx.Done():
		c.release(entry)
		return nil, nil, ctx.Err()
	}
	if entry.err != nil {
		c.release(entry)
		return nil, nil, entry.err
	}
	return io.NewSectionReader(entry.file, 0, entry.size), func() { c.release(entry) }, nil
}

func (e *offlineEntry) expired(now time.Time) bool {
	select {
	case <-e.done:
		return e.err != nil || now.After(e.expires)
	default:
		return false
	}
}

func (c *offlineCache) fill(entry *offlineEntry, fetchTime time.Duration, fetch offlineFetch) {
	file, size, err := c.fetchToFile(fetchTime, fetch)
	c.mu.Lock()
	entry.file, entry.size, entry.err = file, size, err
	entry.expires = c.now().Add(c.ttl)
	close(entry.done)
	<-c.fills
	if err != nil {
		// Failures are not shared beyond the viewers already waiting.
		c.dropLocked(entry)
	} else {
		c.total += size
		c.evictLocked(entry)
	}
	c.mu.Unlock()
}

func (c *offlineCache) fetchToFile(fetchTime time.Duration, fetch offlineFetch) (*os.File, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTime)
	defer cancel()
	file, err := os.CreateTemp("", "ircord-offline-*")
	if err != nil {
		return nil, 0, err
	}
	// The open descriptor keeps the data; the name is not needed.
	_ = os.Remove(file.Name())
	limited := &limitedWriter{dst: file, left: c.maxFile}
	if err = fetch(ctx, limited); err != nil {
		_ = file.Close()
		return nil, 0, err
	}
	return file, c.maxFile - limited.left, nil
}

func (c *offlineCache) release(entry *offlineEntry) {
	c.mu.Lock()
	entry.refs--
	if entry.refs == 0 && !entry.cached {
		entry.closeFile()
	}
	c.mu.Unlock()
}

// dropLocked forgets entry. Its file closes when the last viewer finishes.
func (c *offlineCache) dropLocked(entry *offlineEntry) {
	if !entry.cached {
		return
	}
	entry.cached = false
	delete(c.entries, entry.key)
	select {
	case <-entry.done:
		if entry.err == nil {
			c.total -= entry.size
		}
		if entry.refs == 0 {
			entry.closeFile()
		}
	default:
	}
}

// sweepLocked drops every expired entry except keep, so old copies do not hold
// disk space and descriptors after their lifetime.
func (c *offlineCache) sweepLocked(keep *offlineEntry) {
	now := c.now()
	for _, entry := range c.entries {
		if entry != keep && entry.expired(now) {
			c.dropLocked(entry)
		}
	}
}

// evictLocked drops expired entries, then the oldest ones, until the cache fits.
func (c *offlineCache) evictLocked(keep *offlineEntry) {
	c.sweepLocked(keep)
	for c.total > c.maxBytes || len(c.entries) > c.maxEntries {
		var oldest *offlineEntry
		for _, entry := range c.entries {
			select {
			case <-entry.done:
			default:
				continue
			}
			if entry != keep && (oldest == nil || entry.expires.Before(oldest.expires)) {
				oldest = entry
			}
		}
		if oldest == nil {
			return
		}
		c.dropLocked(oldest)
	}
}

func (e *offlineEntry) closeFile() {
	if e.file != nil && !e.closed {
		e.closed = true
		_ = e.file.Close()
	}
}

// limitedWriter fails with errOfflineTooLarge once more than left bytes arrive.
type limitedWriter struct {
	dst  io.Writer
	left int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.left {
		return 0, errOfflineTooLarge
	}
	n, err := w.dst.Write(p)
	w.left -= int64(n)
	return n, err
}
