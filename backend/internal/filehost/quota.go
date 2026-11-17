package filehost

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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

// createUses keeps one row for each upload a message or another account record
// has used. Uploads without a row are the ones a sweep may remove. Uploads that
// existed before the table are recorded as used, so upgrading never removes a
// file that was shared before IRCord started tracking it.
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
	if _, err = tx.Exec("CREATE TABLE file_uses(id TEXT PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,at_ns INTEGER NOT NULL)"); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO file_uses(id,at_ns) SELECT id,? FROM files", time.Now().UnixNano()); err != nil {
		return err
	}
	return tx.Commit()
}

// usage reports the stored uploads that count toward an account's quota.
// Avatars have their own small limit and upstream files use no local storage.
func usage(q interface {
	QueryRow(string, ...any) *sql.Row
}, owner string) (count, bytes int64, err error) {
	err = q.QueryRow("SELECT count(*),coalesce(sum(size),0) FROM files WHERE owner=? AND backend NOT IN ('upstream','avatar')", owner).Scan(&count, &bytes)
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
	_, err := s.options.Store.DB.Exec("INSERT INTO file_uses(id,at_ns) SELECT id,? FROM files WHERE id=? AND owner=? AND deleted=0 ON CONFLICT(id) DO NOTHING", time.Now().UnixNano(), id, owner)
	return err
}

// referenced reports whether stored account data still names the upload. A
// match in messages or parties is permanent, so it is recorded; a match in the
// owner's profile or settings only keeps the file while the profile uses it.
func (s *Service) referenced(ctx context.Context, owner, id string) (bool, error) {
	needle := "/files/" + id + "/"
	db := s.options.Store.DB
	var found bool
	for _, query := range []string{
		"SELECT EXISTS(SELECT 1 FROM entities WHERE owner=? AND instr(payload,?)>0)",
		"SELECT EXISTS(SELECT 1 FROM app_data WHERE owner=? AND instr(payload,?)>0)",
	} {
		if err := db.QueryRowContext(ctx, query, owner, needle).Scan(&found); err != nil || found {
			return found, err
		}
	}
	for _, check := range []struct {
		query string
		args  []any
	}{
		{"SELECT EXISTS(SELECT 1 FROM party_uploads WHERE upload=?)", []any{id}},
		{"SELECT EXISTS(SELECT 1 FROM outbox WHERE owner=? AND instr(input,?)>0)", []any{owner, needle}},
		{"SELECT EXISTS(SELECT 1 FROM messages WHERE owner=? AND instr(text,?)>0)", []any{owner, needle}},
		{"SELECT EXISTS(SELECT 1 FROM party_history WHERE instr(payload,?)>0)", []any{needle}},
	} {
		if err := db.QueryRowContext(ctx, check.query, check.args...).Scan(&found); err != nil {
			return false, err
		}
		if found {
			return true, s.markUsed(owner, id)
		}
	}
	return false, nil
}

// SweepUnused removes uploads that no message or account record has used and
// that are older than grace. Avatars are removed when replaced instead, and
// upstream files cannot be removed. A file that fails to delete stays marked
// for removal and is retried by the next sweep; it never stops the others.
// It reports how many files it removed.
func (s *Service) SweepUnused(ctx context.Context, grace time.Duration) (int, error) {
	if grace <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-grace).UnixNano()
	rows, err := s.options.Store.DB.QueryContext(ctx, `SELECT f.id,f.owner,f.backend,f.object_key,f.deleted FROM files f
		WHERE f.backend NOT IN ('upstream','avatar') AND (f.deleted=1 OR f.created_ns<? AND NOT EXISTS(SELECT 1 FROM file_uses u WHERE u.id=f.id))
		ORDER BY f.deleted DESC,f.created_ns LIMIT ?`, cutoff, SweepBatch)
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
		if !v.deleted {
			used, err := s.referenced(ctx, v.owner, v.id)
			if err != nil {
				failed = append(failed, fmt.Errorf("check file %s: %w", v.id, err))
				continue
			}
			if used {
				continue
			}
			// Hide the file before removing it. A message being sent now either
			// marked it used first, so this changes nothing, or no longer finds it.
			result, err := s.options.Store.DB.ExecContext(ctx, "UPDATE files SET deleted=1 WHERE id=? AND deleted=0 AND NOT EXISTS(SELECT 1 FROM file_uses u WHERE u.id=files.id)", v.id)
			if err != nil {
				failed = append(failed, fmt.Errorf("hide file %s: %w", v.id, err))
				continue
			}
			if n, _ := result.RowsAffected(); n == 0 {
				continue
			}
		}
		blob := s.blob(v.backend)
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
