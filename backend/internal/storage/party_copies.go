package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"ircord/backend/internal/model"
	"time"
)

func (s *Store) PartyMessageVersion(party, id string) (msg model.Message, version int64, err error) {
	var raw []byte
	err = s.DB.QueryRow("SELECT payload,revision FROM party_history WHERE party=? AND id=?", party, id).Scan(&raw, &version)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(raw, &msg)
	}
	return
}
func (s *Store) MarkPartyCopy(party, owner, message string, revision int64) error {
	_, e := s.DB.Exec("INSERT INTO party_copies(party,owner,message,revision) VALUES(?,?,?,?) ON CONFLICT(party,owner,message) DO UPDATE SET revision=max(revision,excluded.revision)", party, owner, message, revision)
	return e
}

// Retention maintenance is batched rather than scanning the ceiling for every line.
func (s *Store) PrunePartyHistory(party string) error {
	e := s.PrunePartyCanonical(party)
	if e != nil {
		return e
	}
	p, e := s.Party(party)
	if e != nil {
		return e
	}
	for _, member := range p.Members {
		if member.LocalAccountID() == "" {
			continue
		}
		if _, e = s.DB.Exec("DELETE FROM messages WHERE owner=? AND conversation=? AND upstream!='' AND NOT EXISTS(SELECT 1 FROM party_history h WHERE h.party=? AND h.id=messages.id)", member.LocalAccountID(), p.ConversationID, party); e != nil {
			return e
		}
	}
	return nil
}

func (s *Store) PrunePartyCanonical(party string) error {
	_, e := s.DB.Exec("DELETE FROM party_history WHERE party=? AND (created_ns<? OR id IN (SELECT id FROM party_history WHERE party=? ORDER BY created_ns DESC,id DESC LIMIT -1 OFFSET ?))", party, time.Now().Add(-AccountRetention).UnixNano(), party, PartyHistoryLimit)
	return e
}
