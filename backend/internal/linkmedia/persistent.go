package linkmedia

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"
)

const (
	MediaCacheTTL = 90 * 24 * time.Hour
	// Both rendering caches recheck durable manifests and object existence at
	// least this often; a hit never renews the object's absolute expiry.
	StoredResultRecheck  = time.Minute
	maxPublicResultBytes = 512 << 10
	maxPublicResults     = 5000
	maxPublicObjects     = 20000
	cleanupBatch         = 100
)

type publicCache struct {
	db        *sql.DB
	namespace string
}

// StartCache attaches migrated storage before the resolver starts serving.
// It performs no bucket administration and needs only object Put/Get/Delete.
// The caller must cancel ctx and await completion before closing the database.
func (r *Resolver) StartCache(ctx context.Context, db *sql.DB) <-chan struct{} {
	done := make(chan struct{})
	if db == nil {
		close(done)
		return done
	}
	namespace := "native-public-metadata-v1"
	if r.options.CDN != nil {
		namespace = r.options.CDN.Namespace()
	}
	r.persistent = &publicCache{db: db, namespace: namespace}
	go func() {
		defer close(done)
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			work, cancel := context.WithTimeout(ctx, 45*time.Second)
			err := r.cleanupPublicCache(work)
			cancel()
			if err != nil && ctx.Err() == nil {
				// Storage errors can contain signed request URLs. Never log them.
				slog.Warn("CDN media cache cleanup failed; expired objects will be retried")
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	return done
}

func publicResultKey(raw string) string {
	// A versioned key prevents legacy source/social manifests satisfying this path.
	version := resolutionVersion(raw)
	hash := sha256.Sum256([]byte(version + "\x00" + resolutionKey(raw)))
	return version + ":" + hex.EncodeToString(hash[:])
}

func cacheObjectKey(key string) bool {
	nonce, ok := strings.CutPrefix(key, "link-media/")
	if !ok || len(nonce) != 64 {
		return false
	}
	_, err := hex.DecodeString(nonce)
	return err == nil
}

func storedResult(value Result) bool {
	if value.Embed != nil || fileCapabilityURL(value.URL) || len(value.Assets) == 0 && value.Preview == nil {
		return false
	}
	for _, asset := range resultObjects(value) {
		if !cacheObjectKey(asset.ObjectKey) || asset.ExpiresAt.IsZero() || fileCapabilityURL(asset.SourceURL) {
			return false
		}
	}
	return true
}

func hasStoredMedia(value Result) bool {
	for _, asset := range resultObjects(value) {
		if asset.ObjectKey != "" {
			return true
		}
	}
	return false
}

func resultObjects(value Result) []Asset {
	assets := append([]Asset(nil), value.Assets...)
	if p := value.Preview; p != nil {
		if p.ImageAsset != nil {
			assets = append(assets, *p.ImageAsset)
		}
		if p.File != nil && p.File.Asset.URL != "" {
			assets = append(assets, p.File.Asset)
		}
	}
	return assets
}

// IsDeliveryURL recognizes this resolver's configured CDN object namespace.
// Upload capabilities must be authorized by their own API and are never copied.
func (r *Resolver) IsDeliveryURL(raw string) bool {
	if r.options.CDN == nil || raw == "" {
		return false
	}
	const marker = "link-media/"
	base := r.options.CDN.URL(marker + strings.Repeat("0", 64))
	prefix := strings.TrimSuffix(base, strings.Repeat("0", 64))
	if prefix == "" || !strings.HasPrefix(raw, prefix) {
		return false
	}
	key := marker + strings.TrimPrefix(raw, prefix)
	return cacheObjectKey(key) && r.options.CDN.URL(key) == raw
}

func (r *Resolver) loadPublicResult(ctx context.Context, raw string) (Result, bool) {
	p := r.persistent
	var encoded []byte
	var expires int64
	err := p.db.QueryRowContext(ctx, `SELECT payload,expires_ns FROM public_media_results
		WHERE namespace=? AND cache_key=? AND expires_ns>?`, p.namespace, publicResultKey(raw), time.Now().UnixNano()).Scan(&encoded, &expires)
	if err != nil || len(encoded) > maxPublicResultBytes {
		return Result{}, false
	}
	var value Result
	if json.Unmarshal(encoded, &value) != nil || !storedResult(value) {
		return Result{}, false
	}
	value.CacheUntil = time.Unix(0, expires)
	if !resultExpiry(value, time.Now()).After(time.Now()) {
		return Result{}, false
	}
	for _, asset := range resultObjects(value) {
		// The delivery origin must still match this storage configuration.
		if r.options.CDN == nil || r.options.CDN.URL(asset.ObjectKey) != asset.URL || !asset.ExpiresAt.After(time.Now()) {
			return Result{}, false
		}
		// A bucket lifecycle rule or an operator may have removed bytes before
		// their recorded expiry. Never hand out a durable manifest blindly.
		check, cancel := context.WithTimeout(ctx, 5*time.Second)
		object, err := r.options.CDN.Stat(check, asset.ObjectKey)
		cancel()
		if err != nil || object.Size != asset.Size || object.ContentType != asset.ContentType {
			return Result{}, false
		}
	}
	if !resultExpiry(value, time.Now()).After(time.Now()) {
		return Result{}, false
	}
	return value, true
}

func (r *Resolver) savePublicResult(ctx context.Context, raw string, value Result) error {
	// Public native metadata and untrusted oEmbed markup only. Widget execution
	// stays isolated in the account-bound API; no credentials are persisted.
	if !storedResult(value) {
		return ErrUnavailable
	}
	value.URL = raw
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(encoded) > maxPublicResultBytes {
		return ErrLimit
	}
	expires := resultExpiry(value, time.Now())
	if !expires.After(time.Now()) {
		return ErrUnavailable
	}
	p := r.persistent
	result, err := p.db.ExecContext(ctx, `INSERT INTO public_media_results(namespace,cache_key,payload,expires_ns)
		SELECT ?,?,?,? WHERE (SELECT count(*) FROM public_media_results WHERE namespace=?)<?
			OR EXISTS(SELECT 1 FROM public_media_results WHERE namespace=? AND cache_key=?)
		ON CONFLICT(namespace,cache_key) DO UPDATE SET payload=excluded.payload,expires_ns=excluded.expires_ns`,
		p.namespace, publicResultKey(raw), encoded, expires.UnixNano(), p.namespace, maxPublicResults, p.namespace, publicResultKey(raw))
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrBusy
	}
	return nil
}

// Reserve before Put so even a process crash cannot leave an untracked upload.
func (r *Resolver) reserveObject(ctx context.Context, key string, expires time.Time) error {
	p := r.persistent
	if p == nil || !cacheObjectKey(key) {
		return ErrUnavailable
	}
	result, err := p.db.ExecContext(ctx, `INSERT INTO public_media_objects(namespace,object_key,expires_ns,retry_ns)
		SELECT ?,?,?,0 WHERE (SELECT count(*) FROM public_media_objects WHERE namespace=?)<?`,
		p.namespace, key, expires.UnixNano(), p.namespace, maxPublicObjects)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrBusy
	}
	return nil
}

func (r *Resolver) discardObject(ctx context.Context, key string) error {
	if r.persistent == nil || !cacheObjectKey(key) {
		return ErrUnavailable
	}
	p := r.persistent
	// Schedule retry first. If Delete or the process fails, a later cleanup owns it.
	_, markErr := p.db.ExecContext(ctx, `UPDATE public_media_objects SET expires_ns=?,retry_ns=0 WHERE namespace=? AND object_key=?`, time.Now().UnixNano(), p.namespace, key)
	if err := r.options.CDN.Delete(ctx, key); err != nil {
		return errors.Join(markErr, err)
	}
	_, err := p.db.ExecContext(ctx, `DELETE FROM public_media_objects WHERE namespace=? AND object_key=?`, p.namespace, key)
	return errors.Join(markErr, err)
}

func (r *Resolver) discardObjects(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if r.persistent == nil || r.options.CDN == nil || len(keys) > maxNativeAssets {
		return ErrUnavailable
	}
	args := []any{time.Now().UnixNano(), r.persistent.namespace}
	for _, key := range keys {
		if !cacheObjectKey(key) {
			return ErrUnavailable
		}
		args = append(args, key)
	}
	// Schedule the complete collection before any network deletion. A slow or
	// failed delete cannot leave later items waiting for their 90-day deadline.
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	if _, err := r.persistent.db.ExecContext(ctx, `UPDATE public_media_objects SET expires_ns=?,retry_ns=0 WHERE namespace=? AND object_key IN (`+placeholders+`)`, args...); err != nil {
		return err
	}
	var failed error
	for _, key := range keys {
		if ctx.Err() != nil {
			return errors.Join(failed, ctx.Err())
		}
		failed = errors.Join(failed, r.discardObject(ctx, key))
	}
	return failed
}

// A failed Put may have reached storage before its response was lost. Keep a
// delayed deletion record even after immediate deletion succeeds, so a late
// upload completion cannot create an untracked object.
func (r *Resolver) abandonObject(ctx context.Context, key string) error {
	p := r.persistent
	if p == nil || !cacheObjectKey(key) {
		return ErrUnavailable
	}
	_, err := p.db.ExecContext(ctx, `UPDATE public_media_objects SET expires_ns=?,retry_ns=0 WHERE namespace=? AND object_key=?`, time.Now().Add(10*time.Minute).UnixNano(), p.namespace, key)
	return errors.Join(err, r.options.CDN.Delete(ctx, key))
}

// Bounded cleanup is independent of message views. Failed deletions retain
// their record and back off, allowing other expired objects to make progress.
func (r *Resolver) cleanupPublicCache(ctx context.Context) error {
	p := r.persistent
	now := time.Now().UnixNano()
	if _, err := p.db.ExecContext(ctx, `DELETE FROM public_media_results WHERE rowid IN
		(SELECT rowid FROM public_media_results WHERE namespace=? AND expires_ns<=? ORDER BY expires_ns LIMIT ?)`, p.namespace, now, cleanupBatch); err != nil {
		return err
	}
	rows, err := p.db.QueryContext(ctx, `SELECT object_key FROM public_media_objects
		WHERE namespace=? AND expires_ns<=? AND retry_ns<=? ORDER BY expires_ns LIMIT ?`, p.namespace, now, now, cleanupBatch)
	if err != nil {
		return err
	}
	var keys []string
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close() // The backend has one SQLite connection; release it before writes.
	if err != nil {
		return err
	}
	var failed error
	for _, key := range keys {
		if ctx.Err() != nil {
			return errors.Join(failed, ctx.Err())
		}
		if r.options.CDN == nil || !cacheObjectKey(key) {
			return ErrUnavailable
		}
		work, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = r.options.CDN.Delete(work, key)
		cancel()
		if err != nil {
			failed = ErrUnavailable
			if _, err = p.db.ExecContext(ctx, `UPDATE public_media_objects SET retry_ns=? WHERE namespace=? AND object_key=?`, time.Now().Add(5*time.Minute).UnixNano(), p.namespace, key); err != nil {
				return err
			}
			continue
		}
		if _, err = p.db.ExecContext(ctx, `DELETE FROM public_media_objects WHERE namespace=? AND object_key=?`, p.namespace, key); err != nil {
			return err
		}
	}
	return failed
}
