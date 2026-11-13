package storage

import (
	"encoding/json"
	"testing"
)

func seedLegacyParty(t *testing.T, s *Store, p PartyRecord) {
	t.Helper()
	for _, member := range p.Members {
		if err := s.SavePartyIdentity(member.PartyCandidate); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("INSERT INTO parties(id,channel,conversation,creator,client_id,fingerprint,revision,payload) VALUES(?,?,?,?,?,?,?,?)", p.ID, p.Channel, p.ConversationID, p.Creator, p.ClientID, p.Fingerprint, p.Revision, raw); err != nil {
		t.Fatal(err)
	}
	for _, member := range p.Members {
		if _, err := s.DB.Exec("INSERT INTO party_participants(party,account_id,username) VALUES(?,?,?) ON CONFLICT DO NOTHING", p.ID, member.PrincipalID(), member.Username); err != nil {
			t.Fatal(err)
		}
	}
}
