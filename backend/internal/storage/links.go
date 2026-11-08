package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/msgindex"
)

// indexMessage rewrites the message link index rows of one stored message.
// It runs with every write of the message, live or from history, so an edit,
// a deletion or a late link preview replaces the message's rows, and storing
// the same message again (replay, catch-up, retries) never adds a second set.
//
// Gallery entries come first. Every link of the text then joins the link
// catalog: a link that already has a gallery row gains its catalog identity
// there, so one row serves both; any other link gets its own row. A text link
// the gallery does not show (an http:// image, say) is catalogued as a page,
// never as an image the gallery would then pick up.
func indexMessage(q interface {
	Exec(string, ...any) (sql.Result, error)
}, owner, uploadOrigin string, msg model.Message) error {
	if _, err := q.Exec("DELETE FROM message_links WHERE owner=? AND message=?", owner, msg.ID); err != nil {
		return err
	}
	type row struct {
		msgindex.Entry
		identity, host string
		previewed      bool
	}
	rows := []row{}
	byURL := map[string]int{}
	for _, e := range msgindex.Images(msg, uploadOrigin) {
		byURL[e.URL] = len(rows)
		rows = append(rows, row{Entry: e})
	}
	if msgindex.Indexed(msg) {
		for _, l := range msgindex.Links(msg.Text) {
			if i, ok := byURL[l.URL]; ok {
				rows[i].identity, rows[i].host, rows[i].previewed = l.Identity, l.Host, l.Previewed
				continue
			}
			kind := l.Kind
			if kind == msgindex.KindImage || kind == msgindex.KindGIF {
				kind = msgindex.KindLink
			}
			rows = append(rows, row{Entry: msgindex.Entry{URL: l.URL, Kind: kind, Source: msgindex.SourceLink}, identity: l.Identity, host: l.Host, previewed: l.Previewed})
		}
	}
	for i, e := range rows {
		if _, err := q.Exec("INSERT INTO message_links(owner,message,idx,conversation,sender,created_ns,kind,source,url,thumb_url,file_id,width,height,identity,host,previewed) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", owner, msg.ID, i, msg.ConversationID, msg.SenderID, msg.CreatedAt.UnixNano(), e.Kind, e.Source, e.URL, e.ThumbURL, e.FileID, e.Width, e.Height, e.identity, e.host, e.previewed); err != nil {
			return err
		}
	}
	return nil
}

// galleryKinds are the index kinds a gallery shows. The predicate matches the
// partial index message_links_gallery, which serves gallery pages in order.
const galleryKinds = "('" + msgindex.KindImage + "','" + msgindex.KindGIF + "')"

// GalleryRow is one image of a conversation's gallery with its source message.
type GalleryRow struct {
	MessageID      string
	Index          int
	ConversationID string
	SenderID       string
	CreatedAt      time.Time
	Kind           string
	Source         string
	URL            string
	ThumbURL       string
	FileID         string
	Width          int
	Height         int
	// Text is the source message's IRC text, for its excerpt.
	Text string
}

type galleryCursor struct {
	At    int64  `json:"at"`
	ID    string `json:"id"`
	Index int    `json:"i"`
}

// GalleryCursor continues a gallery listing after row.
func GalleryCursor(row GalleryRow) string {
	raw, _ := json.Marshal(galleryCursor{At: row.CreatedAt.UnixNano(), ID: row.MessageID, Index: row.Index})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func parseGalleryCursor(raw string) (galleryCursor, error) {
	var c galleryCursor
	if len(raw) > 512 {
		return c, ErrInvalidCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(b, &c) != nil || c.At <= 0 || c.ID == "" || c.Index < 0 {
		return c, ErrInvalidCursor
	}
	return c, nil
}

// galleryFrom joins index rows l to their stored message m. Only rows of
// owner's own stored history are ever read, of a conversation that is still
// open, whose message is still stored and from a sender owner does not ignore.
const galleryWhere = " WHERE l.owner=? AND l.conversation=? AND l.kind IN " + galleryKinds + " AND EXISTS(SELECT 1 FROM entities e WHERE e.owner=l.owner AND e.kind='conversation' AND e.id=l.conversation) AND l.sender NOT IN (SELECT member FROM ignored_senders WHERE owner=l.owner)"
const galleryFrom = " FROM message_links l JOIN messages m ON m.owner=l.owner AND m.id=l.message AND m.conversation=l.conversation" + galleryWhere

// Gallery pages the images of one of owner's conversations, newest post first
// and in message order within a post. It reads one row past limit to tell
// whether another page exists.
func (s *Store) Gallery(owner, conversation, after string, limit int) ([]GalleryRow, bool, error) {
	q := "SELECT l.message,l.idx,l.conversation,l.sender,l.created_ns,l.kind,l.source,l.url,l.thumb_url,l.file_id,l.width,l.height,m.text" + galleryFrom
	args := []any{owner, conversation}
	if after != "" {
		c, err := parseGalleryCursor(after)
		if err != nil {
			return nil, false, err
		}
		q += " AND (l.created_ns<? OR (l.created_ns=? AND (l.message<? OR (l.message=? AND l.idx>?))))"
		args = append(args, c.At, c.At, c.ID, c.ID, c.Index)
	}
	q += " ORDER BY l.created_ns DESC,l.message DESC,l.idx LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []GalleryRow{}
	for rows.Next() {
		var row GalleryRow
		var at int64
		if err = rows.Scan(&row.MessageID, &row.Index, &row.ConversationID, &row.SenderID, &at, &row.Kind, &row.Source, &row.URL, &row.ThumbURL, &row.FileID, &row.Width, &row.Height, &row.Text); err != nil {
			return nil, false, err
		}
		row.CreatedAt = time.Unix(0, at).UTC()
		out = append(out, row)
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

// GalleryCount counts the images Gallery would list for a conversation. It
// needs no join with messages: every path that deletes a message deletes its
// rows too.
func (s *Store) GalleryCount(owner, conversation string) (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT count(*) FROM message_links l"+galleryWhere, owner, conversation).Scan(&n)
	return n, err
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
		if err = indexMessage(tx, item.owner, s.uploadOrigin, item.msg); err != nil {
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
