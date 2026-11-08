package storage

import (
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

// Edit lookups run while a workspace is locked, for REDACTs of unknown msgids
// and for every stored message; they must use the partial indexes rather than
// scanning a conversation's history.
func TestEditLookupsUseTheirIndexes(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for query, index := range map[string]string{
		"SELECT m.payload FROM messages m INDEXED BY messages_edited, json_each(m.payload,'$.edits') e WHERE m.owner=? AND m.conversation=? AND json_extract(m.payload,'$.editedAt') IS NOT NULL AND json_extract(e.value,'$.upstreamId')=? LIMIT 1": "messages_edited",
		"SELECT payload FROM messages INDEXED BY messages_edit_of WHERE owner=? AND conversation=? AND json_extract(payload,'$.editOf') IS NOT NULL AND json_extract(payload,'$.editOf')=? ORDER BY created_ns":                                      "messages_edit_of",
	} {
		rows, err := s.DB.Query("EXPLAIN QUERY PLAN "+query, "o", "c", "x")
		if err != nil {
			t.Fatal(err)
		}
		plan := ""
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan += detail + "\n"
		}
		rows.Close()
		if !strings.Contains(plan, index) {
			t.Fatalf("%s does not use %s:\n%s", query, index, plan)
		}
	}
}

func TestPendingEditsAndEditTargets(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	at := time.Unix(100, 0).UTC()
	edited := model.Message{ID: "n:m1", UpstreamID: "m1", ConversationID: "room", SenderID: "peer", Kind: "message", Text: "new", CreatedAt: at, Reactions: []model.Reaction{}, EditedAt: &at, OriginalText: "old", Edits: []model.MessageEdit{{UpstreamID: "e1", Text: "new", At: at}}}
	pending := model.Message{ID: "n:e9", UpstreamID: "e9", ConversationID: "room", SenderID: "peer", Kind: "message", Text: "early", CreatedAt: at, Reactions: []model.Reaction{}, EditOf: "m2"}
	for _, msg := range []model.Message{edited, pending} {
		if _, err = s.Commit("alice", "message.created", msg); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := s.EditTarget("alice", "room", "e1"); err != nil || got.ID != "n:m1" {
		t.Fatalf("edit target: %+v %v", got, err)
	}
	if _, err := s.EditTarget("alice", "room", "nope"); err == nil {
		t.Fatal("an unknown edit found a target")
	}
	got, err := s.PendingEdits("alice", "room", "m2")
	if err != nil || len(got) != 1 || got[0].ID != "n:e9" {
		t.Fatalf("pending: %+v %v", got, err)
	}
	if err = s.RemoveMessage("alice", "room", got[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Message("alice", "room", "n:e9"); err == nil {
		t.Fatal("removed edit line still stored")
	}
}
