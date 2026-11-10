package linkmedia

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolutionAdmissionBoundsSixtyFourSourcesAndDeduplicatesAtCapacity(t *testing.T) {
	cache := &resolutionCache{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	var workers sync.WaitGroup
	t.Cleanup(func() { cancel(); workers.Wait() })
	started := make(chan struct{}, 64)
	release := make(chan struct{})
	finished := make(chan error, 64)
	var calls atomic.Int32
	load := func(work context.Context) (Result, error) {
		calls.Add(1)
		started <- struct{}{}
		select {
		case <-work.Done():
			return Result{}, work.Err()
		case <-release:
			return Result{Title: "fixture"}, nil
		}
	}
	for i := 0; i < 64; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := cache.get(ctx, fmt.Sprintf("source:%d", i), time.Second, load)
			finished <- err
		}()
	}
	for range 64 {
		select {
		case <-started:
		case err := <-finished:
			t.Fatalf("source rejected before its load started: %v", err)
		case <-ctx.Done():
			t.Fatal("admitted sources did not start")
		}
	}
	if _, err := cache.get(ctx, "source:overflow", time.Second, load); !errors.Is(err, ErrBusy) {
		t.Fatalf("65th source request: %v", err)
	}
	waiter, stopWaiter := context.WithCancel(ctx)
	defer stopWaiter()
	duplicate := make(chan error, 1)
	go func() {
		_, err := cache.get(waiter, "source:0", time.Second, load)
		duplicate <- err
	}()
	for {
		cache.mu.Lock()
		readers := cache.entries["source:0"].waiters
		cache.mu.Unlock()
		if readers == 2 {
			break
		}
		select {
		case err := <-duplicate:
			t.Fatalf("duplicate source could not join at capacity: %v", err)
		case <-ctx.Done():
			t.Fatal("duplicate reader did not join")
		case <-time.After(time.Millisecond):
		}
	}
	stopWaiter()
	if err := <-duplicate; !errors.Is(err, context.Canceled) {
		t.Fatalf("duplicate reader cancellation: %v", err)
	}
	close(release)
	for range 64 {
		if err := <-finished; err != nil {
			t.Fatalf("admitted source failed: %v", err)
		}
	}
	if calls.Load() != 64 {
		t.Fatalf("duplicate or excess source loads: %d", calls.Load())
	}
}

func TestResolutionCanceledLastReaderFreesAdmission(t *testing.T) {
	cache := &resolutionCache{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := cache.get(ctx, "source:post", time.Second, func(work context.Context) (Result, error) {
			close(started)
			<-work.Done()
			return Result{}, work.Err()
		})
		finished <- err
	}()
	<-started
	cache.mu.Lock()
	done := cache.entries["source:post"].done
	cache.mu.Unlock()
	cancel()
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatalf("last reader cancellation: %v", err)
	}
	<-done
	retried := false
	_, err := cache.get(context.Background(), "source:post", time.Second, func(context.Context) (Result, error) {
		retried = true
		return Result{}, nil
	})
	if err != nil || !retried {
		t.Fatalf("canceled source retained admission or cached cancellation: retry=%v, error=%v", retried, err)
	}
}
