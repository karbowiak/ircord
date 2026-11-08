package storage

import (
	"time"
)

// SessionTouchInterval bounds how often a session's last-seen time is written.
const SessionTouchInterval = time.Minute

// MaxUserAgent is how much of a browser's User-Agent header is kept for its device label.
const MaxUserAgent = 256

// DeviceSession is a live (not revoked, not expired) session with its device metadata.
// A session is one browser profile's sign-in: its tabs share the cookie, so they share it.
type DeviceSession struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
	// LastSeenAt is zero for sessions from before last-seen times were recorded.
	LastSeenAt time.Time
	UserAgent  string
	// Push says whether a Web Push subscription is registered for the session.
	Push bool
}

// SetSessionClient records the browser that signed a session in, and marks it seen now.
func (s *Store) SetSessionClient(id, userAgent string) error {
	if len(userAgent) > MaxUserAgent {
		userAgent = userAgent[:MaxUserAgent]
	}
	_, err := s.DB.Exec("UPDATE sessions SET user_agent=?, last_seen_ns=? WHERE id=?", userAgent, time.Now().UnixNano(), id)
	return err
}

// TouchSession marks a session seen, at most once per SessionTouchInterval.
func (s *Store) TouchSession(id string) error {
	now := time.Now()
	_, err := s.DB.Exec("UPDATE sessions SET last_seen_ns=? WHERE id=? AND revoked=0 AND last_seen_ns<?", now.UnixNano(), id, now.Add(-SessionTouchInterval).UnixNano())
	return err
}

// hasPushTable reports whether the push service has created its table (it does so on start).
func (s *Store) hasPushTable() (bool, error) {
	var found bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='push_subscriptions')").Scan(&found)
	return found, err
}

// DeviceSessions lists an account's currently authorized sessions, newest first.
func (s *Store) DeviceSessions(owner string) ([]DeviceSession, error) {
	push, err := s.hasPushTable()
	if err != nil {
		return nil, err
	}
	query := "SELECT id,created_ns,expires_ns,last_seen_ns,user_agent,0 FROM sessions s WHERE owner=? AND revoked=0 AND expires_ns>? ORDER BY created_ns DESC"
	if push {
		query = "SELECT id,created_ns,expires_ns,last_seen_ns,user_agent,EXISTS(SELECT 1 FROM push_subscriptions p WHERE p.owner=s.owner AND p.session=s.id) FROM sessions s WHERE owner=? AND revoked=0 AND expires_ns>? ORDER BY created_ns DESC"
	}
	rows, err := s.DB.Query(query, owner, time.Now().UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeviceSession{}
	for rows.Next() {
		var v DeviceSession
		var created, expires, seen int64
		if err = rows.Scan(&v.ID, &created, &expires, &seen, &v.UserAgent, &v.Push); err != nil {
			return nil, err
		}
		v.CreatedAt = time.Unix(0, created).UTC()
		v.ExpiresAt = time.Unix(0, expires).UTC()
		if seen > 0 {
			v.LastSeenAt = time.Unix(0, seen).UTC()
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// dropRevokedPush removes the push subscriptions of an account's revoked sessions, so a
// signed-out device stops being a notification target at once (dispatch also rechecks).
func (s *Store) dropRevokedPush(owner string) error {
	push, err := s.hasPushTable()
	if err != nil || !push {
		return err
	}
	_, err = s.DB.Exec("DELETE FROM push_subscriptions WHERE owner=? AND session IN (SELECT id FROM sessions WHERE owner=? AND revoked=1)", owner, owner)
	return err
}
