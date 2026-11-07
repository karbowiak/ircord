package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"ircord/backend/internal/model"
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
	var ended bool
	_ = s.DB.QueryRow("SELECT exhausted FROM history_state WHERE owner=? AND conversation=?", owner, conversation).Scan(&ended)
	return ended
}
func (s *Store) Oldest(owner, conversation string) (model.Message, error) {
	var raw []byte
	err := s.DB.QueryRow("SELECT payload FROM messages WHERE owner=? AND conversation=? ORDER BY created_ns,position,id LIMIT 1", owner, conversation).Scan(&raw)
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

// ClearRedacted drops rows derived from a deleted message: reactions to it and
// its mention marker, so counters and reloads no longer surface the removed text.
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
	return tx.Commit()
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
