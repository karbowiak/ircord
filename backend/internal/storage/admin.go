package storage

import (
	"context"
	"time"
)

// AdminOverview contains aggregate counts only, never account identifiers or data.
type AdminOverview struct {
	Accounts       int64 `json:"accounts" minimum:"0"`
	Sessions       int64 `json:"sessions" minimum:"0"`
	Networks       int64 `json:"networks" minimum:"0"`
	Messages       int64 `json:"messages" minimum:"0"`
	DatabaseBytes  int64 `json:"databaseBytes" minimum:"0" doc:"SQLite allocated page bytes, excluding WAL and uploaded files."`
	IRCConnections int   `json:"ircConnections" minimum:"0" doc:"Currently connected upstream IRC sessions, not IRC users or accounts."`
}

func (s *Store) AdminOverview(ctx context.Context) (AdminOverview, error) {
	var out AdminOverview
	now, cutoff := time.Now().UnixNano(), time.Now().Add(-AccountRetention).UnixNano()
	err := s.DB.QueryRowContext(ctx, `SELECT
 (SELECT count(*) FROM accounts WHERE deleted=0 AND last_seen_ns>?),
 (SELECT count(*) FROM sessions s JOIN accounts a ON a.id=s.owner WHERE s.revoked=0 AND s.expires_ns>? AND a.deleted=0 AND a.last_seen_ns>?),
 (SELECT count(*) FROM networks), (SELECT count(*) FROM messages)`, cutoff, now, cutoff).Scan(&out.Accounts, &out.Sessions, &out.Networks, &out.Messages)
	if err != nil {
		return out, err
	}
	var pages, size int64
	if err = s.DB.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pages); err != nil {
		return out, err
	}
	if err = s.DB.QueryRowContext(ctx, "PRAGMA page_size").Scan(&size); err != nil {
		return out, err
	}
	out.DatabaseBytes = pages * size
	return out, nil
}

// AdminEvent is one audited operator action. It holds no credential material.
type AdminEvent struct {
	Action, Target, Outcome string
	At                      time.Time
}

// RecordAdminEvent appends an operator action to the audit log.
func (s *Store) RecordAdminEvent(action, target, outcome string) error {
	if len(target) > 200 {
		target = target[:200]
	}
	_, err := s.DB.Exec("INSERT INTO admin_audit(action,target,outcome,created_ns) VALUES(?,?,?,?)", action, target, outcome, time.Now().UnixNano())
	return err
}

// AdminEvents returns the audit log, newest first.
func (s *Store) AdminEvents(limit int) ([]AdminEvent, error) {
	rows, err := s.DB.Query("SELECT action,target,outcome,created_ns FROM admin_audit ORDER BY created_ns DESC, id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminEvent{}
	for rows.Next() {
		var v AdminEvent
		var at int64
		if err = rows.Scan(&v.Action, &v.Target, &v.Outcome, &at); err != nil {
			return nil, err
		}
		v.At = time.Unix(0, at).UTC()
		out = append(out, v)
	}
	return out, rows.Err()
}
