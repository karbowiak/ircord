package storage

import (
	"context"
	"database/sql"
	"strings"

	"ircord/backend/internal/notify"
)

// dropBareNameMentions clears mention markers that the old rule gave to unread
// channel messages naming the account without an "@" (#209). It only touches
// unread messages, so read history and its notifications stay as they were,
// and it sends nothing: workspaces recount unread mentions when they load. A
// marker stays when the text holds any "@" (the account's nick at the time is
// not known here) or one of the account's highlight words. Direct messages
// always count as mentions and are left alone.
func dropBareNameMentions(ctx context.Context, tx *sql.Tx) error {
	// Migration 25 runs before tuple columns are introduced; retain its original predicate.
	const legacyUnreadScope = "CASE WHEN r.unread_from_ns IS NOT NULL THEN m.created_ns>=r.unread_from_ns AND (m.sender!=m.owner OR m.id=r.unread_from_id) ELSE m.sender!=m.owner AND m.created_ns>coalesce(r.at_ns,0) END"
	rows, err := tx.QueryContext(ctx, `SELECT t.owner,t.conversation,t.message,m.text,coalesce((SELECT a.payload FROM app_data a WHERE a.owner=t.owner AND a.kind='notification-settings.updated'),'') FROM message_mentions t JOIN messages m ON m.owner=t.owner AND m.id=t.message AND m.conversation=t.conversation JOIN entities e ON e.owner=t.owner AND e.kind='conversation' AND e.id=t.conversation LEFT JOIN conversation_read r ON r.owner=m.owner AND r.conversation=m.conversation WHERE json_extract(e.payload,'$.kind')='channel' AND m.kind IN ('message','action','notice') AND `+legacyUnreadScope)
	if err != nil {
		return err
	}
	type marker struct{ owner, conversation, message string }
	stale := []marker{}
	policies := map[string]*notify.Policy{}
	for rows.Next() {
		var v marker
		var text string
		var rules []byte
		if err = rows.Scan(&v.owner, &v.conversation, &v.message, &text, &rules); err != nil {
			rows.Close()
			return err
		}
		policy, ok := policies[v.owner]
		if !ok {
			policy = notify.Compile(rules)
			policies[v.owner] = policy
		}
		if !strings.Contains(notify.Plain(text), "@") && !policy.Highlighted(text) {
			stale = append(stale, v)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range stale {
		if _, err = tx.ExecContext(ctx, "DELETE FROM message_mentions WHERE owner=? AND conversation=? AND message=?", v.owner, v.conversation, v.message); err != nil {
			return err
		}
		// The inbox keeps the item for its other reasons, such as a reply.
		if _, err = tx.ExecContext(ctx, "UPDATE inbox_items SET reasons=reasons&~1 WHERE owner=? AND message=? AND conversation=?", v.owner, v.message, v.conversation); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM inbox_items WHERE owner=? AND message=? AND conversation=? AND reasons=0", v.owner, v.message, v.conversation); err != nil {
			return err
		}
	}
	return nil
}
