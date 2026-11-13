// Package testsupport provides fixtures for persisted legacy party records.
package testsupport

import (
	"encoding/json"
	"testing"

	"ircord/backend/internal/storage"
)

// SeedLegacyParty inserts a record that predates retirement of party creation.
func SeedLegacyParty(t testing.TB, s *storage.Store, p storage.PartyRecord, invitations ...storage.PartyInviteRecord) {
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
	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO parties(id,channel,conversation,creator,client_id,fingerprint,revision,payload) VALUES(?,?,?,?,?,?,?,?)", p.ID, p.Channel, p.ConversationID, p.Creator, p.ClientID, p.Fingerprint, p.Revision, raw); err != nil {
		t.Fatal(err)
	}
	for _, member := range p.Members {
		if _, err = tx.Exec("INSERT INTO party_participants(party,account_id,username) VALUES(?,?,?) ON CONFLICT DO NOTHING", p.ID, member.PrincipalID(), member.Username); err != nil {
			t.Fatal(err)
		}
	}
	for _, invitation := range invitations {
		raw, err = json.Marshal(invitation.Invitation)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec("INSERT INTO party_invitations(id,party,invitee,state,payload) VALUES(?,?,?,?,?)", invitation.Invitation.ID, p.ID, invitation.Invitee, invitation.State, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
