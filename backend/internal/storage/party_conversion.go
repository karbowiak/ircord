package storage

import (
	"encoding/json"
	"strings"
	"time"
)

// CompletePartyConversion changes presentation only after the caller verifies
// the native founder. Retain canonical rows for history, file and identity
// compatibility; ordinary channels never use them for admission or moderation.
func (s *Store) CompletePartyConversion(p PartyRecord) (PartyRecord, error) {
	previous := p.Revision
	p.State, p.ConversionPending = "converted", false
	p.Revision++
	p.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(p)
	if err != nil {
		return p, err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	r, err := tx.Exec("UPDATE parties SET revision=?,payload=? WHERE id=? AND revision=?", p.Revision, raw, p.ID, previous)
	if err != nil {
		return p, err
	}
	if n, err := r.RowsAffected(); err != nil || n != 1 {
		return p, ErrNotFound
	}
	_, err = tx.Exec("UPDATE entities SET payload=json_set(json_remove(payload,'$.party'),'$.name',?,'$.target',?) WHERE kind='conversation' AND id=?", strings.TrimPrefix(p.Channel, "#"), p.Channel, p.ConversationID)
	if err != nil {
		return p, err
	}
	return p, tx.Commit()
}
