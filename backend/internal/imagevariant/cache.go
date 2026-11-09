package imagevariant

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrBusy leaves the caller's reader untouched so it can serve the original.
var ErrBusy = errors.New("image processing capacity reached")

type Key struct {
	ID            string
	Width, Height int
}

type entry struct {
	image   Image
	expires time.Time
}

// Cache retains at most 32 MiB/128 derivatives for ten minutes. At most two
// callers may decode images; there is no unbounded processing/waiter queue.
// Every caller MUST authorize access before consulting this cache, including
// hits: a cached derivative cannot keep a deleted or revoked file accessible.
type Cache struct {
	mu      sync.Mutex
	entries map[Key]entry
	bytes   int
	active  int
}

func (c *Cache) Get(ctx context.Context, key Key, contentType string, load func() ([]byte, error)) (Image, error) {
	if err := ctx.Err(); err != nil {
		return Image{}, err
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[Key]entry{}
	}
	now := time.Now()
	for k, e := range c.entries {
		if !now.Before(e.expires) {
			delete(c.entries, k)
			c.bytes -= len(e.image.Data)
		}
	}
	if e, found := c.entries[key]; found {
		c.mu.Unlock()
		return e.image, nil
	}
	if c.active >= 2 {
		c.mu.Unlock()
		return Image{}, ErrBusy
	}
	c.active++
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.active--
		c.mu.Unlock()
	}()
	data, err := load()
	if err != nil {
		return Image{}, err
	}
	if err := ctx.Err(); err != nil {
		return Image{}, err
	}
	image, changed := resize(data, contentType, key.Width, key.Height)
	if !changed || ctx.Err() != nil {
		return image, ctx.Err()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, found := c.entries[key]; found {
		return image, nil
	}
	for len(c.entries) >= 128 || c.bytes+len(image.Data) > 32<<20 {
		var oldestKey Key
		var oldest time.Time
		for k, e := range c.entries {
			if oldest.IsZero() || e.expires.Before(oldest) {
				oldestKey, oldest = k, e.expires
			}
		}
		if oldest.IsZero() {
			return image, nil
		}
		c.bytes -= len(c.entries[oldestKey].image.Data)
		delete(c.entries, oldestKey)
	}
	c.entries[key] = entry{image: image, expires: time.Now().Add(10 * time.Minute)}
	c.bytes += len(image.Data)
	return image, nil
}
