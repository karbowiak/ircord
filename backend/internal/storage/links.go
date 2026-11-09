package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/msgindex"
)

// indexMessage replaces search flags, text tokens, and catalogued links.
// No provider fetch or media storage runs during indexing.
func indexMessage(q interface {
	Exec(string, ...any) (sql.Result, error)
}, owner string, msg model.Message) error {
	if _, err := q.Exec("UPDATE messages SET search_visible=?,search_kinds=? WHERE owner=? AND id=?", !msg.Deleted, msgindex.SearchKinds(msg), owner, msg.ID); err != nil {
		return err
	}
	if _, err := q.Exec("DELETE FROM message_links WHERE owner=? AND message=?", owner, msg.ID); err != nil {
		return err
	}
	if !msgindex.Indexed(msg) {
		return nil
	}
	for i, link := range msgindex.Links(msg.Text) {
		if _, err := q.Exec("INSERT INTO message_links(owner,message,idx,conversation,sender,created_ns,kind,source,url,identity,host) VALUES(?,?,?,?,?,?,?,?,?,?,?)", owner, msg.ID, i, msg.ConversationID, msg.SenderID, msg.CreatedAt.UnixNano(), link.Kind, "link", link.URL, link.Identity, link.Host); err != nil {
			return err
		}
	}
	return nil
}

// The backfill indexes messages stored before the index existed, or under
// older extraction rules, in bounded batches. Its checkpoint survives restarts.
const linkBackfill = "message-links"

// LinkBackfillBatch is how many stored messages one backfill step reads.
var LinkBackfillBatch = 500

// LinkIndexComplete reports whether every message stored before the index
// existed has been indexed under the current rules. Messages stored since are
// indexed as they are written, so a complete index covers all stored history.
func (s *Store) LinkIndexComplete() (bool, error) {
	var version, done int
	err := s.DB.QueryRow("SELECT version,done FROM index_jobs WHERE name=?", linkBackfill).Scan(&version, &done)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil && version == msgindex.Version && done == 1, err
}

// BackfillLinksStep indexes the next batch of stored messages after the
// checkpoint and advances it in the same transaction, so a crash repeats at
// most one batch and repeating one never adds rows. It reports whether the
// backfill has finished. Only history already stored for each account is read;
// nothing is fetched.
func (s *Store) BackfillLinksStep() (bool, error) {
	tx, err := s.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var version, done int
	var last int64
	err = tx.QueryRow("SELECT version,last_rowid,done FROM index_jobs WHERE name=?", linkBackfill).Scan(&version, &last, &done)
	if errors.Is(err, sql.ErrNoRows) || err == nil && version != msgindex.Version {
		version, last, done = msgindex.Version, 0, 0
		err = nil
	}
	if err != nil {
		return false, err
	}
	if done == 1 {
		return true, nil
	}
	rows, err := tx.Query("SELECT rowid,owner,payload FROM messages WHERE rowid>? ORDER BY rowid LIMIT ?", last, LinkBackfillBatch)
	if err != nil {
		return false, err
	}
	type stored struct {
		owner string
		msg   model.Message
	}
	batch := []stored{}
	for rows.Next() {
		var raw []byte
		var item stored
		if err = rows.Scan(&last, &item.owner, &raw); err != nil {
			rows.Close()
			return false, err
		}
		// An unreadable payload is skipped rather than stopping the backfill for everyone.
		if json.Unmarshal(raw, &item.msg) == nil && item.msg.ID != "" {
			batch = append(batch, item)
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return false, err
	}
	for _, item := range batch {
		if err = indexMessage(tx, item.owner, item.msg); err != nil {
			return false, err
		}
	}
	finished := 0
	if len(batch) < LinkBackfillBatch {
		var remaining bool
		if err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM messages WHERE rowid>?)", last).Scan(&remaining); err != nil {
			return false, err
		}
		if !remaining {
			finished = 1
		}
	}
	if _, err = tx.Exec("INSERT INTO index_jobs(name,version,last_rowid,done,updated_ns) VALUES(?,?,?,?,?) ON CONFLICT(name) DO UPDATE SET version=excluded.version,last_rowid=excluded.last_rowid,done=excluded.done,updated_ns=excluded.updated_ns", linkBackfill, version, last, finished, time.Now().UnixNano()); err != nil {
		return false, err
	}
	return finished == 1, tx.Commit()
}

// BackfillLinks runs the backfill to completion in the background, pausing
// between batches so live writes keep their turn on the database. A failed
// step is reported through failed and retried with capped backoff, so one
// error never stops indexing for the rest of the process. It returns when the
// backfill finishes or ctx ends; a later start resumes at the checkpoint.
func (s *Store) BackfillLinks(ctx context.Context, pause time.Duration, failed func(error)) {
	wait, backoff := pause, pause
	for {
		done, err := s.BackfillLinksStep()
		if done && err == nil {
			return
		}
		wait = pause
		if err != nil {
			if failed != nil {
				failed(err)
			}
			backoff = min(max(backoff*2, time.Second), 5*time.Minute)
			wait = backoff
		} else {
			backoff = pause
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}
