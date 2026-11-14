package irc

import (
	"errors"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/storage"
)

func TestMemoryHistoryScopedCursorAndLegacyID(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, owner := range []string{"alice", "bob"} {
		w := m.space(owner)
		w.mu.Lock()
		w.conversations["room"] = Conversation{ID: "room"}
		w.conversations["other"] = Conversation{ID: "other"}
		w.messages["room"] = []Message{
			{ID: "d", CreatedAt: at, Position: 3},
			{ID: "c", CreatedAt: at, Position: 1},
			{ID: "eyJhdCI6IngifQ", CreatedAt: at, Position: 1},
			{ID: "b", CreatedAt: at, Position: 2},
			{ID: "e", CreatedAt: at, Position: 4},
		}
		w.mu.Unlock()
	}
	page, err := m.History("alice", "room", "", 2)
	if err != nil || len(page.Items) != 2 || page.Items[0].ID != "d" || page.Items[1].ID != "e" || page.NextCursor == nil {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if _, err := m.History("bob", "room", *page.NextCursor, 2); !errors.Is(err, storage.ErrInvalidCursor) {
		t.Fatalf("wrong owner: %v", err)
	}
	if _, err := m.History("alice", "other", *page.NextCursor, 2); !errors.Is(err, storage.ErrInvalidCursor) {
		t.Fatalf("wrong conversation: %v", err)
	}
	for _, bad := range []string{"123", strings.Repeat("x", storage.MaxHistoryCursorLength+1), storage.SearchCursor(page.Items[0])} {
		if _, err := m.History("alice", "room", bad, 2); !errors.Is(err, storage.ErrInvalidCursor) {
			t.Fatalf("memory invalid cursor: %v", err)
		}
	}
	point := &storage.HistoryUpstream{ID: "prior", At: at.Add(-time.Hour)}
	before, err := storage.ScopedHistoryCursor("alice", "room", page.Items[1], point)
	if err != nil {
		t.Fatal(err)
	}
	continued, err := m.History("alice", "room", before, 2)
	if err != nil || continued.NextCursor == nil {
		t.Fatalf("memory continuation: %+v %v", continued, err)
	}
	parsed, err := storage.ParseHistoryCursor(*continued.NextCursor, "alice", "room")
	if err != nil || parsed.UpstreamBefore == nil || parsed.UpstreamBefore.ID != point.ID {
		t.Fatalf("lost memory upstream continuation: %+v %v", parsed, err)
	}
	// A scoped tuple remains usable after the boundary message is evicted.
	w := m.space("alice")
	w.mu.Lock()
	w.messages["room"] = w.messages["room"][1:]
	w.mu.Unlock()
	older, err := m.History("alice", "room", *page.NextCursor, 2)
	if err != nil || len(older.Items) != 2 || older.Items[0].ID != "eyJhdCI6IngifQ" || older.Items[1].ID != "b" {
		t.Fatalf("older=%+v err=%v", older, err)
	}
	// This real legacy ID decodes as malformed JSON for the cursor schema.
	legacy, err := m.History("alice", "room", "eyJhdCI6IngifQ", 2)
	if err != nil || len(legacy.Items) != 1 || legacy.Items[0].ID != "c" {
		t.Fatalf("legacy=%+v err=%v", legacy, err)
	}
	around, err := m.HistoryAround("alice", "room", "eyJhdCI6IngifQ", 2)
	if err != nil || !around.HasNewer || around.NextCursor == nil {
		t.Fatalf("around=%+v err=%v", around, err)
	}
	if _, err := storage.ParseHistoryCursor(*around.NextCursor, "alice", "room"); err != nil {
		t.Fatal(err)
	}
}
