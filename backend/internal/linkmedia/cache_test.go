package linkmedia

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolutionCacheDedupAndCancellation(t *testing.T) {
	c := &resolutionCache{}
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	load := func(ctx context.Context) (Result, error) {
		calls.Add(1)
		close(started)
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-release:
			return Result{Title: "Cached post"}, nil
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := c.get(ctx, "source:post", time.Second, load); first <- err }()
	<-started
	second := make(chan Result, 1)
	go func() { value, _ := c.get(context.Background(), "source:post", time.Second, load); second <- value }()
	deadline := time.Now().Add(time.Second)
	for {
		c.mu.Lock()
		waiting := c.entries["source:post"].waiters
		c.mu.Unlock()
		if waiting == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second reader did not join")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled caller: %v", err)
	}
	close(release)
	value := <-second
	// Metadata without stored media is kept for a day.
	if value.Title != "Cached post" || time.Until(value.CacheUntil) < 24*time.Hour-time.Second {
		t.Fatal("shared reader or one-day expiry lost")
	}
	if _, err := c.get(context.Background(), "source:post", time.Second, load); err != nil || calls.Load() != 1 {
		t.Fatalf("resolved twice: calls=%d err=%v", calls.Load(), err)
	}
}

func TestResolutionCacheCapsSignedSourcesAndRetriesFailures(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	deadline := now.Add(time.Hour)
	for _, raw := range []string{
		"https://video.example/movie.mp4?expire=" + strconv.FormatInt(deadline.Unix(), 10),
		"https://scontent.cdninstagram.com/photo.jpg?oe=" + strconv.FormatInt(deadline.Unix(), 16),
	} {
		if got := resultExpiry(Result{Assets: []Asset{{URL: raw}}}, now); !got.Equal(deadline.Add(-2 * time.Minute)) {
			t.Fatalf("cached beyond signed source expiry: %v", got)
		}
	}
	c := &resolutionCache{}
	calls := 0
	load := func(context.Context) (Result, error) { calls++; return Result{}, ErrUnavailable }
	for range 2 {
		_, _ = c.get(context.Background(), "source:failed", time.Second, load)
	}
	if calls != 1 {
		t.Fatal("temporary failure not deduplicated")
	}
	c.mu.Lock()
	c.entries["source:failed"].expires = time.Now().Add(-time.Second)
	c.mu.Unlock()
	_, _ = c.get(context.Background(), "source:failed", time.Second, load)
	if calls != 2 {
		t.Fatal("temporary failure never refreshed")
	}
	if resolutionKey("https://example.com/document?token=one") == resolutionKey("https://example.com/document?token=two") {
		t.Fatal("generic URL access parameters were merged")
	}
	if resolutionKey("https://www.instagram.com/p/Example/?utm_source=one") != resolutionKey("https://www.instagram.com/p/Example/?utm_source=two") {
		t.Fatal("native tracking aliases did not share their result")
	}
}

func TestResolutionCacheHoldsRestrictions(t *testing.T) {
	c := &resolutionCache{}
	_, err := c.get(context.Background(), "source:https://www.instagram.com/reel/Private/", time.Second, func(context.Context) (Result, error) {
		return Result{}, errors.Join(ErrUnavailable, ErrRestricted)
	})
	c.mu.Lock()
	expires := c.entries["source:https://www.instagram.com/reel/Private/"].expires
	c.mu.Unlock()
	if !errors.Is(err, ErrRestricted) || time.Until(expires) < restrictedCacheTTL-time.Second {
		t.Fatalf("restricted post retried too soon: %v until %v", err, expires)
	}
}
