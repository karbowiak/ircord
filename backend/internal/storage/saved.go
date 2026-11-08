package storage

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"ircord/backend/internal/model"
)

// SavedEntry is a bookmark with the stored message it names, when that message
// is still in this account's history.
type SavedEntry struct {
	Saved   model.SavedMessage
	Message *model.Message
}

// ErrInvalidCursor rejects a saved-message cursor this store did not issue.
var ErrInvalidCursor = errors.New("invalid cursor")

type savedCursor struct {
	At int64  `json:"at"`
	ID string `json:"id"`
}

// SavedCursor continues a saved-message listing after entry.
func SavedCursor(entry model.SavedMessage) string {
	raw, _ := json.Marshal(savedCursor{At: entry.SavedAt.UnixNano(), ID: entry.MessageID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func parseSavedCursor(raw string) (savedCursor, error) {
	var c savedCursor
	if len(raw) > 512 {
		return c, ErrInvalidCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(b, &c) != nil || c.At <= 0 || c.ID == "" {
		return c, ErrInvalidCursor
	}
	return c, nil
}

// Saved returns one bookmark of owner, or ErrNotFound.
func (s *Store) Saved(owner, message string) (model.SavedMessage, error) {
	var v model.SavedMessage
	var at int64
	err := s.DB.QueryRow("SELECT message,conversation,network,saved_ns FROM saved_messages WHERE owner=? AND message=?", owner, message).Scan(&v.MessageID, &v.ConversationID, &v.NetworkID, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	v.SavedAt = time.Unix(0, at).UTC()
	return v, err
}

// SavedCount is how many bookmarks owner has.
func (s *Store) SavedCount(owner string) (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT count(*) FROM saved_messages WHERE owner=?", owner).Scan(&n)
	return n, err
}

// SavedList pages owner's bookmarks, newest first. A non-empty query keeps only
// bookmarks whose stored, undeleted message text contains it, ignoring ASCII
// case (SQLite's lower() folds only A-Z).
// It reads one entry past limit to tell whether another page exists.
func (s *Store) SavedList(owner, after, query string, limit int) ([]SavedEntry, bool, error) {
	sqlText := "SELECT s.message,s.conversation,s.network,s.saved_ns,m.payload FROM saved_messages s LEFT JOIN messages m ON m.owner=s.owner AND m.id=s.message AND m.conversation=s.conversation WHERE s.owner=?"
	args := []any{owner}
	if after != "" {
		c, err := parseSavedCursor(after)
		if err != nil {
			return nil, false, err
		}
		sqlText += " AND (s.saved_ns<? OR (s.saved_ns=? AND s.message<?))"
		args = append(args, c.At, c.At, c.ID)
	}
	if query != "" {
		sqlText += " AND m.payload IS NOT NULL AND coalesce(json_extract(m.payload,'$.deleted'),0)=0 AND instr(lower(m.text),lower(?))>0"
		args = append(args, query)
	}
	sqlText += " ORDER BY s.saved_ns DESC,s.message DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.DB.Query(sqlText, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []SavedEntry{}
	for rows.Next() {
		var e SavedEntry
		var at int64
		var raw []byte
		if err = rows.Scan(&e.Saved.MessageID, &e.Saved.ConversationID, &e.Saved.NetworkID, &at, &raw); err != nil {
			return nil, false, err
		}
		e.Saved.SavedAt = time.Unix(0, at).UTC()
		if raw != nil {
			var msg model.Message
			if err = json.Unmarshal(raw, &msg); err != nil {
				return nil, false, err
			}
			e.Message = &msg
		}
		out = append(out, e)
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	return out, more, nil
}

// SavedRefs lists every bookmark of owner (bounded by the per-account limit).
func (s *Store) SavedRefs(owner string) ([]model.SavedMessage, error) {
	rows, err := s.DB.Query("SELECT message,conversation,network,saved_ns FROM saved_messages WHERE owner=? ORDER BY saved_ns DESC,message DESC", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.SavedMessage{}
	for rows.Next() {
		var v model.SavedMessage
		var at int64
		if err = rows.Scan(&v.MessageID, &v.ConversationID, &v.NetworkID, &at); err != nil {
			return nil, err
		}
		v.SavedAt = time.Unix(0, at).UTC()
		out = append(out, v)
	}
	return out, rows.Err()
}
