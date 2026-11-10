package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"ircord/backend/internal/model"
)

// LocalPerson is a neutral local account projection, never an admin projection.
// Username remains a local login name until an upstream source verifies it.
type LocalPerson struct {
	BackendAccountID string
	Username         string
	DisplayName      string
	AvatarURL        string
}

func localPerson(id, username string, raw []byte) LocalPerson {
	v := LocalPerson{BackendAccountID: id, Username: username, DisplayName: username}
	var member model.Member
	if json.Unmarshal(raw, &member) == nil {
		if member.DisplayName != "" && member.DisplayName != "ircord" {
			v.DisplayName = member.DisplayName
		}
		v.AvatarURL = member.AvatarURL
	}
	return v
}

// LocalPeople performs bounded, ranked account matching without privileged data.
func (s *Store) LocalPeople(ctx context.Context, query string, limit int) ([]LocalPerson, bool, error) {
	limit = max(1, min(limit, 4096))
	rows, err := s.DB.QueryContext(ctx, `SELECT a.id,a.username,COALESCE(e.payload,'{}') FROM accounts a
		LEFT JOIN entities e ON e.owner=a.id AND e.kind='member' AND e.id=a.id
		WHERE a.deleted=0 AND a.username IS NOT NULL AND a.last_seen_ns>?
		AND (instr(lower(a.username),lower(?))>0 OR instr(lower(COALESCE(json_extract(e.payload,'$.displayName'),'')),lower(?))>0)
		ORDER BY CASE WHEN lower(a.username)=lower(?) THEN 0
		WHEN lower(json_extract(e.payload,'$.displayName'))=lower(?) THEN 1
		WHEN instr(lower(a.username),lower(?))=1 THEN 2
		WHEN instr(lower(COALESCE(json_extract(e.payload,'$.displayName'),'')),lower(?))=1 THEN 3 ELSE 4 END,
		lower(a.username),a.id LIMIT ?`, time.Now().Add(-AccountRetention).UnixNano(), strings.TrimSpace(query), strings.TrimSpace(query), query, query, query, query, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []LocalPerson{}
	for rows.Next() {
		var id, name string
		var raw []byte
		if err = rows.Scan(&id, &name, &raw); err != nil {
			return nil, false, err
		}
		out = append(out, localPerson(id, name, raw))
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		return model.PersonMatchRank(query, "ascii", out[i].DisplayName, out[i].Username) < model.PersonMatchRank(query, "ascii", out[j].DisplayName, out[j].Username)
	})
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	return out, more, nil
}

// LocalPersonByUsername never accepts an account ID as a username alias.
func (s *Store) LocalPersonByUsername(ctx context.Context, username string) (LocalPerson, error) {
	var id, name string
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT a.id,a.username,COALESCE(e.payload,'{}') FROM accounts a
		LEFT JOIN entities e ON e.owner=a.id AND e.kind='member' AND e.id=a.id
		WHERE a.deleted=0 AND a.last_seen_ns>? AND a.username=? COLLATE NOCASE`, time.Now().Add(-AccountRetention).UnixNano(), username).Scan(&id, &name, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return localPerson(id, name, raw), err
}

// RecentPeople returns authors of at most 500 recent messages in one owned conversation.
// The engine checks conversation/network access before calling it.
func (s *Store) RecentPeople(ctx context.Context, owner, conversation string) ([]model.Member, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT e.payload FROM entities e JOIN
		(SELECT DISTINCT sender FROM (SELECT sender FROM messages WHERE owner=? AND conversation=?
		ORDER BY created_ns DESC,id DESC LIMIT 500)) recent ON recent.sender=e.id
		WHERE e.owner=? AND e.kind='member' ORDER BY e.id`, owner, conversation, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Member{}
	for rows.Next() {
		var raw []byte
		var v model.Member
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
