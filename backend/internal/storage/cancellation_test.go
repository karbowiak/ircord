package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

// History and search pages read on their own connections, so they answer while
// a write transaction holds the writer connection.
func TestPageReadsDoNotWaitForWriter(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO accounts(id,key_hash,created_ns,last_seen_ns) VALUES('writer','h',1,1)"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err = s.HistoryContext(ctx, "alice", "room", "", 10); err != nil {
		t.Fatal("history waited for the writer:", err)
	}
	if _, err = s.SearchPageContext(ctx, "alice", SearchFilter{}, "", 10); err != nil {
		t.Fatal("search waited for the writer:", err)
	}
}

func TestReadCancellationWhileWaitingForDatabase(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Occupy the only writer connection and every read-only one, so each read
	// must wait for its pool.
	conn, err := s.DB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for range s.read.Stats().MaxOpenConnections {
		readConn, err := s.read.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer readConn.Close()
	}
	waits := func() int64 { return s.DB.Stats().WaitCount + s.read.Stats().WaitCount }
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
			waiting := waits()
			done := make(chan error, 1)
			go func() { done <- read(ctx) }()
			deadline := time.Now().Add(time.Second)
			for waits() == waiting {
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
