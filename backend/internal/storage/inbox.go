package storage

import (
	"database/sql"
	"errors"
	"encoding/base64"
	"encoding/json"
	"time"

	"ircord/backend/internal/model"
)

// Inbox retention: an account keeps at most InboxLimit items, none older than
// InboxRetention. Older items are hidden at once and deleted by the next write
// or PruneInbox, so the inbox never grows without bound.
var (
	InboxLimit     = 500
	InboxRetention = 30 * 24 * time.Hour
)

// Inbox reasons, stored as a bit set so one message is one row.
var inboxReasons = []string{"mention", "reply", "direct"}

func inboxBits(reasons []string) int {
	bits := 0
	for _, r := range reasons {
		for i, known := range inboxReasons {
			if r == known {
				bits |= 1 << i
			}
		}
	}
	return bits
}

func inboxReasonList(bits int) []string {
	out := []string{}
	for i, r := range inboxReasons {
		if bits&(1<<i) != 0 {
			out = append(out, r)
		}
	}
	return out
}

// InboxRow is a stored inbox item with its current state and the stored
// message it names, when that message is still in the account's history.
type InboxRow struct {
	Entry     model.InboxEntry
	Dismissed bool
	// Unread follows the conversation's read position (#7): reading the
	// conversation reads the item; dismissing it does not.
	Unread bool
	// Open is whether the conversation is still one of the account's.
	Open    bool
	Message *model.Message
}

// InboxFilters are the focused views ListInbox accepts.
var InboxFilters = []string{"all", "unread", "mentions", "replies", "direct", "dismissed"}

type inboxCursor struct {
	At int64  `json:"at"`
	ID string `json:"id"`
}

// InboxCursor continues an inbox listing after row.
func InboxCursor(row InboxRow) string {
	raw, _ := json.Marshal(inboxCursor{At: row.Entry.CreatedAt.UnixNano(), ID: row.Entry.MessageID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func parseInboxCursor(raw string) (inboxCursor, error) {
	var c inboxCursor
	if len(raw) > 512 {
		return c, ErrInvalidCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(b, &c) != nil || c.At <= 0 || c.ID == "" {
		return c, ErrInvalidCursor
	}
	return c, nil
}

// inboxFrom joins an item i to its stored message m, the conversation's read
// position r and whether the conversation is still open. Items from ignored
// senders and items past retention are never visible.
const inboxFrom = " FROM inbox_items i LEFT JOIN messages m ON m.owner=i.owner AND m.id=i.message AND m.conversation=i.conversation LEFT JOIN conversation_read r ON r.owner=i.owner AND r.conversation=i.conversation WHERE i.owner=? AND i.created_ns>=? AND i.sender NOT IN (SELECT member FROM ignored_senders WHERE owner=i.owner)"

// inboxOpen and inboxUnread are SQL expressions over inboxFrom's aliases. An
// unread item needs its message still present, undeleted and in an open
// conversation, and unread by the same rule as conversation unread counts.
const inboxOpen = "EXISTS(SELECT 1 FROM entities e WHERE e.owner=i.owner AND e.kind='conversation' AND e.id=i.conversation)"
const inboxUnread = "(m.payload IS NOT NULL AND coalesce(json_extract(m.payload,'$.deleted'),0)=0 AND " + inboxOpen + " AND " + unreadScope + ")"

func inboxFilter(filter string) string {
	switch filter {
	case "unread":
		return " AND i.dismissed_ns IS NULL AND " + inboxUnread
	case "mentions":
		return " AND i.dismissed_ns IS NULL AND i.reasons&1!=0"
	case "replies":
		return " AND i.dismissed_ns IS NULL AND i.reasons&2!=0"
	case "direct":
		return " AND i.dismissed_ns IS NULL AND i.reasons&4!=0"
	case "dismissed":
		return " AND i.dismissed_ns IS NOT NULL"
	}
	return " AND i.dismissed_ns IS NULL"
}

func inboxCutoff(now time.Time) int64 { return now.Add(-InboxRetention).UnixNano() }

// InboxList pages owner's inbox newest first under one of InboxFilters. It
// reads one row past limit to tell whether another page exists.
func (s *Store) InboxList(owner, filter, after string, limit int) ([]InboxRow, bool, error) {
	q := "SELECT i.message,i.conversation,i.network,i.sender,i.reasons,i.created_ns,i.dismissed_ns IS NOT NULL," + inboxUnread + "," + inboxOpen + ",m.payload" + inboxFrom + inboxFilter(filter)
	args := []any{owner, inboxCutoff(time.Now())}
	if after != "" {
		c, err := parseInboxCursor(after)
		if err != nil {
			return nil, false, err
		}
		q += " AND (i.created_ns<? OR (i.created_ns=? AND i.message<?))"
		args = append(args, c.At, c.At, c.ID)
	}
	q += " ORDER BY i.created_ns DESC,i.message DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []InboxRow{}
	for rows.Next() {
		var row InboxRow
		var bits int
		var at int64
		var raw []byte
		if err = rows.Scan(&row.Entry.MessageID, &row.Entry.ConversationID, &row.Entry.NetworkID, &row.Entry.SenderID, &bits, &at, &row.Dismissed, &row.Unread, &row.Open, &raw); err != nil {
			return nil, false, err
		}
		row.Entry.Reasons = inboxReasonList(bits)
		row.Entry.CreatedAt = time.Unix(0, at).UTC()
		if raw != nil {
			var msg model.Message
			if err = json.Unmarshal(raw, &msg); err != nil {
				return nil, false, err
			}
			row.Message = &msg
		}
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

// InboxItem reads one item of owner, whatever its state, or ErrNotFound.
func (s *Store) InboxItem(owner, message string) (InboxRow, error) {
	var row InboxRow
	var bits int
	var at int64
	err := s.DB.QueryRow("SELECT i.message,i.conversation,i.network,i.sender,i.reasons,i.created_ns,i.dismissed_ns IS NOT NULL"+inboxFrom+" AND i.message=?", owner, inboxCutoff(time.Now()), message).Scan(&row.Entry.MessageID, &row.Entry.ConversationID, &row.Entry.NetworkID, &row.Entry.SenderID, &bits, &at, &row.Dismissed)
	if errors.Is(err, sql.ErrNoRows) {
		return row, ErrNotFound
	}
	row.Entry.Reasons = inboxReasonList(bits)
	row.Entry.CreatedAt = time.Unix(0, at).UTC()
	return row, err
}

// InboxUnread counts owner's undismissed, unread inbox items: the inbox's
// unread indicator.
func (s *Store) InboxUnread(owner string) (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT count(*)"+inboxFrom+inboxFilter("unread"), owner, inboxCutoff(time.Now())).Scan(&n)
	return n, err
}

// pruneInbox deletes owner's items past retention or beyond the item limit.
func pruneInbox(q interface {
	Exec(string, ...any) (sql.Result, error)
}, owner string, now time.Time) error {
	_, err := q.Exec("DELETE FROM inbox_items WHERE owner=? AND (created_ns<? OR message IN (SELECT message FROM inbox_items WHERE owner=? ORDER BY created_ns DESC,message DESC LIMIT -1 OFFSET ?))", owner, inboxCutoff(now), owner, InboxLimit)
	return err
}

// PruneInbox deletes every account's inbox items past retention.
func (s *Store) PruneInbox(now time.Time) error {
	_, err := s.DB.Exec("DELETE FROM inbox_items WHERE created_ns<?", inboxCutoff(now))
	return err
}
