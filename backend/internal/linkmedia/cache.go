package linkmedia

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const ResolutionCacheTTL = 7 * 24 * time.Hour
const restrictedCacheTTL = 5 * time.Minute
const maxPendingResolutions = 64

type resolutionEntry struct {
	value          Result
	err            error
	done           chan struct{}
	cancel         context.CancelFunc
	waiters, bytes int
	expires        time.Time
}

// Public provider results only: this resolver never receives account sessions.
// Message authorization and widget tokens remain in the account-scoped API.
type resolutionCache struct {
	mu      sync.Mutex
	entries map[string]*resolutionEntry
	bytes   int
}

func resolutionKey(raw string) string {
	if canonical, _, _, err := socialSource(raw); err == nil {
		return canonical
	}
	// Preserve query semantics and avoid legacy social canonicalization rules.
	return raw
}

func resolutionVersion(raw string) string {
	if _, _, _, err := socialSource(raw); err == nil {
		return "native-social-v1"
	}
	if NativeProviderSource(raw) {
		return "native-public-media-v1"
	}
	// Restoring Reddit must bypass previously stored generic oEmbed results.
	// Object storage ownership remains in the existing CDN namespace.
	if _, err := redditID(raw); err == nil && !fileCapabilityURL(raw) {
		return "native-reddit-v2"
	}
	if u, err := url.Parse(raw); err == nil {
		host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
		if publicProviderFor(u) != nil || NativeImageSource(raw) || host == "xkcd.com" || host == "giphy.com" {
			return "native-providers-v3"
		}
	}
	return "native-v1"
}

func (c *resolutionCache) get(ctx context.Context, key string, timeout time.Duration, load func(context.Context) (Result, error)) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[string]*resolutionEntry{}
	}
	now := time.Now()
	pending := 0
	for k, e := range c.entries {
		if e.expires.IsZero() {
			pending++
			continue
		}
		if !now.Before(e.expires) {
			delete(c.entries, k)
			c.bytes -= e.bytes
		}
	}
	e := c.entries[key]
	if e == nil {
		if pending >= maxPendingResolutions {
			c.mu.Unlock()
			return Result{}, ErrBusy
		}
		for len(c.entries) >= 512 || c.bytes >= 32<<20 {
			var oldest string
			var at time.Time
			for k, item := range c.entries {
				if !item.expires.IsZero() && (at.IsZero() || item.expires.Before(at)) {
					oldest, at = k, item.expires
				}
			}
			if oldest == "" {
				c.mu.Unlock()
				return Result{}, ErrBusy
			}
			c.bytes -= c.entries[oldest].bytes
			delete(c.entries, oldest)
		}
		work, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		e = &resolutionEntry{done: make(chan struct{}), cancel: cancel}
		c.entries[key] = e
		go func() {
			defer cancel()
			value, err := load(work)
			expires := resultExpiry(value, time.Now())
			if err != nil {
				expires = time.Now().Add(15 * time.Second)
				// A private or age-gated post stays that way; don't pay for a
				// multi-second extraction each time someone scrolls past it.
				if errors.Is(err, ErrRestricted) || errors.Is(err, ErrAgeRestricted) {
					expires = time.Now().Add(restrictedCacheTTL)
				}
			}
			if work.Err() != nil {
				expires = time.Now()
			}
			value.CacheUntil = expires
			raw, _ := json.Marshal(value)
			c.mu.Lock()
			e.value, e.err, e.expires = value, err, expires
			if hasStoredMedia(value) {
				if recheck := time.Now().Add(StoredResultRecheck); recheck.Before(e.expires) {
					e.expires = recheck
				}
			}
			e.bytes = len(raw)
			if e.bytes > 2<<20 {
				e.expires = time.Now()
				e.value.CacheUntil = e.expires
				e.bytes = 0
				delete(c.entries, key)
			}
			c.bytes += e.bytes
			for c.bytes > 32<<20 {
				var oldest string
				var at time.Time
				for k, item := range c.entries {
					if !item.expires.IsZero() && (at.IsZero() || item.expires.Before(at)) {
						oldest, at = k, item.expires
					}
				}
				if oldest == "" {
					break
				}
				c.bytes -= c.entries[oldest].bytes
				delete(c.entries, oldest)
			}
			close(e.done)
			c.mu.Unlock()
		}()
	}
	e.waiters++
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		e.waiters--
		if e.waiters == 0 && e.expires.IsZero() {
			e.cancel()
		}
		c.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-e.done:
		return e.value, e.err
	}
}

func isNativeKey(key string) bool {
	raw := strings.TrimPrefix(strings.TrimPrefix(key, "source:"), "widget:")
	_, _, err := providerTarget(raw)
	return err == nil
}

func resultExpiry(value Result, now time.Time) time.Time {
	until := now.Add(24 * time.Hour)
	if hasStoredMedia(value) {
		until = now.Add(MediaCacheTTL)
	}
	if !value.CacheUntil.IsZero() && value.CacheUntil.Before(until) {
		until = value.CacheUntil
	}
	bound := func(raw string) {
		if expiry := sourceExpiry(raw); !expiry.IsZero() && expiry.Add(-2*time.Minute).Before(until) {
			until = expiry.Add(-2 * time.Minute)
		}
	}
	for _, asset := range resultObjects(value) {
		if asset.ObjectKey != "" {
			if !asset.ExpiresAt.IsZero() && asset.ExpiresAt.Before(until) {
				until = asset.ExpiresAt
			}
		} else {
			bound(asset.URL)
		}
	}
	if p := value.Preview; p != nil && !p.ExpiresAt.IsZero() && p.ExpiresAt.Before(until) {
		until = p.ExpiresAt
	}
	return until
}

func sourceExpiry(raw string) time.Time {
	u, err := url.Parse(raw)
	if err != nil {
		return time.Time{}
	}
	values := url.Values{}
	for key, value := range u.Query() {
		values[strings.ToLower(key)] = value
	}
	for _, key := range []string{"expire", "expires", "x-expires"} {
		if number, err := strconv.ParseInt(values.Get(key), 10, 64); err == nil && number > 0 {
			if number > 1e12 {
				number /= 1000
			}
			return time.Unix(number, 0)
		}
	}
	for _, handler := range providerHandlers {
		if handler.sourceExpiry != nil {
			if expiry := handler.sourceExpiry(u, values); !expiry.IsZero() {
				return expiry
			}
		}
	}
	if issued, err := time.Parse("20060102T150405Z", values.Get("x-amz-date")); err == nil {
		if seconds, err := strconv.ParseInt(values.Get("x-amz-expires"), 10, 64); err == nil && seconds > 0 && seconds <= 604800 {
			return issued.Add(time.Duration(seconds) * time.Second)
		}
	}
	return time.Time{}
}
