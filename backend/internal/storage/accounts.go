package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"ircord/backend/internal/model"
	"time"
)

const AccountRetention = 90 * 24 * time.Hour

type Account struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}
type Session struct {
	ID        string    `json:"id"`
	Owner     string    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Revoked   bool      `json:"revoked"`
}

func accountDates(a *Account, created, seen int64) {
	a.CreatedAt = time.Unix(0, created).UTC()
	a.LastSeenAt = time.Unix(0, seen).UTC()
	a.ExpiresAt = a.LastSeenAt.Add(AccountRetention)
}
func (s *Store) CreateAccount(id, keyHash string) (Account, error) {
	now := time.Now().UTC()
	a := Account{ID: id, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(AccountRetention)}
	tx, err := s.DB.Begin()
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO accounts(id,key_hash,created_ns,last_seen_ns) VALUES(?,?,?,?)", id, keyHash, now.UnixNano(), now.UnixNano()); err != nil {
		return a, err
	}
	profile := model.Member{ID: id, Nick: "ircord", DisplayName: "ircord", NameSource: "nick", Color: "lavender", Status: "online", Role: "member"}
	raw, _ := json.Marshal(profile)
	if _, err = tx.Exec("INSERT INTO entities(owner,kind,id,payload) VALUES(?,'member',?,?)", id, id, raw); err != nil {
		return a, err
	}
	return a, tx.Commit()
}
func (s *Store) AccountByKey(hash string) (Account, error) {
	var a Account
	var created, seen int64
	err := s.DB.QueryRow("SELECT id,created_ns,last_seen_ns FROM accounts WHERE key_hash=? AND deleted=0 AND last_seen_ns>?", hash, time.Now().Add(-AccountRetention).UnixNano()).Scan(&a.ID, &created, &seen)
	accountDates(&a, created, seen)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return a, err
}
func (s *Store) Touch(owner string) error {
	now := time.Now()
	_, err := s.DB.Exec("UPDATE accounts SET last_seen_ns=? WHERE id=? AND last_seen_ns<?", now.UnixNano(), owner, now.Add(-time.Minute).UnixNano())
	return err
}
func (s *Store) CreateSession(id, owner string, hash []byte, expires time.Time) error {
	_, err := s.DB.Exec("INSERT INTO sessions(id,owner,token_hash,created_ns,expires_ns) VALUES(?,?,?,?,?)", id, owner, hash, time.Now().UnixNano(), expires.UnixNano())
	return err
}
func (s *Store) SessionByToken(hash []byte) (Account, Session, error) {
	var a Account
	var v Session
	var accountAt, seen, created, expires int64
	err := s.DB.QueryRow("SELECT a.id,a.created_ns,a.last_seen_ns,s.id,s.created_ns,s.expires_ns,s.revoked FROM sessions s JOIN accounts a ON a.id=s.owner WHERE s.token_hash=? AND s.revoked=0 AND s.expires_ns>? AND a.deleted=0 AND a.last_seen_ns>?", hash, time.Now().UnixNano(), time.Now().Add(-AccountRetention).UnixNano()).Scan(&a.ID, &accountAt, &seen, &v.ID, &created, &expires, &v.Revoked)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	accountDates(&a, accountAt, seen)
	v.Owner = a.ID
	v.CreatedAt = time.Unix(0, created).UTC()
	v.ExpiresAt = time.Unix(0, expires).UTC()
	return a, v, err
}
func (s *Store) SessionActive(owner, id string) bool {
	var n int
	err := s.DB.QueryRow("SELECT count(*) FROM sessions s JOIN accounts a ON a.id=s.owner WHERE s.owner=? AND s.id=? AND s.revoked=0 AND s.expires_ns>? AND a.deleted=0 AND a.last_seen_ns>?", owner, id, time.Now().UnixNano(), time.Now().Add(-AccountRetention).UnixNano()).Scan(&n)
	return err == nil && n == 1
}
func (s *Store) Sessions(owner string) ([]Session, error) {
	rows, err := s.DB.Query("SELECT id,created_ns,expires_ns,revoked FROM sessions WHERE owner=? AND expires_ns>? ORDER BY created_ns DESC", owner, time.Now().UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var v Session
		var created, expires int64
		if err = rows.Scan(&v.ID, &created, &expires, &v.Revoked); err != nil {
			return nil, err
		}
		v.Owner = owner
		v.CreatedAt = time.Unix(0, created).UTC()
		v.ExpiresAt = time.Unix(0, expires).UTC()
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) RevokeSession(owner, id string) error {
	result, err := s.DB.Exec("UPDATE sessions SET revoked=1 WHERE owner=? AND id=?", owner, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) RotateKey(owner, hash, keepSession string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE accounts SET key_hash=? WHERE id=?", hash, owner); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE sessions SET revoked=1 WHERE owner=? AND id!=?", owner, keepSession); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) ExpiredAccounts(before time.Time) ([]string, error) {
	rows, err := s.DB.Query("SELECT id FROM accounts WHERE last_seen_ns<? OR deleted=1", before.UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
func (s *Store) DeleteAccount(owner string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"history_state", "conversation_read", "reactions"} {
		if _, err = tx.Exec("DELETE FROM "+table+" WHERE owner=?", owner); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("DELETE FROM accounts WHERE id=?", owner); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MarkDeleting(owner string) error {
	_, err := s.DB.Exec("UPDATE accounts SET deleted=1 WHERE id=?", owner)
	return err
}
