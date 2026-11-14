package storage

import (
	"fmt"
	"ircord/backend/internal/model"
	"testing"
	"time"
)

func TestReadPositionSurvivesRestartAndIsOwnerScoped(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	account(t, s, "alice")
	account(t, s, "bob")
	at := time.Now().UTC().Truncate(time.Millisecond)
	for _, owner := range []string{"alice", "bob"} {
		for i, kind := range []string{"message", "action", "notice", "event"} {
			msg := model.Message{ID: kind, UpstreamID: "server-" + kind, ConversationID: "room", SenderID: "peer", Kind: kind, CreatedAt: at.Add(time.Duration(i) * time.Second), Mention: true}
			if err = s.SaveHistory(owner, msg); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = s.MarkRead("alice", "room", at.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = s.MarkRead("alice", "room", at); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.ReadTime("alice", "room"); err != nil || !got.Equal(at.Add(time.Second)) {
		t.Fatal("read position regressed or was lost", got, err)
	}
	if got, err := s.ReadTime("bob", "room"); err != nil || !got.IsZero() {
		t.Fatal("read position crossed accounts", got, err)
	}
	for _, owner := range []string{"alice", "bob"} {
		want := map[string]int{"alice": 1, "bob": 3}[owner]
		if n, err := s.Unread(owner, "room"); err != nil || n != want {
			t.Fatal("incorrect unread after restart", owner, n, err)
		}
		if n, err := s.Mentions(owner, "room"); err != nil || n != want {
			t.Fatal("incorrect mentions after restart", owner, n, err)
		}
	}
	if got, err := s.LatestMessageTime("alice", "room", time.Time{}, true); err != nil || !got.Equal(at.Add(2*time.Second)) {
		t.Fatal("events advanced the read position", got, err)
	}
	if got, err := s.LatestMessageTime("alice", "room", at, true); err != nil || !got.Equal(at) {
		t.Fatal("partial read position ignored", got, err)
	}
}

func TestManualUnreadBoundarySurvivesRestartAndClears(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	account(t, s, "alice")
	at := time.Now().UTC().Truncate(time.Millisecond)
	for i, sender := range []string{"peer", "alice", "peer", "alice"} {
		msg := model.Message{ID: fmt.Sprint("m", i), ConversationID: "room", SenderID: sender, Kind: "message", CreatedAt: at.Add(time.Duration(i) * time.Second), Mention: sender == "peer"}
		if err = s.SaveHistory("alice", msg); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.MarkRead("alice", "room", at.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if found, err := s.MarkUnread("alice", "room", "missing"); err != nil || found {
		t.Fatal("unknown boundary accepted", found, err)
	}
	// Your own message starts the boundary; later own messages stay read.
	if found, err := s.MarkUnread("alice", "room", "m1"); err != nil || !found {
		t.Fatal("boundary not saved", found, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if s, err = Open(dir); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Unread("alice", "room"); err != nil || n != 2 {
		t.Fatal("manual unread count lost", n, err)
	}
	if n, err := s.Mentions("alice", "room"); err != nil || n != 1 {
		t.Fatal("manual mention count lost", n, err)
	}
	if id, err := s.UnreadFrom("alice", "room"); err != nil || id != "m1" {
		t.Fatal("boundary lost", id, err)
	}
	if got, err := s.ReadTime("alice", "room"); err != nil || !got.Equal(at.Add(3*time.Second)) {
		t.Fatal("marking unread moved the monotonic read position", got, err)
	}
	if err = s.ClearUnread("alice", "room"); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.Unread("alice", "room"); n != 0 {
		t.Fatal("cleared boundary still unread", n)
	}
	if id, _ := s.UnreadFrom("alice", "room"); id != "" {
		t.Fatal("boundary not cleared", id)
	}
}

func TestTupleReadBoundariesSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	account(t, s, "alice")
	account(t, s, "bob")
	at := time.Now().UTC().Truncate(time.Millisecond)
	for _, id := range []string{"a", "b", "c"} {
		if err = s.SaveHistory("alice", model.Message{ID: id, ConversationID: "room", SenderID: "peer", Kind: "message", CreatedAt: at, Mention: true}); err != nil {
			t.Fatal(err)
		}
	}
	if found, applied, err := s.MarkReadThrough("alice", "room", "a"); err != nil || !found || !applied {
		t.Fatal(found, applied, err)
	}
	if found, _, err := s.MarkReadThrough("bob", "room", "a"); err != nil || found {
		t.Fatal("cross-account boundary", found, err)
	}
	if found, applied, err := s.MarkReadThrough("alice", "room", "a"); err != nil || !found || applied {
		t.Fatal("retry", found, applied, err)
	}
	if n, err := s.Unread("alice", "room"); err != nil || n != 2 {
		t.Fatal("equal time unread", n, err)
	}
	if found, err := s.MarkUnread("alice", "room", "b"); err != nil || !found {
		t.Fatal(found, err)
	}
	if found, applied, err := s.MarkReadThrough("alice", "room", "c"); err != nil || !found || applied {
		t.Fatal("manual override", found, applied, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, err := s.Unread("alice", "room"); err != nil || n != 2 {
		t.Fatal("manual tuple lost after restart", n, err)
	}
	if err := s.ClearUnread("alice", "room"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Mentions("alice", "room"); err != nil || n != 2 {
		t.Fatal("read tuple lost after restart", n, err)
	}
	if found, applied, err := s.MarkReadThrough("alice", "room", "b"); err != nil || !found || !applied {
		t.Fatal(found, applied, err)
	}
	if found, applied, err := s.MarkReadThrough("alice", "room", "a"); err != nil || !found || applied {
		t.Fatal("regression", found, applied, err)
	}
	if n, err := s.Unread("alice", "room"); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}

func TestTimestampLegacyAndLateHistory(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	at := time.Now().UTC().Truncate(time.Millisecond)
	for _, item := range []struct {
		id       string
		position int64
	}{{"a", 1}, {"b", 2}} {
		if err := s.SaveHistory("alice", model.Message{ID: item.id, Position: item.position, ConversationID: "room", SenderID: "peer", Kind: "message", CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.MarkRead("alice", "room", at); err != nil {
		t.Fatal(err)
	}
	if _, applied, err := s.MarkReadThrough("alice", "room", "a"); err != nil || applied {
		t.Fatal("legacy timestamp must include equal ties", applied, err)
	}
	if n, err := s.Unread("alice", "room"); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	// A later exact boundary supersedes the legacy timestamp.
	if err := s.SaveHistory("alice", model.Message{ID: "c", Position: 3, ConversationID: "room", SenderID: "peer", Kind: "message", CreatedAt: at.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	if _, applied, err := s.MarkReadThrough("alice", "room", "c"); err != nil || !applied {
		t.Fatal(applied, err)
	}
	for _, item := range []struct {
		id       string
		position int64
	}{{"backfill", -1}, {"late", 4}} {
		if err := s.SaveHistory("alice", model.Message{ID: item.id, Position: item.position, ConversationID: "room", SenderID: "peer", Kind: "message", CreatedAt: at.Add(time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := s.Unread("alice", "room"); err != nil || n != 1 {
		t.Fatal("late history ordering", n, err)
	}
}
