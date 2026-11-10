package storage

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDatabaseDiagnosticsReadOnlyAndBounded(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var before, after int64
	if err = s.DB.QueryRow("SELECT total_changes()").Scan(&before); err != nil {
		t.Fatal(err)
	}
	state := s.DatabaseDiagnostics(context.Background())
	if !state.Available || state.Version == "" || state.JournalMode != "wal" || state.Synchronous != "full" || !state.ForeignKeys || state.BusyTimeoutMS != 10000 {
		t.Fatalf("unexpected configuration: %+v", state)
	}
	if state.AllocatedBytes <= 0 || state.ReusableBytes > state.AllocatedBytes || state.FileBytes == nil || state.WALBytes == nil || state.Pool.MaxOpen != 1 {
		t.Fatalf("missing storage/pool facts: %+v", state)
	}
	if err = s.DB.QueryRow("SELECT total_changes()").Scan(&after); err != nil || before != after {
		t.Fatal("diagnostics changed database", before, after, err)
	}
	raw, err := json.Marshal(state)
	if err != nil || strings.Contains(string(raw), dir) || strings.Contains(string(raw), "ircord.db") {
		t.Fatal("diagnostics exposed a private path")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	busy := s.DatabaseDiagnostics(ctx)
	if busy.Available || busy.Pool.InUse != 1 || ctx.Err() == nil {
		t.Fatalf("busy database lost bounded pool evidence: %+v", busy)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	recovered := s.DatabaseDiagnostics(context.Background())
	if !recovered.Available || recovered.Pool.WaitCount < 1 || recovered.Pool.WaitTotalMS <= 0 {
		t.Fatalf("wait counters missing: %+v", recovered)
	}
}
