package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrimCacheKeepsRecentFilesAndStaysInsideCache(t *testing.T) {
	cache, outside := t.TempDir(), t.TempDir()
	write := func(path string, size int) {
		t.Helper()
		if err := os.WriteFile(path, make([]byte, size), 0600); err != nil {
			t.Fatal(err)
		}
	}
	old, recent := filepath.Join(cache, "old-d"), filepath.Join(cache, "recent-d")
	write(old, 100)
	write(recent, 100)
	write(filepath.Join(cache, "README"), 10)
	write(filepath.Join(outside, "keep"), 100)
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cache, "external")); err != nil {
		t.Fatal(err)
	}
	if err := trimCache(cache, 150); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old cache entry was retained")
	}
	for _, path := range []string{recent, filepath.Join(cache, "README"), filepath.Join(outside, "keep")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("recent entry, metadata or outside file removed:", err)
		}
	}
	if err := trimCache(filepath.Join(cache, "external"), 1); err == nil {
		t.Fatal("accepted a cache root symlink")
	}
}

func TestCacheLimitValidation(t *testing.T) {
	for _, raw := range []string{"0", "-1", "invalid", "9223372036854775807"} {
		if _, err := cacheLimit(raw); err == nil {
			t.Fatalf("accepted invalid cache limit %q", raw)
		}
	}
	if value, err := cacheLimit(""); err != nil || value != 2<<30 {
		t.Fatal("default cache budget is not 2 GiB")
	}
	if value, err := cacheLimit("512"); err != nil || value != 512<<20 {
		t.Fatal("cache budget override ignored")
	}
}
