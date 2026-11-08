// Package mediacdn stores bounded, content-addressed copies of public linked media.
// Source URL hashes and removal tombstones stay durable across backend restarts.
package mediacdn

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"ircord/backend/internal/filehost"
	"ircord/backend/internal/preview"
	"ircord/backend/internal/storage"
)

var ErrRemoved = errors.New("this media was removed")
var ErrLimit = errors.New("media storage limit reached")

type Record struct {
	Hash        string `json:"hash"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Animated    bool   `json:"animated,omitempty"`
}
type Options struct {
	Store                       *storage.Store
	Blob                        filehost.Blob
	Fetcher                     *preview.Service
	PublicURL                   string
	MaxFileBytes, MaxTotalBytes int64
	Retention                   time.Duration
}
type flight struct {
	done   chan struct{}
	record Record
	err    error
}
type Service struct {
	options Options
	mu      sync.Mutex
	flights map[string]*flight
}

func New(o Options) (*Service, error) {
	u, err := url.Parse(o.PublicURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost"))) {
		return nil, errors.New("media public URL requires HTTPS (HTTP allowed only on loopback)")
	}
	if o.Store == nil || o.Blob == nil {
		return nil, errors.New("media metadata and object storage are required")
	}
	if o.Fetcher == nil {
		o.Fetcher = preview.New(preview.Options{})
	}
	if o.MaxFileBytes <= 0 {
		o.MaxFileBytes = 50 << 20
	}
	if o.MaxTotalBytes <= 0 {
		o.MaxTotalBytes = 10 << 30
	}
	if o.MaxFileBytes > o.MaxTotalBytes {
		return nil, errors.New("media file limit exceeds total storage limit")
	}
	if o.Retention <= 0 {
		o.Retention = 30 * 24 * time.Hour
	}
	o.PublicURL = strings.TrimRight(o.PublicURL, "/")
	return &Service{options: o, flights: map[string]*flight{}}, nil
}
func digest(value []byte) string { sum := sha256.Sum256(value); return hex.EncodeToString(sum[:]) }
func source(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || len(raw) > 2048 {
		return "", errors.New("media requires a public HTTPS URL")
	}
	u.Fragment = ""
	return digest([]byte(u.String())), nil
}
func (s *Service) objectURL(hash string) string { return s.options.PublicURL + "/" + hash }
func (s *Service) lookup(ctx context.Context, key string) (Record, error) {
	var r Record
	var removed int
	var created int64
	var denied int
	if err := s.options.Store.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM media_denied_sources WHERE source_hash=?)", key).Scan(&denied); err != nil {
		return r, err
	}
	if denied != 0 {
		return r, ErrRemoved
	}
	err := s.options.Store.DB.QueryRowContext(ctx, `SELECT o.hash,o.size,o.content_type,o.width,o.height,o.animated,o.created_ns,(o.removed OR s.removed) FROM media_sources s JOIN media_objects o ON o.hash=s.object_hash WHERE s.source_hash=?`, key).Scan(&r.Hash, &r.Size, &r.ContentType, &r.Width, &r.Height, &r.Animated, &created, &removed)
	if err != nil {
		return r, err
	}
	if removed != 0 || time.Since(time.Unix(0, created)) > s.options.Retention {
		return r, ErrRemoved
	}
	r.URL = s.objectURL(r.Hash)
	return r, nil
}
func (s *Service) Fetch(ctx context.Context, raw string) (record Record, err error) {
	key, err := source(raw)
	if err != nil {
		return record, err
	}
	s.mu.Lock()
	if f, ok := s.flights[key]; ok {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return record, ctx.Err()
		case <-f.done:
			return f.record, f.err
		}
	}
	f := &flight{done: make(chan struct{})}
	s.flights[key] = f
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		f.record, f.err = record, err
		delete(s.flights, key)
		close(f.done)
		s.mu.Unlock()
	}()
	if record, err = s.lookup(ctx, key); err == nil || !errors.Is(err, sql.ErrNoRows) {
		return record, err
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	downloaded, err := s.options.Fetcher.DownloadMedia(ctx, raw, s.options.MaxFileBytes)
	if err != nil {
		return Record{}, err
	}
	hash := digest(downloaded.Data)
	s.mu.Lock()
	defer s.mu.Unlock()
	// Removal may have raced an in-flight download. Never publish it afterwards.
	if _, err = s.lookup(ctx, key); errors.Is(err, ErrRemoved) {
		return Record{}, err
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Record{}, err
	}
	var removed int
	var created int64
	err = s.options.Store.DB.QueryRowContext(ctx, "SELECT removed,created_ns FROM media_objects WHERE hash=?", hash).Scan(&removed, &created)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Record{}, err
	}
	exists := err == nil
	if exists && (removed != 0 || time.Since(time.Unix(0, created)) > s.options.Retention) {
		return Record{}, ErrRemoved
	}
	var sources int
	if err = s.options.Store.DB.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM media_sources)+(SELECT count(*) FROM media_denied_sources)").Scan(&sources); err != nil {
		return Record{}, err
	}
	if sources >= 100000 {
		return Record{}, ErrLimit
	}
	if !exists {
		var used int64
		if err = s.options.Store.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(size),0) FROM media_objects WHERE removed=0 OR purged=0").Scan(&used); err != nil {
			return Record{}, err
		}
		if used+int64(len(downloaded.Data)) > s.options.MaxTotalBytes {
			return Record{}, ErrLimit
		}
		if err = s.options.Blob.Put(ctx, hash, bytes.NewReader(downloaded.Data), int64(len(downloaded.Data)), downloaded.ContentType); err != nil {
			return Record{}, errors.New("media object storage is unavailable")
		}
	}
	tx, err := s.options.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback()
	if !exists {
		_, err = tx.ExecContext(ctx, "INSERT INTO media_objects(hash,size,content_type,width,height,animated,created_ns) VALUES(?,?,?,?,?,?,?)", hash, len(downloaded.Data), downloaded.ContentType, downloaded.Width, downloaded.Height, downloaded.Animated, time.Now().UnixNano())
		if err != nil {
			return Record{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO media_sources(source_hash,object_hash) VALUES(?,?) ON CONFLICT(source_hash) DO NOTHING", key, hash); err != nil {
		return Record{}, err
	}
	if err = tx.Commit(); err != nil {
		return Record{}, err
	}
	return Record{Hash: hash, URL: s.objectURL(hash), Size: int64(len(downloaded.Data)), ContentType: downloaded.ContentType, Width: downloaded.Width, Height: downloaded.Height, Animated: downloaded.Animated}, nil
}

// Track associates only hashes with private message context. It performs no remote I/O.
func (s *Service) Track(ctx context.Context, owner, conversation, message, raw string) error {
	key, err := source(raw)
	if err != nil {
		return err
	}
	_, err = s.options.Store.DB.ExecContext(ctx, "INSERT INTO media_references(owner,conversation,message,source_hash) VALUES(?,?,?,?) ON CONFLICT DO NOTHING", owner, conversation, message, key)
	return err
}
func validHash(hash string) bool {
	return len(hash) == 64 && strings.Trim(hash, "0123456789abcdef") == ""
} // gitleaks:allow
func (s *Service) remove(ctx context.Context, hash string) error {
	if !validHash(hash) {
		return errors.New("invalid media hash")
	}
	result, err := s.options.Store.DB.ExecContext(ctx, "UPDATE media_objects SET removed=1 WHERE hash=?", hash)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	if err = s.options.Blob.Delete(ctx, hash); err != nil {
		return errors.New("media is blocked; object removal will be retried")
	}
	_, err = s.options.Store.DB.ExecContext(ctx, "UPDATE media_objects SET purged=1 WHERE hash=?", hash)
	return err
}
func (s *Service) Remove(ctx context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.remove(ctx, hash)
}

// RemoveMessage releases this account's references. Shared files remain while a
// different retained message references them. Orphaned sources are tombstoned
// before deletion so a queued download cannot recreate removed media.
func (s *Service) RemoveMessage(ctx context.Context, owner, conversation, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.options.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT source_hash FROM media_references WHERE owner=? AND conversation=? AND message=?", owner, conversation, message)
	if err != nil {
		return err
	}
	var keys []string
	for rows.Next() {
		var k string
		if err = rows.Scan(&k); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, k)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM media_references WHERE owner=? AND conversation=? AND message=?", owner, conversation, message); err != nil {
		return err
	}
	hashes := map[string]bool{}
	for _, key := range keys {
		var refs int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM media_references WHERE source_hash=?", key).Scan(&refs); err != nil {
			return err
		}
		if refs != 0 {
			continue
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO media_denied_sources(source_hash) VALUES(?) ON CONFLICT DO NOTHING", key); err != nil {
			return err
		}
		var hash string
		if err = tx.QueryRowContext(ctx, "SELECT object_hash FROM media_sources WHERE source_hash=?", key).Scan(&hash); errors.Is(err, sql.ErrNoRows) {
			continue
		} else if err != nil {
			return err
		}
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM media_references r JOIN media_sources s ON s.source_hash=r.source_hash WHERE s.object_hash=?", hash).Scan(&refs); err != nil {
			return err
		}
		if refs == 0 {
			hashes[hash] = true
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for hash := range hashes {
		if err = s.remove(ctx, hash); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Sweep(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Recover reference cleanup after a full queue, restart, or transient failure.
	tx, err := s.options.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO media_denied_sources(source_hash)
 SELECT DISTINCT r.source_hash FROM media_references r
 WHERE EXISTS(SELECT 1 FROM messages m WHERE m.owner=r.owner AND m.conversation=r.conversation AND m.id=r.message AND json_extract(m.payload,'$.deleted')=1)
 AND NOT EXISTS(SELECT 1 FROM media_references other WHERE other.source_hash=r.source_hash
 AND NOT EXISTS(SELECT 1 FROM messages m WHERE m.owner=other.owner AND m.conversation=other.conversation AND m.id=other.message AND json_extract(m.payload,'$.deleted')=1))
 ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM media_references AS r WHERE EXISTS(SELECT 1 FROM messages m WHERE m.owner=r.owner AND m.conversation=r.conversation AND m.id=r.message AND json_extract(m.payload,'$.deleted')=1)`); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	rows, err := s.options.Store.DB.QueryContext(ctx, `SELECT hash FROM media_objects o WHERE (removed=1 AND purged=0) OR
 (removed=0 AND (created_ns<? OR (created_ns<? AND NOT EXISTS(SELECT 1 FROM media_sources s JOIN media_references r ON r.source_hash=s.source_hash WHERE s.object_hash=o.hash)))) LIMIT 100`, time.Now().Add(-s.options.Retention).UnixNano(), time.Now().Add(-10*time.Minute).UnixNano())
	if err != nil {
		return err
	}
	var hashes []string
	for rows.Next() {
		var hash string
		if err = rows.Scan(&hash); err != nil {
			rows.Close()
			return err
		}
		hashes = append(hashes, hash)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, hash := range hashes {
		if err = s.remove(ctx, hash); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Open(ctx context.Context, hash string) (io.ReadCloser, Record, error) {
	if !validHash(hash) {
		return nil, Record{}, sql.ErrNoRows
	}
	r := Record{Hash: hash}
	var created int64
	err := s.options.Store.DB.QueryRowContext(ctx, "SELECT size,content_type,width,height,animated,created_ns FROM media_objects WHERE hash=? AND removed=0", hash).Scan(&r.Size, &r.ContentType, &r.Width, &r.Height, &r.Animated, &created)
	if err != nil {
		return nil, r, err
	}
	if time.Since(time.Unix(0, created)) > s.options.Retention {
		return nil, r, ErrRemoved
	}
	reader, err := s.options.Blob.Open(ctx, hash)
	if err != nil {
		return nil, r, errors.New("media unavailable")
	}
	return reader, r, nil
}

func (s *Service) Lookup(ctx context.Context, raw string) (Record, error) {
	key, err := source(raw)
	if err != nil {
		return Record{}, err
	}
	return s.lookup(ctx, key)
}
