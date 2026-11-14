package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

// The prior query is retained here only to compare the plan and result on a
// disposable fixture. It must not be used for the production read path.
var previousSafeReadMarkerQuery = `SELECT m.created_ns FROM messages m JOIN conversation_read r ON r.owner=m.owner AND r.conversation=m.conversation
	WHERE m.owner=? AND m.conversation=? AND m.kind IN ('message','action','notice') AND m.upstream!='' AND substr(m.upstream,1,1)!='_'
	AND (m.created_ns<r.at_ns OR (m.created_ns=r.at_ns AND (r.at_position IS NULL OR (m.position,m.id)<=(r.at_position,r.at_id))))
	AND NOT EXISTS (SELECT 1 FROM messages u LEFT JOIN conversation_read ur ON ur.owner=u.owner AND ur.conversation=u.conversation
		WHERE u.owner=m.owner AND u.conversation=m.conversation AND u.kind IN ('message','action','notice') AND u.sender NOT IN (SELECT member FROM ignored_senders WHERE owner=u.owner)
		AND u.created_ns/1000000<=m.created_ns/1000000 AND ` + strings.NewReplacer("m.", "u.", "r.", "ur.").Replace(unreadScope) + `)
	ORDER BY m.created_ns DESC,m.position DESC,m.id DESC LIMIT 1`

func markerPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(plan, " | ")
}

func TestSafeReadMarkerMatchesPreviousPolicies(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, msg := range []model.Message{
		{ID: "old", SenderID: "peer", CreatedAt: at.Add(-time.Second), Position: 1},
		{ID: "a", SenderID: "peer", CreatedAt: at, Position: 2},
		{ID: "b", SenderID: "ignored", CreatedAt: at, Position: 3},
		{ID: "c", SenderID: "peer", CreatedAt: at.Add(500 * time.Microsecond), Position: 4},
		{ID: "own", SenderID: "alice", CreatedAt: at.Add(time.Millisecond), Position: 5},
	} {
		msg.ConversationID, msg.Kind, msg.UpstreamID = "room", "message", "up-"+msg.ID
		if err := s.SaveHistory("alice", msg); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec("INSERT INTO ignored_senders(owner,member) VALUES('alice','ignored')"); err != nil {
		t.Fatal(err)
	}
	compare := func(label string, want time.Time) {
		t.Helper()
		var oldNS int64
		oldErr := s.DB.QueryRow(previousSafeReadMarkerQuery, "alice", "room").Scan(&oldNS)
		got, err := s.SafeReadMarker("alice", "room")
		if errors.Is(oldErr, sql.ErrNoRows) {
			if err != nil || !got.IsZero() {
				t.Fatalf("%s: missing old marker, new=%v err=%v", label, got, err)
			}
			return
		}
		if oldErr != nil || err != nil || oldNS != got.UnixNano() || !got.Equal(want) {
			t.Fatalf("%s: old=%d %v new=%v %v want=%v", label, oldNS, oldErr, got, err, want)
		}
	}
	compare("without boundary", time.Time{})
	if _, _, err := s.MarkReadThrough("alice", "room", "a"); err != nil {
		t.Fatal(err)
	}
	compare("unread sub-millisecond tie", at.Add(-time.Second))
	if _, err := s.DB.Exec("INSERT INTO ignored_senders(owner,member) VALUES('alice','peer')"); err != nil {
		t.Fatal(err)
	}
	compare("ignored messages", at)
	if _, err := s.DB.Exec("DELETE FROM ignored_senders WHERE owner='alice' AND member='peer'"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkAllRead("alice", "room"); err != nil {
		t.Fatal(err)
	}
	compare("all read", at.Add(time.Millisecond))
	if found, err := s.MarkUnread("alice", "room", "own"); err != nil || !found {
		t.Fatal(found, err)
	}
	compare("manual own boundary", at.Add(500*time.Microsecond))
	if found, err := s.MarkUnread("alice", "room", "c"); err != nil || !found {
		t.Fatal(found, err)
	}
	compare("manual tied boundary", at.Add(-time.Second))
	if err := s.ClearUnread("alice", "room"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkRead("alice", "room", at.Add(2*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	compare("legacy timestamp boundary", at.Add(time.Millisecond))
}

func TestSafeReadMarkerPlanAndDisposableHistory(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	const size = 3000
	for i := range size {
		id := fmt.Sprintf("m%05d", i)
		sender := "peer"
		if i < size/2 {
			sender = "ignored"
		}
		_, err = tx.Exec("INSERT INTO messages(owner,conversation,id,upstream,created_ns,text,sender,kind,payload,position) VALUES(?,?,?,?,?,?,?,?,?,?)", "alice", "room", id, "up-"+id, base.Add(time.Duration(i)*time.Millisecond).UnixNano(), "", sender, "message", "{}", i+1)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("INSERT INTO ignored_senders(owner,member) VALUES(?,?)", "alice", "ignored"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkAllRead("alice", "room"); err != nil {
		t.Fatal(err)
	}
	if found, err := s.MarkUnread("alice", "room", "m00001"); err != nil || !found {
		t.Fatal(found, err)
	}
	oldPlan := markerPlan(t, s.DB, previousSafeReadMarkerQuery, "alice", "room")
	newPlan := markerPlan(t, s.DB, safeReadMarkerQuery, "alice", "room", "alice", "room")
	if !strings.Contains(oldPlan, "CORRELATED SCALAR SUBQUERY") {
		t.Fatalf("expected prior correlated scan: %s", oldPlan)
	}
	if strings.Contains(newPlan, "CORRELATED SCALAR SUBQUERY") || !strings.Contains(newPlan, "MATERIALIZE earliest") {
		t.Fatalf("unexpected new plan: %s", newPlan)
	}
	start := time.Now()
	var oldNS int64
	if err := s.DB.QueryRow(previousSafeReadMarkerQuery, "alice", "room").Scan(&oldNS); err != nil {
		t.Fatal(err)
	}
	oldDuration := time.Since(start)
	start = time.Now()
	got, err := s.SafeReadMarker("alice", "room")
	newDuration := time.Since(start)
	if err != nil || got.UnixNano() != oldNS || !got.Equal(base.Add((size/2-1)*time.Millisecond)) {
		t.Fatalf("markers differ: old=%d new=%v err=%v", oldNS, got, err)
	}
	t.Logf("fixture=%d previous=%s new=%s", size, oldDuration, newDuration)
	t.Logf("previous plan: %s", oldPlan)
	t.Logf("new plan: %s", newPlan)
}
