package storage

import (
	"database/sql"
	"errors"
	"ircord/backend/internal/model"
	"time"
)

type Watermark struct {
	ID  string
	At  time.Time
	Gap bool
}

func (s *Store) AdvanceGap(owner, conversation, id string, at time.Time) error {
	_, err := s.DB.Exec("UPDATE history_state SET synced_id=?,synced_ns=? WHERE owner=? AND conversation=? AND gap_open=1", id, at.UnixNano(), owner, conversation)
	return err
}

func (s *Store) BeginGap(owner, conversation string) error {
	// A new upstream session must revalidate exhaustion: access and retention
	// may have changed, or an old msgid may have produced a false empty page.
	_, err := s.DB.Exec("INSERT INTO history_state(owner,conversation,gap_open) VALUES(?,?,1) ON CONFLICT(owner,conversation) DO UPDATE SET gap_open=1,exhausted=0", owner, conversation)
	return err
}
func (s *Store) Watermark(owner, conversation string) (Watermark, error) {
	var w Watermark
	var at int64
	err := s.DB.QueryRow("SELECT synced_id,synced_ns,gap_open FROM history_state WHERE owner=? AND conversation=?", owner, conversation).Scan(&w.ID, &at, &w.Gap)
	w.At = time.Unix(0, at).UTC()
	if errors.Is(err, sql.ErrNoRows) {
		return Watermark{Gap: true}, nil
	}
	return w, err
}
func (s *Store) CompleteGap(owner, conversation string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id string
	var at int64
	err = tx.QueryRow("SELECT upstream,created_ns FROM messages WHERE owner=? AND conversation=? AND kind IN ('message','action','notice') AND upstream!='' AND substr(upstream,1,1)!='_' ORDER BY created_ns DESC,position DESC,id DESC LIMIT 1", owner, conversation).Scan(&id, &at)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.Exec("INSERT INTO history_state(owner,conversation,synced_id,synced_ns,gap_open) VALUES(?,?,?,?,0) ON CONFLICT(owner,conversation) DO UPDATE SET synced_id=excluded.synced_id,synced_ns=excluded.synced_ns,gap_open=0", owner, conversation, id, at)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) MarkRead(owner, conversation string, at time.Time) error {
	_, err := s.DB.Exec("INSERT INTO conversation_read(owner,conversation,at_ns) VALUES(?,?,?) ON CONFLICT(owner,conversation) DO UPDATE SET at_ns=max(at_ns,excluded.at_ns)", owner, conversation, at.UnixNano())
	return err
}

func (s *Store) ReadTime(owner, conversation string) (time.Time, error) {
	var at int64
	err := s.DB.QueryRow("SELECT at_ns FROM conversation_read WHERE owner=? AND conversation=?", owner, conversation).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, at).UTC(), nil
}

// LatestMessageTime returns an actual stored message time, never the local clock.
// serverOnly restricts reconciliation to messages with a canonical upstream ID.
func (s *Store) LatestMessageTime(owner, conversation string, through time.Time, serverOnly bool) (time.Time, error) {
	query := "SELECT max(created_ns) FROM messages WHERE owner=? AND conversation=? AND kind IN ('message','action','notice')"
	args := []any{owner, conversation}
	if !through.IsZero() {
		query += " AND created_ns<=?"
		args = append(args, through.UnixNano())
	}
	if serverOnly {
		query += " AND upstream!='' AND substr(upstream,1,1)!='_'"
	}
	var at sql.NullInt64
	if err := s.DB.QueryRow(query, args...).Scan(&at); err != nil {
		return time.Time{}, err
	}
	if !at.Valid {
		return time.Time{}, nil
	}
	return time.Unix(0, at.Int64).UTC(), nil
}

// unreadScope selects the unread messages of m (aliased) against the read position r.
// A manual boundary counts the chosen message (even your own) and what others sent
// after it; otherwise everything others sent after the read position counts.
const unreadScope = "CASE WHEN r.unread_from_ns IS NOT NULL THEN m.created_ns>=r.unread_from_ns AND (m.sender!=m.owner OR m.id=r.unread_from_id) ELSE m.sender!=m.owner AND m.created_ns>coalesce(r.at_ns,0) END"

// NotificationMessageUnread is authoritative even after a manual unread
// boundary or an eviction from the workspace's in-memory history window.
func (s *Store) NotificationMessageUnread(owner, conversation, message string) (bool, error) {
	var unread bool
	err := s.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM messages m LEFT JOIN conversation_read r ON r.owner=m.owner AND r.conversation=m.conversation WHERE m.owner=? AND m.conversation=? AND m.id=? AND coalesce(json_extract(m.payload,'$.deleted'),0)=0 AND m.sender NOT IN(SELECT member FROM ignored_senders WHERE owner=m.owner) AND `+unreadScope+`)`, owner, conversation, message).Scan(&unread)
	return unread, err
}

func (s *Store) Unread(owner, conversation string) (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT count(*) FROM messages m LEFT JOIN conversation_read r ON r.owner=m.owner AND r.conversation=m.conversation WHERE m.owner=? AND m.conversation=? AND m.sender NOT IN (SELECT member FROM ignored_senders WHERE owner=m.owner) AND m.kind IN ('message','action','notice') AND "+unreadScope, owner, conversation).Scan(&n)
	return n, err
}

// MarkUnread moves the unread boundary back to a stored message of the conversation.
// The monotonic read position is kept, so upstream read markers never move backwards.
func (s *Store) MarkUnread(owner, conversation, message string) (bool, error) {
	var at int64
	err := s.DB.QueryRow("SELECT created_ns FROM messages WHERE owner=? AND conversation=? AND id=? AND kind IN ('message','action','notice')", owner, conversation, message).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = s.DB.Exec("INSERT INTO conversation_read(owner,conversation,at_ns,unread_from_ns,unread_from_id) VALUES(?,?,0,?,?) ON CONFLICT(owner,conversation) DO UPDATE SET unread_from_ns=excluded.unread_from_ns,unread_from_id=excluded.unread_from_id", owner, conversation, at, message)
	return err == nil, err
}

// ClearUnread drops a manual unread boundary, returning to the read position.
func (s *Store) ClearUnread(owner, conversation string) error {
	_, err := s.DB.Exec("UPDATE conversation_read SET unread_from_ns=NULL,unread_from_id=NULL WHERE owner=? AND conversation=?", owner, conversation)
	return err
}

// UnreadFrom is the message a manual unread boundary starts at, or "".
func (s *Store) UnreadFrom(owner, conversation string) (string, error) {
	var id sql.NullString
	err := s.DB.QueryRow("SELECT unread_from_id FROM conversation_read WHERE owner=? AND conversation=?", owner, conversation).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id.String, err
}
func (s *Store) HistoryInsert(owner string, msg model.Message) (bool, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var n int
	if err = tx.QueryRow("SELECT count(*) FROM messages WHERE owner=? AND id=?", owner, msg.ID).Scan(&n); err != nil {
		return false, err
	}
	if n != 0 {
		return false, nil
	}
	if err = putMessage(tx, owner, msg); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *Store) Mentions(owner, conversation string) (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT count(*) FROM message_mentions t JOIN messages m ON m.owner=t.owner AND m.id=t.message LEFT JOIN conversation_read r ON r.owner=m.owner AND r.conversation=m.conversation WHERE t.owner=? AND t.conversation=? AND m.sender!=m.owner AND m.kind IN ('message','action','notice') AND m.sender NOT IN (SELECT member FROM ignored_senders WHERE owner=m.owner) AND "+unreadScope, owner, conversation).Scan(&n)
	return n, err
}
