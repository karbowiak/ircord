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
	"maps"
	"slices"
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

// Attachment commands require upload ownership. File URLs remain ordinary
// bearer URLs and may be shared in any conversation.
func (s *Store) validateAttachmentOwnership(tx *sql.Tx, owner string, input model.SendInput) error {
	for _, id := range input.Attachments {
		var found bool
		if e := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM files WHERE id=? AND owner=? AND deleted=0)", id, owner).Scan(&found); e != nil {
			return e
		}
		if !found {
			return errors.New("upload is not available to this account")
		}
	}
	return nil
}

func (s *Store) BeginCommand(owner string, input model.SendInput) (Command, bool, error) {
	raw, _ := json.Marshal(input)
	fp := fingerprint(input)
	tx, err := s.DB.Begin()
	if err != nil {
		return Command{}, false, err
	}
	defer tx.Rollback()
	if err = s.validateAttachmentOwnership(tx, owner, input); err != nil {
		return Command{}, false, err
	}
	result, err := tx.Exec("INSERT INTO outbox(owner,client_id,conversation,fingerprint,input,state,created_ns) VALUES(?,?,?,?,?,'pending',?) ON CONFLICT(owner,client_id) DO NOTHING", owner, input.ClientID, input.ConversationID, fp, raw, time.Now().UnixNano())
	if err != nil {
		return Command{}, false, err
	}
	inserted, _ := result.RowsAffected()
	if err = tx.Commit(); err != nil {
		return Command{}, false, err
	}
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

// The database keeps no planner statistics, so the lookups a deletion runs name
// their index (see TestDeletionLookupsStayIndexedWithoutStatistics).
const (
	repliesQuery         = "SELECT payload FROM messages INDEXED BY messages_replies WHERE owner=? AND conversation=? AND json_extract(payload,'$.replyTo.id')=?"
	redactedEventsUpdate = "UPDATE events INDEXED BY events_message SET payload=json_set(payload,'$.data',json(?)) WHERE owner=? AND json_extract(payload,'$.type') IN ('message.created','message.updated') AND json_extract(payload,'$.data.id')=?"
)

// serverLineLookup finds server-window line events whose value at path is one
// of the given values, using the partial index that covers that path.
type serverLineLookup struct {
	index, path, filter string
}

func (l serverLineLookup) query(values int) string {
	return "SELECT seq,json_extract(payload,'$.data') FROM events INDEXED BY " + l.index +
		" WHERE owner=? AND json_extract(payload,'$.type')='server.line'" + l.filter +
		" AND json_extract(payload,'" + l.path + "') IN (" + strings.TrimSuffix(strings.Repeat("?,", values), ",") + ")" +
		" AND json_extract(payload,'$.data.networkId')=?"
}

var (
	// Tags that tie a line to a message by one of its msgids (its own or an edit's).
	lineByMsgid = []serverLineLookup{
		{index: "events_line_msgid", path: "$.data.tags.msgid"},
		{index: "events_line_edit", path: `$.data.tags."+ircord/edit"`},
	}
	// Tags that tie a line to a message by its IRCord client id.
	lineByClient = []serverLineLookup{
		{index: "events_line_client", path: `$.data.tags."+ircord/client-id"`},
		{index: "events_line_label", path: "$.data.tags.label"},
	}
	// Lines inside a batch, and the BATCH lines that open ("+ref") or close ("-ref") it.
	lineInBatch  = serverLineLookup{index: "events_line_batch", path: "$.data.tags.batch"}
	lineBatchRef = serverLineLookup{index: "events_line_batch_ref", path: "$.data.params[0]", filter: " AND json_extract(payload,'$.data.command')='BATCH'"}
)

// Replies returns stored messages whose reply preview quotes the given message.
func (s *Store) Replies(owner, conversation, parent string) ([]model.Message, error) {
	rows, err := s.DB.Query(repliesQuery, owner, conversation, parent)
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

// ScrubServerLines replaces text and removes attachment/GIF metadata from raw
// PRIVMSG/NOTICE lines that carried a
// deleted message: lines tagged with its msgid or IRCord client id, edits of it,
// and their multiline batches, including metadata on BATCH openings. lines must
// be in the order they arrived: servers reuse batch refs across sessions, so a
// batch is the run from its BATCH +ref line to the next BATCH -ref line, not
// every line with that ref. It edits lines in place and reports the indexes it
// changed.
func ScrubServerLines(lines []model.ServerLine, upstream, clientID, text string) []int {
	matches := func(tags map[string]string) bool {
		// An edit's lines name the message they edit, including the author's
		// outgoing copy, which has no msgid of its own.
		return upstream != "" && (tags["msgid"] == upstream || tags["+ircord/edit"] == upstream) || clientID != "" && (tags["+ircord/client-id"] == clientID || tags["label"] == clientID)
	}
	// batchOf maps each line to the index of the multiline BATCH +ref line
	// that opened its batch: members, and the opening and closing lines.
	batchOf := make([]int, len(lines))
	open := map[string]int{}
	for i, line := range lines {
		batchOf[i] = -1
		if ref, ok := open[line.Tags["batch"]]; ok && line.Tags["batch"] != "" {
			batchOf[i] = ref
		}
		if line.Command != "BATCH" || len(line.Params) == 0 || len(line.Params[0]) < 2 {
			continue
		}
		ref := line.Params[0][1:]
		switch line.Params[0][0] {
		case '+':
			// A new opening ends any earlier batch left open with this ref.
			delete(open, ref)
			if len(line.Params) >= 2 && line.Params[1] == "draft/multiline" {
				open[ref] = i
				batchOf[i] = i
			}
		case '-':
			if start, ok := open[ref]; ok {
				batchOf[i] = start
				delete(open, ref)
			}
		}
	}
	batches := map[int]bool{}
	for i, line := range lines {
		if batchOf[i] >= 0 && matches(line.Tags) {
			batches[batchOf[i]] = true
		}
	}
	changed := []int{}
	for i, line := range lines {
		if !matches(line.Tags) && (batchOf[i] < 0 || !batches[batchOf[i]]) {
			continue
		}
		scrubbed := false
		for _, key := range []string{"+ircord/attachment", "+ircord/attachments", "+ircord/gif"} {
			if _, ok := lines[i].Tags[key]; ok {
				if !scrubbed {
					lines[i].Tags = maps.Clone(line.Tags)
				}
				delete(lines[i].Tags, key)
				scrubbed = true
			}
		}
		if (line.Command == "PRIVMSG" || line.Command == "NOTICE") && len(line.Params) > 0 {
			params := append([]string{}, line.Params...)
			params[len(params)-1] = text
			lines[i].Params = params
			lines[i].Text = text
			scrubbed = true
		}
		if scrubbed {
			changed = append(changed, i)
		}
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
	if _, err = tx.Exec(redactedEventsUpdate, payload, owner, deleted.ID); err != nil {
		return err
	}
	scrub := func(keys []any, lines []model.ServerLine, update string, extra ...any) error {
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
	// The server window is capped at 500 lines per network, so it is read whole,
	// in arrival order so batches can be told apart.
	rows, err := tx.Query("SELECT id,payload FROM server_lines WHERE owner=? AND network=? ORDER BY seq", owner, network)
	if err != nil {
		return err
	}
	keys, lines, err := scanServerLines(rows)
	if err != nil {
		return err
	}
	if err = scrub(keys, lines, "UPDATE server_lines SET payload=? WHERE owner=? AND id=? AND network=?", network); err != nil {
		return err
	}
	if keys, lines, err = redactedLineEvents(tx, owner, network, deleted.UpstreamID, clientID, edits); err != nil {
		return err
	}
	if err = scrub(keys, lines, "UPDATE events SET payload=json_set(payload,'$.data',json(?)) WHERE owner=? AND seq=?"); err != nil {
		return err
	}
	return tx.Commit()
}

// scanServerLines reads (key, line) rows and closes them. Rows whose line does
// not parse are skipped.
func scanServerLines(rows *sql.Rows) ([]any, []model.ServerLine, error) {
	defer rows.Close()
	keys := []any{}
	lines := []model.ServerLine{}
	for rows.Next() {
		var key any
		var raw []byte
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, nil, err
		}
		var line model.ServerLine
		if json.Unmarshal(raw, &line) == nil {
			keys = append(keys, key)
			lines = append(lines, line)
		}
	}
	return keys, lines, rows.Err()
}

// redactedLineEvents returns the server-line events in the replay log that
// ScrubServerLines can change for a deleted message: lines tagged with one of
// its msgids or its client id, plus every line of a batch such a line opens or
// belongs to, with the BATCH lines around it. Other lines can never match, so
// the rest of the replay log is not read. Keys are event seqs, in seq order.
func redactedLineEvents(tx *sql.Tx, owner, network, upstream, clientID string, edits []string) ([]any, []model.ServerLine, error) {
	found := map[int64]model.ServerLine{}
	load := func(lookup serverLineLookup, values []string) error {
		if len(values) == 0 {
			return nil
		}
		args := []any{owner}
		for _, value := range values {
			args = append(args, value)
		}
		rows, err := tx.Query(lookup.query(len(values)), append(args, network)...)
		if err != nil {
			return err
		}
		keys, lines, err := scanServerLines(rows)
		for i, key := range keys {
			found[key.(int64)] = lines[i]
		}
		return err
	}
	nonEmpty := func(values ...string) []string {
		out := []string{}
		for _, value := range values {
			if value != "" && !slices.Contains(out, value) {
				out = append(out, value)
			}
		}
		return out
	}
	msgids := nonEmpty(append([]string{upstream}, edits...)...)
	for _, lookup := range lineByMsgid {
		if err := load(lookup, msgids); err != nil {
			return nil, nil, err
		}
	}
	for _, lookup := range lineByClient {
		if err := load(lookup, nonEmpty(clientID)); err != nil {
			return nil, nil, err
		}
	}
	refs := []string{}
	for _, line := range found {
		refs = append(refs, line.Tags["batch"])
		if line.Command == "BATCH" && len(line.Params) > 0 && len(line.Params[0]) > 1 && line.Params[0][0] == '+' {
			refs = append(refs, line.Params[0][1:])
		}
	}
	refs = nonEmpty(refs...)
	slices.Sort(refs)
	if err := load(lineInBatch, refs); err != nil {
		return nil, nil, err
	}
	boundaries := []string{}
	for _, ref := range refs {
		boundaries = append(boundaries, "+"+ref, "-"+ref)
	}
	if err := load(lineBatchRef, boundaries); err != nil {
		return nil, nil, err
	}
	seqs := slices.Sorted(maps.Keys(found))
	keys := make([]any, len(seqs))
	lines := make([]model.ServerLine, len(seqs))
	for i, seq := range seqs {
		keys[i], lines[i] = seq, found[seq]
	}
	return keys, lines, nil
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
