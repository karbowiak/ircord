package filehost

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ErrQuota reports that an upload would take its account past the server's
// upload limits. The error text names the limit and what the person can do.
var ErrQuota = errors.New("upload quota reached")

type quotaError struct{ detail string }

func (e quotaError) Error() string { return e.detail }
func (e quotaError) Unwrap() error { return ErrQuota }

// SweepBatch is how many unused uploads one sweep examines.
var SweepBatch = 100

// createUses keeps one row for each upload a message or a profile uses.
// expires_ns is 0 for a use that never ends, such as a sent message, or the
// time a profile use must be checked again. Uploads without a current use are
// the ones a sweep may remove. Uploads that existed before the table are
// recorded as used, so upgrading never removes a file that was shared before
// IRCord started tracking it. Avatars are not: a sweep keeps the ones a
// profile still shows and removes those replaced earlier.
func createUses(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='file_uses')").Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err = tx.Exec("CREATE TABLE file_uses(id TEXT PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,expires_ns INTEGER NOT NULL)"); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO file_uses(id,expires_ns) SELECT id,0 FROM files WHERE backend!='avatar'"); err != nil {
		return err
	}
	return tx.Commit()
}

// currentUse matches a use of files.id that keeps it now; argument: now in ns.
const currentUse = "EXISTS(SELECT 1 FROM file_uses u WHERE u.id=f.id AND (u.expires_ns=0 OR u.expires_ns>?))"

// usage reports the stored uploads that count toward an account's quota.
// Avatars have their own small limit and upstream files use no local storage.
// Files a sweep is removing no longer count.
func usage(q interface {
	QueryRow(string, ...any) *sql.Row
}, owner string) (count, bytes int64, err error) {
	err = q.QueryRow("SELECT count(*),coalesce(sum(size),0) FROM files WHERE owner=? AND deleted=0 AND backend NOT IN ('upstream','avatar')", owner).Scan(&count, &bytes)
	return count, bytes, err
}

// checkQuota returns a quotaError when adding size bytes would exceed a limit.
func (s *Service) checkQuota(q interface {
	QueryRow(string, ...any) *sql.Row
}, owner string, size int64) error {
	if s.options.QuotaBytes <= 0 && s.options.QuotaFiles <= 0 {
		return nil
	}
	count, used, err := usage(q, owner)
	if err != nil {
		return fmt.Errorf("file metadata is unavailable")
	}
	if s.options.QuotaFiles > 0 && count+1 > s.options.QuotaFiles {
		return quotaError{fmt.Sprintf("You've reached this server's limit of %d uploaded files. Delete messages with files you no longer need, then try again.", s.options.QuotaFiles)}
	}
	if s.options.QuotaBytes > 0 && used+size > s.options.QuotaBytes {
		return quotaError{fmt.Sprintf("This file would take your uploads past this server's limit of %s. Delete messages with files you no longer need, then try again.", formatBytes(s.options.QuotaBytes))}
	}
	return nil
}

func formatBytes(n int64) string {
	units := []string{"bytes", "KiB", "MiB", "GiB", "TiB"}
	value, unit := float64(n), 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d bytes", n)
	}
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d %s", int64(value), units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

// markUsed records that the owner's upload is in use, so a sweep keeps it. It
// records nothing for a file a sweep has already started to remove; the
// caller's following Record lookup then reports the upload as missing.
func (s *Service) markUsed(owner, id string) error {
	_, err := s.options.Store.DB.Exec("INSERT INTO file_uses(id,expires_ns) SELECT id,0 FROM files WHERE id=? AND owner=? AND deleted=0 ON CONFLICT(id) DO UPDATE SET expires_ns=0", id, owner)
	return err
}

// hideUnused hides one sweep candidate unless stored account data still names
// it, and reports whether it did. It holds the database write lock from the
// first check to the hiding update, so a message or profile saved meanwhile
// is either seen by the checks or saved after the file is already hidden.
// A match in messages, the outbox or parties is recorded as a use that never
// ends. A match in the owner's profile or settings keeps the file until
// recheck, and the sweep after that looks again.
func (s *Service) hideUnused(ctx context.Context, owner, id string, recheck time.Time) (hidden bool, err error) {
	conn, err := s.options.Store.DB.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
			hidden = false
		}
	}()
	needle := "/files/" + id + "/"
	checks := []struct {
		query   string
		args    []any
		expires int64
	}{
		{"SELECT EXISTS(SELECT 1 FROM entities WHERE owner=? AND instr(payload,?)>0)", []any{owner, needle}, recheck.UnixNano()},
		{"SELECT EXISTS(SELECT 1 FROM app_data WHERE owner=? AND instr(payload,?)>0)", []any{owner, needle}, recheck.UnixNano()},
		{"SELECT EXISTS(SELECT 1 FROM party_uploads WHERE upload=?)", []any{id}, 0},
		{"SELECT EXISTS(SELECT 1 FROM outbox WHERE owner=? AND instr(input,?)>0)", []any{owner, needle}, 0},
		{"SELECT EXISTS(SELECT 1 FROM messages WHERE owner=? AND instr(text,?)>0)", []any{owner, needle}, 0},
		{"SELECT EXISTS(SELECT 1 FROM party_history WHERE instr(payload,?)>0)", []any{needle}, 0},
	}
	for _, check := range checks {
		var found bool
		if err = conn.QueryRowContext(ctx, check.query, check.args...).Scan(&found); err != nil {
			return false, err
		}
		if found {
			// A use that never ends is never shortened by a profile match.
			if _, err = conn.ExecContext(ctx, "INSERT INTO file_uses(id,expires_ns) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET expires_ns=excluded.expires_ns WHERE file_uses.expires_ns!=0", id, check.expires); err != nil {
				return false, err
			}
			_, err = conn.ExecContext(ctx, "COMMIT")
			return false, err
		}
	}
	// A message being sent now either recorded its use before this lock, which
	// keeps the file, or no longer finds the file once it is hidden.
	result, err := conn.ExecContext(ctx, "UPDATE files AS f SET deleted=1 WHERE f.id=? AND f.deleted=0 AND NOT "+currentUse, id, time.Now().UnixNano())
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		return false, err
	}
	return n > 0, nil
}

// SweepUnused removes uploads, including avatars, that no message or profile
// uses and that are older than grace. A profile's use is checked again after
// grace, so a replaced banner or avatar is removed within about twice that.
// Upstream files cannot be removed. A file that fails to delete stays hidden
// and is retried by the next sweep; it never stops the others. It reports how
// many files it removed.
func (s *Service) SweepUnused(ctx context.Context, grace time.Duration) (int, error) {
	if grace <= 0 {
		return 0, nil
	}
	// Only storage configured now is swept, so files in other storage neither
	// change nor take up the batch.
	var backends []string
	if s.options.Blob != nil {
		backends = append(backends, s.options.Blob.Name())
	}
	if s.options.AvatarBlob != nil {
		backends = append(backends, "avatar")
	}
	if len(backends) == 0 {
		return 0, nil
	}
	now := time.Now()
	args := []any{}
	for _, b := range backends {
		args = append(args, b)
	}
	args = append(args, now.Add(-grace).UnixNano(), now.UnixNano(), SweepBatch)
	rows, err := s.options.Store.DB.QueryContext(ctx, `SELECT f.id,f.owner,f.backend,f.object_key,f.deleted FROM files f
		WHERE f.backend IN (?`+strings.Repeat(",?", len(backends)-1)+`) AND (f.deleted=1 OR f.created_ns<? AND NOT `+currentUse+`)
		ORDER BY f.deleted DESC,f.created_ns LIMIT ?`, args...)
	if err != nil {
		return 0, err
	}
	type item struct {
		id, owner, backend, key string
		deleted                 bool
	}
	items := []item{}
	for rows.Next() {
		var v item
		if err = rows.Scan(&v.id, &v.owner, &v.backend, &v.key, &v.deleted); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	removed := 0
	var failed []error
	for _, v := range items {
		if err = ctx.Err(); err != nil {
			return removed, err
		}
		blob := s.blob(v.backend)
		if !v.deleted {
			if blob == nil {
				// Its storage is not configured now; leave it visible and untouched.
				continue
			}
			hidden, err := s.hideUnused(ctx, v.owner, v.id, now.Add(grace))
			if err != nil {
				failed = append(failed, fmt.Errorf("check file %s: %w", v.id, err))
				continue
			}
			if !hidden {
				continue
			}
		}
		if blob == nil {
			slog.Warn("Removing an unused upload is waiting for file storage that is not configured", "file", v.id, "storage", v.backend, "object", v.key)
			failed = append(failed, fmt.Errorf("file %s is in %s storage, which is not configured", v.id, v.backend))
			continue
		}
		if err = blob.Delete(ctx, v.key); err != nil {
			failed = append(failed, fmt.Errorf("delete file %s: %w", v.id, err))
			continue
		}
		if _, err = s.options.Store.DB.ExecContext(ctx, "DELETE FROM files WHERE id=? AND deleted=1", v.id); err != nil {
			failed = append(failed, fmt.Errorf("forget file %s: %w", v.id, err))
			continue
		}
		removed++
	}
	return removed, errors.Join(failed...)
}

// DeleteAvatarURL deletes the owner's stored profile image served at value.
// It does nothing for other URLs, including regular uploads used as a banner.
func (s *Service) DeleteAvatarURL(ctx context.Context, owner, value string) error {
	var id string
	err := s.options.Store.DB.QueryRowContext(ctx, "SELECT f.id FROM file_metadata m JOIN files f ON f.id=m.id WHERE f.owner=? AND f.backend='avatar' AND f.deleted=0 AND json_extract(m.payload,'$.url')=? LIMIT 1", owner, value).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.Delete(ctx, owner, id)
}
