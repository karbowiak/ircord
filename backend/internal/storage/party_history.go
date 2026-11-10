package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"ircord/backend/internal/model"
	"time"
)

func (s *Store) PartyLatest(party string) (model.Message, error) {
	var raw []byte
	var msg model.Message
	e := s.DB.QueryRow("SELECT payload FROM party_history WHERE party=? ORDER BY created_ns DESC,id DESC LIMIT 1", party).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return msg, nil
	}
	if e == nil {
		e = json.Unmarshal(raw, &msg)
	}
	return msg, e
}

func (s *Store) PartyAudit(party, generation string) (before time.Time, complete bool, err error) {
	now := time.Now()
	_, err = s.DB.Exec("INSERT INTO party_history_sync(party,generation,before_ns,complete) VALUES(?,?,?,0) ON CONFLICT(party) DO UPDATE SET generation=excluded.generation,before_ns=excluded.before_ns,complete=0 WHERE generation!=excluded.generation", party, generation, now.UnixNano())
	if err != nil {
		return
	}
	var ns int64
	err = s.DB.QueryRow("SELECT before_ns,complete FROM party_history_sync WHERE party=? AND generation=?", party, generation).Scan(&ns, &complete)
	before = time.Unix(0, ns).UTC()
	return
}
func (s *Store) SavePartyAudit(party, generation string, before time.Time, complete bool) error {
	_, e := s.DB.Exec("UPDATE party_history_sync SET before_ns=?,complete=? WHERE party=? AND generation=?", before.UnixNano(), complete, party, generation)
	return e
}
func (s *Store) PartyWindow(party string, lower, upper time.Time) ([]model.Message, error) {
	rows, e := s.DB.Query("SELECT payload FROM party_history WHERE party=? AND ((created_ns>? AND created_ns<?) OR (json_extract(payload,'$.editedAt') IS NOT NULL AND EXISTS(SELECT 1 FROM json_each(payload,'$.edits') x WHERE unixepoch(json_extract(x.value,'$.at'),'subsec') BETWEEN ? AND ?)))", party, lower.UnixNano(), upper.UnixNano(), float64(lower.UnixNano())/1e9-1, float64(upper.UnixNano())/1e9+1)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.Message{}
	for rows.Next() {
		var raw []byte
		var v model.Message
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) PartyEditTarget(party, upstream string) (model.Message, error) {
	var raw []byte
	var msg model.Message
	e := s.DB.QueryRow("SELECT h.payload FROM party_history h,json_each(h.payload,'$.edits') e WHERE h.party=? AND json_extract(e.value,'$.upstreamId')=? LIMIT 1", party, upstream).Scan(&raw)
	if e == nil {
		e = json.Unmarshal(raw, &msg)
	}
	return msg, e
}
