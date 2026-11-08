// devbuild is Air's build command. It only compiles; it never starts IRCord.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"
	"time"
)

const defaultCacheMiB = 2048

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := build(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "dev build:", err)
		os.Exit(1)
	}
}

func cacheLimit(raw string) (int64, error) {
	if raw == "" {
		return defaultCacheMiB << 20, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 || value > (1<<63-1)>>20 {
		return 0, fmt.Errorf("IRCORD_GO_CACHE_MAX_MB must be a positive whole number of MiB")
	}
	return value << 20, nil
}

func build(ctx context.Context) (err error) {
	limit, err := cacheLimit(os.Getenv("IRCORD_GO_CACHE_MAX_MB"))
	if err != nil {
		return err
	}
	// Air runs this command from backend/. Never prune a caller's global cache.
	cache, err := filepath.Abs(filepath.Join("..", ".data", "go-build-cache"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cache, 0700); err != nil {
		return err
	}
	if err := trimCache(cache, limit); err != nil {
		return err
	}
	defer func() {
		if trimErr := trimCache(cache, limit); trimErr != nil {
			if err == nil {
				err = trimErr
			} else {
				fmt.Fprintln(os.Stderr, "dev cache:", trimErr)
			}
		}
	}()
	if err := os.MkdirAll(filepath.Join("..", ".data", "bin"), 0755); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "go", "build", "-o", "../.data/bin/ircord", "./cmd/ircord")
	// Set GOCACHE explicitly even when the build helper is invoked without make.
	command.Env = append(os.Environ(), "GOCACHE="+cache, "GOCACHEPROG=")
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	return command.Run()
}

type cacheFile struct {
	path string
	size int64
	used time.Time
}

// Go refreshes cache entry mtimes on use. Prune oldest entries between builds;
// no Go build is using this cache while we remove its files. WalkDir does not
// follow symlinks, and cache metadata is preserved.
func trimCache(root string, limit int64) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("development cache must be a directory, not a symlink")
	}
	var files []cacheFile
	var total int64
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if path == filepath.Join(root, "README") || path == filepath.Join(root, "trim.txt") {
			return nil
		}
		files = append(files, cacheFile{path: path, size: info.Size(), used: info.ModTime()})
		return nil
	})
	if err != nil || total <= limit {
		return err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].used.Before(files[j].used) })
	before := total
	for _, file := range files {
		if total <= limit {
			break
		}
		if err := os.Remove(file.path); err != nil {
			return err
		}
		total -= file.size
	}
	if total > limit {
		return fmt.Errorf("cache metadata alone exceeds the configured limit")
	}
	fmt.Fprintf(os.Stderr, "dev cache: pruned %d MiB; %d MiB remaining (limit %d MiB)\n", (before-total)>>20, total>>20, limit>>20)
	return nil
}
