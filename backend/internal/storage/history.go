package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"ircord/backend/internal/model"
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
	_, err := s.DB.Exec("INSERT INTO conversation_read(owner,conversation,at_ns) VALUES(?,?,?) ON CONFLICT(owner,conversation) DO UPDATE SET at_ns=excluded.at_ns,at_position=NULL,at_id=NULL WHERE excluded.at_ns>at_ns", owner, conversation, at.UnixNano())
	return err
}

// ReadBoundary is a retained timeline position, independent of message retention.
type ReadBoundary struct {
	At       time.Time
	Position int64
	ID       string
}

// MarkReadThrough resolves and applies a local message boundary in one transaction.
// A manual unread boundary always wins over an automatic read.
func (s *Store) MarkReadThrough(owner, conversation, message string) (found, applied bool, err error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return false, false, err
	}
	defer tx.Rollback()
	var at, position int64
	err = tx.QueryRow("SELECT created_ns,position FROM messages WHERE owner=? AND conversation=? AND id=? AND kind IN ('message','action','notice')", owner, conversation, message).Scan(&at, &position)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	res, err := tx.Exec(`INSERT INTO conversation_read(owner,conversation,at_ns,at_position,at_id) VALUES(?,?,?,?,?)
		ON CONFLICT(owner,conversation) DO UPDATE SET at_ns=excluded.at_ns,at_position=excluded.at_position,at_id=excluded.at_id
		WHERE unread_from_ns IS NULL AND (excluded.at_ns>at_ns OR (excluded.at_ns=at_ns AND at_position IS NOT NULL AND (excluded.at_position,excluded.at_id)>(at_position,at_id)))`, owner, conversation, at, position, message)
	if err != nil {
		return true, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return true, false, err
	}
	return true, n > 0, tx.Commit()
}

// MarkAllRead clears the manual boundary and advances to the latest retained row atomically.
func (s *Store) MarkAllRead(owner, conversation string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var at, position int64
	var id string
	err = tx.QueryRow("SELECT created_ns,position,id FROM messages WHERE owner=? AND conversation=? AND kind IN ('message','action','notice') ORDER BY created_ns DESC,position DESC,id DESC LIMIT 1", owner, conversation).Scan(&at, &position, &id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		_, err = tx.Exec(`INSERT INTO conversation_read(owner,conversation,at_ns,at_position,at_id) VALUES(?,?,?,?,?)
			ON CONFLICT(owner,conversation) DO UPDATE SET at_ns=excluded.at_ns,at_position=excluded.at_position,at_id=excluded.at_id
			WHERE excluded.at_ns>at_ns OR (excluded.at_ns=at_ns AND at_position IS NOT NULL AND (excluded.at_position,excluded.at_id)>(at_position,at_id))`, owner, conversation, at, position, id)
		if err != nil {
			return err
		}
	}
	if _, err = tx.Exec("UPDATE conversation_read SET unread_from_ns=NULL,unread_from_id=NULL,unread_from_position=NULL WHERE owner=? AND conversation=?", owner, conversation); err != nil {
		return err
	}
	return tx.Commit()
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

// The first unread wire millisecond is independent of the marker candidate.
// Materialize it once instead of rescanning unread messages for each candidate.
const safeReadMarkerQuery = `WITH earliest AS MATERIALIZED (
		SELECT m.created_ns/1000000 AS wire_ms FROM messages m
		LEFT JOIN conversation_read r ON r.owner=m.owner AND r.conversation=m.conversation
		WHERE m.owner=? AND m.conversation=? AND m.kind IN ('message','action','notice')
		AND m.sender NOT IN (SELECT member FROM ignored_senders WHERE owner=m.owner) AND ` + unreadScope + `
		ORDER BY m.created_ns,m.position,m.id LIMIT 1
	)
	SELECT m.created_ns FROM messages m JOIN conversation_read r ON r.owner=m.owner AND r.conversation=m.conversation
		WHERE m.owner=? AND m.conversation=? AND m.kind IN ('message','action','notice') AND m.upstream!='' AND substr(m.upstream,1,1)!='_'
		AND (m.created_ns<r.at_ns OR (m.created_ns=r.at_ns AND (r.at_position IS NULL OR (m.position,m.id)<=(r.at_position,r.at_id))))
		AND (NOT EXISTS (SELECT 1 FROM earliest) OR m.created_ns/1000000 < (SELECT wire_ms FROM earliest))
		ORDER BY m.created_ns DESC,m.position DESC,m.id DESC LIMIT 1`

// SafeReadMarker returns a known upstream message no later than the local read
// boundary and strictly before every known unread wire-millisecond timestamp.
func (s *Store) SafeReadMarker(owner, conversation string) (time.Time, error) {
	var at int64
	err := s.DB.QueryRow(safeReadMarkerQuery, owner, conversation, owner, conversation).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, at).UTC(), nil
}

// unreadScope selects the unread messages of m (aliased) against the read position r.
// A manual boundary counts the chosen message (even your own) and what others sent
// after it; otherwise everything others sent after the read position counts.
const unreadScope = "CASE WHEN r.unread_from_ns IS NOT NULL THEN (CASE WHEN r.unread_from_position IS NULL THEN m.created_ns>=r.unread_from_ns ELSE (m.created_ns,m.position,m.id)>=(r.unread_from_ns,r.unread_from_position,r.unread_from_id) END) AND (m.sender!=m.owner OR m.id=r.unread_from_id) ELSE m.sender!=m.owner AND (CASE WHEN r.at_position IS NULL THEN m.created_ns>coalesce(r.at_ns,0) ELSE (m.created_ns,m.position,m.id)>(r.at_ns,r.at_position,r.at_id) END) END"

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
	var at, position int64
	err := s.DB.QueryRow("SELECT created_ns,position FROM messages WHERE owner=? AND conversation=? AND id=? AND kind IN ('message','action','notice')", owner, conversation, message).Scan(&at, &position)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = s.DB.Exec("INSERT INTO conversation_read(owner,conversation,at_ns,unread_from_ns,unread_from_id,unread_from_position) VALUES(?,?,0,?,?,?) ON CONFLICT(owner,conversation) DO UPDATE SET unread_from_ns=excluded.unread_from_ns,unread_from_id=excluded.unread_from_id,unread_from_position=excluded.unread_from_position", owner, conversation, at, message, position)
	return err == nil, err
}

// ClearUnread drops a manual unread boundary, returning to the read position.
func (s *Store) ClearUnread(owner, conversation string) error {
	_, err := s.DB.Exec("UPDATE conversation_read SET unread_from_ns=NULL,unread_from_id=NULL,unread_from_position=NULL WHERE owner=? AND conversation=?", owner, conversation)
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

func putMessage(q interface {
	Exec(string, ...any) (sql.Result, error)
	QueryRow(string, ...any) *sql.Row
}, owner string, msg model.Message) error {
	if err := positionMessage(q, owner, &msg); err != nil {
		return err
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = q.Exec("INSERT INTO messages(owner,conversation,id,upstream,created_ns,text,sender,kind,payload,position) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(owner,id) DO UPDATE SET payload=excluded.payload,text=excluded.text,upstream=excluded.upstream,created_ns=excluded.created_ns,sender=excluded.sender,kind=excluded.kind", owner, msg.ConversationID, msg.ID, msg.UpstreamID, msg.CreatedAt.UnixNano(), msg.Text, msg.SenderID, msg.Kind, raw, msg.Position)
	if err == nil && msg.Mention {
		_, err = q.Exec("INSERT INTO message_mentions(owner,conversation,message) VALUES(?,?,?) ON CONFLICT DO NOTHING", owner, msg.ConversationID, msg.ID)
	}
	if err == nil {
		err = indexMessage(q, owner, msg)
	}
	return err
}

// SaveHistory stores a message and its position in a single transaction.
func (s *Store) SaveHistory(owner string, msg model.Message) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = putMessage(tx, owner, msg); err != nil {
		return err
	}
	return tx.Commit()
}

// Every incoming message is looked up by its ID and then its upstream ID. One
// query per ID keeps each on an index; a combined OR lets SQLite walk the
// conversation's whole history. The database keeps no planner statistics
// (sqlite_stat1), so each of these must pick its index without them.
var (
	// Arguments: owner, conversation, ID.
	messageLookupQueries = []string{
		"SELECT payload FROM messages WHERE owner=? AND conversation=? AND id=?",
		"SELECT payload FROM messages WHERE owner=? AND conversation=? AND upstream=? LIMIT 1",
	}
	// Arguments: party, ID.
	partyMessageLookupQueries = []string{
		"SELECT payload FROM party_history WHERE party=? AND id=?",
		"SELECT payload FROM party_history WHERE party=? AND json_extract(payload,'$.upstreamId')=? LIMIT 1",
	}
)

const nextPositionQuery = "SELECT coalesce(max(position),0)+1 FROM messages WHERE owner=? AND conversation=?"

// lookupMessage returns the first message one of the queries finds.
func (s *Store) lookupMessage(ctx context.Context, queries []string, args ...any) (model.Message, error) {
	var msg model.Message
	var raw []byte
	err := sql.ErrNoRows
	for _, query := range queries {
		if err = s.DB.QueryRowContext(ctx, query, args...).Scan(&raw); !errors.Is(err, sql.ErrNoRows) {
			break
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return msg, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(raw, &msg)
	}
	return msg, err
}

func (s *Store) Message(owner, conversation, id string) (model.Message, error) {
	return s.MessageContext(context.Background(), owner, conversation, id)
}

func (s *Store) MessageContext(ctx context.Context, owner, conversation, id string) (model.Message, error) {
	return s.lookupMessage(ctx, messageLookupQueries, owner, conversation, id)
}

// HistoryCursor retains the old tuple fields so older servers can read normal
// sized tokens, while newer servers detect accidental scope and cursor misuse.
type HistoryCursor struct {
	Position       int64            `json:"p,omitempty"`
	At             int64            `json:"at"`
	ID             string           `json:"id"`
	Version        json.RawMessage  `json:"v,omitempty"`
	Kind           string           `json:"k,omitempty"`
	Scope          string           `json:"s,omitempty"`
	UpstreamBefore *HistoryUpstream `json:"upstreamBefore,omitempty"`
}

type HistoryUpstream struct {
	ID string
	At time.Time
}

const MaxHistoryCursorLength = 8192
const maxHistoryMessageID = 768

func historyScope(owner, conversation string) string {
	h := sha256.New()
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(owner)))
	_, _ = h.Write(size[:])
	_, _ = h.Write([]byte(owner))
	_, _ = h.Write([]byte(conversation))
	return fmt.Sprintf("%x", h.Sum(nil)[:16])
}

func Cursor(msg model.Message) string {
	raw, _ := json.Marshal(HistoryCursor{At: msg.CreatedAt.UnixNano(), ID: msg.ID, Position: msg.Position})
	return base64.RawURLEncoding.EncodeToString(raw)
}

// ParseHistoryCursor validates both the local boundary and optional upstream
// continuation before any storage or upstream history query.
func ParseHistoryCursor(raw, owner, conversation string) (*HistoryCursor, error) {
	c, err := parseHistoryCursor(raw)
	if err == nil && len(c.Version) != 0 && c.Scope != historyScope(owner, conversation) {
		err = ErrInvalidCursor
	}
	if err != nil {
		return nil, ErrInvalidCursor
	}
	return c, nil
}

func parseHistoryCursor(raw string) (*HistoryCursor, error) {
	if len(raw) == 0 || len(raw) > MaxHistoryCursorLength {
		return nil, ErrInvalidCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	var c HistoryCursor
	if err != nil || json.Unmarshal(b, &c) != nil || !validHistoryCursor(c, len(raw)) {
		return nil, ErrInvalidCursor
	}
	return &c, nil
}

func validHistoryCursor(c HistoryCursor, encodedLength int) bool {
	if encodedLength > MaxHistoryCursorLength || c.At <= 0 || len(c.ID) == 0 || len(c.ID) > maxHistoryMessageID {
		return false
	}
	if len(c.Version) == 0 {
		if c.Kind != "" || c.Scope != "" || encodedLength > 1024 {
			return false
		}
	} else if string(c.Version) != "2" || c.Kind != "history" || len(c.Scope) != 32 {
		return false
	}
	if c.UpstreamBefore != nil && (c.UpstreamBefore.At.IsZero() || len(c.UpstreamBefore.ID) > 100 || strings.ContainsAny(c.UpstreamBefore.ID, "\r\n\x00")) {
		return false
	}
	return true
}

// ParseCursor reads the legacy tuple for existing internal callers; it is not an authorization check.
func ParseCursor(raw string) (int64, string, error) {
	c, err := parseHistoryCursor(raw)
	if err != nil {
		return 0, "", err
	}
	return c.At, c.ID, nil
}

// ScopedHistoryCursor issues a bounded cursor bound to the account and conversation.
func ScopedHistoryCursor(owner, conversation string, msg model.Message, upstream *HistoryUpstream) (string, error) {
	c := HistoryCursor{At: msg.CreatedAt.UnixNano(), Position: msg.Position, ID: msg.ID, UpstreamBefore: upstream}
	return EncodeHistoryCursor(owner, conversation, c)
}

// EncodeHistoryCursor also upgrades legacy local boundaries when preserving an upstream point.
func EncodeHistoryCursor(owner, conversation string, c HistoryCursor) (string, error) {
	c.Version, c.Kind, c.Scope = json.RawMessage("2"), "history", historyScope(owner, conversation)
	if !validHistoryCursor(c, 0) {
		return "", ErrInvalidCursor
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return "", ErrInvalidCursor
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if len(token) > MaxHistoryCursorLength {
		return "", ErrInvalidCursor
	}
	return token, nil
}

func (s *Store) History(owner, conversation, before string, limit int) (model.MessagePage, error) {
	return s.HistoryContext(context.Background(), owner, conversation, before, limit)
}

func (s *Store) HistoryContext(ctx context.Context, owner, conversation, before string, limit int) (model.MessagePage, error) {
	var boundary *HistoryCursor
	var err error
	if before != "" {
		boundary, err = ParseHistoryCursor(before, owner, conversation)
		if err != nil {
			return model.MessagePage{Items: []model.Message{}}, err
		}
	}
	return s.HistoryCursorContext(ctx, owner, conversation, boundary, limit)
}

// HistoryCursorContext consumes a previously validated boundary.
func (s *Store) HistoryCursorContext(ctx context.Context, owner, conversation string, boundary *HistoryCursor, limit int) (model.MessagePage, error) {
	page := model.MessagePage{Items: []model.Message{}}
	query := "SELECT payload FROM messages WHERE owner=? AND conversation=?"
	args := []any{owner, conversation}
	if boundary != nil {
		query += " AND (created_ns<? OR (created_ns=? AND (position<? OR (position=? AND id<?))))"
		args = append(args, boundary.At, boundary.At, boundary.Position, boundary.Position, boundary.ID)
	}
	query += " ORDER BY created_ns DESC,position DESC,id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var msg model.Message
		if err = rows.Scan(&raw); err != nil {
			return page, err
		}
		if err = json.Unmarshal(raw, &msg); err != nil {
			return page, err
		}
		page.Items = append(page.Items, msg)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		next, err := ScopedHistoryCursor(owner, conversation, page.Items[len(page.Items)-1], nil)
		if err != nil {
			return page, err
		}
		page.NextCursor = &next
	}
	slices.Reverse(page.Items)
	return page, nil
}

// HistoryAround reads a window of at most limit messages centred on message:
// up to half of it older, then the message, then newer ones. It reports
// whether older and newer messages exist beyond the window, and ErrNotFound
// when message is not a stored message of the conversation.
func (s *Store) HistoryAround(owner, conversation, message string, limit int) (page model.MessagePage, older bool, err error) {
	return s.HistoryAroundContext(context.Background(), owner, conversation, message, limit)
}

func (s *Store) HistoryAroundContext(ctx context.Context, owner, conversation, message string, limit int) (page model.MessagePage, older bool, err error) {
	page = model.MessagePage{Items: []model.Message{}}
	var at, position int64
	var id string
	err = s.DB.QueryRowContext(ctx, "SELECT created_ns,position,id FROM messages WHERE owner=? AND conversation=? AND id=?", owner, conversation, message).Scan(&at, &position, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return page, false, ErrNotFound
	}
	if err != nil {
		return page, false, err
	}
	read := func(query string, n int) ([]model.Message, error) {
		rows, err := s.DB.QueryContext(ctx, query, owner, conversation, at, at, position, position, id, n)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []model.Message{}
		for rows.Next() {
			var raw []byte
			var msg model.Message
			if err = rows.Scan(&raw); err != nil {
				return nil, err
			}
			if err = json.Unmarshal(raw, &msg); err != nil {
				return nil, err
			}
			out = append(out, msg)
		}
		return out, rows.Err()
	}
	before := (limit - 1) / 2
	after := limit - 1 - before
	olderItems, err := read("SELECT payload FROM messages WHERE owner=? AND conversation=? AND (created_ns<? OR (created_ns=? AND (position<? OR (position=? AND id<?)))) ORDER BY created_ns DESC,position DESC,id DESC LIMIT ?", before+1)
	if err != nil {
		return page, false, err
	}
	newerItems, err := read("SELECT payload FROM messages WHERE owner=? AND conversation=? AND (created_ns>? OR (created_ns=? AND (position>? OR (position=? AND id>?)))) ORDER BY created_ns,position,id LIMIT ?", after+1)
	if err != nil {
		return page, false, err
	}
	older = len(olderItems) > before
	if older {
		olderItems = olderItems[:before]
	}
	page.HasNewer = len(newerItems) > after
	if page.HasNewer {
		newerItems = newerItems[:after]
	}
	target, err := s.MessageContext(ctx, owner, conversation, id)
	if err != nil {
		return page, false, err
	}
	slices.Reverse(olderItems)
	page.Items = append(append(olderItems, target), newerItems...)
	if older {
		next, err := ScopedHistoryCursor(owner, conversation, page.Items[0], nil)
		if err != nil {
			return page, false, err
		}
		page.NextCursor = &next
	}
	return page, older, nil
}

// Position orders messages which share IRC's millisecond server-time precision.
// Older batches reserve a range before the current window, in server batch order.
func positionMessage(q interface{ QueryRow(string, ...any) *sql.Row }, owner string, msg *model.Message) error {
	var existing int64
	err := q.QueryRow("SELECT position FROM messages WHERE owner=? AND id=?", owner, msg.ID).Scan(&existing)
	if err == nil {
		msg.Position = existing
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if msg.Position != 0 {
		return nil
	}
	return q.QueryRow(nextPositionQuery, owner, msg.ConversationID).Scan(&msg.Position)
}

func (s *Store) HistoryBase(owner, conversation string) (int64, error) {
	var base int64
	err := s.DB.QueryRow("SELECT min(coalesce((SELECT min(position) FROM messages WHERE owner=? AND conversation=?),0),0)-1000000", owner, conversation).Scan(&base)
	return base, err
}
