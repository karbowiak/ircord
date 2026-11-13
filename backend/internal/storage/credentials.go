package storage

import (
	"database/sql"
	"errors"
	"time"
)

// Credential operations are encrypted with the same master key as saved IRC logins.
// They live outside accounts so signup can recover even before a local account exists.
func (s *Store) SaveCredentialOperation(username string, raw []byte) error {
	_, err := s.DB.Exec("INSERT INTO credential_operations(username,payload) VALUES(?,?) ON CONFLICT(username) DO UPDATE SET payload=excluded.payload", username, s.seal(username, "credentials", raw))
	return err
}
func (s *Store) CredentialOperations() (map[string][]byte, error) {
	rows, err := s.DB.Query("SELECT username,payload FROM credential_operations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]byte{}
	for rows.Next() {
		var username string
		var raw []byte
		if err = rows.Scan(&username, &raw); err != nil {
			return nil, err
		}
		raw, err = s.open(username, "credentials", raw)
		if err != nil {
			return nil, err
		}
		out[username] = raw
	}
	return out, rows.Err()
}
func (s *Store) DeleteCredentialOperation(username string) error {
	_, err := s.DB.Exec("DELETE FROM credential_operations WHERE username=? COLLATE NOCASE", username)
	return err
}

// CreateVerifiedSession makes password verification and session insertion conditional
// on the same password version, including when a concurrent change revokes sessions.
func (s *Store) CreateVerifiedSession(id, owner string, token []byte, expires time.Time, passwordHash, userAgent string) error {
	if len(userAgent) > MaxUserAgent {
		userAgent = userAgent[:MaxUserAgent]
	}
	now := time.Now().UnixNano()
	result, err := s.DB.Exec(`INSERT INTO sessions(id,owner,token_hash,created_ns,expires_ns,last_seen_ns,user_agent)
 SELECT ?,id,?,?,?,?,? FROM accounts WHERE id=? AND deleted=0 AND password_hash=?
 AND last_seen_ns>? AND NOT EXISTS(SELECT 1 FROM credential_operations WHERE username=accounts.username COLLATE NOCASE)`,
		id, token, now, expires.UnixNano(), now, userAgent, owner, passwordHash, time.Now().Add(-AccountRetention).UnixNano())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return ErrNotFound
	}
	return err
}

// SessionsBeyond lists an account's authorized sessions after its keep newest ones,
// newest first.
func (s *Store) SessionsBeyond(owner string, keep int) ([]string, error) {
	rows, err := s.DB.Query("SELECT id FROM sessions WHERE owner=? AND revoked=0 AND expires_ns>? ORDER BY created_ns DESC, id LIMIT -1 OFFSET ?", owner, time.Now().UnixNano(), keep)
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

// PruneSessions deletes sessions that expired before the cutoff, and revoked sessions last
// used before it, with their push subscriptions.
func (s *Store) PruneSessions(before time.Time) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const stale = "expires_ns<? OR (revoked=1 AND last_seen_ns<?)"
	cutoff := before.UnixNano()
	if err = dropSessionPush(tx, "session IN (SELECT id FROM sessions WHERE "+stale+")", cutoff, cutoff); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM sessions WHERE "+stale, cutoff, cutoff); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AccountExistsByID(owner string) (bool, error) {
	var n int
	err := s.DB.QueryRow("SELECT 1 FROM accounts WHERE id=? AND deleted=0", owner).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return n == 1, err
}
