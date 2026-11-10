package api

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLinkMediaAdmissionAllowsFiftyPerAccountAndBoundsGlobalWork(t *testing.T) {
	cache := &linkMediaCache{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	var workers sync.WaitGroup
	t.Cleanup(func() { cancel(); workers.Wait() })
	started := make(chan struct{}, 64)
	release := make(chan struct{})
	finished := make(chan error, 64)
	var calls atomic.Int32
	load := func(entry *linkMediaEntry) error {
		calls.Add(1)
		started <- struct{}{}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			entry.response.Title = entry.owner
			return nil
		}
	}
	start := func(owner string, count int) {
		for i := 0; i < count; i++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				_, err := cache.get(ctx, fmt.Sprintf("%s:%d", owner, i), &linkMediaEntry{owner: owner}, load)
				finished <- err
			}()
		}
		for range count {
			select {
			case <-started:
			case err := <-finished:
				t.Fatalf("%s request rejected before its load started: %v", owner, err)
			case <-ctx.Done():
				t.Fatal("admitted loads did not start")
			}
		}
	}
	start("alice", 50)
	unexpectedLoad := func(*linkMediaEntry) error {
		t.Error("rejected request started work")
		return nil
	}
	if _, err := cache.get(ctx, "alice:overflow", &linkMediaEntry{owner: "alice"}, unexpectedLoad); !errors.Is(err, errMediaBusy) {
		t.Fatalf("51st account request: %v", err)
	}
	// A second account can use the remaining global capacity.
	start("bob", 14)
	if _, err := cache.get(ctx, "carol:overflow", &linkMediaEntry{owner: "carol"}, unexpectedLoad); !errors.Is(err, errMediaBusy) {
		t.Fatalf("65th global request: %v", err)
	}
	// Existing work is deduplicated before either admission limit. A canceled
	// reader can leave while the original load keeps its slot and completes.
	waiter, stopWaiter := context.WithCancel(ctx)
	stopWaiter()
	if _, err := cache.get(waiter, "alice:0", &linkMediaEntry{owner: "alice"}, unexpectedLoad); !errors.Is(err, context.Canceled) {
		t.Fatalf("duplicate reader was rejected by the capacity limit: %v", err)
	}
	close(release)
	for range 64 {
		if err := <-finished; err != nil {
			t.Fatalf("admitted load failed: %v", err)
		}
	}
	if calls.Load() != 64 {
		t.Fatalf("duplicate or excess work started: %d", calls.Load())
	}
	for _, owner := range []string{"alice", "bob"} {
		value, err := cache.get(ctx, owner+":0", &linkMediaEntry{owner: owner}, unexpectedLoad)
		if err != nil || value.Title != owner {
			t.Fatalf("account cache result changed: owner=%s, title=%s, error=%v", owner, value.Title, err)
		}
	}
}

func TestLinkMediaCanceledLoadCanBeRetried(t *testing.T) {
	cache := &linkMediaCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := cache.get(ctx, "alice:post", &linkMediaEntry{owner: "alice"}, func(*linkMediaEntry) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
		finished <- err
	}()
	<-started
	cancel()
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled load: %v", err)
	}
	retried := false
	_, err := cache.get(context.Background(), "alice:post", &linkMediaEntry{owner: "alice"}, func(*linkMediaEntry) error {
		retried = true
		return nil
	})
	if err != nil || !retried {
		t.Fatalf("canceled load retained its slot or cached cancellation: retry=%v, error=%v", retried, err)
	}
}
