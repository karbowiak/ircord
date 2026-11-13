package storage

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

func fillConversation(t *testing.T, s *Store, owner, conversation string, count int) {
	t.Helper()
	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	at := time.Now().UTC()
	for i := range count {
		msg := model.Message{ID: fmt.Sprintf("%s-%d", conversation, i), UpstreamID: fmt.Sprintf("up-%s-%d", conversation, i), ConversationID: conversation, Text: "hello", Kind: "message", CreatedAt: at.Add(time.Duration(i) * time.Millisecond), Reactions: []model.Reaction{}}
		if err = putMessage(tx, owner, msg); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func queryPlan(t *testing.T, s *Store, query string, args ...any) string {
	t.Helper()
	rows, err := s.DB.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(plan, "; ")
}

// The database keeps no planner statistics, so the per-message lookups must
// pick an index that finds one row instead of walking the conversation's history.
func TestMessageLookupsStayIndexedWithoutStatistics(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	fillConversation(t, s, "alice", "room", 500)
	// These are the query lists MessageContext and PartyMessage run, in order.
	if len(messageLookupQueries) != 2 || len(partyMessageLookupQueries) != 2 {
		t.Fatalf("lookup query lists changed; update the expected plans")
	}
	for _, check := range []struct {
		name, query, want string
		args              []any
	}{
		{"message by ID", messageLookupQueries[0], "USING INDEX sqlite_autoindex_messages_1", []any{"alice", "room", "room-1"}},
		{"message by upstream ID", messageLookupQueries[1], "USING INDEX messages_upstream", []any{"alice", "room", "up-room-1"}},
		{"next position", nextPositionQuery, "messages_next_position", []any{"alice", "room"}},
		{"party message by ID", partyMessageLookupQueries[0], "USING INDEX sqlite_autoindex_party_history_1", []any{"party", "message"}},
		{"party message by upstream ID", partyMessageLookupQueries[1], "USING INDEX party_history_upstream", []any{"party", "message"}},
	} {
		plan := queryPlan(t, s, check.query, check.args...)
		if !strings.Contains(plan, check.want) || strings.Contains(plan, "SCAN") {
			t.Errorf("lookup %s plan = %q, want %q", check.name, plan, check.want)
		}
	}
	for _, id := range []string{"room-250", "up-room-250"} {
		msg, err := s.Message("alice", "room", id)
		if err != nil || msg.ID != "room-250" {
			t.Fatalf("Message(%q) = %q, %v; want room-250", id, msg.ID, err)
		}
	}
	if _, err = s.Message("alice", "other", "room-250"); err != ErrNotFound {
		t.Fatalf("lookup in another conversation = %v, want ErrNotFound", err)
	}
	next := model.Message{ID: "next", ConversationID: "room"}
	if err = positionMessage(s.DB, "alice", &next); err != nil || next.Position != 501 {
		t.Fatalf("next position = %d, %v; want 501", next.Position, err)
	}
}

func TestPartyMessageLookupByIDAndUpstreamID(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	candidate, err := s.PartyCandidate("alice")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	p := PartyRecord{Party: model.Party{ID: "party", ConversationID: "conversation", NetworkID: "home", OwnerID: "alice", State: "active", Revision: 1, Members: []model.PartyMember{{PartyCandidate: candidate, JoinedAt: now}}, CreatedAt: now, UpdatedAt: now}, Channel: "#private-fixture", Creator: "alice", ClientID: "create"}
	seedLegacyParty(t, s, p)
	if err = s.SavePartyMessage(p.ID, model.Message{ID: "local", UpstreamID: "upstream", ConversationID: p.ConversationID, SenderID: "alice", Text: "hello", CreatedAt: now, Kind: "message"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"local", "upstream"} {
		msg, err := s.PartyMessage(p.ID, id)
		if err != nil || msg.ID != "local" {
			t.Fatalf("PartyMessage(%q) = %q, %v; want local", id, msg.ID, err)
		}
	}
	if _, err = s.PartyMessage(p.ID, "missing"); err != ErrNotFound {
		t.Fatalf("missing party message = %v, want ErrNotFound", err)
	}
}
