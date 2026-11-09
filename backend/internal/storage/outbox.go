package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"ircord/backend/internal/model"
	"strings"
	"time"
)

type Command struct {
	ClientID       string          `json:"clientId"`
	ConversationID string          `json:"conversationId"`
	State          string          `json:"state" enum:"pending,sent,unconfirmed,uncertain,failed"`
	MessageID      string          `json:"messageId,omitempty"`
	Error          string          `json:"error,omitempty"`
	Input          model.SendInput `json:"-"`
}

func fingerprint(input model.SendInput) string {
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func (s *Store) BeginCommand(owner string, input model.SendInput) (Command, bool, error) {
	raw, _ := json.Marshal(input)
	fp := fingerprint(input)
	result, err := s.DB.Exec("INSERT INTO outbox(owner,client_id,conversation,fingerprint,input,state,created_ns) VALUES(?,?,?,?,?,'pending',?) ON CONFLICT(owner,client_id) DO NOTHING", owner, input.ClientID, input.ConversationID, fp, raw, time.Now().UnixNano())
	if err != nil {
		return Command{}, false, err
	}
	inserted, _ := result.RowsAffected()
	command, existing, err := s.command(owner, input.ClientID)
	if err != nil {
		return command, false, err
	}
	if existing != fp {
		return command, false, fmt.Errorf("clientId already belongs to a different command")
	}
	return command, inserted == 1, nil
}
func (s *Store) command(owner, clientID string) (Command, string, error) {
	var c Command
	var fp string
	var input []byte
	var message sql.NullString
	err := s.DB.QueryRow("SELECT client_id,conversation,state,message_id,error,input,fingerprint FROM outbox WHERE owner=? AND client_id=?", owner, clientID).Scan(&c.ClientID, &c.ConversationID, &c.State, &message, &c.Error, &input, &fp)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	c.MessageID = message.String
	if err == nil {
		err = json.Unmarshal(input, &c.Input)
	}
	return c, fp, err
}
func (s *Store) Command(owner, clientID string) (Command, error) {
	c, _, err := s.command(owner, clientID)
	return c, err
}
func (s *Store) FailCommand(owner, clientID, message string) error {
	_, err := s.DB.Exec("UPDATE outbox SET state='failed',error=? WHERE owner=? AND client_id=? AND state NOT IN ('sent','uncertain','unconfirmed')", message, owner, clientID)
	return err
}
func (s *Store) UncertainCommand(owner, clientID, message string) error {
	_, err := s.DB.Exec("UPDATE outbox SET state='uncertain',error=? WHERE owner=? AND client_id=? AND state='pending'", message, owner, clientID)
	return err
}
func (s *Store) RetryFailed(owner, clientID string) error {
	_, err := s.DB.Exec("UPDATE outbox SET state='pending',error='' WHERE owner=? AND client_id=? AND state='failed'", owner, clientID)
	return err
}
func (s *Store) UncertainCommands(owner string) error {
	_, err := s.DB.Exec("UPDATE outbox SET state='uncertain',error='Backend restarted before IRC confirmation; reconcile history before retrying.' WHERE owner=? AND state='pending'", owner)
	return err
}
func (s *Store) SetHistoryEnd(owner, conversation string, end bool) error {
	_, err := s.DB.Exec("INSERT INTO history_state(owner,conversation,exhausted) VALUES(?,?,?) ON CONFLICT(owner,conversation) DO UPDATE SET exhausted=excluded.exhausted", owner, conversation, end)
	return err
}
func (s *Store) HistoryEnded(owner, conversation string) bool {
	return s.HistoryEndedContext(context.Background(), owner, conversation)
}

func (s *Store) HistoryEndedContext(ctx context.Context, owner, conversation string) bool {
	var ended bool
	_ = s.DB.QueryRowContext(ctx, "SELECT exhausted FROM history_state WHERE owner=? AND conversation=?", owner, conversation).Scan(&ended)
	return ended
}
func (s *Store) Oldest(owner, conversation string) (model.Message, error) {
	return s.OldestContext(context.Background(), owner, conversation)
}

func (s *Store) OldestContext(ctx context.Context, owner, conversation string) (model.Message, error) {
	var raw []byte
	err := s.DB.QueryRowContext(ctx, "SELECT payload FROM messages WHERE owner=? AND conversation=? ORDER BY created_ns,position,id LIMIT 1", owner, conversation).Scan(&raw)
	var msg model.Message
	if errors.Is(err, sql.ErrNoRows) {
		return msg, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(raw, &msg)
	}
	return msg, err
}
func (s *Store) Reaction(owner, conversation, parent, sender, emoji string, active bool, at time.Time) error {
	return s.SetReaction(owner, conversation, parent, sender, emoji, active, at, false)
}
func (s *Store) SetReaction(owner, conversation, parent, sender, emoji string, active bool, at time.Time, force bool) error {

	_, err := s.DB.Exec("INSERT INTO reactions(owner,conversation,parent,sender,emoji,active,at_ns) VALUES(?,?,?,?,?,?,?) ON CONFLICT(owner,conversation,parent,sender,emoji) DO UPDATE SET active=excluded.active,at_ns=excluded.at_ns WHERE excluded.at_ns>=reactions.at_ns OR ?", owner, conversation, parent, sender, emoji, active, at.UnixNano(), force)
	return err
}
func (s *Store) Reactions(owner, conversation, parent string) ([]model.Reaction, error) {
	rows, err := s.DB.Query("SELECT emoji,sender FROM reactions WHERE owner=? AND conversation=? AND parent=? AND active=1 ORDER BY emoji,sender", owner, conversation, parent)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Reaction{}
	for rows.Next() {
		var emoji, sender string
		if err = rows.Scan(&emoji, &sender); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].Emoji != emoji {
			out = append(out, model.Reaction{Emoji: emoji, MemberIDs: []string{}})
		}
		out[len(out)-1].MemberIDs = append(out[len(out)-1].MemberIDs, sender)
	}
	return out, rows.Err()
}

// ClearRedacted drops rows derived from a deleted message: reactions to it, its
// mention marker and its catalogued images, so counters, galleries and reloads
// no longer surface the removed content.
func (s *Store) ClearRedacted(owner, conversation, id, upstream string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if upstream != "" {
		if _, err = tx.Exec("DELETE FROM reactions WHERE owner=? AND conversation=? AND parent=?", owner, conversation, upstream); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("DELETE FROM message_mentions WHERE owner=? AND conversation=? AND message=?", owner, conversation, id); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM message_links WHERE owner=? AND conversation=? AND message=?", owner, conversation, id); err != nil {
		return err
	}
	return tx.Commit()
}

// EditTarget returns the message that one of its edits, named by the edit's
// own msgid, was applied to.
func (s *Store) EditTarget(owner, conversation, upstream string) (model.Message, error) {
	var raw []byte
	err := s.DB.QueryRow("SELECT m.payload FROM messages m INDEXED BY messages_edited, json_each(m.payload,'$.edits') e WHERE m.owner=? AND m.conversation=? AND json_extract(m.payload,'$.editedAt') IS NOT NULL AND json_extract(e.value,'$.upstreamId')=? LIMIT 1", owner, conversation, upstream).Scan(&raw)
	var msg model.Message
	if err == nil {
		err = json.Unmarshal(raw, &msg)
	}
	return msg, err
}

// PendingEdits returns edits stored as ordinary lines because they arrived
// before the original message they name.
func (s *Store) PendingEdits(owner, conversation, upstream string) ([]model.Message, error) {
	rows, err := s.DB.Query("SELECT payload FROM messages INDEXED BY messages_edit_of WHERE owner=? AND conversation=? AND json_extract(payload,'$.editOf') IS NOT NULL AND json_extract(payload,'$.editOf')=? ORDER BY created_ns", owner, conversation, upstream)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Message{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var msg model.Message
		if json.Unmarshal(raw, &msg) == nil {
			out = append(out, msg)
		}
	}
	return out, rows.Err()
}

// RemoveMessage deletes one stored message and the rows derived from it. It is
// used for an edit that was stored as a line before its original arrived.
func (s *Store) RemoveMessage(owner, conversation string, msg model.Message) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		"DELETE FROM messages WHERE owner=? AND conversation=? AND id=?",
		"DELETE FROM message_mentions WHERE owner=? AND conversation=? AND message=?",
		"DELETE FROM message_links WHERE owner=? AND conversation=? AND message=?",
		"DELETE FROM saved_messages WHERE owner=? AND conversation=? AND message=?",
		"DELETE FROM inbox_items WHERE owner=? AND conversation=? AND message=?",
	} {
		if _, err = tx.Exec(q, owner, conversation, msg.ID); err != nil {
			return err
		}
	}
	if msg.UpstreamID != "" {
		if _, err = tx.Exec("DELETE FROM reactions WHERE owner=? AND conversation=? AND parent=?", owner, conversation, msg.UpstreamID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ClearMention drops the mention marker of a message that no longer mentions you.
func (s *Store) ClearMention(owner, conversation, id string) error {
	_, err := s.DB.Exec("DELETE FROM message_mentions WHERE owner=? AND conversation=? AND message=?", owner, conversation, id)
	return err
}

// Replies returns stored messages whose reply preview quotes the given message.
func (s *Store) Replies(owner, conversation, parent string) ([]model.Message, error) {
	rows, err := s.DB.Query("SELECT payload FROM messages WHERE owner=? AND conversation=? AND json_extract(payload,'$.replyTo.id')=?", owner, conversation, parent)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Message{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var msg model.Message
		if err = json.Unmarshal(raw, &msg); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, rows.Err()
}

// ScrubServerLines replaces the text of raw PRIVMSG/NOTICE lines that carried a
// deleted message: lines tagged with its msgid or IRCord client id, edits of it,
// and the lines of a multiline batch opened by such a line. It edits lines in place and
// reports the indexes it changed.
func ScrubServerLines(lines []model.ServerLine, upstream, clientID, text string) []int {
	matches := func(tags map[string]string) bool {
		// An edit's lines name the message they edit, including the author's
		// outgoing copy, which has no msgid of its own.
		return upstream != "" && (tags["msgid"] == upstream || tags["+ircord/edit"] == upstream) || clientID != "" && (tags["+ircord/client-id"] == clientID || tags["label"] == clientID)
	}
	batches := map[string]bool{}
	for _, line := range lines {
		if line.Command == "BATCH" && len(line.Params) > 0 && strings.HasPrefix(line.Params[0], "+") && matches(line.Tags) {
			batches[line.Params[0][1:]] = true
		}
	}
	changed := []int{}
	for i, line := range lines {
		if line.Command != "PRIVMSG" && line.Command != "NOTICE" || len(line.Params) == 0 {
			continue
		}
		if !matches(line.Tags) && !batches[line.Tags["batch"]] {
			continue
		}
		params := append([]string{}, line.Params...)
		params[len(params)-1] = text
		lines[i].Params = params
		lines[i].Text = text
		changed = append(changed, i)
	}
	return changed
}

// ScrubRedacted rewrites stored copies of a deleted message's text outside the
// messages table: earlier message events in the replay log now carry the
// deleted payload, and server-window lines and their replay events lose the text.
// edits are the msgids of the message's edits, whose lines are scrubbed too.
func (s *Store) ScrubRedacted(owner, network string, deleted model.Message, clientID string, edits ...string) error {
	payload, err := json.Marshal(deleted)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE events SET payload=json_set(payload,'$.data',json(?)) WHERE owner=? AND json_extract(payload,'$.type') IN ('message.created','message.updated') AND json_extract(payload,'$.data.id')=?", payload, owner, deleted.ID); err != nil {
		return err
	}
	scrub := func(query string, args []any, update string, extra ...any) error {
		rows, err := tx.Query(query, args...)
		if err != nil {
			return err
		}
		keys := []any{}
		lines := []model.ServerLine{}
		for rows.Next() {
			var key any
			var raw []byte
			if err = rows.Scan(&key, &raw); err != nil {
				rows.Close()
				return err
			}
			var line model.ServerLine
			if json.Unmarshal(raw, &line) == nil {
				keys = append(keys, key)
				lines = append(lines, line)
			}
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return err
		}
		changed := map[int]bool{}
		for _, i := range ScrubServerLines(lines, deleted.UpstreamID, clientID, deleted.Text) {
			changed[i] = true
		}
		for _, edit := range edits {
			for _, i := range ScrubServerLines(lines, edit, "", deleted.Text) {
				changed[i] = true
			}
		}
		for i := range changed {
			raw, _ := json.Marshal(lines[i])
			if _, err = tx.Exec(update, append([]any{raw, owner, keys[i]}, extra...)...); err != nil {
				return err
			}
		}
		return nil
	}
	if err = scrub("SELECT id,payload FROM server_lines WHERE owner=? AND network=?", []any{owner, network}, "UPDATE server_lines SET payload=? WHERE owner=? AND id=? AND network=?", network); err != nil {
		return err
	}
	if err = scrub("SELECT seq,json_extract(payload,'$.data') FROM events WHERE owner=? AND json_extract(payload,'$.type')='server.line' AND json_extract(payload,'$.data.networkId')=?", []any{owner, network}, "UPDATE events SET payload=json_set(payload,'$.data',json(?)) WHERE owner=? AND seq=?"); err != nil {
		return err
	}
	return tx.Commit()
}

// MessagesBetween returns stored chat messages with an upstream msgid created
// strictly between after and before.
func (s *Store) MessagesBetween(owner, conversation string, after, before time.Time) ([]model.Message, error) {
	rows, err := s.DB.Query("SELECT payload FROM messages WHERE owner=? AND conversation=? AND upstream!='' AND kind IN ('message','action','notice') AND created_ns>? AND created_ns<? ORDER BY created_ns LIMIT 1000", owner, conversation, after.UnixNano(), before.UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Message{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var msg model.Message
		if err = json.Unmarshal(raw, &msg); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, rows.Err()
}
