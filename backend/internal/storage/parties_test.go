package storage

import (
	"ircord/backend/internal/model"
	"testing"
	"time"
)

func TestPartySchemaCompletesEarlyLocalMigrationWithoutLosingHistory(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	account(t, s, "alice")
	candidate, e := s.PartyCandidate("alice")
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	p := PartyRecord{Party: model.Party{ID: "party", ConversationID: "conversation", NetworkID: "home", OwnerID: "alice", State: "active", Revision: 1, Members: []model.PartyMember{{PartyCandidate: candidate, JoinedAt: now}}, CreatedAt: now, UpdatedAt: now}, Channel: "#private-fixture", Creator: "alice", ClientID: "create"}
	seedLegacyParty(t, s, p)
	if e = s.SavePartyMessage(p.ID, model.Message{ID: "message", UpstreamID: "message", ConversationID: p.ConversationID, SenderID: "alice", Text: "retained", CreatedAt: now, Kind: "message"}); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"party_copies", "party_participants", "party_identities", "party_history_sync", "party_reactions"} {
		if _, e = s.DB.Exec("DROP TABLE " + table); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = s.DB.Exec("ALTER TABLE party_history DROP COLUMN revision"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("DROP INDEX party_file_urls"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("DELETE FROM goose_db_version WHERE version_id>=32"); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if restored, e := s.Party(p.ID); e != nil || !restored.HasMember("alice") {
		t.Fatal("party state was not preserved", e)
	}
	if msg, _, e := s.PartyMessageVersion(p.ID, "message"); e != nil || msg.Text != "retained" {
		t.Fatal("canonical history was not preserved", e)
	}
	if identity, e := s.PartyIdentity(candidate.Username); e != nil || identity.AccountID != "alice" {
		t.Fatal("participant identity was not restored", e)
	}
}
