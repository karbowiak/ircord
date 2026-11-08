package activity

import (
	"database/sql"
	"encoding/json"
	"log/slog"
)

// persistLocked queues the account's open interval for saving, or its removal once there is
// none. Only intervals are kept: leases and connections belong to running processes, so after
// a restart every account starts inactive and a saved interval resumes from its original T0.
// The write happens in the background (see flush), never under the router or workspace locks.
func (r *Router) persistLocked(owner string, a *account) {
	var raw []byte
	if a.interval != nil {
		var err error
		if raw, err = json.Marshal(a.interval); err != nil {
			return
		}
	}
	r.queueLocked(owner, raw)
}

// queueLocked records the latest snapshot for an owner (nil removes it) and wakes the writer.
func (r *Router) queueLocked(owner string, raw []byte) {
	if r.options.Store == nil {
		return
	}
	r.persistMu.Lock()
	r.pending[owner] = raw
	r.persistMu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// flush writes every queued snapshot. Writes are serialized and each owner's latest snapshot
// replaces any older one still queued, so an older state never overwrites a newer one.
func (r *Router) flush() {
	if r.options.Store == nil {
		return
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	r.persistMu.Lock()
	batch := r.pending
	r.pending = map[string][]byte{}
	r.persistMu.Unlock()
	for owner, raw := range batch {
		var err error
		if raw == nil {
			err = r.options.Store.DeleteInterval(owner)
		} else {
			err = r.options.Store.SaveInterval(owner, raw)
		}
		if err != nil {
			// The restart guarantee degrades for this account; nothing about it is logged.
			slog.Warn("A notification interval could not be saved", "error", err)
		}
	}
}

func decodeInterval(raw []byte) *interval {
	var iv interval
	if json.Unmarshal(raw, &iv) != nil || iv.T0.IsZero() || iv.Phase < phaseGrace || iv.Phase >= phaseAll {
		return nil
	}
	if len(iv.Candidates) > MaxCandidates {
		iv.Candidates = iv.Candidates[len(iv.Candidates)-MaxCandidates:]
	}
	return &iv
}

// SQLStore keeps intervals in the server database. Each account has at most one row, and a
// row only exists while an interval is waiting to escalate.
type SQLStore struct{ DB *sql.DB }

// NewSQLStore creates the table when needed.
func NewSQLStore(db *sql.DB) (*SQLStore, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS notification_intervals(owner TEXT PRIMARY KEY, data BLOB NOT NULL)`)
	if err != nil {
		return nil, err
	}
	return &SQLStore{db}, nil
}

func (s *SQLStore) SaveInterval(owner string, data []byte) error {
	_, err := s.DB.Exec("INSERT INTO notification_intervals(owner,data) VALUES(?,?) ON CONFLICT(owner) DO UPDATE SET data=excluded.data", owner, data)
	return err
}

func (s *SQLStore) DeleteInterval(owner string) error {
	_, err := s.DB.Exec("DELETE FROM notification_intervals WHERE owner=?", owner)
	return err
}

func (s *SQLStore) LoadIntervals() (map[string][]byte, error) {
	rows, err := s.DB.Query("SELECT owner,data FROM notification_intervals")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]byte{}
	for rows.Next() {
		var owner string
		var data []byte
		if err = rows.Scan(&owner, &data); err != nil {
			return nil, err
		}
		out[owner] = data
	}
	return out, rows.Err()
}
