package storage

import (
	"context"
	"errors"
	"os"
	"time"
)

// DatabasePool describes acquisition pressure, not SQLite lock waits or query latency.
type DatabasePool struct {
	MaxOpen     int     `json:"maxOpen" minimum:"0"`
	Open        int     `json:"open" minimum:"0"`
	InUse       int     `json:"inUse" minimum:"0"`
	Idle        int     `json:"idle" minimum:"0"`
	WaitCount   int64   `json:"waitCount" minimum:"0" doc:"Cumulative waits for a pooled connection since this backend started."`
	WaitTotalMS float64 `json:"waitTotalMs" minimum:"0" doc:"Cumulative time all callers spent waiting for a pooled connection. This is not SQLite lock time."`
}

// DatabaseDiagnostics contains aggregate, read-only diagnostics. No paths or query text leave storage.
type DatabaseDiagnostics struct {
	SampledAt           time.Time    `json:"sampledAt"`
	Available           bool         `json:"available"`
	Engine              string       `json:"engine" enum:"sqlite"`
	Version             string       `json:"version,omitempty"`
	JournalMode         string       `json:"journalMode,omitempty"`
	Synchronous         string       `json:"synchronous,omitempty"`
	ForeignKeys         bool         `json:"foreignKeys"`
	BusyTimeoutMS       int64        `json:"busyTimeoutMs" minimum:"0"`
	AutoCheckpointPages int64        `json:"autoCheckpointPages" minimum:"0"`
	AllocatedBytes      int64        `json:"allocatedBytes" minimum:"0" doc:"SQLite page count times page size, including pages currently represented in WAL."`
	ReusableBytes       int64        `json:"reusableBytes" minimum:"0" doc:"Unused pages within allocated space, reusable by SQLite without shrinking the file."`
	FileBytes           *int64       `json:"fileBytes,omitempty" nullable:"true" minimum:"0"`
	WALBytes            *int64       `json:"walBytes,omitempty" nullable:"true" minimum:"0" doc:"Physical WAL file size, not pending work or the number of live WAL frames."`
	Pool                DatabasePool `json:"pool"`
}

// DatabaseDiagnostics takes only bounded metadata reads; it never checkpoints, scans tables or changes PRAGMAs.
// Pool counters are captured before acquiring this diagnostic request's connection.
func (s *Store) DatabaseDiagnostics(ctx context.Context) DatabaseDiagnostics {
	stats := s.DB.Stats()
	out := DatabaseDiagnostics{
		SampledAt: time.Now().UTC(), Engine: "sqlite",
		Pool: DatabasePool{MaxOpen: stats.MaxOpenConnections, Open: stats.OpenConnections,
			InUse: stats.InUse, Idle: stats.Idle, WaitCount: stats.WaitCount,
			WaitTotalMS: float64(stats.WaitDuration) / float64(time.Millisecond)},
	}
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return out
	}
	defer conn.Close()
	var pages, pageSize, free, synchronous, foreignKeys int64
	for _, read := range []struct {
		query string
		value any
	}{
		{"SELECT sqlite_version()", &out.Version},
		{"PRAGMA journal_mode", &out.JournalMode},
		{"PRAGMA synchronous", &synchronous},
		{"PRAGMA foreign_keys", &foreignKeys},
		{"PRAGMA busy_timeout", &out.BusyTimeoutMS},
		{"PRAGMA wal_autocheckpoint", &out.AutoCheckpointPages},
		{"PRAGMA page_count", &pages},
		{"PRAGMA page_size", &pageSize},
		{"PRAGMA freelist_count", &free},
	} {
		if err = conn.QueryRowContext(ctx, read.query).Scan(read.value); err != nil {
			return out
		}
	}
	out.Synchronous = map[int64]string{0: "off", 1: "normal", 2: "full", 3: "extra"}[synchronous]
	out.ForeignKeys = foreignKeys == 1
	out.AllocatedBytes, out.ReusableBytes = pages*pageSize, free*pageSize
	out.Available = true
	// This internal path is never serialized. A missing WAL is zero; inaccessible files remain unknown.
	var path string
	if err = conn.QueryRowContext(ctx, "SELECT file FROM pragma_database_list WHERE name='main'").Scan(&path); err == nil && path != "" {
		if info, err := os.Stat(path); err == nil {
			size := info.Size()
			out.FileBytes = &size
		}
		if info, err := os.Stat(path + "-wal"); err == nil {
			size := info.Size()
			out.WALBytes = &size
		} else if errors.Is(err, os.ErrNotExist) {
			zero := int64(0)
			out.WALBytes = &zero
		}
	}
	return out
}
