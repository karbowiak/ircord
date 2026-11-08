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
	Username   string    `json:"username"`
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

// ErrUsernameTaken is returned when another account already has the username (any case).
var ErrUsernameTaken = errors.New("username is already taken")

// CreateAccount creates an account without a password (tests and internal fixtures).
func (s *Store) CreateAccount(id, username string) (Account, error) {
	return s.CreateUser(id, username, "")
}

// CreateUser creates a username and password account. key_hash is a unique placeholder
// left over from key accounts.
func (s *Store) CreateUser(id, username, passwordHash string) (Account, error) {
	now := time.Now().UTC()
	a := Account{ID: id, Username: username, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(AccountRetention)}
	tx, err := s.DB.Begin()
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	var taken bool
	if err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM accounts WHERE username=? COLLATE NOCASE)", username).Scan(&taken); err != nil {
		return a, err
	}
	if taken {
		return a, ErrUsernameTaken
	}
	if _, err = tx.Exec("INSERT INTO accounts(id,key_hash,username,password_hash,created_ns,last_seen_ns) VALUES(?,?,?,?,?,?)", id, "user:"+id, username, passwordHash, now.UnixNano(), now.UnixNano()); err != nil {
		return a, err
	}
	profile := model.Member{ID: id, Nick: "ircord", DisplayName: "ircord", NameSource: "nick", Color: "lavender", Status: "online", Role: "member"}
	raw, _ := json.Marshal(profile)
	if _, err = tx.Exec("INSERT INTO entities(owner,kind,id,payload) VALUES(?,'member',?,?)", id, id, raw); err != nil {
		return a, err
	}
	return a, tx.Commit()
}

// AccountByUsername finds a live account by username (any case), with its password hash.
func (s *Store) AccountByUsername(username string) (Account, string, error) {
	var a Account
	var hash sql.NullString
	var created, seen int64
	err := s.DB.QueryRow("SELECT id,username,password_hash,created_ns,last_seen_ns FROM accounts WHERE username=? COLLATE NOCASE AND deleted=0 AND last_seen_ns>?", username, time.Now().Add(-AccountRetention).UnixNano()).Scan(&a.ID, &a.Username, &hash, &created, &seen)
	accountDates(&a, created, seen)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return a, hash.String, err
}

// UsernameTaken reports whether any account, live or awaiting deletion, has the username.
func (s *Store) UsernameTaken(username string) (bool, error) {
	var taken bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM accounts WHERE username=? COLLATE NOCASE)", username).Scan(&taken)
	return taken, err
}

// PasswordHash returns an account's password hash.
func (s *Store) PasswordHash(owner string) (string, error) {
	var hash sql.NullString
	err := s.DB.QueryRow("SELECT password_hash FROM accounts WHERE id=? AND deleted=0", owner).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return hash.String, err
}

// SetPassword replaces an account's password hash and revokes its other sessions.
func (s *Store) SetPassword(owner, hash, keepSession string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE accounts SET password_hash=? WHERE id=?", hash, owner); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE sessions SET revoked=1 WHERE owner=? AND id!=?", owner, keepSession); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.dropRevokedPush(owner)
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
	var username sql.NullString
	err := s.DB.QueryRow("SELECT a.id,a.username,a.created_ns,a.last_seen_ns,s.id,s.created_ns,s.expires_ns,s.revoked FROM sessions s JOIN accounts a ON a.id=s.owner WHERE s.token_hash=? AND s.revoked=0 AND s.expires_ns>? AND a.deleted=0 AND a.last_seen_ns>? AND a.username IS NOT NULL", hash, time.Now().UnixNano(), time.Now().Add(-AccountRetention).UnixNano()).Scan(&a.ID, &username, &accountAt, &seen, &v.ID, &created, &expires, &v.Revoked)
	a.Username = username.String
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
	return s.revokeSession("UPDATE sessions SET revoked=1 WHERE owner=? AND id=?", owner, id)
}

// RevokeLiveSession revokes a session that is still authorized; ErrNotFound means another
// request revoked it first (or it expired).
func (s *Store) RevokeLiveSession(owner, id string) error {
	return s.revokeSession("UPDATE sessions SET revoked=1 WHERE owner=? AND id=? AND revoked=0 AND expires_ns>?", owner, id, time.Now().UnixNano())
}
func (s *Store) revokeSession(query string, args ...any) error {
	owner := args[0].(string)
	result, err := s.DB.Exec(query, args...)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return s.dropRevokedPush(owner)
}
func (s *Store) ExpiredAccounts(before time.Time) ([]string, error) {
	// Key accounts (no username) were retired with #64 and are removed too.
	rows, err := s.DB.Query("SELECT id FROM accounts WHERE last_seen_ns<? OR deleted=1 OR username IS NULL", before.UnixNano())
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
