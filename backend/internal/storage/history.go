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
	_, err := s.DB.Exec("INSERT INTO history_state(owner,conversation,gap_open) VALUES(?,?,1) ON CONFLICT(owner,conversation) DO UPDATE SET gap_open=1", owner, conversation)
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
func (s *Store) Unread(owner, conversation string) (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT count(*) FROM messages WHERE owner=? AND conversation=? AND sender!=? AND sender NOT IN (SELECT member FROM ignored_senders WHERE owner=messages.owner) AND kind IN ('message','action','notice') AND created_ns>coalesce((SELECT at_ns FROM conversation_read WHERE owner=? AND conversation=?),0)", owner, conversation, owner, owner, conversation).Scan(&n)
	return n, err
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
	err := s.DB.QueryRow("SELECT count(*) FROM message_mentions t JOIN messages m ON m.owner=t.owner AND m.id=t.message WHERE t.owner=? AND t.conversation=? AND m.sender!=? AND m.kind IN ('message','action','notice') AND m.sender NOT IN (SELECT member FROM ignored_senders WHERE owner=m.owner) AND m.created_ns>coalesce((SELECT at_ns FROM conversation_read WHERE owner=? AND conversation=?),0)", owner, conversation, owner, owner, conversation).Scan(&n)
	return n, err
}
