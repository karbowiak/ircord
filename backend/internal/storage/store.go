package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("record not found")

type Store struct {
	DB     *sql.DB
	cipher cipher.AEAD
	lock   *flock.Flock
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, "backend.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		return nil, fmt.Errorf("data directory already has a running backend")
	}
	fail := func(err error) (*Store, error) { _ = lock.Unlock(); return nil, err }
	keyPath := filepath.Join(dir, "master.key")
	key, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return fail(err)
		}
		var f *os.File
		f, err = os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fail(err)
		}
		_, err = f.Write(key)
		if err == nil {
			err = f.Sync()
		}
		_ = f.Close()
	}
	if err != nil {
		return fail(err)
	}
	if len(key) != 32 {
		return fail(fmt.Errorf("invalid master.key"))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fail(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return fail(err)
	}
	path := filepath.Join(dir, "ircord.db")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fail(err)
	}
	file.Close()
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=foreign_keys(ON)&_pragma=synchronous(FULL)")
	if err != nil {
		return fail(err)
	}
	// SQLite serializes writes. A short transaction owns the account sequence and
	// its entity mutation; readers use the same handle without retrying partial state.
	db.SetMaxOpenConns(1)
	s := &Store{DB: db, cipher: aead, lock: lock}
	if err = s.migrate(dir); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { err := s.DB.Close(); _ = s.lock.Unlock(); return err }

func (s *Store) seal(owner, network string, raw []byte) []byte {
	nonce := make([]byte, s.cipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return s.cipher.Seal(nonce, nonce, raw, []byte(owner+"/"+network))
}

func (s *Store) open(owner, network string, raw []byte) ([]byte, error) {
	n := s.cipher.NonceSize()
	if len(raw) < n {
		return nil, errors.New("invalid encrypted configuration")
	}
	return s.cipher.Open(nil, raw[:n], raw[n:], []byte(owner+"/"+network))
}

func (s *Store) Entities(owner string) (map[string][]json.RawMessage, error) {
	rows, err := s.DB.Query("SELECT kind,payload FROM entities WHERE owner=?", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]json.RawMessage{}
	for rows.Next() {
		var kind string
		var raw []byte
		if err = rows.Scan(&kind, &raw); err != nil {
			return nil, err
		}
		out[kind] = append(out[kind], json.RawMessage(raw))
	}
	return out, rows.Err()
}

func (s *Store) AppData(owner, kind string) (json.RawMessage, error) {
	var raw []byte
	err := s.DB.QueryRow("SELECT payload FROM app_data WHERE owner=? AND kind=?", owner, kind).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return raw, err
}

func (s *Store) SealSecret(owner, purpose string, raw []byte) []byte {
	return s.seal(owner, purpose, raw)
}

func (s *Store) OpenSecret(owner, purpose string, raw []byte) ([]byte, error) {
	return s.open(owner, purpose, raw)
}
