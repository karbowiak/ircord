package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"ircord/backend/internal/model"
)

// Minimal participant identity survives account deletion with retained history.
// It grants no login, candidate visibility or current membership.
func (s *Store) SavePartyIdentity(v model.PartyCandidate) error {
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if v.ID != "" {
		_, e = s.DB.Exec("INSERT INTO party_services_identities(id,account,registered_ns,payload) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", v.ID, v.ServicesAccount, v.ServicesRegisteredAt.UnixNano(), raw)
		return e
	}
	_, e = s.DB.Exec("INSERT INTO party_identities(account_id,username,payload) VALUES(?,?,?) ON CONFLICT(account_id) DO UPDATE SET payload=excluded.payload", v.AccountID, v.Username, raw)
	return e
}

// Call only after this local account's own live IRC connection proves the
// services incarnation. A candidate/profile lookup never establishes access.
func (s *Store) BindPartyPrincipal(owner, principal string) error {
	_, e := s.DB.Exec("INSERT INTO party_local_bindings(owner,principal) VALUES(?,?) ON CONFLICT(owner) DO UPDATE SET principal=excluded.principal", owner, principal)
	return e
}
func (s *Store) PartyIdentity(username string) (model.PartyCandidate, error) {
	var raw []byte
	var v model.PartyCandidate
	e := s.DB.QueryRow("SELECT payload FROM party_services_identities WHERE account=? COLLATE NOCASE ORDER BY registered_ns DESC LIMIT 1", username).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		e = s.DB.QueryRow("SELECT payload FROM party_identities WHERE username=? COLLATE NOCASE", username).Scan(&raw)
	}
	if errors.Is(e, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(raw, &v)
	}
	return v, e
}

func (s *Store) PartyPrincipal(owner string) string {
	var principal string
	if s.DB.QueryRow("SELECT principal FROM party_local_bindings WHERE owner=?", owner).Scan(&principal) == nil {
		return principal
	}
	return owner
}

func (s *Store) PartyServiceIdentity(id string) (v model.PartyCandidate, err error) {
	var raw []byte
	err = s.DB.QueryRow("SELECT payload FROM party_services_identities WHERE id=?", id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(raw, &v)
	}
	return
}
func (s *Store) PartyHistoryIdentities(party string) ([]model.PartyCandidate, error) {
	rows, e := s.DB.Query("SELECT i.payload FROM party_identities i WHERE EXISTS(SELECT 1 FROM party_history h WHERE h.party=? AND json_extract(h.payload,'$.senderId')=i.account_id) UNION ALL SELECT i.payload FROM party_services_identities i WHERE EXISTS(SELECT 1 FROM party_history h WHERE h.party=? AND json_extract(h.payload,'$.senderId')=i.id)", party, party)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.PartyCandidate{}
	for rows.Next() {
		var raw []byte
		var v model.PartyCandidate
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) MigratePartyIdentities(p PartyRecord, previous int64, mapping map[string]model.PartyCandidate) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	raw, e := json.Marshal(p)
	if e != nil {
		return e
	}
	r, e := tx.Exec("UPDATE parties SET revision=?,payload=? WHERE id=? AND revision=?", p.Revision, raw, p.ID, previous)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrNotFound
	}
	for old, c := range mapping {
		for _, member := range p.Members {
			if member.PrincipalID() == c.ID && c.LocalAccountID() == old {
				if _, e = tx.Exec("INSERT INTO party_local_bindings(owner,principal) VALUES(?,?) ON CONFLICT(owner) DO UPDATE SET principal=excluded.principal", old, c.ID); e != nil {
					return e
				}
			}
		}
		if _, e = tx.Exec("UPDATE party_invitations SET invitee=? WHERE party=? AND invitee=?", c.ID, p.ID, old); e != nil {
			return e
		}
		if _, e = tx.Exec("INSERT INTO party_participants(party,account_id,username) SELECT party,?,username FROM party_participants WHERE party=? AND account_id=? ON CONFLICT DO NOTHING", c.ID, p.ID, old); e != nil {
			return e
		}
	}
	return tx.Commit()
}
