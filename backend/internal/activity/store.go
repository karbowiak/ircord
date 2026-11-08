package activity

import (
	"database/sql"
	"encoding/json"
)

// persistLocked saves the account's open interval, or removes it once there is none. Only
// intervals are kept: leases and connections belong to running processes, so after a restart
// every account starts inactive and a saved interval resumes from its original T0.
func (r *Router) persistLocked(owner string, a *account) {
	if r.options.Store == nil {
		return
	}
	if a.interval == nil {
		_ = r.options.Store.DeleteInterval(owner)
		return
	}
	raw, err := json.Marshal(a.interval)
	if err == nil {
		_ = r.options.Store.SaveInterval(owner, raw)
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
