package storage

import (
	"ircord/backend/internal/model"
	"time"
)

func (s *Store) SetPartyReaction(party, parent, sender, emoji string, active bool, at time.Time, live bool) error {
	_, e := s.DB.Exec("INSERT INTO party_reactions(party,parent,sender,emoji,active,at_ns) VALUES(?,?,?,?,?,?) ON CONFLICT(party,parent,sender,emoji) DO UPDATE SET active=excluded.active,at_ns=excluded.at_ns WHERE excluded.at_ns>at_ns OR (? AND excluded.at_ns=at_ns) OR (excluded.at_ns=at_ns AND excluded.active=0)", party, parent, sender, emoji, active, at.UnixNano(), live)
	return e
}
func (s *Store) PartyReactions(party, parent string) ([]model.Reaction, error) {
	rows, e := s.DB.Query("SELECT emoji,sender FROM party_reactions WHERE party=? AND parent=? AND active=1 ORDER BY emoji,sender", party, parent)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.Reaction{}
	for rows.Next() {
		var emoji, sender string
		if e = rows.Scan(&emoji, &sender); e != nil {
			return nil, e
		}
		if len(out) == 0 || out[len(out)-1].Emoji != emoji {
			out = append(out, model.Reaction{Emoji: emoji, MemberIDs: []string{}})
		}
		out[len(out)-1].MemberIDs = append(out[len(out)-1].MemberIDs, sender)
	}
	return out, rows.Err()
}
