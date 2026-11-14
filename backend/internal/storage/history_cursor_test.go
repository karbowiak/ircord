package storage

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

func TestHistoryCursorScopedOrderingAndLegacy(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	account(t, s, "bob")
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, owner := range []string{"alice", "bob"} {
		for _, msg := range []model.Message{
			{ID: "a", Position: 1}, {ID: "c", Position: 1}, {ID: "b", Position: 2}, {ID: "d", Position: 3},
		} {
			msg.ConversationID, msg.CreatedAt, msg.SenderID, msg.Kind = "room", at, "peer", "message"
			if err := s.SaveHistory(owner, msg); err != nil {
				t.Fatal(err)
			}
		}
	}
	first, err := s.History("alice", "room", "", 2)
	if err != nil || len(first.Items) != 2 || first.Items[0].ID != "b" || first.Items[1].ID != "d" || first.NextCursor == nil {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	next, err := ParseHistoryCursor(*first.NextCursor, "alice", "room")
	if err != nil || next.ID != "b" || next.Position != 2 {
		t.Fatalf("cursor=%+v err=%v", next, err)
	}
	second, err := s.History("alice", "room", *first.NextCursor, 2)
	if err != nil || len(second.Items) != 2 || second.Items[0].ID != "a" || second.Items[1].ID != "c" || second.NextCursor != nil {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	legacy := Cursor(first.Items[0])
	older, err := s.History("alice", "room", legacy, 2)
	if err != nil || len(older.Items) != 2 || older.Items[0].ID != "a" || older.Items[1].ID != "c" {
		t.Fatalf("legacy=%+v err=%v", older, err)
	}
	if _, err := s.History("bob", "room", *first.NextCursor, 2); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("other owner: %v", err)
	}
	if _, err := s.History("alice", "other", *first.NextCursor, 2); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("other conversation: %v", err)
	}
	// Old servers ignore new fields and retain the ordinary tuple for typical tokens.
	raw, err := base64.RawURLEncoding.DecodeString(*first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	var old struct {
		At       int64  `json:"at"`
		Position int64  `json:"p"`
		ID       string `json:"id"`
	}
	if err = json.Unmarshal(raw, &old); err != nil || len(*first.NextCursor) > 1024 || old.At != at.UnixNano() || old.Position != 2 || old.ID != "b" {
		t.Fatalf("old parser: %+v %v", old, err)
	}
	around, _, err := s.HistoryAround("alice", "room", "b", 2)
	if err != nil || around.NextCursor == nil {
		t.Fatalf("around=%+v err=%v", around, err)
	}
	if _, err = ParseHistoryCursor(*around.NextCursor, "alice", "room"); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryCursorValidationAndBounds(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	msg := model.Message{ID: strings.Repeat("\x01", 768), CreatedAt: at, Position: -20}
	upstream := &HistoryUpstream{ID: strings.Repeat("\x01", 100), At: at}
	token, err := ScopedHistoryCursor("alice", "room", msg, upstream)
	if err != nil || len(token) > MaxHistoryCursorLength {
		t.Fatalf("worst-case token length=%d err=%v", len(token), err)
	}
	parsed, err := ParseHistoryCursor(token, "alice", "room")
	if err != nil || parsed.ID != msg.ID || parsed.UpstreamBefore == nil || parsed.UpstreamBefore.ID != upstream.ID {
		t.Fatalf("roundtrip=%+v err=%v", parsed, err)
	}
	encode := func(raw string) string { return base64.RawURLEncoding.EncodeToString([]byte(raw)) }
	for _, bad := range []string{
		"!", "123", strings.Repeat("A", MaxHistoryCursorLength+1),
		encode(`{"at":1,"id":"x","v":1}`), encode(`{"at":1,"id":"x","v":null}`),
		encode(`{"at":1,"id":"x","v":3,"k":"history","s":"01234567890123456789012345678901"}`),
		encode(`{"at":1,"id":"x","upstreamBefore":{"ID":"bad\nvalue","At":"2026-01-01T00:00:00Z"}}`),
		SearchCursor(model.Message{ID: "x", CreatedAt: at}),
	} {
		if _, err := ParseHistoryCursor(bad, "alice", "room"); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("bad cursor accepted: %v", err)
		}
	}
	tooLong := model.Message{ID: strings.Repeat("x", maxHistoryMessageID+1), CreatedAt: at}
	if _, err := ScopedHistoryCursor("alice", "room", tooLong, nil); !errors.Is(err, ErrInvalidCursor) {
		t.Fatal("oversized identifier accepted", err)
	}
}
