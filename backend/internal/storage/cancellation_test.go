package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReadCancellationWhileWaitingForDatabase(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Occupy the only connection, so each read must wait for the pool.
	conn, err := s.DB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for name, read := range map[string]func(context.Context) error{
		"search": func(ctx context.Context) error {
			_, err := s.SearchPageContext(ctx, "alice", SearchFilter{}, "", 10)
			return err
		},
		"history": func(ctx context.Context) error { _, err := s.HistoryContext(ctx, "alice", "room", "", 10); return err },
		"around": func(ctx context.Context) error {
			_, _, err := s.HistoryAroundContext(ctx, "alice", "room", "message", 10)
			return err
		},
		"message": func(ctx context.Context) error {
			_, err := s.MessageContext(ctx, "alice", "room", "message")
			return err
		},
		"oldest": func(ctx context.Context) error { _, err := s.OldestContext(ctx, "alice", "room"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			waiting := s.DB.Stats().WaitCount
			done := make(chan error, 1)
			go func() { done <- read(ctx) }()
			deadline := time.Now().Add(time.Second)
			for s.DB.Stats().WaitCount == waiting {
				if time.Now().After(deadline) {
					t.Fatal("read never entered the connection queue")
				}
				time.Sleep(time.Millisecond)
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("got %v, want cancellation", err)
				}
			case <-time.After(time.Second):
				t.Fatal("canceled read stayed blocked on the connection")
			}
		})
	}
}
