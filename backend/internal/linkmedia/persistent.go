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
	// Live stored objects share this many bytes; the object cap alone would
	// allow 20,000 files of the largest permitted size.
	maxPublicObjectBytes int64 = 10 << 30
	// An object outlives the last stored result that references it by this
	// long, so the daily refresh of a page preview finds and reuses its
	// thumbnail instead of storing another copy.
	objectGrace = 24 * time.Hour
	// A reservation reads eviction candidates in batches of this size.
	evictionBatch = 1000
	cleanupBatch = 100
)

type publicCache struct {
	db        *sql.DB
	namespace string
	// Tests lower these; zero selects the production limits.
	maxResults, maxObjects int
	maxBytes               int64
}

func (p *publicCache) resultLimit() int {
	if p.maxResults > 0 {
		return p.maxResults
	}
	return maxPublicResults
}

func (p *publicCache) objectLimit() int {
	if p.maxObjects > 0 {
		return p.maxObjects
	}
	return maxPublicObjects
}

func (p *publicCache) byteLimit() int64 {
	if p.maxBytes > 0 {
		return p.maxBytes
	}
	return maxPublicObjectBytes
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
	// A read keeps the entry and its objects at the back of the eviction
	// order. This is best effort and never extends an expiry.
	p.touch(ctx, publicResultKey(raw), objectKeys(value))
	return value, true
}

func (p *publicCache) touch(ctx context.Context, resultKey string, objects []string) {
	now := time.Now().UnixNano()
	_, _ = p.db.ExecContext(ctx, `UPDATE public_media_results SET used_ns=? WHERE namespace=? AND cache_key=?`, now, p.namespace, resultKey)
	if len(objects) > 0 {
		args := append([]any{now, p.namespace}, keyArgs(objects)...)
		_, _ = p.db.ExecContext(ctx, `UPDATE public_media_objects SET used_ns=? WHERE namespace=? AND object_key IN (`+placeholders(len(objects))+`)`, args...)
	}
}

// objectKeys lists each stored object of a result once; a gallery can reuse
// one stored copy for identical items.
func objectKeys(value Result) []string {
	var keys []string
	seen := map[string]bool{}
	for _, asset := range resultObjects(value) {
		if asset.ObjectKey != "" && !seen[asset.ObjectKey] {
			seen[asset.ObjectKey] = true
			keys = append(keys, asset.ObjectKey)
		}
	}
	return keys
}

func keyArgs(keys []string) []any {
	args := make([]any, len(keys))
	for i, key := range keys {
		args[i] = key
	}
	return args
}

func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

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
	key, now := publicResultKey(raw), time.Now()
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Each object lives as long as the latest result that references it, plus
	// a grace period, never past its own absolute expiry. An object already
	// scheduled for deletion cannot back a new result.
	if objects := objectKeys(value); len(objects) > 0 {
		args := append([]any{expires.Add(objectGrace).UnixNano(), now.UnixNano(), p.namespace, now.UnixNano()}, keyArgs(objects)...)
		updated, err := tx.ExecContext(ctx, `UPDATE public_media_objects SET expires_ns=min(limit_ns,max(expires_ns,?)),used_ns=?
			WHERE namespace=? AND expires_ns>? AND object_key IN (`+placeholders(len(objects))+`)`, args...)
		if err != nil {
			return err
		}
		if n, _ := updated.RowsAffected(); n != int64(len(objects)) {
			return ErrBusy
		}
	}
	// A full table drops its least recently used entries rather than
	// refusing the new one.
	var exists bool
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM public_media_results WHERE namespace=? AND cache_key=?),
		(SELECT count(*) FROM public_media_results WHERE namespace=?)`, p.namespace, key, p.namespace).Scan(&exists, &count); err != nil {
		return err
	}
	if over := count - p.resultLimit() + 1; !exists && over > 0 {
		if _, err = tx.ExecContext(ctx, `DELETE FROM public_media_results WHERE rowid IN
			(SELECT rowid FROM public_media_results WHERE namespace=? ORDER BY used_ns,rowid LIMIT ?)`, p.namespace, over); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO public_media_results(namespace,cache_key,payload,expires_ns,used_ns) VALUES(?,?,?,?,?)
		ON CONFLICT(namespace,cache_key) DO UPDATE SET payload=excluded.payload,expires_ns=excluded.expires_ns,used_ns=excluded.used_ns`,
		p.namespace, key, encoded, expires.UnixNano(), now.UnixNano()); err != nil {
		return err
	}
	return tx.Commit()
}

// objectContentKey identifies identical stored bytes delivered the same way.
func objectContentKey(sha []byte, contentType string, attachment bool) string {
	disposition := "inline"
	if attachment {
		disposition = "attachment"
	}
	hash := sha256.Sum256([]byte(hex.EncodeToString(sha) + "\x00" + contentType + "\x00" + disposition))
	return hex.EncodeToString(hash[:])
}

// reuseObject returns a stored copy of identical bytes, extending its cleanup
// deadline within its absolute expiry. Only copies that keep at least the
// grace period are reused, so a refreshed result is not cut short.
func (r *Resolver) reuseObject(ctx context.Context, contentKey string, size int64, contentType string) (string, time.Time, bool) {
	p := r.persistent
	if p == nil || r.options.CDN == nil || contentKey == "" {
		return "", time.Time{}, false
	}
	// Live: not scheduled for deletion and keeping at least the grace period.
	const live = `namespace=? AND object_key=? AND retry_ns=0 AND expires_ns>? AND limit_ns>?`
	now := time.Now()
	var key string
	var limit int64
	err := p.db.QueryRowContext(ctx, `SELECT object_key,limit_ns FROM public_media_objects
		WHERE namespace=? AND content_key=? AND size_bytes=? AND retry_ns=0 AND expires_ns>? AND limit_ns>?
		ORDER BY limit_ns DESC LIMIT 1`, p.namespace, contentKey, size, now.Add(time.Minute).UnixNano(), now.Add(objectGrace).UnixNano()).Scan(&key, &limit)
	if err != nil || !cacheObjectKey(key) || r.options.CDN.URL(key) == "" {
		return "", time.Time{}, false
	}
	// The bytes may have been removed outside this cache. Never reuse blindly.
	// A missing object can still be uploading for another request, so only
	// different bytes stop the copy being offered again.
	check, cancel := context.WithTimeout(ctx, 5*time.Second)
	object, err := r.options.CDN.Stat(check, key)
	cancel()
	if err != nil {
		return "", time.Time{}, false
	}
	if object.Size != size || object.ContentType != contentType {
		_, _ = p.db.ExecContext(ctx, `UPDATE public_media_objects SET content_key='' WHERE namespace=? AND object_key=?`, p.namespace, key)
		return "", time.Time{}, false
	}
	// Eviction or cleanup may have claimed the copy while it was checked.
	now = time.Now()
	updated, err := p.db.ExecContext(ctx, `UPDATE public_media_objects SET expires_ns=min(limit_ns,max(expires_ns,?)),used_ns=? WHERE `+live,
		now.Add(objectGrace).UnixNano(), now.UnixNano(), p.namespace, key, now.Add(time.Minute).UnixNano(), now.Add(objectGrace).UnixNano())
	if err != nil {
		return "", time.Time{}, false
	}
	if n, _ := updated.RowsAffected(); n != 1 {
		return "", time.Time{}, false
	}
	return key, time.Unix(0, limit), true
}

// Reserve before Put so even a process crash cannot leave an untracked upload.
// When the object count or byte budget is full, the least recently used live
// objects are scheduled for deletion to make room. Only a backlog of failed
// deletions refuses new objects.
func (r *Resolver) reserveObject(ctx context.Context, key string, size int64, contentKey string, limit time.Time) error {
	p := r.persistent
	if p == nil || !cacheObjectKey(key) || size < 0 {
		return ErrUnavailable
	}
	if size > p.byteLimit() {
		return ErrLimit
	}
	now := time.Now().UnixNano()
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var live, total int
	var liveBytes int64
	if err = tx.QueryRowContext(ctx, `SELECT coalesce(sum(expires_ns>?),0),coalesce(sum(CASE WHEN expires_ns>? THEN size_bytes END),0),count(*)
		FROM public_media_objects WHERE namespace=?`, now, now, p.namespace).Scan(&live, &liveBytes, &total); err != nil {
		return err
	}
	if total-live >= p.objectLimit() {
		return ErrBusy
	}
	needCount, needBytes := live+1-p.objectLimit(), liveBytes+size-p.byteLimit()
	// Small objects can cover little of a large upload, so mark victims in
	// batches until there is room. Marked rows are no longer live and drop
	// out of the next batch. Only an empty table refuses.
	for needCount > 0 || needBytes > 0 {
		rows, err := tx.QueryContext(ctx, `SELECT object_key,size_bytes FROM public_media_objects
			WHERE namespace=? AND expires_ns>? ORDER BY used_ns,rowid LIMIT ?`, p.namespace, now, evictionBatch)
		if err != nil {
			return err
		}
		var victims []string
		for (needCount > 0 || needBytes > 0) && rows.Next() {
			var victim string
			var bytes int64
			if err = rows.Scan(&victim, &bytes); err != nil {
				rows.Close()
				return err
			}
			victims = append(victims, victim)
			needCount, needBytes = needCount-1, needBytes-bytes
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(victims) == 0 {
			return ErrBusy
		}
		// Cleanup deletes the bytes. Stored results that still name a victim
		// fail their storage check on the next read and resolve again.
		args := append([]any{now, p.namespace}, keyArgs(victims)...)
		if _, err = tx.ExecContext(ctx, `UPDATE public_media_objects SET expires_ns=?,retry_ns=0
			WHERE namespace=? AND object_key IN (`+placeholders(len(victims))+`)`, args...); err != nil {
			return err
		}
	}
	// Until a stored result references it, the object is kept for the grace
	// period only; that covers the request that is delivering it.
	deadline := min(limit.UnixNano(), time.Now().Add(objectGrace).UnixNano())
	if _, err = tx.ExecContext(ctx, `INSERT INTO public_media_objects(namespace,object_key,expires_ns,retry_ns,size_bytes,content_key,limit_ns,used_ns)
		VALUES(?,?,?,0,?,?,?,?)`, p.namespace, key, deadline, size, contentKey, limit.UnixNano(), now); err != nil {
		return err
	}
	return tx.Commit()
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
	if _, err := r.persistent.db.ExecContext(ctx, `UPDATE public_media_objects SET expires_ns=?,retry_ns=0 WHERE namespace=? AND object_key IN (`+placeholders(len(keys))+`)`, args...); err != nil {
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
	_, err := p.db.ExecContext(ctx, `UPDATE public_media_objects SET expires_ns=?,retry_ns=0,content_key='' WHERE namespace=? AND object_key=?`, time.Now().Add(10*time.Minute).UnixNano(), p.namespace, key)
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
