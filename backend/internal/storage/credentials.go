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
func (s *Store) CreateVerifiedSession(id, owner string, token []byte, expires time.Time, passwordHash string) error {
	result, err := s.DB.Exec(`INSERT INTO sessions(id,owner,token_hash,created_ns,expires_ns)
 SELECT ?,id,?,?,? FROM accounts WHERE id=? AND deleted=0 AND password_hash=?
 AND last_seen_ns>? AND NOT EXISTS(SELECT 1 FROM credential_operations WHERE username=accounts.username COLLATE NOCASE)`,
		id, token, time.Now().UnixNano(), expires.UnixNano(), owner, passwordHash, time.Now().Add(-AccountRetention).UnixNano())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return ErrNotFound
	}
	return err
}

func (s *Store) AccountExistsByID(owner string) (bool, error) {
	var n int
	err := s.DB.QueryRow("SELECT 1 FROM accounts WHERE id=? AND deleted=0", owner).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return n == 1, err
}
