package storage

import (
	"database/sql"
	"errors"
	"ircord/backend/internal/model"
)

// Attachment commands require upload ownership.
// File URLs remain ordinary bearer URLs and may be shared in any conversation.
func (s *Store) scopePartyUploads(tx *sql.Tx, owner string, input model.SendInput) error {
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
