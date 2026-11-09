package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"slices"
	"time"

	"ircord/backend/internal/model"
)

type offlineCursor struct {
	Since int64  `json:"since"`
	At    int64  `json:"at"`
	ID    string `json:"id"`
}

func offlineHistorySQL(owner string, c offlineCursor, limit int) (string, []any) {
	query := "SELECT m.payload FROM messages m JOIN entities e ON e.owner=m.owner AND e.kind='conversation' AND e.id=m.conversation WHERE m.owner=? AND m.created_ns>=?"
	args := []any{owner, c.Since}
	if c.ID != "" {
		query += " AND (m.created_ns,m.id)<(?,?)"
		args = append(args, c.At, c.ID)
	}
	query += " ORDER BY m.created_ns DESC,m.id DESC LIMIT ?"
	return query, append(args, limit+1)
}

// OfflineHistory exports retained account-visible messages without contacting
// IRC. Keyset pagination remains stable when messages are edited or redacted.
func (s *Store) OfflineHistory(ctx context.Context, owner, before string, limit int) ([]model.Message, *string, time.Time, error) {
	c := offlineCursor{Since: time.Now().Add(-7 * 24 * time.Hour).UnixNano()}
	if before != "" {
		if len(before) > 512 {
			return nil, nil, time.Time{}, ErrInvalidCursor
		}
		raw, err := base64.RawURLEncoding.DecodeString(before)
		if err != nil || json.Unmarshal(raw, &c) != nil || c.ID == "" || len(c.ID) > 200 || c.Since < time.Now().Add(-7*24*time.Hour-time.Hour).UnixNano() || c.Since > time.Now().UnixNano() {
			return nil, nil, time.Time{}, ErrInvalidCursor
		}
	}
	query, args := offlineHistorySQL(owner, c, limit)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	defer rows.Close()
	items := []model.Message{}
	for rows.Next() {
		var raw []byte
		var message model.Message
		if err = rows.Scan(&raw); err != nil {
			return nil, nil, time.Time{}, err
		}
		if err = json.Unmarshal(raw, &message); err != nil {
			return nil, nil, time.Time{}, err
		}
		items = append(items, message)
	}
	var next *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		raw, _ := json.Marshal(offlineCursor{c.Since, last.CreatedAt.UnixNano(), last.ID})
		value := base64.RawURLEncoding.EncodeToString(raw)
		next = &value
	}
	return slices.Clip(items), next, time.Unix(0, c.Since).UTC(), rows.Err()
}
